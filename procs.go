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

const (
	startTimeout = 30 * time.Second
	stopTimeout  = 10 * time.Second
)

var errPortBusy = errors.New("in use")

type proc struct {
	cmd      *exec.Cmd
	port     string
	logs     *ringLog
	status   Status
	err      string
	stopping bool
	ready    chan struct{} // closed once the port responds or the start failed
	exited   chan struct{} // closed when the process exits
}

func (p *proc) active() bool {
	return p.status == StatusStarting || p.status == StatusRunning
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

// Logs returns the captured output of a tool's last run.
func (m *Manager) Logs(name string) []string {
	m.mu.Lock()
	p, ok := m.procs[name]
	m.mu.Unlock()
	if !ok || p.logs == nil {
		return []string{}
	}
	return p.logs.Tail(logLines)
}

// PortOwner returns the name of an active pult tool (other than exclude)
// that uses port, or "".
func (m *Manager) PortOwner(port, exclude string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.portOwner(port, exclude)
}

func (m *Manager) portOwner(port, exclude string) string {
	for name, p := range m.procs {
		if name != exclude && p.active() && p.port == port {
			return name
		}
	}
	return ""
}

// Running returns the names of all active tools.
func (m *Manager) Running() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for name, p := range m.procs {
		if p.active() {
			names = append(names, name)
		}
	}
	return names
}

// EnsureRunning starts the tool if it is not running and waits until its
// port responds. It returns nil once the tool is reachable.
func (m *Manager) EnsureRunning(cfg *Config, t Tool) error {
	m.mu.Lock()
	p, ok := m.procs[t.Name]
	if !ok || !p.active() {
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
	switch {
	case p.status == StatusRunning:
		return nil
	case p.stopping:
		return errors.New("stopped")
	}
	return errors.New(p.err)
}

// Stop terminates the tool's whole process tree and waits for it to exit.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	p, ok := m.procs[name]
	if !ok || !p.active() {
		m.mu.Unlock()
		return nil
	}
	p.stopping = true
	pid := p.cmd.Process.Pid
	m.mu.Unlock()

	if err := stopTree(pid, p.exited); err != nil {
		return err
	}
	select {
	case <-p.exited:
		return nil
	case <-time.After(stopTimeout):
		return fmt.Errorf("process %d did not exit", pid)
	}
}

// StopAll stops all active tools in parallel.
func (m *Manager) StopAll() {
	var wg sync.WaitGroup
	for _, name := range m.Running() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.Stop(name); err != nil {
				fmt.Fprintf(os.Stderr, "stop %s: %v\n", name, err)
			}
		}()
	}
	wg.Wait()
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
	port, _ := toolPort(t.URL)
	if owner := m.portOwner(port, t.Name); owner != "" {
		return nil, fmt.Errorf("port %s %w by: %s", port, errPortBusy, owner)
	}
	if portOpen(addr) {
		return nil, fmt.Errorf("port %s %w by another process", port, errPortBusy)
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
		port:   port,
		logs:   logs,
		status: StatusStarting,
		ready:  make(chan struct{}),
		exited: make(chan struct{}),
	}
	go m.wait(p)
	go m.waitReady(p, addr)
	return p, nil
}

// wait reaps the process and records how it ended.
func (m *Manager) wait(p *proc) {
	err := p.cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case p.stopping:
		p.status, p.err = StatusStopped, ""
	case err != nil:
		p.status, p.err = StatusError, "exited: "+err.Error()
	case p.status == StatusStarting:
		p.status, p.err = StatusError, "exited before the port responded"
	default:
		p.status = StatusStopped
	}
	if p.status == StatusError {
		if tail := p.logs.Tail(8); len(tail) > 0 {
			p.err += "\n" + strings.Join(tail, "\n")
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
