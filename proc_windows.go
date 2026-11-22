//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// stopTree kills the process and all its descendants (npm → cmd → node).
func stopTree(pid int, exited <-chan struct{}) error {
	out, err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).CombinedOutput()
	if err != nil {
		select {
		case <-exited:
			return nil // already gone
		default:
		}
		return fmt.Errorf("taskkill: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

const createNoWindow = 0x08000000

// sysProcAttr keeps child processes from opening their own console window.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

// openBrowser opens a URL with the default handler.
func openBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
