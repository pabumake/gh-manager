package doctor

import (
	"context"
	"fmt"
	"os/exec"

	"gh-manager/internal/app"
)

func Check(ctx context.Context, runner app.CommandRunner) error {
	for _, bin := range []string{"gh", "git"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("missing dependency %q in PATH", bin)
		}
	}
	if _, err := runner.Run(ctx, "gh", "auth", "status"); err != nil {
		return fmt.Errorf("gh auth status failed: %w", err)
	}
	return nil
}
