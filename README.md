# pult

A small local launcher. It shows your tools and links as tiles in the browser, starts a tool with a click, opens it once its port answers, and stops it again (the whole process tree). It runs as a tray icon and listens on `127.0.0.1` only.

See [SPEC.md](SPEC.md) for the full behavior.

## Features

- Tiles for tools (with a start command) and links (`https://…`, `obsidian://…`), grouped.
- Start, stop, status (running / starting / stopped / error) and a port check per tool.
- Shows "Port 8765 in use by: …" instead of starting into a port conflict.
- Captured stdout/stderr per tool, shown in the UI.
- Remembers tools left running across pult restarts, so they can still be stopped.
- Search: press `/`, type, Enter opens the first match.
- `tools.json` is re-read on every page load; no restart needed after editing it.

## Build

Requires Go and Node.js.

```powershell
npm --prefix web install          # once
npm --prefix web run build        # the frontend is embedded, so build it first
go build -ldflags "-H=windowsgui" -o bin/pult.exe ./cmd/pult
```

For a build with a console window (shows the log live): `go build -o bin/pult-console.exe ./cmd/pult`.

Linux: `GOOS=linux go build -o bin/pult ./cmd/pult`. The tray icon needs a StatusNotifier host (on GNOME, the AppIndicator extension).

## Run

```
pult.exe [-port 7300] [-config path/to/tools.json] [-no-browser]
```

pult opens its UI at <http://127.0.0.1:7300/> and puts an icon in the tray. Left-click the icon to open the UI. The tray menu has:

- **Open pult**
- **Edit tools.json**: opens the config with the default app for `.json` (creates an empty one if missing)
- **Show log**: opens `pult.log`
- **Quit and stop tools** / **Quit and leave running**

Starting pult a second time opens the running instance's UI.

## Configuration

| File | Windows | Linux |
| --- | --- | --- |
| `tools.json` | `%AppData%\pult\` | `~/.config/pult/` |
| `pult-state.json`, `pult.log` | `%LocalAppData%\pult\` | `~/.cache/pult/` |

`-config` uses a different `tools.json`; the state file and log then sit next to it.

```json
{
  "tools": [
    { "name": "JLPT prep", "group": "Tools", "dir": "C:/dev/jlpt-prep", "start": "./jlpt", "url": "http://localhost:4747" },
    { "name": "Meal Planner", "group": "Tools", "dir": "C:/dev/family-meal-planner", "start": "npm run dev", "url": "http://localhost:5173",
      "env": { "NODE_ENV": "development" } }
  ],
  "links": [
    { "name": "Obsidian", "group": "Start", "url": "obsidian://open?vault=my-vault" }
  ]
}
```

- `dir`: working directory, absolute or relative to `tools.json`.
- `start`: the command, without a platform extension (`.exe`, `.cmd` and `.bat` are found via PATHEXT). `start_windows` overrides it on Windows.
- `url`: opened in the browser; its port is used for the port check and passed to the tool as `PORT`.
- Environment, later wins: system environment, `.env` in the tool folder, `PORT` from `url`, `env` from `tools.json`.

## Start at login

**Windows:** press Win+R, enter `shell:startup`, and create a shortcut there with the target:

```
C:\path\to\pult.exe -no-browser
```

`-no-browser` keeps a browser tab from opening at every login; the tray icon is enough. To turn it off, delete the shortcut or disable it under Task Manager → Startup apps.

Windows locks a running exe, so a rebuild into the same path fails while pult runs. Quit pult from the tray first, or point the shortcut at a copy outside `bin/`, e.g. `%LocalAppData%\Programs\pult\pult.exe`.

**Linux:** create `~/.config/autostart/pult.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=pult
Exec=/path/to/pult -no-browser
X-GNOME-Autostart-enabled=true
```

## License

[MIT](LICENSE)
