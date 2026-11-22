package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeTreeTool creates a tool whose process tree mirrors npm: a script
// wrapper starts node parent.js, which spawns node server.js (the listener).
func writeTreeTool(t *testing.T, port string) (*Config, Tool) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	parent := `const c = require('child_process').spawn(process.execPath, ['server.js'], { stdio: 'inherit' });
console.log('parent up'); c.on('exit', (code) => process.exit(code ?? 1));`
	server := `require('http').createServer((q, s) => s.end('ok'))
  .listen(+process.env.PORT, '127.0.0.1', () => console.log('listening on ' + process.env.PORT));`
	os.WriteFile(filepath.Join(dir, "parent.js"), []byte(parent), 0o644)
	os.WriteFile(filepath.Join(dir, "server.js"), []byte(server), 0o644)
	if runtime.GOOS == "windows" {
		os.WriteFile(filepath.Join(dir, "serve.cmd"), []byte("@node parent.js\r\n"), 0o755)
	} else {
		os.WriteFile(filepath.Join(dir, "serve"), []byte("#!/bin/sh\nnode parent.js\n"), 0o755)
	}
	return &Config{baseDir: dir}, Tool{Name: "tree", Dir: ".", Start: "./serve", URL: "http://127.0.0.1:" + port}
}

func TestStopKillsTree(t *testing.T) {
	cfg, tool := writeTreeTool(t, "47393")
	m := NewManager("")
	if err := m.EnsureRunning(cfg, tool); err != nil {
		t.Fatal(err)
	}
	if !portOpen("127.0.0.1:47393") {
		t.Fatal("port not open after start")
	}
	if err := m.Stop(tool.Name); err != nil {
		t.Fatal(err)
	}
	if st, msg := m.State(tool.Name); st != StatusStopped {
		t.Errorf("status = %s (%s), want stopped", st, msg)
	}
	// The grandchild held the port; it must be gone too.
	deadline := time.Now().Add(3 * time.Second)
	for portOpen("127.0.0.1:47393") {
		if time.Now().After(deadline) {
			t.Fatal("grandchild still listening after Stop")
		}
		time.Sleep(100 * time.Millisecond)
	}
	logs := strings.Join(m.Logs(tool.Name), "\n")
	if !strings.Contains(logs, "parent up") || !strings.Contains(logs, "listening on 47393") {
		t.Errorf("logs = %q", logs)
	}
	if got := m.Running(); len(got) != 0 {
		t.Errorf("Running() = %v after stop", got)
	}
}

func TestPortOwnerConflict(t *testing.T) {
	cfg, tool := writeTreeTool(t, "47394")
	m := NewManager("")
	if err := m.EnsureRunning(cfg, tool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Stop(tool.Name) })

	other := tool
	other.Name = "other"
	err := m.EnsureRunning(cfg, other)
	if err == nil || !strings.Contains(err.Error(), "in use by: tree") {
		t.Errorf("err = %v, want 'in use by: tree'", err)
	}
	if owner := m.PortOwner("47394", "other"); owner != "tree" {
		t.Errorf("PortOwner = %q", owner)
	}
}

// While a tool is still starting (port not yet open), a second tool on the
// same port must be refused as well.
func TestPortOwnerWhileStarting(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "slow.js"), []byte(`setTimeout(() => {}, 60000);`), 0o644)
	cfg := &Config{baseDir: dir}
	slow := Tool{Name: "slow", Dir: ".", Start: "node slow.js", URL: "http://127.0.0.1:47395"}
	m := NewManager("")
	go m.EnsureRunning(cfg, slow)
	t.Cleanup(func() { m.Stop(slow.Name) })

	deadline := time.Now().Add(3 * time.Second)
	for {
		if st, _ := m.State("slow"); st == StatusStarting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("slow tool never reached starting")
		}
		time.Sleep(20 * time.Millisecond)
	}
	other := slow
	other.Name = "other"
	if err := m.EnsureRunning(cfg, other); err == nil || !strings.Contains(err.Error(), "by: slow") {
		t.Errorf("err = %v, want refusal naming slow", err)
	}
}
