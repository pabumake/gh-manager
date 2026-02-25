package app

import (
	"os"
	"path/filepath"
	"time"
)

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gh-manager"), nil
}

func DefaultBackupRoot(now time.Time) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "gh-manager-archive-"+now.Format("2006-01-02-150405")), nil
}
