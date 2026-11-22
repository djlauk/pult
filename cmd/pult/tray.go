package main

import (
	"log"
	"runtime"

	"fyne.io/systray"

	"pult/internal/launcher"
)

// runTray shows the tray icon and blocks until pult quits. It must run on
// the main goroutine.
func (s *server) runTray() {
	systray.Run(s.onTrayReady, nil)
}

func (s *server) onTrayReady() {
	if runtime.GOOS == "windows" {
		systray.SetIcon(iconICO())
	} else {
		systray.SetIcon(iconPNG())
	}
	systray.SetTitle("pult")
	systray.SetTooltip("pult")
	systray.SetOnTapped(s.openUI) // left click

	open := systray.AddMenuItem("Open pult", "Open the launcher in the browser")
	showLog := systray.AddMenuItem("Show log", "Open pult's log file in the default editor")
	systray.AddSeparator()
	quitStop := systray.AddMenuItem("Quit and stop tools", "Stop all running tools, then quit")
	quitLeave := systray.AddMenuItem("Quit and leave running", "Quit pult; running tools keep running")

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				s.openUI()
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

// exit removes the tray icon and ends the tray loop; main then exits.
// Every way of quitting goes through here so no stale icon is left behind.
func (s *server) exit() {
	systray.Quit()
}
