package process

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

const (
	crashWindow  = 1 * time.Second
	restartDelay = 3 * time.Second
)

type ProcessManager struct {
	command string

	mu sync.Mutex

	cmd  *exec.Cmd
	done chan struct{}

	restartBlockedUntil time.Time
	stopRequestedPID    int
}

func NewProcessManager(cmd string) *ProcessManager {
	return &ProcessManager{
		command: cmd,
	}
}

func (p *ProcessManager) Start() error {
	for {
		p.mu.Lock()
		if p.cmd != nil {
			p.mu.Unlock()
			return fmt.Errorf("process already running")
		}

		wait := time.Until(p.restartBlockedUntil)
		if wait > 0 {
			p.mu.Unlock()
			time.Sleep(wait)
			continue
		}

		cmd := shellCommand(p.command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		configureCmd(cmd)

		startedAt := time.Now()
		if err := cmd.Start(); err != nil {
			p.mu.Unlock()
			return fmt.Errorf("failed to start process: %w", err)
		}

		p.cmd = cmd
		p.done = make(chan struct{})
		done := p.done
		p.mu.Unlock()

		go p.waitForExit(cmd, done, startedAt)
		return nil
	}
}

func (p *ProcessManager) waitForExit(cmd *exec.Cmd, done chan struct{}, startedAt time.Time) {
	_ = cmd.Wait()
	uptime := time.Since(startedAt)

	p.mu.Lock()
	defer p.mu.Unlock()

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	intentional := pid != 0 && p.stopRequestedPID == pid
	if intentional {
		p.stopRequestedPID = 0
	}

	if p.cmd == cmd {
		p.cmd = nil
		p.done = nil
	}

	if !intentional && uptime < crashWindow {
		p.restartBlockedUntil = time.Now().Add(restartDelay)
	}

	close(done)
}

func (p *ProcessManager) Stop() error {
	p.mu.Lock()
	cmd := p.cmd
	done := p.done
	if cmd != nil && cmd.Process != nil {
		p.stopRequestedPID = cmd.Process.Pid
	}
	p.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}

	killErr := killProcessTree(cmd.Process.Pid)

	// Ensure the process is reaped to avoid zombies/orphans.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}

	if killErr != nil {
		return killErr
	}
	return nil
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}
