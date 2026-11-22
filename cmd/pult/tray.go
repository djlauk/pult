package main

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/systray"

	"pult/internal/brand"
	"pult/internal/launcher"
)

// runTray shows the tray icon and blocks until pult quits. It must run on
// the main goroutine.
func (s *server) runTray() {
	systray.Run(s.onTrayReady, nil)
}

func (s *server) onTrayReady() {
	if runtime.GOOS == "windows" {
		systray.SetIcon(brand.ICO())
	} else {
		systray.SetIcon(brand.PNG())
	}
	systray.SetTitle("pult")
	systray.SetTooltip("pult " + brand.Version)
	systray.SetOnTapped(s.openUI) // left click

	open := systray.AddMenuItem("Open pult", "Open the launcher in the browser")
	editConfig := systray.AddMenuItem("Edit tools.json", "Open the configuration in the default editor")
	showLog := systray.AddMenuItem("Show log", "Open pult's log file in the default editor")
	systray.AddSeparator()
	quitStop := systray.AddMenuItem("Quit and stop tools", "Stop all running tools, then quit")
	quitLeave := systray.AddMenuItem("Quit and leave running", "Quit pult; running tools keep running")

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				s.openUI()
			case <-editConfig.ClickedCh:
				s.editConfig()
			case <-showLog.ClickedCh:
				if err := launcher.OpenDefault(s.logPath); err != nil {
					log.Printf("open log: %v", err)
				}
			case <-quitStop.ClickedCh:
				log.Printf("quit from tray (stop tools)")
				s.procs.StopAll()
				s.exit()
			case <-quitLeave.ClickedCh:
				log.Printf("quit from tray (leave tools running)")
				s.exit()
			}
		}
	}()
}

func (s *server) openUI() {
	if err := launcher.OpenDefault(s.uiURL); err != nil {
		log.Printf("open browser: %v", err)
	}
}

// emptyConfig is written when tools.json does not exist yet, so there is
// something to edit.
const emptyConfig = `{
  "tools": [],
  "links": []
}
`

// editConfig opens tools.json with the default handler for .json files,
// creating an empty one first if needed.
func (s *server) editConfig() {
	if err := ensureConfig(s.configPath); err != nil {
		log.Printf("create config: %v", err)
		return
	}
	if err := launcher.OpenDefault(s.configPath); err != nil {
		log.Printf("open config: %v", err)
	}
}

// ensureConfig creates an empty tools.json (and its folder) if it does not
// exist. An existing file is never touched.
func ensureConfig(path string) error {
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(emptyConfig); err != nil {
		return err
	}
	log.Printf("created empty %s", path)
	return nil
}

// exit removes the tray icon and ends the tray loop; main then exits.
// Every way of quitting goes through here so no stale icon is left behind.
func (s *server) exit() {
	systray.Quit()
}
