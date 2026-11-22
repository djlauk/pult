package main

import (
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"pult/web"
)

type server struct {
	configPath string
	procs      *Manager
}

type tileDTO struct {
	Kind   string `json:"kind"` // "tool" or "link"
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status Status `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
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
	if _, err := LoadConfig(*configPath); err != nil {
		log.Printf("warning: %v", err)
	}

	s := &server{configPath: *configPath, procs: NewManager()}
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tiles", s.handleTiles)
	mux.HandleFunc("POST /api/tools/{name}/open", s.handleOpen)
	mux.Handle("GET /", http.FileServerFS(dist))

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	uiURL := "http://" + addr + "/"
	log.Printf("pult listening on %s (config: %s)", uiURL, *configPath)
	if !*noBrowser {
		if err := openBrowser(uiURL); err != nil {
			log.Printf("open browser: %v", err)
		}
	}
	log.Fatal(http.Serve(ln, mux))
}

// handleTiles re-reads tools.json and returns all tiles grouped.
func (s *server) handleTiles(w http.ResponseWriter, r *http.Request) {
	cfg, err := LoadConfig(s.configPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error(), "groups": []groupDTO{}})
		return
	}

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
	for _, t := range cfg.Tools {
		st, msg := s.procs.State(t.Name)
		add(t.Group, tileDTO{Kind: "tool", Name: t.Name, URL: t.URL, Status: st, Error: msg})
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
	cfg, err := LoadConfig(s.configPath)
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
		writeJSON(w, http.StatusOK, map[string]string{"status": string(StatusError), "error": err.Error()})
		return
	}
	if err := openBrowser(t.URL); err != nil {
		log.Printf("open %s: %v", t.URL, err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(StatusRunning)})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
