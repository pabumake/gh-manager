# Changelog

## v0.1.1 - 2026-02-26

- Added startup update detection using GitHub Releases (`releases/latest`).
- Added update indicator next to the TUI version banner.
- Added Settings hierarchy: `Configuration` with `Theme` and `Update` submenus.
- Added in-app self-update action (`Settings -> Configuration -> Update -> Update now`).
- Added installer and uninstaller one-liners for Linux/macOS and Windows.
- Added restore modal horizontal scrolling improvements for long entries.

Known limitations:
- Update checks depend on GitHub API/network availability.
- Successful self-update requires manual app restart.
