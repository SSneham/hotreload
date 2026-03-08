package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func RunBuild(command string) error {
	return RunBuildContext(context.Background(), command)
}

func RunBuildContext(ctx context.Context, command string) error {
	const maxAttempts = 3

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cmd := shellCommand(ctx, command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			lastErr = err
			if !isTransientWindowsBuildFailure(err) || attempt == maxAttempts {
				return fmt.Errorf("build command failed: %w", err)
			}

			// Defender/file-lock races on Windows are often transient.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
			continue
		}

		return nil
	}

	return fmt.Errorf("build command failed: %w", lastErr)
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func isTransientWindowsBuildFailure(err error) bool {
	if runtime.GOOS != "windows" || err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "contains a virus or potentially unwanted software") ||
		strings.Contains(msg, "access is denied")
}
