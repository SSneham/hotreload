package main

import (
	"context"
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

	runBuildAndMaybeRestart := func() {
		logger.Info("running build", "command", buildCmd)
		if err := builder.RunBuild(buildCmd); err != nil {
			logger.Error("build failed", "error", err)
			return
		}

		logger.Info("build succeeded")
		if err := server.Stop(); err != nil {
			logger.Error("failed to stop previous server", "error", err)
			return
		}
		if err := server.Start(); err != nil {
			logger.Error("failed to restart server", "error", err)
		}
	}

	// Requirement: build immediately on startup.
	runBuildAndMaybeRestart()

	fsWatcher, err := watcher.NewWatcher(rootDir)
	if err != nil {
		logger.Error("failed to initialize watcher", "error", err)
		os.Exit(1)
	}
	if err := fsWatcher.Start(); err != nil {
		logger.Error("failed to start watcher", "error", err)
		os.Exit(1)
	}

	debounced := debounce.Debounce(fsWatcher.Events(), 500*time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			if err := server.Stop(); err != nil {
				logger.Error("failed to stop server", "error", err)
			}
			return
		case <-debounced:
			logger.Info("detected file changes; rebuilding")
			runBuildAndMaybeRestart()
		}
	}
}
