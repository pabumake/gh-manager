# gh-manager

`gh-manager` is a safety-first CLI + TUI for reviewing, backing up, deleting, and restoring GitHub repositories.

## Preview
![No Theme](/docs/img/gh-manager-default.png)
No Theme

![Default Catpucchin Mocha inspired Theme](/docs/img/gh-manager-catpucchin-mocha.png)
Default Catpucchin Mocha inspired Theme

## Quick Prerequisites

- `gh` authenticated (`gh auth login`)
- `git`
- SSH access to `git@github.com`

## Build

```bash
go build -o gh-manager ./cmd/gh-manager
```

## Install (Latest Release)

Linux/macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.sh | bash
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.ps1 | iex
```

Uninstall one-liners:

Linux/macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.sh | bash -s -- --uninstall
```

Windows PowerShell:

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.ps1))) -Uninstall
```

Installer behavior:
- Linux/macOS installs `gh-manager` to `/usr/local/bin/gh-manager`.
- Windows installs to `%LOCALAPPDATA%\\Programs\\gh-manager\\bin\\gh-manager.exe` and updates user `PATH`.
- Installer auto-installs and applies `catppuccin-mocha`.
- Uninstaller mode removes binary and cleans PATH entry (Windows user PATH).
- If `PATH` updates are not visible immediately, open a new shell.
- TUI checks for updates on startup and shows an update indicator next to version when available.

## Quick Start

1. Launch the TUI: `./gh-manager`
2. Create and review a plan: `gh-manager plan` then `gh-manager inspect --plan <plan.json>`
3. Backup before deletion: `gh-manager backup --plan <plan.json>`
4. Execute deletion safely: `gh-manager execute --plan <plan.json>`

## Full Documentation

- [User Guide](docs/user-guide.md)
- [Changelog](CHANGELOG.md)
- [Commands](docs/user-guide.md#commands)
- [TUI Controls](docs/user-guide.md#tui-controls)
- [Configuration and Themes](docs/user-guide.md#configuration-and-themes)
- [Restore Workflow](docs/user-guide.md#restore-workflow)
- [Delete Workflow](docs/user-guide.md#delete-workflow)
- [Safety Model](docs/user-guide.md#safety-model)
