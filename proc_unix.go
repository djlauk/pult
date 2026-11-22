//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// sysProcAttr puts each child in its own process group so the whole tree
// can be signalled at once.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// openBrowser opens a URL with the default handler.
func openBrowser(url string) error {
	return exec.Command("xdg-open", url).Start()
}
