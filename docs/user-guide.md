# User Guide

## Overview

`gh-manager` is a safety-first CLI + TUI for reviewing, backing up, deleting, and restoring GitHub repositories.

## Requirements

- Linux/macOS/Windows shell environment
- `gh` authenticated (`gh auth login`)
- `git`
- SSH access to `git@github.com`
- Nerd Font (recommended for TUI icons): `HackNerdFontMono-Regular.ttf` is vendored in `third_party/fonts/hack-nerd-font/`

## Install / Build

```bash
go build -o gh-manager ./cmd/gh-manager
```

Install latest release (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.sh | bash
```

Install latest release (Windows PowerShell):

```powershell
irm https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.ps1 | iex
```

Uninstall (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.sh | bash -s -- --uninstall
```

Uninstall (Windows PowerShell):

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/pabumake/gh-manager/main/scripts/install.ps1))) -Uninstall
```

Installer defaults:
- Linux/macOS installs to `/usr/local/bin/gh-manager`.
- Windows installs to `%LOCALAPPDATA%\\Programs\\gh-manager\\bin\\gh-manager.exe` and updates user `PATH`.
- Installer auto-installs and applies `catppuccin-mocha`.
- Uninstaller mode removes the installed binary and cleans PATH entry on Windows.
- If PATH updates are not visible, open a new shell.

## Commands

- `gh-manager` (launches TUI home)
- `gh-manager doctor`
- `gh-manager plan [--owner <user>] [--out <plan.json>]`
- `gh-manager backup --plan <plan.json> [--backup-location <dir>] [--resume=true|false] [--dry-run] [--archive-repo <owner/name>] [--archive-branch <branch>] [--archive-visibility private|public] [--no-archive]`
- `gh-manager restore --archive-root <dir> --repo <owner/name> [--target-owner <owner>] [--target-name <name>] [--visibility private|public]`
- `gh-manager delete --repo <owner/name> [--force]`
- `gh-manager theme list [--remote]`
- `gh-manager theme current`
- `gh-manager theme install <theme-id>`
- `gh-manager theme apply <theme-id|default>`
- `gh-manager theme uninstall <theme-id>`
- `gh-manager inspect --plan <plan.json>`
- `gh-manager execute --plan <plan.json> [--backup-location <dir>] [--resume=true|false] [--dry-run]`
- `gh-manager version`

## Configuration and Themes

Config root:

```text
~/.config/gh-manager
```

Files and directories:

```text
~/.config/gh-manager/config.json
~/.config/gh-manager/secret.hex
~/.config/gh-manager/themes/<theme-id>.json
```

Notes:
- Theme files use hex colors (`#RRGGBB`).
- On truecolor terminals, hex colors are used directly.
- On non-truecolor terminals, colors are converted to nearest xterm-256 colors at runtime.
- If no theme is configured or loading fails, `gh-manager` falls back to built-in default styling.
- Layout is stow-friendly: the entire `~/.config/gh-manager` directory can be symlink-managed.

Theme management:

```bash
gh-manager theme list
gh-manager theme list --remote
gh-manager theme install catppuccin-mocha
gh-manager theme apply catppuccin-mocha
gh-manager theme current
gh-manager theme apply default
gh-manager theme uninstall catppuccin-mocha
```

Default remote theme index:

```text
https://raw.githubusercontent.com/pabumake/gh-manager/main/themes/index.json
```

Local preview before push:

- `gh-manager theme install <id>` automatically falls back to local `themes/index.json` when the remote index is unavailable.
- Theme URLs in the index can be relative (for example `catppuccin-mocha.json`) and are resolved relative to the index file location.

## Typical Workflow

1. Run `gh-manager plan`.
2. In the TUI, filter/sort/select repositories and press `s` to save the signed plan.
3. Review with `gh-manager inspect --plan <plan.json>`.
4. Run `gh-manager backup --plan <plan.json>` to create mirror + bundle backups (optional archive publish).
5. Run `gh-manager execute --plan <plan.json>` and type the exact confirmation phrase for deletion.
6. For `backup` and `execute`, confirmation accepts either `ACCEPT` or `CONFIRM`.
7. Use `Restore` in the TUI Commands pane to restore from an archive folder to GitHub (bundle-first, snapshot fallback).

## TUI Controls

