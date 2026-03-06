//go:build !windows

package process

import (
	"fmt"
	"os/exec"
	"syscall"
)

func configureCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessTree(pid int) error {
	// Negative pid targets the full process group.
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to kill process group: %w", err)
	}
	return nil
}
