//go:build !windows

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

// stopTree sends SIGTERM to the child's process group, waits for the child
// to exit (up to a grace period), then SIGKILLs whatever is left in the group.
func stopTree(pid int, exited <-chan struct{}) error {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

// sysProcAttr puts each child in its own process group so the whole tree
// can be signalled at once.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// openBrowser opens a URL with the default handler.
func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}
