//go:build !windows

package launcher

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// processStartTime returns the start time of a running process (clock ticks
// since boot, from /proc). alive is false if it is gone or a zombie.
func processStartTime(pid int) (start int64, alive bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	// The command name in field 2 may contain spaces; parse after its ')'.
	s := string(data)
	fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
	if len(fields) < 20 || fields[0] == "Z" {
		return 0, false
	}
	start, err = strconv.ParseInt(fields[19], 10, 64) // field 22: starttime
	return start, err == nil
}

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

// OpenDefault opens a URL or file with its default handler (browser, editor).
func OpenDefault(url string) error {
	return exec.Command("xdg-open", url).Start()
}
