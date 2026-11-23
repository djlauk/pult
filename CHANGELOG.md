# Changelog

All notable changes to pult. Generated from the commit history with
[git-cliff](https://git-cliff.org); the format follows
[Conventional Commits](https://www.conventionalcommits.org).

## [1.0.0] - 2026-11-23

### Features

- Tiles, groups, search and tool start (milestone 1)
- Environment, port check and live status (milestone 2)
- Capture tool output and show it when a start fails
- Stop button, logs view, port owner and quit (milestone 3)
- Remember started tools and adopt them on the next run
- Default config and state to the user config and cache dirs
- **tray:** Run as a tray icon
- **ui:** Serve the tray icon as the favicon
- **icon:** Use four shades of green instead of blue
- Add icon and version info to the Windows EXE
- **tray:** Add Edit tools.json; remove the Quit button from the UI
- **ui:** Show pult's version in the header

### Bug fixes

- Keep remembered tools until their process is gone

### Refactoring

- Restructure into cmd/pult, internal/launcher and bin/

### Documentation

- **spec:** Fix example commands, document quitting and remembered tools
- Add README (incl. start at login) and MIT license
- **readme:** Recommend installing to %LocalAppData%\Programs\pult

### Build and CI

- Add release workflow and changelog generation
- Add scripts/release.sh alongside release.ps1

### Other

- Ignore tools.json
- Ensure LF line endings


