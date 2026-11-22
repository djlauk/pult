package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRingLog(t *testing.T) {
	r := &ringLog{}
	r.Write([]byte("a\r\nb\npar"))
	r.Write([]byte("tial"))
	if got, want := r.Tail(10), []string{"a", "b", "partial"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	for i := 0; i < logLines+5; i++ {
		fmt.Fprintf(r, "%d\n", i)
	}
	tail := r.Tail(2)
	if want := []string{fmt.Sprint(logLines + 3), fmt.Sprint(logLines + 4)}; !reflect.DeepEqual(tail, want) {
		t.Errorf("tail = %q, want %q", tail, want)
	}
}

// A tool that fails during start reports its last output lines.
func TestStartFailureShowsOutput(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fail.js"), []byte(`console.error("boom: missing config"); process.exit(1);`), 0o644)
	cfg := &Config{baseDir: dir}
	tool := Tool{Name: "fail", Dir: ".", Start: "node fail.js", URL: "http://127.0.0.1:47392"}

	err := NewManager("").EnsureRunning(cfg, tool)
	if err == nil || !strings.Contains(err.Error(), "exit status 1") || !strings.Contains(err.Error(), "boom: missing config") {
		t.Errorf("err = %v", err)
	}
}
