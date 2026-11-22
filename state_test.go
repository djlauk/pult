package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readState(t *testing.T, path string) []stateEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []stateEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

func waitStatus(t *testing.T, m *Manager, name string, want Status) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, msg := m.State(name)
		if st == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: status = %s (%s), want %s", name, st, msg, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// A tool left running by one pult run is adopted and stoppable by the next.
func TestAdoptAndStop(t *testing.T) {
	cfg, tool := writeTreeTool(t, "47396")
	cfg.Tools = []Tool{tool}
	statePath := filepath.Join(t.TempDir(), "pult-state.json")

	first := NewManager(statePath)
	if err := first.EnsureRunning(cfg, tool); err != nil {
		t.Fatal(err)
	}
	entries := readState(t, statePath)
	if len(entries) != 1 || entries[0].Name != "tree" || entries[0].Port != "47396" || entries[0].StartTime == 0 {
		t.Fatalf("state after start = %+v", entries)
	}

	// "Leave running and quit": a new run reads the state file.
	second := NewManager(statePath)
	second.Adopt(cfg)
	t.Cleanup(func() { second.Stop(tool.Name); first.Stop(tool.Name) })
	waitStatus(t, second, tool.Name, StatusRunning)
	if !second.Adopted(tool.Name) {
		t.Error("tool not marked as adopted")
	}
	if owner := second.PortOwner("47396", "other"); owner != "tree" {
		t.Errorf("PortOwner = %q", owner)
	}

	if err := second.Stop(tool.Name); err != nil {
		t.Fatal(err)
	}
	if st, _ := second.State(tool.Name); st != StatusStopped {
		t.Errorf("status after stop = %s", st)
	}
	deadline := time.Now().Add(3 * time.Second)
	for portOpen("127.0.0.1:47396") {
		if time.Now().After(deadline) {
			t.Fatal("port still open after stopping adopted tool")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if entries := readState(t, statePath); len(entries) != 0 {
		t.Errorf("state after stop = %+v", entries)
	}
}

// A dead process or a reused pid drops the entry; the process is not ours.
func TestCheckProcess(t *testing.T) {
	self := os.Getpid()
	selfStart, alive := processStartTime(self)
	if !alive || selfStart == 0 {
		t.Fatal("processStartTime does not see the test process")
	}
	if r := checkProcess(stateEntry{PID: self, StartTime: selfStart}); r != "" {
		t.Errorf("live process rejected: %s", r)
	}
	if r := checkProcess(stateEntry{PID: self, StartTime: selfStart + 1}); r == "" {
		t.Error("reused pid accepted")
	}
	if r := checkProcess(stateEntry{PID: 999999, StartTime: 1}); r == "" {
		t.Error("dead process accepted")
	}

	statePath := filepath.Join(t.TempDir(), "pult-state.json")
	data, _ := json.Marshal([]stateEntry{{Name: "a", PID: self, StartTime: selfStart + 1, Port: "47397"}})
	os.WriteFile(statePath, data, 0o600)
	m := NewManager(statePath)
	m.Adopt(&Config{Tools: []Tool{{Name: "a", URL: "http://127.0.0.1:47397"}}})
	if st, _ := m.State("a"); st != StatusStopped {
		t.Errorf("reused pid adopted, status %s", st)
	}
	if entries := readState(t, statePath); len(entries) != 0 {
		t.Errorf("state = %+v, want empty", entries)
	}
}

// The reported scenario: start pult with a changed port (not adopted), then
// revert the port. The entry must survive and be adopted on the next page
// load, without restarting pult.
func TestAdoptAfterConfigReverted(t *testing.T) {
	self := os.Getpid()
	selfStart, _ := processStartTime(self)
	entry := stateEntry{Name: "a", PID: self, StartTime: selfStart, Port: "47397"}
	statePath := filepath.Join(t.TempDir(), "pult-state.json")
	data, _ := json.Marshal([]stateEntry{entry})
	os.WriteFile(statePath, data, 0o600)

	changed := &Config{Tools: []Tool{{Name: "a", URL: "http://127.0.0.1:1234"}}}
	removed := &Config{}
	reverted := &Config{Tools: []Tool{{Name: "a", URL: "http://127.0.0.1:47397"}}}

	m := NewManager(statePath)
	m.Adopt(changed)
	m.TryAdopt(removed)
	if st, _ := m.State("a"); st != StatusStopped {
		t.Fatalf("adopted despite mismatch, status %s", st)
	}
	if entries := readState(t, statePath); len(entries) != 1 {
		t.Fatalf("state = %+v, want the entry kept", entries)
	}

	m.TryAdopt(reverted)
	if !m.Adopted("a") {
		t.Fatal("not adopted after tools.json was reverted")
	}
	// Don't let the test Stop its own process: mark it gone instead.
	m.mu.Lock()
	m.procs["a"].startTime = -1
	m.mu.Unlock()
	waitStatus(t, m, "a", StatusError)
}

