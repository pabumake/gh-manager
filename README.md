# gh-manager

`gh-manager` is a safety-first TUI for reviewing and deleting GitHub repositories with mandatory mirror backups.

## Requirements

- Linux shell environment
- `gh` authenticated (`gh auth login`)
- `git`
- SSH access to `git@github.com`

## Install / Build

```bash
go build -o gh-manager ./cmd/gh-manager
```

## Commands

- `gh-manager doctor`
- `gh-manager plan [--owner <user>] [--out <plan.json>]`
- `gh-manager backup --plan <plan.json> [--backup-location <dir>] [--resume=true|false] [--dry-run] [--archive-repo <owner/name>] [--archive-branch <branch>] [--archive-visibility private|public] [--no-archive]`
- `gh-manager inspect --plan <plan.json>`
- `gh-manager execute --plan <plan.json> [--backup-location <dir>] [--resume=true|false] [--dry-run]`
- `gh-manager version`

## Workflow

1. Run `gh-manager plan`.
2. In the TUI, filter/sort/select repositories and press `s` to save the signed plan.
3. Review with `gh-manager inspect --plan <plan.json>`.
4. Run `gh-manager backup --plan <plan.json>` to create mirror + bundle backups (optional archive publish).
5. Run `gh-manager execute --plan <plan.json>` and type the exact confirmation phrase for deletion.
6. For `backup` and `execute`, confirmation accepts either `ACCEPT` or `CONFIRM`.

## TUI Controls

- `j` / `k`: move cursor
- `space`: toggle selected repo
- `a`: select all currently filtered repos
- `x`: clear all currently filtered repos
- `tab`: cycle sort mode (name / updatedAt / visibility)
- `enter`: show/hide rounded details drawer
- `type`: append filter text
- `backspace`: remove filter text
- `s`: save plan and exit
- `q`: quit without saving

## Safety Model

- Plan files are signed with HMAC-SHA256 using `~/.config/gh-manager/secret.hex`.
- `execute` validates plan fingerprint, signature, actor, and host before deletion.
- Every repo is `git clone --mirror` backed up before delete.
- `backup` creates local browsable snapshots and `.bundle` artifacts, and can publish bundles to a private archive repo.
- Archive publishing is size-aware: oversized bundles are moved to a local skip folder and reported instead of failing the full archive push.
- Deletion is skipped when backup fails.
- Execution status is persisted in `<backup-root>/manifest.json`.
- Resume is supported; already deleted repos are skipped.

## Artifacts

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

## Restore From Bundle

Restore from a local bundle:

```bash
git clone /path/to/alice__my-repo.bundle restored-my-repo
cd restored-my-repo
git branch -a
```

Restore from archive repo snapshot:

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

## Dry Run

Preview backup operations without side effects:

```bash
gh-manager backup --plan plan.json --dry-run
```

Preview delete operations without side effects:

```bash
gh-manager execute --plan plan.json --dry-run
```

## Notes

- Scope is user repositories only in v1.
- Org repository deletion is intentionally out of scope.
