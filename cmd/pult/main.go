// Windows EXE icon and version info; see internal/winresgen.
//go:generate go run ../../internal/winresgen

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pult/internal/brand"
	"pult/internal/launcher"
	"pult/web"
)

type server struct {
	configPath string
	uiURL      string
	logPath    string
	procs      *launcher.Manager
}

type tileDTO struct {
	Kind   string          `json:"kind"` // "tool" or "link"
	Name   string          `json:"name"`
	URL    string          `json:"url"`
	Status launcher.Status `json:"status,omitempty"`
	Error  string          `json:"error,omitempty"`
	// PortBusy is the port number when someone else holds this tool's port.
	PortBusy string `json:"port_busy,omitempty"`
	// BusyBy names the pult tool holding the port; empty for a foreign process.
	BusyBy string `json:"busy_by,omitempty"`
	// Adopted is true for a tool started by an earlier pult run.
	Adopted bool `json:"adopted,omitempty"`
}

type groupDTO struct {
	Name  string    `json:"name"`
	Tiles []tileDTO `json:"tiles"`
}

func main() {
	port := flag.Int("port", 7300, "port for the launcher UI")
	configFlag := flag.String("config", "", "path to tools.json (default: pult/tools.json in the user config dir)")
	noBrowser := flag.Bool("no-browser", false, "do not open the launcher UI on startup")
	flag.Parse()

	configPath, statePath, err := resolvePaths(*configFlag)
	if err != nil {
		log.Fatal(err)
	}
	logPath := filepath.Join(filepath.Dir(statePath), "pult.log")
	setupLog(logPath)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	s := &server{
		configPath: configPath,
		uiURL:      "http://" + addr + "/",
		logPath:    logPath,
		procs:      launcher.NewManager(statePath),
	}
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tiles", s.handleTiles)
	mux.HandleFunc("POST /api/tools/{name}/open", s.handleOpen)
	mux.HandleFunc("POST /api/tools/{name}/stop", s.handleStop)
	mux.HandleFunc("GET /api/tools/{name}/logs", s.handleLogs)
	mux.HandleFunc("GET /favicon.ico", handleFavicon)
	mux.Handle("GET /", http.FileServerFS(dist))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Without a console the error would go unseen. If pult already runs
		// there, starting it again just opens its UI.
		if pultRunning(s.uiURL) {
			log.Printf("pult already running on %s; opening it", s.uiURL)
			s.openUI()
			return
		}
		log.Fatal(err)
	}
	// Adopt only after binding, so a second pult instance exits before it
	// touches the state file.
	if cfg, err := launcher.LoadConfig(configPath); err != nil {
		log.Printf("warning: %v", err)
	} else {
		s.procs.Adopt(cfg)
	}
	log.Printf("pult %s listening on %s", brand.Version, s.uiURL)
	log.Printf("config: %s", configPath)
	log.Printf("state:  %s", statePath)
	log.Printf("log:    %s", logPath)
	if !*noBrowser {
		s.openUI()
	}
	go s.handleSignals()
	go func() { log.Fatal(http.Serve(ln, mux)) }()
	s.runTray() // blocks until quit
}

// setupLog writes pult's own log to a file, and to stderr when there is a
// console. The file is truncated on each start.
func setupLog(path string) {
	f, err := os.Create(path)
	if err != nil {
		log.Printf("log file: %v", err)
		return
	}
	// The file comes first: without a console, writes to stderr fail.
	log.SetOutput(io.MultiWriter(f, os.Stderr))
}

