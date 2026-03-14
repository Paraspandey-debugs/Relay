package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
	"github.com/Paraspandey-debugs/Relay/internal/tui"
)

func main() {
	statePath := flag.String("state", "relay-downloads.state.json", "path to manager state file")
	concurrency := flag.Int("concurrency", 3, "max number of concurrent downloads")
	theme := flag.String("theme", "ocean", "tui theme: ocean|sunset|mono")
	cleanupOnRemove := flag.Bool("cleanup", true, "remove partial files when deleting a download")
	defaultWorkers := flag.Int("workers", 0, "default parallel workers for newly added downloads (0 uses downloader defaults)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mgr, err := manager.New(manager.Config{
		MaxConcurrent: *concurrency,
		StatePath:     *statePath,
		EventBuffer:   512,
		AutoStart:     true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create manager: %v\n", err)
		os.Exit(1)
	}

	defaultOpts := download.Options{}
	if *defaultWorkers > 0 {
		defaultOpts.Workers = *defaultWorkers
	}

	if err := tui.Run(
		ctx,
		mgr,
		tui.WithTheme(*theme),
		tui.WithCleanupOnRemove(*cleanupOnRemove),
		tui.WithDefaultAddOptions(defaultOpts),
	); err != nil {
		fmt.Fprintf(os.Stderr, "tui exited with error: %v\n", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mgr.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "manager shutdown error: %v\n", err)
	}
}
