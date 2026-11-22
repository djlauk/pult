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
- Never call `os.Chdir`; use `cmd.Dir` for child processes.
- Bind to 127.0.0.1 only.
- Platform-specific code (process-tree kill) lives in `proc_windows.go` / `proc_unix.go` with build tags.

## Commands
- Frontend deps (once): `npm --prefix web install`
- Build frontend (required before `go build`, it is embedded): `npm --prefix web run build`
- Build: `go build -o pult.exe .`
- Run: `./pult.exe` (flags: `-port 7300`, `-config path/to/tools.json`, `-no-browser`)
- Frontend dev: run `pult.exe -no-browser`, then `npm --prefix web run dev` (Vite proxies `/api` to :7300)
- Cross-compile Linux: `$env:GOOS='linux'; go build -o pult-linux .; Remove-Item Env:GOOS`
- Tests: `go test ./...`
