package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
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

	"pult/internal/launcher"
	"pult/web"
)

type server struct {
	configPath string
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
	configPath := flag.String("config", "", "path to tools.json (default: next to the binary)")
	noBrowser := flag.Bool("no-browser", false, "do not open the launcher UI on startup")
	flag.Parse()

	if *configPath == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		*configPath = filepath.Join(filepath.Dir(exe), "tools.json")
	}
	statePath := filepath.Join(filepath.Dir(*configPath), "pult-state.json")
	s := &server{configPath: *configPath, procs: launcher.NewManager(statePath)}
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tiles", s.handleTiles)
	mux.HandleFunc("POST /api/tools/{name}/open", s.handleOpen)
	mux.HandleFunc("POST /api/tools/{name}/stop", s.handleStop)
	mux.HandleFunc("GET /api/tools/{name}/logs", s.handleLogs)
	mux.HandleFunc("POST /api/quit", s.handleQuit)
	mux.Handle("GET /", http.FileServerFS(dist))

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	// Adopt only after binding, so a second pult instance exits before it
	// touches the state file.
	if cfg, err := launcher.LoadConfig(*configPath); err != nil {
		log.Printf("warning: %v", err)
	} else {
		s.procs.Adopt(cfg)
	}
	uiURL := "http://" + addr + "/"
	log.Printf("pult listening on %s (config: %s)", uiURL, *configPath)
	log.Printf("press Ctrl+C to quit")
	if !*noBrowser {
		if err := launcher.OpenBrowser(uiURL); err != nil {
			log.Printf("open browser: %v", err)
		}
	}
	go s.handleSignals()
	log.Fatal(http.Serve(ln, mux))
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
			os.Exit(0)
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
			os.Exit(0)
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
			os.Exit(0)
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
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups, "running": len(s.procs.Running())})
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
	if err := launcher.OpenBrowser(t.URL); err != nil {
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

// handleQuit shuts the launcher down, optionally stopping running tools.
func (s *server) handleQuit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StopTools bool `json:"stop_tools"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.StopTools {
		s.procs.StopAll()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	log.Printf("quit from UI (stop tools: %v)", req.StopTools)
	go func() {
		time.Sleep(200 * time.Millisecond) // let the response reach the browser
		os.Exit(0)
	}()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
