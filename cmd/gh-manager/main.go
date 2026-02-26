package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gh-manager/internal/app"
	"gh-manager/internal/backup"
	configpkg "gh-manager/internal/config"
	"gh-manager/internal/doctor"
	"gh-manager/internal/executor"
	"gh-manager/internal/github"
	"gh-manager/internal/manifest"
	"gh-manager/internal/planfile"
	"gh-manager/internal/restore"
	themepkg "gh-manager/internal/theme"
	"gh-manager/internal/tui"
	"gh-manager/internal/version"
)

func main() {
	ctx := context.Background()
	runner := app.ExecRunner{}
	gh := github.NewClient(runner)

	if len(os.Args) < 2 {
		if err := runApp(ctx, gh, runner); err != nil {
			fatal(err)
		}
		return
	}

	switch os.Args[1] {
	case "doctor":
		if err := doctor.Check(ctx, runner); err != nil {
			fatal(err)
		}
		fmt.Println("doctor: ok")
	case "version":
		fmt.Println(version.Value)
	case "plan":
		if err := runPlan(ctx, gh, runner, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "inspect":
		if err := runInspect(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "execute":
		if err := runExecute(ctx, gh, runner, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "backup":
		if err := runBackup(ctx, gh, runner, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "restore":
		if err := runRestore(ctx, gh, runner, os.Args[2:]); err != nil {
			fatal(err)
		}
	case "delete":
		if err := runDelete(ctx, gh, runner, os.Args[2:], os.Stdin, os.Stdout); err != nil {
			fatal(err)
		}
	case "theme":
		if err := runTheme(ctx, os.Args[2:], os.Stdout); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runApp(ctx context.Context, gh github.Client, runner app.CommandRunner) error {
	if err := doctor.Check(ctx, runner); err != nil {
		return err
	}
	actor, err := gh.CurrentUser(ctx)
	if err != nil {
		return err
	}
	repos, err := gh.ListUserRepos(ctx, actor)
	if err != nil {
		return err
	}
	uiTheme := resolveUITheme(os.Stderr)
	return tui.RunApp(repos, tui.AppCallbacks{
		Version:                  version.Value,
		Theme:                    uiTheme,
		RestoreDefaultOwner:      actor,
		RestoreDefaultArchiveDir: preferredRestoreArchiveDir(),
		ThemeCurrent: func() (string, error) {
			return themeCurrentLabel()
		},
		ThemeListLocal: func() ([]string, string, error) {
			return themeListLocal()
		},
		ThemeListRemote: func() ([]tui.ThemeOption, string, error) {
			return themeListRemote(ctx)
		},
		ThemeInstall: func(id string) (string, error) {
			return themeInstall(ctx, id)
		},
		ThemeApply: func(id string) (tui.UITheme, string, error) {
			return themeApply(id)
		},
		ThemeUninstall: func(id string) (tui.UITheme, string, error) {
			return themeUninstall(id)
		},
		RefreshRepos: func() ([]planfile.RepoRecord, error) {
			return gh.ListUserRepos(ctx, actor)
		},
		Plan: func(selected []planfile.RepoRecord, outPath string) (string, error) {
			planPath, count, err := createSignedPlan(actor, selected, outPath, time.Now())
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("plan saved: %s (%d repos)", planPath, count), nil
		},
		Inspect: func(planPath string) (string, error) {
			return inspectToString(planPath, "")
		},
		Backup: func(planPath, backupLocation string, dryRun bool, confirmation string, selected []planfile.RepoRecord) (string, error) {
			var out bytes.Buffer
			resolvedPlanPath := strings.TrimSpace(planPath)
			if resolvedPlanPath == "" {
				p, _, err := createSignedPlan(actor, selected, "", time.Now())
				if err != nil {
					return "", err
				}
				resolvedPlanPath = p
				fmt.Fprintf(&out, "auto-generated plan: %s (%d repos)\n", resolvedPlanPath, len(selected))
			}
			err := runBackupTask(ctx, gh, runner, backupConfig{
				PlanPath:       resolvedPlanPath,
				BackupLocation: backupLocation,
				Resume:         true,
				DryRun:         dryRun,
				Confirmation:   confirmation,
			}, strings.NewReader(confirmation+"\n"), &out)
			return out.String(), err
		},
		Execute: func(planPath, backupLocation string, dryRun bool, confirmation string, selected []planfile.RepoRecord) (string, error) {
			var out bytes.Buffer
			resolvedPlanPath := strings.TrimSpace(planPath)
			if resolvedPlanPath == "" {
				p, _, err := createSignedPlan(actor, selected, "", time.Now())
				if err != nil {
					return "", err
				}
				resolvedPlanPath = p
				fmt.Fprintf(&out, "auto-generated plan: %s (%d repos)\n", resolvedPlanPath, len(selected))
			}
			err := runExecuteTask(ctx, gh, runner, executeConfig{
				PlanPath:       resolvedPlanPath,
				BackupLocation: backupLocation,
				Resume:         true,
				DryRun:         dryRun,
				Confirmation:   confirmation,
			}, strings.NewReader(confirmation+"\n"), &out)
			return out.String(), err
		},
		Restore: func(req tui.RestoreRequest) (string, error) {
			svc := restore.NewService(runner)
			res, err := svc.Restore(ctx, restore.Request{
				ArchiveRoot:      req.ArchiveRoot,
				RepoFullName:     req.RepoFullName,
				SourceKind:       req.SourceKind,
				SourcePath:       req.SourcePath,
				TargetOwner:      req.TargetOwner,
				TargetName:       req.TargetName,
				TargetVisibility: req.TargetVisibility,
			})
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("restore complete: %s from %s (%s)\nworkdir: %s", res.TargetFullName, res.SourcePath, res.SourceKind, res.WorkDir), nil
		},
		Delete: func(repo planfile.RepoRecord) (string, error) {
			if strings.TrimSpace(repo.FullName) == "" {
				return "", errors.New("repository full name is empty")
			}
			if err := gh.DeleteRepo(ctx, repo.FullName); err != nil {
				return "", err
			}
			return fmt.Sprintf("delete complete: %s", repo.FullName), nil
		},
	})
}

func runPlan(ctx context.Context, gh github.Client, runner app.CommandRunner, args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	owner := fs.String("owner", "", "GitHub owner (defaults to authenticated user)")
	out := fs.String("out", "", "Output plan file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := doctor.Check(ctx, runner); err != nil {
		return err
	}
	actor, err := gh.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("fetch current user: %w", err)
	}
	repos, err := gh.ListUserRepos(ctx, *owner)
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}
	selected, err := tui.SelectReposWithTheme(repos, resolveUITheme(os.Stderr))
	if err != nil {
		return err
	}
	planPath, count, err := createSignedPlan(actor, selected, *out, time.Now())
	if err != nil {
		return err
	}
	fmt.Printf("plan saved: %s (%d repos)\n", planPath, count)
	return nil
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	planPath := fs.String("plan", "", "Path to plan file")
	manifestPath := fs.String("manifest", "", "Optional manifest path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *planPath == "" {
		return errors.New("--plan is required")
	}
	out, err := inspectToString(*planPath, *manifestPath)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

func runExecute(ctx context.Context, gh github.Client, runner app.CommandRunner, args []string) error {
	fs := flag.NewFlagSet("execute", flag.ContinueOnError)
	planPath := fs.String("plan", "", "Path to plan file")
	backupDir := fs.String("backup-dir", "", "Override backup directory (deprecated: use --backup-location)")
	backupLocation := fs.String("backup-location", "", "Override backup location")
	resume := fs.Bool("resume", true, "Resume from existing manifest if available")
	dryRun := fs.Bool("dry-run", false, "Show actions without making changes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runExecuteTask(ctx, gh, runner, executeConfig{
		PlanPath:       *planPath,
		BackupDir:      *backupDir,
		BackupLocation: *backupLocation,
		Resume:         *resume,
		DryRun:         *dryRun,
	}, os.Stdin, os.Stdout)
}

func runBackup(ctx context.Context, gh github.Client, runner app.CommandRunner, args []string) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	planPath := fs.String("plan", "", "Path to plan file")
	backupDir := fs.String("backup-dir", "", "Override backup directory (deprecated: use --backup-location)")
	backupLocation := fs.String("backup-location", "", "Override backup location")
	resume := fs.Bool("resume", true, "Resume from existing manifest if available")
	dryRun := fs.Bool("dry-run", false, "Show actions without making changes")
	archiveRepo := fs.String("archive-repo", "", "Archive repository (owner/name)")
	archiveBranch := fs.String("archive-branch", "main", "Archive branch name")
	archiveVisibility := fs.String("archive-visibility", "private", "Archive repo visibility: private|public")
	noArchive := fs.Bool("no-archive", false, "Disable archive publishing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runBackupTask(ctx, gh, runner, backupConfig{
		PlanPath:          *planPath,
		BackupDir:         *backupDir,
		BackupLocation:    *backupLocation,
		Resume:            *resume,
		DryRun:            *dryRun,
		ArchiveRepo:       *archiveRepo,
		ArchiveBranch:     *archiveBranch,
		ArchiveVisibility: *archiveVisibility,
		NoArchive:         *noArchive,
	}, os.Stdin, os.Stdout)
}

func runRestore(ctx context.Context, gh github.Client, runner app.CommandRunner, args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	archiveRoot := fs.String("archive-root", "", "Archive root folder")
	repoName := fs.String("repo", "", "Source full repo name (owner/name) from archive")
	targetOwner := fs.String("target-owner", "", "Target owner (defaults to authenticated user)")
	targetName := fs.String("target-name", "", "Target repository name (defaults to source name)")
	visibility := fs.String("visibility", "private", "Target visibility: private|public")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*archiveRoot) == "" || strings.TrimSpace(*repoName) == "" {
		return errors.New("--archive-root and --repo are required")
	}
	owner := strings.TrimSpace(*targetOwner)
	if owner == "" {
		u, err := gh.CurrentUser(ctx)
		if err != nil {
			return err
		}
		owner = u
	}
	name := strings.TrimSpace(*targetName)
	if name == "" {
		name = repoBasename(*repoName)
	}

	entries, err := restore.LoadIndex(*archiveRoot)
	if err != nil {
		return err
	}
	var selected restore.ArchiveEntry
	found := false
	for _, e := range entries {
		if e.FullName == *repoName {
			selected = e
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("repo not found in archive index: %s", *repoName)
	}
	src, ok := restore.PreferredSource(selected)
	if !ok {
		return fmt.Errorf("repo has no valid restore source: %s", *repoName)
	}

	svc := restore.NewService(runner)
	res, err := svc.Restore(ctx, restore.Request{
		ArchiveRoot:      *archiveRoot,
		RepoFullName:     selected.FullName,
		SourceKind:       src.Kind,
		SourcePath:       src.Path,
		TargetOwner:      owner,
		TargetName:       name,
		TargetVisibility: *visibility,
	})
	if err != nil {
		return err
	}
	fmt.Printf("restore complete: %s from %s (%s)\n", res.TargetFullName, res.SourcePath, res.SourceKind)
	fmt.Printf("workdir: %s\n", res.WorkDir)
	return nil
}

func runDelete(ctx context.Context, gh github.Client, runner app.CommandRunner, args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	repo := fs.String("repo", "", "Repository full name (owner/name)")
	force := fs.Bool("force", false, "Skip warning prompt and delete immediately")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fullName := strings.TrimSpace(*repo)
	if fullName == "" {
		return errors.New("--repo is required")
	}
	if err := doctor.Check(ctx, runner); err != nil {
		return err
	}
	if !*force {
		base := repoBasename(fullName)
		fmt.Fprintf(out, "WARNING: deleting %s without backup can permanently lose data.\n", fullName)
		fmt.Fprintf(out, "Type %q to confirm delete: ", base)
		var typed string
		if _, err := fmt.Fscanln(in, &typed); err != nil {
			return fmt.Errorf("read confirmation: %w", err)
		}
		if strings.TrimSpace(typed) != base {
			return errors.New("confirmation mismatch; delete canceled")
		}
	}
	if err := gh.DeleteRepo(ctx, fullName); err != nil {
		return err
	}
	fmt.Fprintf(out, "delete complete: %s\n", fullName)
	return nil
}

func usage() {
	fmt.Println("gh-manager")
	fmt.Println("Runs interactive TUI when no command is provided.")
	fmt.Println("gh-manager <command>")
	fmt.Println("Commands: plan, backup, execute, restore, delete, theme, inspect, doctor, version")
}

func resolveUITheme(w io.Writer) tui.UITheme {
	cfg, err := configpkg.Load()
	if err != nil {
		fmt.Fprintf(w, "warning: loading config failed, using default theme: %v\n", err)
		return tui.UITheme{}
	}
	palette, _, err := themepkg.LoadActivePaletteHex(cfg)
	if err != nil {
		fmt.Fprintf(w, "warning: loading theme %q failed, using default: %v\n", cfg.Theme.Active, err)
		palette = themepkg.DefaultPaletteHex()
	}
	resolved := themepkg.ResolveForTerminal(palette, themepkg.DetectTrueColor())
	return resolvedToUITheme(resolved)
}

func runTheme(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("theme subcommand required: list, current, apply, install, uninstall")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("theme list", flag.ContinueOnError)
		remote := fs.Bool("remote", false, "List installable remote themes from index")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		cfg, err := configpkg.Load()
		if err != nil {
			return err
		}
		if *remote {
			remoteThemes, sourceURL, err := themeListRemote(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "installable themes (%s):\n", sourceURL)
			for _, th := range remoteThemes {
				fmt.Fprintf(out, "- %s: %s\n", th.ID, th.Name)
			}
			return nil
		}
		ids, active, err := themeListLocal()
		if err != nil {
			return err
		}
		_ = cfg
		fmt.Fprintf(out, "local themes (active: %s):\n", active)
		fmt.Fprintln(out, "- default")
		for _, id := range ids {
			prefix := "-"
			if id == active {
				prefix = "*"
			}
			fmt.Fprintf(out, "%s %s\n", prefix, id)
		}
		return nil
	case "current":
		label, err := themeCurrentLabel()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\n", label)
		return nil
	case "apply":
		if len(args) < 2 {
			return errors.New("usage: gh-manager theme apply <theme-id|default>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("theme id is required")
		}
		_, msg, err := themeApply(id)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\n", msg)
		return nil
	case "install":
		if len(args) < 2 {
			return errors.New("usage: gh-manager theme install <theme-id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("theme id is required")
		}
		msg, err := themeInstall(ctx, id)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\n", msg)
		return nil
	case "uninstall":
		if len(args) < 2 {
			return errors.New("usage: gh-manager theme uninstall <theme-id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("theme id is required")
		}
		_, msg, err := themeUninstall(id)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\n", msg)
		return nil
	default:
		return fmt.Errorf("unknown theme subcommand: %s", args[0])
	}
}

func themeCurrentLabel() (string, error) {
	cfg, err := configpkg.Load()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("active theme: %s", cfg.Theme.Active), nil
}

func themeListLocal() ([]string, string, error) {
	cfg, err := configpkg.Load()
	if err != nil {
		return nil, "", err
	}
	ids, err := themepkg.ListLocalThemeIDs()
	if err != nil {
		return nil, "", err
	}
	return ids, cfg.Theme.Active, nil
}

func themeListRemote(ctx context.Context) ([]tui.ThemeOption, string, error) {
	cfg, err := configpkg.Load()
	if err != nil {
		return nil, "", err
	}
	idx, sourceURL, err := fetchThemeIndexWithLocalFallback(ctx, cfg.Theme.IndexURL)
	if err != nil {
		return nil, "", err
	}
	out := make([]tui.ThemeOption, 0, len(idx.Themes))
	for _, th := range idx.Themes {
		out = append(out, tui.ThemeOption{ID: th.ID, Name: th.Name, Description: th.Description})
	}
	return out, sourceURL, nil
}

func themeInstall(ctx context.Context, id string) (string, error) {
	cfg, err := configpkg.Load()
	if err != nil {
		return "", err
	}
	_, sourceURL, err := fetchThemeIndexWithLocalFallback(ctx, cfg.Theme.IndexURL)
	if err != nil {
		return "", err
	}
	themeFile, err := themepkg.FetchThemeByID(ctx, sourceURL, id)
	if err != nil {
		return "", err
	}
	if err := themepkg.SaveThemeFile(themeFile); err != nil {
		return "", err
	}
	return fmt.Sprintf("installed theme: %s", themeFile.ID), nil
}

func themeApply(id string) (tui.UITheme, string, error) {
	cfg, err := configpkg.Load()
	if err != nil {
		return tui.UITheme{}, "", err
	}
	if id != "default" {
		themesDir, err := configpkg.ThemesDir()
		if err != nil {
			return tui.UITheme{}, "", err
		}
		b, err := os.ReadFile(filepath.Join(themesDir, id+".json"))
		if err != nil {
			return tui.UITheme{}, "", fmt.Errorf("theme not installed: %s", id)
		}
		if _, err := themepkg.ParseThemeFile(b); err != nil {
			return tui.UITheme{}, "", fmt.Errorf("invalid installed theme %s: %w", id, err)
		}
	}
	cfg.Theme.Active = id
	if err := configpkg.Save(cfg); err != nil {
		return tui.UITheme{}, "", err
	}

	palette := themepkg.DefaultPaletteHex()
	if id != "default" {
		p, _, err := themepkg.LoadActivePaletteHex(cfg)
		if err != nil {
			return tui.UITheme{}, "", err
		}
		palette = p
	}
	resolved := themepkg.ResolveForTerminal(palette, themepkg.DetectTrueColor())
	return resolvedToUITheme(resolved), fmt.Sprintf("applied theme: %s", id), nil
}

func themeUninstall(id string) (tui.UITheme, string, error) {
	if id == "default" {
		return tui.UITheme{}, "", errors.New("cannot uninstall built-in theme: default")
	}
	cfg, err := configpkg.Load()
	if err != nil {
		return tui.UITheme{}, "", err
	}
	if err := themepkg.RemoveLocalTheme(id); err != nil {
		return tui.UITheme{}, "", err
	}
	msg := fmt.Sprintf("uninstalled theme: %s", id)
	if cfg.Theme.Active == id {
		uiTheme, appliedMsg, err := themeApply("default")
		if err != nil {
			return tui.UITheme{}, "", err
		}
		return uiTheme, msg + "; " + appliedMsg, nil
	}
	uiTheme, err := resolveCurrentUITheme()
	if err != nil {
		return tui.UITheme{}, "", err
	}
	return uiTheme, msg, nil
}

func resolveCurrentUITheme() (tui.UITheme, error) {
	cfg, err := configpkg.Load()
	if err != nil {
		return tui.UITheme{}, err
	}
	palette, _, err := themepkg.LoadActivePaletteHex(cfg)
	if err != nil {
		return tui.UITheme{}, err
	}
	resolved := themepkg.ResolveForTerminal(palette, themepkg.DetectTrueColor())
	return resolvedToUITheme(resolved), nil
}

func resolvedToUITheme(resolved themepkg.PaletteResolved) tui.UITheme {
	return tui.UITheme{
		PaneBorderActive:   resolved.PaneBorderActive,
		PaneBorderInactive: resolved.PaneBorderInactive,
		PopupBorder:        resolved.PopupBorder,
		PopupOuterBorder:   resolved.PopupOuterBorder,
		Danger:             resolved.Danger,
		DangerText:         resolved.DangerText,
		TextPrimary:        resolved.TextPrimary,
		TextMuted:          resolved.TextMuted,
		SelectionBg:        resolved.SelectionBg,
		SelectionFg:        resolved.SelectionFg,
		LogoLine1:          resolved.LogoLine1,
		LogoLine2:          resolved.LogoLine2,
		LogoLine3:          resolved.LogoLine3,
		LogoLine4:          resolved.LogoLine4,
		LogoLine5:          resolved.LogoLine5,
		LogoLine6:          resolved.LogoLine6,
		HeaderText:         resolved.HeaderText,
		HelpText:           resolved.HelpText,
		StatusText:         resolved.StatusText,
		TableHeader:        resolved.TableHeader,
		ColSel:             resolved.ColSel,
		ColName:            resolved.ColName,
		ColVisibility:      resolved.ColVisibility,
		ColFork:            resolved.ColFork,
		ColArchived:        resolved.ColArchived,
		ColUpdated:         resolved.ColUpdated,
		ColDescription:     resolved.ColDescription,
		DetailsLabel:       resolved.DetailsLabel,
		DetailsValue:       resolved.DetailsValue,
	}
}

func fetchThemeIndexWithLocalFallback(ctx context.Context, primaryURL string) (themepkg.ThemeIndex, string, error) {
	idx, err := themepkg.FetchIndex(ctx, primaryURL)
	if err == nil {
		return idx, primaryURL, nil
	}
	localPath := filepath.Join("themes", "index.json")
	localIdx, localErr := themepkg.FetchIndex(ctx, localPath)
	if localErr != nil {
		return themepkg.ThemeIndex{}, "", fmt.Errorf("fetch theme index failed (%s): %w", primaryURL, err)
	}
	return localIdx, localPath, nil
}

func preferredRestoreArchiveDir() string {
	candidates := restoreDocumentsCandidates()
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	if len(candidates) > 0 && strings.TrimSpace(candidates[0]) != "" {
		return candidates[0]
	}
	return "."
}

func restoreDocumentsCandidates() []string {
	home, _ := os.UserHomeDir()
	candidates := make([]string, 0, 4)

	if runtime.GOOS == "windows" {
		if p := strings.TrimSpace(os.Getenv("USERPROFILE")); p != "" {
			candidates = append(candidates, filepath.Join(p, "Documents"))
		}
		if p := strings.TrimSpace(os.Getenv("OneDrive")); p != "" {
			candidates = append(candidates, filepath.Join(p, "Documents"))
		}
		if p := strings.TrimSpace(os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")); p != "" {
			candidates = append(candidates, filepath.Join(p, "Documents"))
		}
	}
	if strings.TrimSpace(home) != "" {
		candidates = append(candidates, filepath.Join(home, "Documents"))
	}
	return candidates
}

func repoBasename(fullName string) string {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullName
}

type executeConfig struct {
	PlanPath       string
	BackupDir      string
	BackupLocation string
	Resume         bool
	DryRun         bool
	Confirmation   string
}

type backupConfig struct {
	PlanPath          string
	BackupDir         string
	BackupLocation    string
	Resume            bool
	DryRun            bool
	ArchiveRepo       string
	ArchiveBranch     string
	ArchiveVisibility string
	NoArchive         bool
	Confirmation      string
}

func createSignedPlan(actor string, selected []planfile.RepoRecord, outPath string, now time.Time) (string, int, error) {
	if len(selected) == 0 {
		return "", 0, errors.New("no repositories selected")
	}
	configDir, err := app.ConfigDir()
	if err != nil {
		return "", 0, err
	}
	secret, err := planfile.EnsureSecret(configDir)
	if err != nil {
		return "", 0, err
	}
	plan := planfile.New(actor, "github.com", version.Value, selected, now)
	if err := plan.Sign(secret); err != nil {
		return "", 0, err
	}
	planPath := outPath
	if planPath == "" {
		planPath = filepath.Join(".", "deletion-plan-"+now.Format("20060102-150405")+".json")
	}
	if err := planfile.Write(planPath, plan); err != nil {
		return "", 0, err
	}
	return planPath, plan.Count, nil
}

func inspectToString(planPath, manifestPath string) (string, error) {
	if strings.TrimSpace(planPath) == "" {
		return "", errors.New("--plan is required")
	}
	p, err := planfile.Read(planPath)
	if err != nil {
		return "", err
	}
	verification := "unknown"
	if configDir, cErr := app.ConfigDir(); cErr == nil {
		if secret, sErr := planfile.EnsureSecret(configDir); sErr == nil {
			if vErr := p.Validate(secret); vErr == nil {
				verification = "valid"
			} else {
				verification = "invalid (" + vErr.Error() + ")"
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "schemaVersion: %s\n", p.SchemaVersion)
	fmt.Fprintf(&b, "createdAt: %s\n", p.CreatedAt)
	fmt.Fprintf(&b, "actor: %s\n", p.Actor)
	fmt.Fprintf(&b, "host: %s\n", p.Host)
	fmt.Fprintf(&b, "count: %d\n", p.Count)
	fmt.Fprintf(&b, "fingerprint: %s\n", p.Fingerprint)
	fmt.Fprintf(&b, "signature: %s\n", verification)
	fmt.Fprintln(&b, "backupMode: supported (use `gh-manager backup`)")
	fmt.Fprintln(&b, "repos:")
	for _, r := range p.Repos {
		fmt.Fprintf(&b, "- %s (%s)\n", r.FullName, truncateInspect(r.Description, 60))
	}
	if manifestPath != "" {
		m, err := manifest.Read(manifestPath)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "manifestMode: %s\n", m.Mode)
		fmt.Fprintf(&b, "archiveRepo: %s\n", m.ArchiveRepo)
		fmt.Fprintf(&b, "archiveBranch: %s\n", m.ArchiveBranch)
	}
	return b.String(), nil
}

func validatePlanForExecution(ctx context.Context, gh github.Client, runner app.CommandRunner, planPath string) (planfile.DeletionPlanV1, error) {
	var p planfile.DeletionPlanV1
	if strings.TrimSpace(planPath) == "" {
		return p, errors.New("--plan is required")
	}
	if err := doctor.Check(ctx, runner); err != nil {
		return p, err
	}
	configDir, err := app.ConfigDir()
	if err != nil {
		return p, err
	}
	secret, err := planfile.EnsureSecret(configDir)
	if err != nil {
		return p, err
	}
	p, err = planfile.Read(planPath)
	if err != nil {
		return p, err
	}
	if err := p.Validate(secret); err != nil {
		return p, fmt.Errorf("plan validation failed: %w", err)
	}
	actor, err := gh.CurrentUser(ctx)
	if err != nil {
		return p, err
	}
	if actor != p.Actor {
		return p, fmt.Errorf("actor mismatch: plan=%s current=%s", p.Actor, actor)
	}
	if p.Host != "github.com" {
		return p, fmt.Errorf("unsupported host: %s", p.Host)
	}
	return p, nil
}

func runExecuteTask(ctx context.Context, gh github.Client, runner app.CommandRunner, cfg executeConfig, in io.Reader, out io.Writer) error {
	p, err := validatePlanForExecution(ctx, gh, runner, cfg.PlanPath)
	if err != nil {
		return err
	}
	resolvedBackupDir, err := resolveBackupLocation(cfg.BackupDir, cfg.BackupLocation)
	if err != nil {
		return err
	}
	if cfg.Confirmation != "" {
		in = strings.NewReader(cfg.Confirmation + "\n")
	}
	exec := executor.Executor{
		GH:     gh,
		Backup: backup.NewService(runner),
		Now:    time.Now,
		In:     in,
		Out:    out,
	}
	res, err := exec.Execute(ctx, executor.Config{
		PlanPath:  cfg.PlanPath,
		Resume:    cfg.Resume,
		BackupDir: resolvedBackupDir,
		Mode:      executor.ModeDelete,
		DryRun:    cfg.DryRun,
	}, p)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		fmt.Fprintln(out, "execution dry-run complete")
	}
	fmt.Fprintf(out, "execution complete: deleted=%d failed=%d total=%d\n", res.Deleted, res.Failed, res.Total)
	fmt.Fprintf(out, "backup root: %s\n", res.BackupRoot)
	fmt.Fprintf(out, "manifest: %s\n", res.ManifestPath)
	return nil
}

func runBackupTask(ctx context.Context, gh github.Client, runner app.CommandRunner, cfg backupConfig, in io.Reader, out io.Writer) error {
	p, err := validatePlanForExecution(ctx, gh, runner, cfg.PlanPath)
	if err != nil {
		return err
	}
	resolvedBackupDir, err := resolveBackupLocation(cfg.BackupDir, cfg.BackupLocation)
	if err != nil {
		return err
	}
	if cfg.Confirmation != "" {
		in = strings.NewReader(cfg.Confirmation + "\n")
	}
	exec := executor.Executor{
		RepoMgr: gh,
		Backup:  backup.NewService(runner),
		Archive: backup.NewArchiveService(runner),
		Now:     time.Now,
		In:      in,
		Out:     out,
	}
	res, err := exec.Execute(ctx, executor.Config{
		PlanPath:          cfg.PlanPath,
		Resume:            cfg.Resume,
		BackupDir:         resolvedBackupDir,
		Mode:              executor.ModeBackup,
		DryRun:            cfg.DryRun,
		ArchiveRepo:       cfg.ArchiveRepo,
		ArchiveBranch:     cfg.ArchiveBranch,
		ArchiveVisibility: cfg.ArchiveVisibility,
		NoArchive:         cfg.NoArchive,
	}, p)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		fmt.Fprintln(out, "backup dry-run complete")
	}
	fmt.Fprintf(out, "backup complete: local_failed=%d archive_failed=%d archive_skipped_size=%d total=%d\n", res.Failed, res.ArchiveFailed, res.ArchiveSkippedSize, res.Total)
	fmt.Fprintf(out, "backup root: %s\n", res.BackupRoot)
	fmt.Fprintf(out, "manifest: %s\n", res.ManifestPath)
	if res.ArchiveSkippedSize > 0 {
		fmt.Fprintf(out, "archive skipped folder: %s\n", filepath.Join(res.BackupRoot, "archive-skipped-size"))
		fmt.Fprintln(out, "archive skipped repos:")
		for _, repo := range res.ArchiveSkippedRepos {
			fmt.Fprintf(out, "- %s\n", repo)
		}
	}
	if !cfg.NoArchive {
		fmt.Fprintf(out, "archive repo: %s\n", res.ArchiveRepo)
		fmt.Fprintf(out, "archive branch: %s\n", res.ArchiveBranch)
		if res.ArchiveCommit != "" {
			fmt.Fprintf(out, "archive commit: %s\n", res.ArchiveCommit)
		}
	}
	return nil
}

func resolveBackupLocation(backupDir, backupLocation string) (string, error) {
	if backupDir != "" && backupLocation != "" && backupDir != backupLocation {
		return "", errors.New("use either --backup-location or --backup-dir, not both with different values")
	}
	if backupLocation != "" {
		return backupLocation, nil
	}
	return backupDir, nil
}

func truncateInspect(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "~"
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
