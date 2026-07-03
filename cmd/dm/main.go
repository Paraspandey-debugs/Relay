package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/kardianos/service"

	"github.com/Paraspandey-debugs/Relay/internal/api"
	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
	"github.com/Paraspandey-debugs/Relay/internal/tui"
)

// daemonConfigPath is the path to the daemon config JSON file.
// It can be overridden via the -daemon-config flag.
var daemonConfigPath = manager.DefaultConfigPath()

type program struct {
	exit           chan struct{}
	cancel         context.CancelFunc
	ctx            context.Context
	headless       bool
	openWeb        bool
	apiPort        int
	statePath      string
	logPath        string
	concurrency    int
	refreshMS      int
	cleanup        bool
	workers        int
	theme          string
	themeOverrides map[string]string
}

func (p *program) Start(s service.Service) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.exit = make(chan struct{})

	go p.run()
	return nil
}

func (p *program) run() {
	defer close(p.exit)

	// Load daemon config from file (or use defaults)
	dc, err := manager.LoadDaemonConfig(daemonConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load daemon config: %v\n", err)
		// Continue with flag-based defaults
		dc = &manager.DaemonConfig{
			Concurrency: p.concurrency,
			Workers:     p.workers,
			APIPort:     p.apiPort,
			StatePath:   p.statePath,
			LogPath:     p.logPath,
			Headless:    p.headless,
			OpenWeb:     p.openWeb,
			Theme:       p.theme,
			RefreshMS:   p.refreshMS,
			Cleanup:     p.cleanup,
		}
	}

	// Apply runtime-settable values from the persisted config file.
	// These were saved via the web UI and should survive restarts.
	// CLI flags remain the source of truth for startup-only settings
	// (api port, state path, log path, etc.).
	if dc.Concurrency > 0 {
		p.concurrency = dc.Concurrency
	}
	if dc.Workers > 0 {
		p.workers = dc.Workers
	}

	mgr, err := manager.New(manager.Config{
		MaxConcurrent: p.concurrency,
		StatePath:     p.statePath,
		EventBuffer:   512,
		AutoStart:     true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create manager: %v\n", err)
		return
	}

	// Apply persisted defaults to the manager
	if p.workers > 0 {
		mgr.SetDefaultWorkers(p.workers)
	}

	defaultOpts := download.Options{}
	if p.workers > 0 {
		defaultOpts.Workers = p.workers
	}

	if p.apiPort > 0 {
		apiServer := api.NewServer(mgr, dc)
		go func() {
			if err := apiServer.Start(fmt.Sprintf(":%d", p.apiPort)); err != nil {
				fmt.Fprintf(os.Stderr, "api server exited with error: %v\n", err)
			}
		}()
	}

	if p.openWeb && p.apiPort > 0 {
		go func() {
			time.Sleep(500 * time.Millisecond) // Give server time to start
			url := fmt.Sprintf("http://localhost:%d", p.apiPort)
			if err := openBrowser(url); err != nil {
				fmt.Fprintf(os.Stderr, "failed to open browser: %v\n", err)
			}
		}()
	}

	if p.headless || !service.Interactive() {
		// Route standard logger to the log file when a path is configured.
		if p.logPath != "" {
			f, err := os.OpenFile(p.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[relay] cannot open log file %s: %v\n", p.logPath, err)
			} else {
				// Tee: write to both the file and stderr (captured by journald).
				log.SetOutput(io.MultiWriter(f, os.Stderr))
				log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
				log.SetPrefix("[relay] ")
			}
		}
		log.Printf("Relay running in headless mode.")
		if p.apiPort > 0 {
			log.Printf("Web UI available at http://localhost:%d", p.apiPort)
		}
		if service.Interactive() {
			log.Printf("Press Ctrl+C to stop.")
		}
		<-p.ctx.Done()
	} else {
		if err := tui.Run(
			p.ctx,
			mgr,
			tui.WithTheme(p.theme),
			tui.WithThemeOverrides(p.themeOverrides),
			tui.WithTickEvery(time.Duration(p.refreshMS)*time.Millisecond),
			tui.WithCleanupOnRemove(p.cleanup),
			tui.WithDefaultAddOptions(defaultOpts),
		); err != nil {
			fmt.Fprintf(os.Stderr, "tui exited with error: %v\n", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mgr.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "manager shutdown error: %v\n", err)
	}
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	<-p.exit
	return nil
}

func main() {
	serviceAction := flag.String("service", "", "manage system service: install, uninstall, start, stop, status")
	headless := flag.Bool("headless", false, "run as a headless daemon without TUI")
	openWeb := flag.Bool("open-web", false, "automatically open the web UI in default browser")
	statePath := flag.String("state", "relay-downloads.state.json", "path to manager state file")
	logPath := flag.String("log", "", "path to log file (daemon mode); empty = stderr only")
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
	apiPort := flag.Int("api-port", 8080, "port to run the web API server on (0 to disable)")
	flag.Parse()

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

	prg := &program{
		headless:       *headless,
		openWeb:        *openWeb,
		apiPort:        *apiPort,
		statePath:      *statePath,
		logPath:        *logPath,
		concurrency:    *concurrency,
		refreshMS:      *refreshMS,
		cleanup:        *cleanupOnRemove,
		workers:        *defaultWorkers,
		theme:          *theme,
		themeOverrides: themeOverrides,
	}

	svcConfig := &service.Config{
		Name:        "relay",
		DisplayName: "Relay Download Daemon",
		Description: "Background download manager service for Relay.",
		Arguments:   []string{"-headless"},
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if *serviceAction != "" {
		if *serviceAction == "status" {
			st, err := s.Status()
			if err != nil {
				log.Fatalf("Failed to get status: %v", err)
			}
			switch st {
			case service.StatusRunning:
				fmt.Println("Service is running.")
			case service.StatusStopped:
				fmt.Println("Service is stopped.")
			default:
				fmt.Println("Service status is unknown or not installed.")
			}
			return
		}

		err = service.Control(s, *serviceAction)
		if err != nil {
			log.Fatal(err)
		}
		if *serviceAction == "install" {
			fmt.Println("Service installed successfully.")
		} else if *serviceAction == "uninstall" {
			fmt.Println("Service uninstalled successfully.")
		} else {
			fmt.Printf("Service command %s completed.\n", *serviceAction)
		}
		return
	}

	err = s.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
