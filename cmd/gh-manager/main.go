package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gh-manager/internal/app"
	"gh-manager/internal/backup"
	"gh-manager/internal/doctor"
	"gh-manager/internal/executor"
	"gh-manager/internal/github"
	"gh-manager/internal/manifest"
	"gh-manager/internal/planfile"
	"gh-manager/internal/tui"
	"gh-manager/internal/version"
)

func main() {
	ctx := context.Background()
	runner := app.ExecRunner{}
	gh := github.NewClient(runner)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
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
	default:
		usage()
		os.Exit(2)
	}
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
	selected, err := tui.SelectRepos(repos)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return errors.New("no repositories selected")
	}
	configDir, err := app.ConfigDir()
	if err != nil {
		return err
	}
	secret, err := planfile.EnsureSecret(configDir)
	if err != nil {
		return err
	}
	plan := planfile.New(actor, "github.com", version.Value, selected, time.Now())
	if err := plan.Sign(secret); err != nil {
		return err
	}
	planPath := *out
	if planPath == "" {
		planPath = filepath.Join(".", "deletion-plan-"+time.Now().Format("20060102-150405")+".json")
	}
	if err := planfile.Write(planPath, plan); err != nil {
		return err
	}
	fmt.Printf("plan saved: %s (%d repos)\n", planPath, plan.Count)
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
	p, err := planfile.Read(*planPath)
	if err != nil {
		return err
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
	fmt.Printf("schemaVersion: %s\n", p.SchemaVersion)
	fmt.Printf("createdAt: %s\n", p.CreatedAt)
	fmt.Printf("actor: %s\n", p.Actor)
	fmt.Printf("host: %s\n", p.Host)
	fmt.Printf("count: %d\n", p.Count)
	fmt.Printf("fingerprint: %s\n", p.Fingerprint)
	fmt.Printf("signature: %s\n", verification)
	fmt.Println("backupMode: supported (use `gh-manager backup`)")
	fmt.Println("repos:")
	for _, r := range p.Repos {
		fmt.Printf("- %s (%s)\n", r.FullName, truncateInspect(r.Description, 60))
	}
	if *manifestPath != "" {
		m, err := manifest.Read(*manifestPath)
		if err != nil {
			return err
		}
		fmt.Printf("manifestMode: %s\n", m.Mode)
		fmt.Printf("archiveRepo: %s\n", m.ArchiveRepo)
		fmt.Printf("archiveBranch: %s\n", m.ArchiveBranch)
	}
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
	if *planPath == "" {
		return errors.New("--plan is required")
	}
	if err := doctor.Check(ctx, runner); err != nil {
		return err
	}
	configDir, err := app.ConfigDir()
	if err != nil {
		return err
	}
	secret, err := planfile.EnsureSecret(configDir)
	if err != nil {
		return err
	}
	p, err := planfile.Read(*planPath)
	if err != nil {
		return err
	}
	if err := p.Validate(secret); err != nil {
		return fmt.Errorf("plan validation failed: %w", err)
	}
	actor, err := gh.CurrentUser(ctx)
	if err != nil {
		return err
	}
	if actor != p.Actor {
		return fmt.Errorf("actor mismatch: plan=%s current=%s", p.Actor, actor)
	}
	if p.Host != "github.com" {
		return fmt.Errorf("unsupported host: %s", p.Host)
	}

	exec := executor.Executor{
		GH:     gh,
		Backup: backup.NewService(runner),
		Now:    time.Now,
		In:     os.Stdin,
		Out:    os.Stdout,
	}
	resolvedBackupDir, err := resolveBackupLocation(*backupDir, *backupLocation)
	if err != nil {
		return err
	}
	res, err := exec.Execute(ctx, executor.Config{
		PlanPath:  *planPath,
		Resume:    *resume,
		BackupDir: resolvedBackupDir,
		Mode:      executor.ModeDelete,
		DryRun:    *dryRun,
	}, p)
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Println("execution dry-run complete")
	}
	fmt.Printf("execution complete: deleted=%d failed=%d total=%d\n", res.Deleted, res.Failed, res.Total)
	fmt.Printf("backup root: %s\n", res.BackupRoot)
	fmt.Printf("manifest: %s\n", res.ManifestPath)
	return nil
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
	if *planPath == "" {
		return errors.New("--plan is required")
	}
	if err := doctor.Check(ctx, runner); err != nil {
		return err
	}
	configDir, err := app.ConfigDir()
	if err != nil {
		return err
	}
	secret, err := planfile.EnsureSecret(configDir)
	if err != nil {
		return err
	}
	p, err := planfile.Read(*planPath)
	if err != nil {
		return err
	}
	if err := p.Validate(secret); err != nil {
		return fmt.Errorf("plan validation failed: %w", err)
	}
	actor, err := gh.CurrentUser(ctx)
	if err != nil {
		return err
	}
	if actor != p.Actor {
		return fmt.Errorf("actor mismatch: plan=%s current=%s", p.Actor, actor)
	}
	if p.Host != "github.com" {
		return fmt.Errorf("unsupported host: %s", p.Host)
	}

	exec := executor.Executor{
		RepoMgr: gh,
		Backup:  backup.NewService(runner),
		Archive: backup.NewArchiveService(runner),
		Now:     time.Now,
		In:      os.Stdin,
		Out:     os.Stdout,
	}
	resolvedBackupDir, err := resolveBackupLocation(*backupDir, *backupLocation)
	if err != nil {
		return err
	}
	res, err := exec.Execute(ctx, executor.Config{
		PlanPath:          *planPath,
		Resume:            *resume,
		BackupDir:         resolvedBackupDir,
		Mode:              executor.ModeBackup,
		DryRun:            *dryRun,
		ArchiveRepo:       *archiveRepo,
		ArchiveBranch:     *archiveBranch,
		ArchiveVisibility: *archiveVisibility,
		NoArchive:         *noArchive,
	}, p)
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Println("backup dry-run complete")
	}
	fmt.Printf("backup complete: local_failed=%d archive_failed=%d archive_skipped_size=%d total=%d\n", res.Failed, res.ArchiveFailed, res.ArchiveSkippedSize, res.Total)
	fmt.Printf("backup root: %s\n", res.BackupRoot)
	fmt.Printf("manifest: %s\n", res.ManifestPath)
	if res.ArchiveSkippedSize > 0 {
		fmt.Printf("archive skipped folder: %s\n", filepath.Join(res.BackupRoot, "archive-skipped-size"))
		fmt.Println("archive skipped repos:")
		for _, repo := range res.ArchiveSkippedRepos {
			fmt.Printf("- %s\n", repo)
		}
	}
	if !*noArchive {
		fmt.Printf("archive repo: %s\n", res.ArchiveRepo)
		fmt.Printf("archive branch: %s\n", res.ArchiveBranch)
		if res.ArchiveCommit != "" {
			fmt.Printf("archive commit: %s\n", res.ArchiveCommit)
		}
	}
	return nil
}

func usage() {
	fmt.Println("gh-manager <command>")
	fmt.Println("Commands: plan, backup, execute, inspect, doctor, version")
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