- Global:
- `1`: Browse/Select mode
- `2`: Commands mode
- `3`: Details mode
- `tab`: switch pane focus (table / commands)
- `q`: quit
- Browse/Select:
- `j` / `k`: move cursor
- `pgup` / `pgdown`: page navigation
- `space`: toggle selected repo
- `a`: select all currently filtered repos
- `x`: clear all currently filtered repos
- `type`: append filter text
- `backspace`: remove filter text
- `n`: sort by name (press again to toggle asc/desc)
- `u`: sort by updatedAt (press again to toggle asc/desc)
- `v`: sort by visibility (press again to toggle asc/desc)
- Commands panel:
- `j` / `k`: move command cursor
- `enter`: open form / run command (includes Restore flow and Settings popup)
- `tab`: move to next form field
- `space`: toggle boolean form fields
- `esc`: cancel command form
- Popup forms/prompts:
- while popup is open, global shortcuts are suspended until `enter` or `esc`
- active text input shows a blinking cursor
- command results open in a dedicated popup (instead of inline output at the bottom)
- popups render with a backdrop scrim over the rest of the TUI
- after mutating GitHub actions (`Execute`, `Restore`, `Delete`), the repo table auto-refreshes
- in command forms, `space` toggles boolean fields (for example `dry_run`)
- Backup and Execute auto-generate a signed plan from current selected repos when plan path is left empty
- Backup and Execute auto-generate backup location when left empty (same default behavior as CLI mode)
- Placeholders are visual examples; blank input triggers auto-generation where supported
- Restore flow:
- archive browser popup: `j/k`, `enter` open/select, `backspace` parent, `esc` cancel
- repo select popup: `j/k`, `enter` choose, `esc` back
- popup question: type `yes/no` and press `enter`
- rename popup (if no/conflict): type new name and press `enter`
- Delete flow:
- command: `Delete` from commands pane (uses highlighted repo)
- popup warning is red and includes repo details + no-backup warning
- user must type the exact repository name and press `enter`
- Settings flow:
- command: `Settings` from commands pane
- popup opens `Configuration` with submenus: `Theme` and `Update`
- Theme submenu supports current/list/apply/install/uninstall actions
- applying a theme updates the live TUI immediately (no restart)
- uninstalling the active theme automatically switches back to `default`
- Update submenu supports `Check now` and `Update now` (self-update)
- successful update requires restarting `gh-manager` to run the new binary
- Plan TUI (`gh-manager plan`) compatibility:
- `s`: save plan and exit
- `q`: quit without saving

## Restore Workflow

TUI restore (`gh-manager` -> Commands -> Restore):

1. Select an archive root from the file browser (default opens your `Documents` folder: `~/Documents` on Linux/macOS, user Documents on Windows).
2. Select repository artifact from indexed entries.
3. Answer popup: `Use original name?` (`yes`/`no` variants accepted).
4. If `no`, enter a new repository name; restore continues on `enter`.
5. Restore target defaults to current authenticated user and private visibility.
6. If target already exists, a conflict message appears and rename input reopens with suggested suffix `-ghm`.

CLI restore:

```bash
gh-manager restore --archive-root /home/pabumake/Documents/gh-archive-2026-02-25 --repo pabumake/reppy
```

Manual restore from a local bundle:

```bash
git clone /path/to/alice__my-repo.bundle restored-my-repo
cd restored-my-repo
git branch -a
```

Manual restore from archive repo snapshot:

```bash
gh repo clone <owner>/gh-manager-archive
cd gh-manager-archive/archives/<timestamp>/bundles
git clone alice__my-repo.bundle restored-my-repo
```

Create a new GitHub repo and push restored history:

```bash
gh repo create <owner>/<new-repo> --private --confirm
cd restored-my-repo
git remote add origin git@github.com:<owner>/<new-repo>.git
git push --all origin
git push --tags origin
```

## Delete Workflow

CLI delete with prompt:

```bash
gh-manager delete --repo pabumake/reppy
```

Skip delete warning prompt:

```bash
gh-manager delete --repo pabumake/reppy --force
```

In TUI, use `Delete` from the commands pane, then type the exact repo name in the danger popup to confirm deletion.

## Safety Model

- Plan files are signed with HMAC-SHA256 using `~/.config/gh-manager/secret.hex`.
- `execute` validates plan fingerprint, signature, actor, and host before deletion.
- Every repo is `git clone --mirror` backed up before delete.
- `backup` creates local browsable snapshots and `.bundle` artifacts, and can publish bundles to a private archive repo.
- Archive publishing is size-aware: oversized bundles are moved to a local skip folder and reported instead of failing the full archive push.
- Deletion is skipped when backup fails.
- Execution status is persisted in `<backup-root>/manifest.json`.
- Resume is supported; already deleted repos are skipped.

## Artifacts Reference

Default backup root:

```text
~/gh-manager-archive-YYYY-MM-DD-HHMMSS
```

Manifest path:

```text
<backup-root>/manifest.json
```

Bundle artifact path pattern:

```text
<backup-root>/bundles/<owner>__<repo>.bundle
```

Archive size-skip folder:

```text
<backup-root>/archive-skipped-size/
```

Browsable snapshot path pattern:

```text
<backup-root>/snapshots/<owner>__<repo>/
```

## Dry Run

Preview backup operations without side effects:

```bash
gh-manager backup --plan plan.json --dry-run
```

Preview delete operations without side effects:

```bash
gh-manager execute --plan plan.json --dry-run
```

## Troubleshooting / Notes

- Scope is user repositories only in v1.
- Org repository deletion is intentionally out of scope.
- TUI visibility uses Nerd Font glyphs (`` private, `` public). If glyphs render incorrectly, set your terminal font to `HackNerdFontMono-Regular.ttf`.
- Third-party font license is included at `third_party/fonts/hack-nerd-font/LICENSE.md`.
- Restore source preference is bundle-first, then snapshot fallback.
- If installer theme setup fails due to network/API limits, rerun:
  - `gh-manager theme install catppuccin-mocha`
  - `gh-manager theme apply catppuccin-mocha`
- To roll back to built-in styling:
  - `gh-manager theme apply default`
- Update checks use GitHub Releases API and may fail under API/network restrictions; use Settings -> Update -> Check now to retry.
