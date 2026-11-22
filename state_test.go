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

// Entries that do not match tools.json or the live process are not adopted,
// and their processes are left alone.
func TestAdoptRejectsMismatches(t *testing.T) {
	self := os.Getpid()
	selfStart, alive := processStartTime(self)
	if !alive || selfStart == 0 {
		t.Fatal("processStartTime does not see the test process")
	}
	cfg := &Config{Tools: []Tool{{Name: "a", URL: "http://127.0.0.1:47397"}}}
	cases := map[string]stateEntry{
		"unknown tool": {Name: "gone", PID: self, StartTime: selfStart, Port: "47397"},
		"port changed": {Name: "a", PID: self, StartTime: selfStart, Port: "1234"},
		"pid reused":   {Name: "a", PID: self, StartTime: selfStart + 1, Port: "47397"},
		"dead process": {Name: "a", PID: 999999, StartTime: 1, Port: "47397"},
	}
	for name, e := range cases {
		if reason := checkEntry(cfg, e); reason == "" {
			t.Errorf("%s: entry was accepted", name)
		}
	}
	good := stateEntry{Name: "a", PID: self, StartTime: selfStart, Port: "47397"}
	if reason := checkEntry(cfg, good); reason != "" {
		t.Errorf("matching entry rejected: %s", reason)
	}

	// Through Adopt: a rejected entry is dropped from the state file.
	statePath := filepath.Join(t.TempDir(), "pult-state.json")
	data, _ := json.Marshal([]stateEntry{cases["pid reused"]})
	os.WriteFile(statePath, data, 0o600)
	m := NewManager(statePath)
	m.Adopt(cfg)
	if st, _ := m.State("a"); st != StatusStopped {
		t.Errorf("mismatched entry adopted, status %s", st)
	}
	if entries := readState(t, statePath); len(entries) != 0 {
		t.Errorf("state = %+v, want empty", entries)
	}
}
