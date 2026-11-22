package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestMain lets the test binary act as a short-lived pult run: started with
// PULT_TEST_FIRST_RUN set, it starts one tool and exits, leaving it running.
func TestMain(m *testing.M) {
	if dir := os.Getenv("PULT_TEST_FIRST_RUN"); dir != "" {
		cfg := &Config{baseDir: dir}
		tool := Tool{Name: "tree", Dir: ".", Start: "./serve", URL: "http://127.0.0.1:" + os.Getenv("PULT_TEST_PORT")}
		mgr := NewManager(filepath.Join(dir, "pult-state.json"))
		if err := mgr.EnsureRunning(cfg, tool); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		os.Exit(0) // "Leave running and quit"
	}
	os.Exit(m.Run())
}

// A real pult process exits while its tool keeps running; the next run
// must still be able to adopt the tool, even after the tool writes output.
func TestAdoptAfterRealExit(t *testing.T) {
	cfg, tool := writeTreeTool(t, "47398")
	cfg.Tools = []Tool{tool}

	first := exec.Command(os.Args[0], "-test.run=^$")
	first.Env = append(os.Environ(), "PULT_TEST_FIRST_RUN="+cfg.baseDir, "PULT_TEST_PORT=47398")
	if out, err := first.CombinedOutput(); err != nil {
		t.Fatalf("first run: %v: %s", err, out)
	}

	// Give the orphaned tool time to notice its output pipe is gone.
	time.Sleep(3 * time.Second)

	second := NewManager(filepath.Join(cfg.baseDir, "pult-state.json"))
	second.Adopt(cfg)
	t.Cleanup(func() { second.Stop(tool.Name) })
	waitStatus(t, second, tool.Name, StatusRunning)

	// Keep watching for a while: the adopted tool must stay running.
	time.Sleep(3 * time.Second)
	if st, msg := second.State(tool.Name); st != StatusRunning {
		t.Fatalf("adopted tool status = %s (%s)", st, msg)
	}
	if err := second.Stop(tool.Name); err != nil {
		t.Fatal(err)
	}
	if portOpen("127.0.0.1:47398") {
		t.Error("port still open after stop")
	}
}
