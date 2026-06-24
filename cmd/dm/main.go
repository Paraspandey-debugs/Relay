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
	refreshMS := flag.Int("refresh-ms", 250, "tui refresh interval in milliseconds")
	cleanupOnRemove := flag.Bool("cleanup", true, "remove partial files when deleting a download")
	defaultWorkers := flag.Int("workers", 0, "default parallel workers for newly added downloads (0 uses downloader defaults)")
	colorBackground := flag.String("color-background", "", "override background color (hex or ANSI color)")
	colorForeground := flag.String("color-foreground", "", "override foreground color (hex or ANSI color)")
	colorAccent := flag.String("color-accent", "", "override accent color (hex or ANSI color)")
	colorSecondary := flag.String("color-secondary", "", "override secondary color (hex or ANSI color)")
	colorSuccess := flag.String("color-success", "", "override success color (hex or ANSI color)")
	colorWarning := flag.String("color-warning", "", "override warning color (hex or ANSI color)")
	colorError := flag.String("color-error", "", "override error color (hex or ANSI color)")
	colorMuted := flag.String("color-muted", "", "override muted color (hex or ANSI color)")
	colorHeader := flag.String("color-header", "", "override header color (hex or ANSI color)")
	colorCard := flag.String("color-card", "", "override card background color (hex or ANSI color)")
	colorSelectedCard := flag.String("color-selected-card", "", "override selected card color (hex or ANSI color)")
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

	themeOverrides := map[string]string{
		"background":    *colorBackground,
		"foreground":    *colorForeground,
		"accent":        *colorAccent,
		"secondary":     *colorSecondary,
		"success":       *colorSuccess,
		"warning":       *colorWarning,
		"error":         *colorError,
		"muted":         *colorMuted,
		"header":        *colorHeader,
		"card":          *colorCard,
		"selected-card": *colorSelectedCard,
	}

	if err := tui.Run(
		ctx,
		mgr,
		tui.WithTheme(*theme),
		tui.WithThemeOverrides(themeOverrides),
		tui.WithTickEvery(time.Duration(*refreshMS)*time.Millisecond),
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