// pultRunning reports whether a pult instance answers at uiURL.
func pultRunning(uiURL string) bool {
	c := http.Client{Timeout: time.Second}
	res, err := c.Get(uiURL + "api/tiles")
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

// resolvePaths returns where tools.json and the state file live. Without
// -config, tools.json is in the user config dir (%AppData%\pult,
// ~/.config/pult) and the state in the user cache dir (%LocalAppData%\pult,
// ~/.cache/pult), since pids only make sense on this machine. With -config,
// the state file sits next to that config so separate setups don't mix.
func resolvePaths(configFlag string) (configPath, statePath string, err error) {
	if configFlag != "" {
		configPath, err = filepath.Abs(configFlag)
		if err != nil {
			return "", "", err
		}
		return configPath, filepath.Join(filepath.Dir(configPath), "pult-state.json"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", "", err
	}
	stateDir := filepath.Join(cacheDir, "pult")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return "", "", err
	}
	return filepath.Join(configDir, "pult", "tools.json"), filepath.Join(stateDir, "pult-state.json"), nil
}

// handleSignals implements quitting from the console. Ctrl+C asks whether
// running tools should be stopped; a second Ctrl+C quits immediately and
// leaves them running. SIGTERM (or closing the console window) cannot ask,
// so it quits and leaves tools running.
func (s *server) handleSignals() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	for sig := range sigs {
		running := s.procs.Running()
		if len(running) == 0 || sig != os.Interrupt {
			s.exit()
			return
		}
		fmt.Printf("\nStop %d running tool(s) too? (%s)\n[Y]es, [n]o (leave running), [c]ancel: ",
			len(running), strings.Join(running, ", "))
		answer := make(chan string, 1)
		go func() {
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			answer <- strings.ToLower(strings.TrimSpace(line))
		}()
		select {
		case <-sigs:
			fmt.Println("\nQuitting, tools left running.")
			s.exit()
			return
		case a := <-answer:
			switch a {
			case "c":
				fmt.Println("Cancelled.")
				continue
			case "n":
				fmt.Println("Quitting, tools left running.")
			default:
				fmt.Println("Stopping tools...")
				s.procs.StopAll()
			}
			s.exit()
			return
		}
	}
}

// handleTiles re-reads tools.json and returns all tiles grouped.
func (s *server) handleTiles(w http.ResponseWriter, r *http.Request) {
	cfg, err := launcher.LoadConfig(s.configPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error(), "groups": []groupDTO{}})
		return
	}
	s.procs.TryAdopt(cfg) // tools.json may now match a remembered process

	var groups []groupDTO
	index := map[string]int{}
	add := func(group string, t tileDTO) {
		if group == "" {
			group = "Other"
		}
		i, ok := index[group]
		if !ok {
			i = len(groups)
			index[group] = i
			groups = append(groups, groupDTO{Name: group})
		}
		groups[i].Tiles = append(groups[i].Tiles, t)
	}
	busy := launcher.ProbePorts(cfg.Tools)
	for _, t := range cfg.Tools {
		st, msg := s.procs.State(t.Name)
		tile := tileDTO{Kind: "tool", Name: t.Name, URL: t.URL, Status: st, Error: msg, Adopted: s.procs.Adopted(t.Name)}
		if st == launcher.StatusStopped || st == launcher.StatusError {
			port, _ := launcher.ToolPort(t.URL)
			if owner := s.procs.PortOwner(port, t.Name); owner != "" {
				tile.PortBusy, tile.BusyBy = port, owner
			} else if busy[t.URL] {
				tile.PortBusy = port
			}
		}
		add(t.Group, tile)
	}
	for _, l := range cfg.Links {
		add(l.Group, tileDTO{Kind: "link", Name: l.Name, URL: l.URL})
	}
	if groups == nil {
		groups = []groupDTO{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

// handleOpen starts a tool if needed, waits for its port, then opens its URL.
func (s *server) handleOpen(w http.ResponseWriter, r *http.Request) {
	cfg, err := launcher.LoadConfig(s.configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	t, ok := cfg.Tool(r.PathValue("name"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown tool"})
		return
	}
	if err := s.procs.EnsureRunning(cfg, t); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": string(launcher.StatusError), "error": err.Error()})
		return
	}
	if err := launcher.OpenDefault(t.URL); err != nil {
		log.Printf("open %s: %v", t.URL, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(launcher.StatusRunning)})
}

// handleStop terminates a tool's process tree.
func (s *server) handleStop(w http.ResponseWriter, r *http.Request) {
	if err := s.procs.Stop(r.PathValue("name")); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(launcher.StatusStopped)})
}

// handleLogs returns the captured output of a tool's last run.
func (s *server) handleLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"lines": s.procs.Logs(r.PathValue("name"))})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
