package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Status of a tool tile.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

const startTimeout = 30 * time.Second

var errPortBusy = errors.New("in use")

type proc struct {
	cmd    *exec.Cmd
	logs   *ringLog
	status Status
	err    string
	ready  chan struct{} // closed once the port responds or the start failed
	exited chan struct{} // closed when the process exits
}

// Manager tracks the child processes started by the launcher.
type Manager struct {
	mu    sync.Mutex
	procs map[string]*proc
}

func NewManager() *Manager {
	return &Manager{procs: map[string]*proc{}}
}

// State returns the status and last error of a tool.
func (m *Manager) State(name string) (Status, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.procs[name]
	if !ok {
		return StatusStopped, ""
	}
	return p.status, p.err
}

// EnsureRunning starts the tool if it is not running and waits until its
// port responds. It returns nil once the tool is reachable.
func (m *Manager) EnsureRunning(cfg *Config, t Tool) error {
	m.mu.Lock()
	p, ok := m.procs[t.Name]
	if !ok || p.status == StatusStopped || p.status == StatusError {
		var err error
		p, err = m.start(cfg, t)
		if errors.Is(err, errPortBusy) {
			m.mu.Unlock()
			return err // not a tool failure; the tile shows the busy port
		}
		if err != nil {
			m.procs[t.Name] = &proc{status: StatusError, err: err.Error()}
			m.mu.Unlock()
			return err
		}
		m.procs[t.Name] = p
	}
	m.mu.Unlock()

	<-p.ready
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.status != StatusRunning {
		return errors.New(p.err)
	}
	return nil
}

// start launches the process. Caller holds m.mu.
func (m *Manager) start(cfg *Config, t Tool) (*proc, error) {
	dir := cfg.ToolDir(t)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("directory not found: %s", dir)
	}
	line := t.Start
	if runtime.GOOS == "windows" && t.StartWindows != "" {
		line = t.StartWindows
	}
	args := splitCommand(line)
	if len(args) == 0 {
		return nil, errors.New("empty start command")
	}
	exe, err := resolveExe(args[0], dir)
	if err != nil {
		return nil, err
	}
	addr, err := toolAddr(t.URL)
	if err != nil {
		return nil, err
	}
	if portOpen(addr) {
		port, _ := toolPort(t.URL)
		return nil, fmt.Errorf("port %s: %w", port, errPortBusy)
	}
	env, err := buildEnv(t, dir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(exe, args[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.SysProcAttr = sysProcAttr()
	logs := &ringLog{}
	cmd.Stdout, cmd.Stderr = logs, logs
	// Grandchildren may keep the output pipe open after the child exits.
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start failed: %w", err)
	}

	p := &proc{
		cmd:    cmd,
		logs:   logs,
		status: StatusStarting,
		ready:  make(chan struct{}),
		exited: make(chan struct{}),
	}
	go m.wait(t.Name, p)
	go m.waitReady(p, addr)
	return p, nil
}

// wait reaps the process and records how it ended.
func (m *Manager) wait(name string, p *proc) {
	err := p.cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.status == StatusStarting || p.status == StatusRunning {
		if err != nil {
			p.status, p.err = StatusError, "exited: "+err.Error()
		} else if p.status == StatusStarting {
			p.status, p.err = StatusError, "exited before the port responded"
		} else {
			p.status = StatusStopped
		}
		if p.status == StatusError {
			if tail := p.logs.Tail(8); len(tail) > 0 {
				p.err += "\n" + strings.Join(tail, "\n")
			}
		}
	}
	close(p.exited)
}

// waitReady polls the port until it responds, the process exits, or the
// timeout passes.
func (m *Manager) waitReady(p *proc, addr string) {
	defer close(p.ready)
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		if portOpen(addr) {
			m.mu.Lock()
			if p.status == StatusStarting {
				p.status = StatusRunning
			}
			m.mu.Unlock()
			return
		}
		select {
		case <-p.exited:
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
	m.mu.Lock()
	if p.status == StatusStarting {
		p.status, p.err = StatusError, fmt.Sprintf("port %s did not respond within %s", addr, startTimeout)
	}
	m.mu.Unlock()
}

func portOpen(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// resolveExe turns the first word of a start command into an executable path.
// Names containing a slash are resolved against dir; others are looked up in
// PATH. On Windows, LookPath also tries the PATHEXT extensions.
func resolveExe(name, dir string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
	}
	exe, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable not found: %s", name)
	}
	return exe, nil
}

// splitCommand splits a command line on whitespace, honoring double quotes.
func splitCommand(s string) []string {
	var args []string
	var cur strings.Builder
	inQuote, have := false, false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			have = true
		case !inQuote && (r == ' ' || r == '\t'):
			if have {
				args = append(args, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteRune(r)
			have = true
		}
	}
	if have {
		args = append(args, cur.String())
	}
	return args
}
