# pult: Spec v1

## Goal
A small local launcher that shows tools and links as tiles, starts tools with a click, and can stop them again. It replaces no method and no vault; it is only the entry point.

## Stack
- Go backend, Svelte frontend, embedded via `go:embed`. A single binary with no runtime dependencies.
- Binary: `pult.exe` (Windows) or `pult` (Linux). Repo folder: `pult`.
- Target 1: Windows. Target 2: Linux (cross-compile with `GOOS=linux`). No Mac support planned.
- Listens on `127.0.0.1` only.

## Configuration: `tools.json` next to the binary

**Tool fields:** `name`, `group`, `dir`, `start`, `url`, optional `env`, optional `start_windows`.
**Link fields:** `name`, `group`, `url` (also `obsidian://…`). Links have no start command.

- `dir`: working directory, relative to `tools.json`. Set via `cmd.Dir` on the child process. The launcher itself never changes its own working directory.
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
    { "name": "Lernatelier Liveview IM25b", "group": "Tools", "dir": "../lernatelier-dashboard/lernatelier-mgmt", "start": "uv run scripts.liveview IM25b", "url": "http://localhost:8765/settings" },
    { "name": "Lernatelier Liveview IM25d", "group": "Tools", "dir": "../lernatelier-dashboard/lernatelier-mgmt", "start": "uv run scripts.liveview IM25d", "url": "http://localhost:8765/settings" }
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
- **Quitting the launcher:** Asks whether running tools should be stopped as well.
- **Groups:** Tiles grouped by `group`, order as in the file. Optional "Experiments" group for new things.
- **Search:** Pressing `/` focuses a search field, Enter opens the first matching tile.
- `tools.json` is re-read on every page load so new tools appear without a restart.

## Milestones
1. Tiles and start of the JLPT tool.
2. Node and Python tools (environment, port check, status).
3. Stop button, logs, port conflict display.

## Not in v1
Snippet tool, mail/Teams sending, Mac build, autostart at login.

## Open / follow-up in the other projects

These will be taken care of separately, in their own Claud Code sessions.

- Make the port configurable via `PORT` where it isn't yet (Liveview IM25b/IM25d, NetAcad).
- Linux test once the machine is set up again.
