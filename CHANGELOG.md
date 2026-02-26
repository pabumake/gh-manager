# Changelog

## v0.1.2 - Unreleased

- Added theme token variable support in JSON themes:
  - top-level `vars`
  - `var(--token)` references in `colors`
  - nested variable resolution with cycle detection
- Kept backward compatibility for existing flat-hex themes.
- Migrated Catppuccin Mocha theme to tokenized color variables.
- Improved restore modal list behavior:
  - command-style highlighted selection rows
  - reliable vertical cursor visibility with viewport scrolling
  - horizontal scroll indicators
  - popup/list height scaling with terminal size
- Expanded docs for theme variable syntax and strict validation behavior.

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
