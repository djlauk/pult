package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestReadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "\ufeff# comment\n\nA=1\nexport B = two \nC=\"quoted # kept\"\nD='single'\nE=val # trailing\nF=\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]string{{"A", "1"}, {"B", "two"}, {"C", "quoted # kept"}, {"D", "single"}, {"E", "val"}, {"F", ""}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if got, err := readDotEnv(filepath.Join(dir, "missing")); err != nil || got != nil {
		t.Errorf("missing file: got %q, %v", got, err)
	}

	os.WriteFile(path, []byte("NOEQUALS\n"), 0o600)
	if _, err := readDotEnv(path); err == nil {
		t.Error("expected error for malformed line")
	}
}

func TestBuildEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PULT_A=dotenv\nPULT_B=dotenv\nPORT=1\n"), 0o600)
	t.Setenv("PULT_A", "system")
	t.Setenv("PULT_SYS", "system")

	tool := Tool{URL: "http://localhost:4321", Env: map[string]string{"PULT_B": "json"}}
	list, err := buildEnv(tool, dir)
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, kv := range list {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	want := map[string]string{"PULT_SYS": "system", "PULT_A": "dotenv", "PULT_B": "json", "PORT": "4321"}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
}

// TestStartNodeTool runs a real child through a script wrapper (like npm)
// and checks working directory, environment and port readiness.
func TestStartNodeTool(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "tool")
	os.Mkdir(dir, 0o755)
	server := `require('http').createServer((q, s) => s.end(JSON.stringify({
  cwd: process.cwd(), port: process.env.PORT, a: process.env.PULT_A, b: process.env.PULT_B
}))).listen(+process.env.PORT, '127.0.0.1');`
	os.WriteFile(filepath.Join(dir, "server.js"), []byte(server), 0o644)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PULT_A=from-dotenv\n"), 0o600)
	if runtime.GOOS == "windows" {
		os.WriteFile(filepath.Join(dir, "serve.cmd"), []byte("@node server.js\r\n"), 0o755)
	} else {
		os.WriteFile(filepath.Join(dir, "serve"), []byte("#!/bin/sh\nexec node server.js\n"), 0o755)
	}

	cfg := &Config{baseDir: base}
	tool := Tool{
		Name:  "node test",
		Dir:   "tool",
		Start: "./serve",
		URL:   "http://127.0.0.1:47391/some/path",
		Env:   map[string]string{"PULT_B": "from-json"},
	}
	m := NewManager()
	if err := m.EnsureRunning(cfg, tool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Stop(tool.Name) })

	if st, _ := m.State(tool.Name); st != StatusRunning {
		t.Fatalf("status = %s", st)
	}
	res, err := http.Get("http://127.0.0.1:47391/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	if !strings.EqualFold(filepath.Clean(got["cwd"]), filepath.Clean(dir)) {
		t.Errorf("cwd = %q, want %q", got["cwd"], dir)
	}
	if got["port"] != "47391" || got["a"] != "from-dotenv" || got["b"] != "from-json" {
		t.Errorf("env = %v", got)
	}

	// A second tool on the same port must not start blindly.
	other := tool
	other.Name = "other"
	if err := m.EnsureRunning(cfg, other); err == nil || !strings.Contains(err.Error(), "47391") {
		t.Errorf("expected port busy error, got %v", err)
	}
	if st, _ := m.State(other.Name); st != StatusStopped {
		t.Errorf("busy refusal should not mark tool as error, got %s", st)
	}
}
