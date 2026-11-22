//go:build windows

package launcher

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

const (
	createNoWindow                 = 0x08000000
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// processStartTime returns the creation time of a running process.
// alive is false if no such process is running.
func processStartTime(pid int) (start int64, alive bool) {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return 0, false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil || code != stillActive {
		return 0, false
	}
	var creation, exit, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	return creation.Nanoseconds(), true
}

// sysProcAttr keeps child processes from opening their own console window.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
	}
}

// OpenDefault opens a URL or file with its default handler (browser, editor).
func OpenDefault(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
