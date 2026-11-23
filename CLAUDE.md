# pult

Local launcher for my tools. Read SPEC.md first; it is the source of truth.

## Working agreement
- Work milestone by milestone as listed in SPEC.md. Stop after each one and tell me what to test.
- Always work on a branch; user merges to main.
- Ask before adding dependencies. Prefer the Go standard library.
- Do not modify the other tool projects (jlpt-prep, family-meal-planner, etc.). Only suggest changes there.
- Keep the launcher small. If a feature isn't in SPEC.md, ask first.

## Conventions
- Code, comments, commit messages and UI strings in English.
- Commit messages follow Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `ci:`, optional scope like `feat(tray):`); `CHANGELOG.md` is generated from them. Never edit `CHANGELOG.md` by hand and never use `chore(release):` (reserved for the release script).
- Never call `os.Chdir`; use `cmd.Dir` for child processes.
- Bind to 127.0.0.1 only.
- Platform-specific code (process-tree kill, process start time, opening the browser) lives in `internal/launcher/proc_windows.go` / `proc_unix.go` with build tags.

## Layout
- `cmd/pult/`: entry point, HTTP handlers, tray icon and menu. `rsrc_windows_amd64.syso` (committed, generated) holds the EXE icon and version info.
- `internal/brand/`: version number and the icon, drawn in code (tray, favicon, EXE).
- `internal/winresgen/`: generator for the `.syso`; the only user of `github.com/tc-hib/winres`.
- `internal/launcher/`: config, environment, processes, logs, state file.
- `web/`: Svelte frontend; `web/embed.go` embeds `web/dist`.
- `bin/` (gitignored): build output only.
- `scripts/release.ps1`, `scripts/release.sh`, `cliff.toml`, `.github/workflows/release.yml`: release tooling.
- Data files live outside the repo: `tools.json` in `%AppData%\pult\` (`~/.config/pult/`), `pult-state.json` and `pult.log` in `%LocalAppData%\pult\` (`~/.cache/pult/`). Never commit data files.

## Commands
- Frontend deps (once): `npm --prefix web install`
- Build frontend (required before `go build`, it is embedded): `npm --prefix web run build`
- After changing the icon or `brand.Version`: `go generate ./cmd/pult`, then commit `cmd/pult/rsrc_windows_amd64.syso`.
- Build (tray app, no console window): `go build -ldflags "-H=windowsgui" -o bin/pult.exe ./cmd/pult`
- Build with console (debugging, shows the log live): `go build -o bin/pult-console.exe ./cmd/pult`
- Run: `bin/pult.exe` (flags: `-port 7300`, `-config path/to/tools.json` to override the default location, `-no-browser`)
- Frontend dev: run `bin/pult.exe -no-browser`, then `npm --prefix web run dev` (Vite proxies `/api` to :7300)
- Cross-compile Linux: `$env:GOOS='linux'; go build -o bin/pult-linux ./cmd/pult; Remove-Item Env:GOOS`
- Tests: `go test ./...`
- Release (user only, on a clean `main`): `pwsh scripts/release.ps1 X.Y.Z` (or `scripts/release.sh X.Y.Z`; keep both in sync), then `git push origin main vX.Y.Z`. The tag triggers `.github/workflows/release.yml`, which builds and publishes but never commits.
