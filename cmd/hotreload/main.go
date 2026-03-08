package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hotreload/internal/builder"
	"hotreload/internal/debounce"
	"hotreload/internal/process"
	"hotreload/internal/watcher"
)

func main() {
	var (
		rootDir  string
		buildCmd string
		execCmd  string
	)

	flag.StringVar(&rootDir, "root", "", "directory to watch")
	flag.StringVar(&buildCmd, "build", "", "build command")
	flag.StringVar(&execCmd, "exec", "", "run command")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if rootDir == "" || buildCmd == "" || execCmd == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := process.NewProcessManager(execCmd)

	fsWatcher, err := watcher.NewWatcher(rootDir)
	if err != nil {
		logger.Error("failed to initialize watcher", "error", err)
		os.Exit(1)
	}
	if err := fsWatcher.Start(); err != nil {
		logger.Error("failed to start watcher", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := fsWatcher.Close(); err != nil {
			logger.Warn("failed to close watcher", "error", err)
		}
	}()

	debounced := debounce.Debounce(fsWatcher.Events(), 500*time.Millisecond)
	buildTriggers := make(chan struct{}, 1)
	buildDone := make(chan error, 1)

	queueBuild := func() {
		select {
		case buildTriggers <- struct{}{}:
		default:
		}
	}

	// Requirement: build immediately on startup.
	queueBuild()

	var (
		buildRunning bool
		buildQueued  bool
		buildCancel  context.CancelFunc
	)

	startBuild := func() {
		logger.Info("running build", "command", buildCmd)
		buildCtx, cancel := context.WithCancel(ctx)
		buildCancel = cancel
		buildRunning = true

		go func() {
			buildDone <- builder.RunBuildContext(buildCtx, buildCmd)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			if buildCancel != nil {
				buildCancel()
			}
			if err := server.Stop(); err != nil {
				logger.Error("failed to stop server", "error", err)
			}
			return
		case <-debounced:
			logger.Info("detected file changes")
			queueBuild()
		case <-buildTriggers:
			if buildRunning {
				buildQueued = true
				if buildCancel != nil {
					logger.Info("canceling in-progress build due to newer changes")
					buildCancel()
				}
				continue
			}
			startBuild()
		case err := <-buildDone:
			buildRunning = false

			if errors.Is(err, context.Canceled) {
				logger.Info("build canceled")
			} else if err != nil {
				logger.Error("build failed", "error", err)
				logger.Info("keeping current server process running")
			} else if buildQueued {
				logger.Info("build finished but newer changes are pending; skipping restart")
			} else {
				logger.Info("build succeeded")
				if stopErr := server.Stop(); stopErr != nil {
					logger.Error("failed to stop previous server", "error", stopErr)
				} else if startErr := server.Start(); startErr != nil {
					logger.Error("failed to restart server", "error", startErr)
				}
			}

			if buildQueued {
				buildQueued = false
				queueBuild()
			}
		}
	}
}
