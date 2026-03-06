//go:build windows

package process

import (
	"fmt"
	"os/exec"
	"syscall"
)

func configureCmd(cmd *exec.Cmd) {
	const createNewProcessGroup = 0x00000200
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup,
	}
}

func killProcessTree(pid int) error {
	// /T kills child processes too, /F force kills.
	cmd := exec.Command("taskkill", "/PID", fmt.Sprintf("%d", pid), "/T", "/F")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("taskkill failed: %w", err)
	}
	return nil
}
