package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"time"
)

// stateEntry records a running tool so a later pult run can adopt it.
type stateEntry struct {
	Name      string `json:"name"`
	PID       int    `json:"pid"`
	StartTime int64  `json:"start_time"` // OS process start time, guards against PID reuse
	Port      string `json:"port"`
}

// saveState writes all tracked processes to the state file. Caller holds m.mu.
func (m *Manager) saveState() {
	if m.statePath == "" {
		return
	}
	entries := []stateEntry{}
	for name, p := range m.procs {
		if p.active() && p.pid != 0 {
			entries = append(entries, stateEntry{Name: name, PID: p.pid, StartTime: p.startTime, Port: p.port})
		}
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	tmp := m.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("save state: %v", err)
		return
	}
	if err := os.Rename(tmp, m.statePath); err != nil {
		log.Printf("save state: %v", err)
	}
}

// Adopt reads the state file and takes over tools left running by an earlier
// pult run, if they still match tools.json and the recorded process.
// Entries that do not match are dropped; their processes are left alone.
func (m *Manager) Adopt(cfg *Config) {
	data, err := os.ReadFile(m.statePath)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	var entries []stateEntry
	if err == nil {
		err = json.Unmarshal(data, &entries)
	}
	if err != nil {
		log.Printf("ignoring state file %s: %v", m.statePath, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		if reason := checkEntry(cfg, e); reason != "" {
			log.Printf("not adopting %q (pid %d): %s", e.Name, e.PID, reason)
			continue
		}
		t, _ := cfg.Tool(e.Name)
		addr, _ := toolAddr(t.URL)
		p := &proc{
			pid:       e.PID,
			startTime: e.StartTime,
			port:      e.Port,
			adopted:   true,
			status:    StatusStarting,
			ready:     make(chan struct{}),
			exited:    make(chan struct{}),
		}
		m.procs[e.Name] = p
		go m.watchAdopted(p)
		go m.waitReady(p, addr)
		log.Printf("adopted %q (pid %d) from an earlier run", e.Name, e.PID)
	}
	m.saveState()
}

// checkEntry returns why a state entry cannot be adopted, or "".
func checkEntry(cfg *Config, e stateEntry) string {
	t, ok := cfg.Tool(e.Name)
	if !ok {
		return "no longer in tools.json"
	}
	if port, _ := toolPort(t.URL); port != e.Port {
		return "port changed in tools.json"
	}
	st, alive := processStartTime(e.PID)
	if !alive {
		return "process is gone"
	}
	if st != e.StartTime {
		return "pid now belongs to a different process"
	}
	return ""
}

// watchAdopted polls an adopted process, which pult cannot Wait on, and
// records its exit.
func (m *Manager) watchAdopted(p *proc) {
	for {
		time.Sleep(time.Second)
		if st, alive := processStartTime(p.pid); alive && st == p.startTime {
			continue
		}
		m.mu.Lock()
		if p.stopping || p.status == StatusRunning {
			p.status, p.err = StatusStopped, ""
		} else {
			p.status, p.err = StatusError, "exited"
		}
		m.saveState()
		m.mu.Unlock()
		close(p.exited)
		return
	}
}
