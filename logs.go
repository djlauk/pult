package main

import (
	"strings"
	"sync"
)

const logLines = 500

// ringLog keeps the last logLines lines written to it. It is safe for
// concurrent use as cmd.Stdout and cmd.Stderr.
type ringLog struct {
	mu      sync.Mutex
	lines   []string
	next    int    // write position once the buffer is full
	partial string // unterminated last line
}

func (r *ringLog) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.partial + strings.ReplaceAll(string(b), "\r\n", "\n")
	parts := strings.Split(s, "\n")
	r.partial = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		r.add(line)
	}
	return len(b), nil
}

func (r *ringLog) add(line string) {
	if len(r.lines) < logLines {
		r.lines = append(r.lines, line)
		return
	}
	r.lines[r.next] = line
	r.next = (r.next + 1) % logLines
}

// Tail returns up to n of the most recent lines, oldest first, including
// an unterminated last line.
func (r *ringLog) Tail(n int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	all := append(append([]string{}, r.lines[r.next:]...), r.lines[:r.next]...)
	if r.partial != "" {
		all = append(all, r.partial)
	}
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all
}
