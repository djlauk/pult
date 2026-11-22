# pult: Spec v1

## Goal
A small local launcher that shows tools and links as tiles, starts tools with a click, and can stop them again. It replaces no method and no vault; it is only the entry point.

## Stack
- Go backend, Svelte frontend, embedded via `go:embed`. A single binary with no runtime dependencies. Go dependencies: standard library plus `fyne.io/systray` (tray icon).
- Binary: `pult.exe` (Windows) or `pult` (Linux). Repo folder: `pult`.
- Target 1: Windows. Target 2: Linux (cross-compile with `GOOS=linux`). No Mac support planned.
- Listens on `127.0.0.1` only.

## Configuration: `tools.json`

Location: `pult/tools.json` in the user config directory (`os.UserConfigDir()`: `%AppData%\pult\tools.json` on Windows, `~/.config/pult/tools.json` on Linux). `-config path/to/tools.json` overrides it. If the file is missing, the launcher says where it looked. The config lives outside the source tree so it can't be deleted by accident.

**Tool fields:** `name`, `group`, `dir`, `start`, `url`, optional `env`, optional `start_windows`.
**Link fields:** `name`, `group`, `url` (also `obsidian://…`). Links have no start command.

- `dir`: working directory, absolute or relative to `tools.json` (not to the binary). Set via `cmd.Dir` on the child process. The launcher itself never changes its own working directory.
- `start`: command without platform extension (`./jlpt`, `npm run start`, `uv run …`). Windows resolves `.exe/.bat/.cmd` via PATHEXT. Paths containing a slash are resolved to absolute paths against `dir`.
- `start_windows`: optional override, only if needed.
- `url`: the port is read from it (a path like `/settings` is ignored for the check but opened in full in the browser).

### Example `tools.json`
```json
{
  "tools": [
    { "name": "JLPT prep", "group": "Tools", "dir": "../jlpt-prep", "start": "./jlpt", "url": "http://localhost:4747" },
    { "name": "Family Meal Planner", "group": "Tools", "dir": "../family-meal-planner", "start": "npm run start", "url": "http://localhost:5173" },
    { "name": "NetAcad Progress Tracker", "group": "Tools", "dir": "../netacad-progress-tracker", "start": "npm run start", "url": "http://localhost:8765" },
    { "name": "Lernatelier Liveview IM25b", "group": "Tools", "dir": "../lernatelier-dashboard/lernatelier-mgmt", "start": "uv run -m scripts.liveview IM25b", "url": "http://localhost:8765/settings" },
    { "name": "Lernatelier Liveview IM25d", "group": "Tools", "dir": "../lernatelier-dashboard/lernatelier-mgmt", "start": "uv run -m scripts.liveview IM25d", "url": "http://localhost:8765/settings" }
  ],
  "links": [
    { "name": "Obsidian Schule", "group": "Start", "url": "obsidian://open?vault=bbb" },
    { "name": "Obsidian privat", "group": "Start", "url": "obsidian://open?vault=my-vault" },
    { "name": "SchulNetz BBB", "group": "SchulNetz", "url": "https://schulnetz.bbbaden.ch/" },
    { "name": "SchulNetz KSBA", "group": "SchulNetz", "url": "https://schulnetz-ag.ch/ksba" },
    { "name": "SchulNetz AKSA", "group": "SchulNetz", "url": "https://schulnetz-ag.ch/aksa" }
  ]
}
```

## Environment (order of precedence, later wins)
1. System environment
2. `.env` in the tool folder (not synced; the place for secrets)
3. `PORT` derived from `url`
4. `env` from `tools.json`

## Behavior
- **Start:** Click on a tile → if not running, start the process, wait until the port responds (timeout about 30 s), then open the URL in the browser. If already running, just open it.
- **Status:** Per tile: running / stopped / starting / error. The launcher tracks its own child processes, plus a port check.
- **Port conflict:** If another tool (or a foreign process) already occupies the port, the launcher does not start blindly but shows "Port 8765 in use by: NetAcad". (Currently NetAcad and both Liveviews share port 8765; only one runs at a time.)
- **Stop:** A button on the tile terminates the whole process tree.
  - Windows: `taskkill /T /F /PID …` (because `npm` → `node`).
  - Linux: own process group (`Setpgid`) and signal sent to the group.
- **Logs:** stdout/stderr of each tool goes into a ring buffer, shown in the UI as a collapsible view per tile. Helps with "doesn't start".
- **Tray icon:** pult runs as a tray icon without a console window (Windows build with `-H=windowsgui`; uses `fyne.io/systray`). Left click opens the UI. Menu: *Open pult*, *Show log* (opens `pult.log` in the default editor), *Quit and stop tools*, *Quit and leave running*. On Linux the tray needs a StatusNotifier host (GNOME: AppIndicator extension).
- **Launcher log:** pult's own messages (adoption, quitting, errors) go to `pult.log` next to `pult-state.json`, truncated on each start, and to the console if there is one.
- **Second start:** If pult is already running, starting it again opens the running instance's UI and exits.
- **Quitting the launcher:** Asks whether running tools should be stopped as well: the two tray menu items, the Quit button in the UI, or Ctrl+C in a console. SIGTERM or closing a console window cannot ask; it leaves tools running.
- **Remembering tools across runs:** Running tools are recorded in `pult-state.json` (name, root PID, process start time, port). It lives in the user cache directory (`os.UserCacheDir()`: `%LocalAppData%\pult\` on Windows, `~/.cache/pult/` on Linux), because PIDs only make sense on this machine; with `-config`, it sits next to that `tools.json` so separate setups don't mix. Losing it only means forgetting tools left running. On startup and on every page load, the launcher adopts an entry if the tool is still in `tools.json` with the same port and the PID still belongs to the same process (start time matches). Adopted tools show as running and can be stopped from the UI; their earlier output is not available. Entries that don't match `tools.json` are kept and retried; only a dead process or a reused PID drops an entry. Remembered processes are never killed automatically.
- **Groups:** Tiles grouped by `group`, order as in the file. Optional "Experiments" group for new things.
- **Search:** Pressing `/` focuses a search field, Enter opens the first matching tile.
- `tools.json` is re-read on every page load so new tools appear without a restart.

## Milestones
1. Tiles and start of the JLPT tool.
2. Node and Python tools (environment, port check, status).
3. Stop button, logs, port conflict display, remembering tools across runs.

## Not in v1
Snippet tool, mail/Teams sending, Mac build, autostart at login.

## Open / follow-up in the other projects

These will be taken care of separately, in their own Claud Code sessions.

- Make the port configurable via `PORT` where it isn't yet (Liveview IM25b/IM25d, NetAcad).
- Linux test once the machine is set up again.
