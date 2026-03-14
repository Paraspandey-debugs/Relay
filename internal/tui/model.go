package tui

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

const relayStartupASCII = `

____  _____ _         _ __   __
|  _ \| ____| |      / \\ \ / /
| |_) |  _| | |     / _ \\ V / 
|  _ <| |___| |___ / ___ \| |  
|_| \_\_____|_____/_/   \_\_|  
`

const startupSplashDuration = 1400 * time.Millisecond

type screen int

const (
	splashScreen screen = iota
	listScreen
	addScreen
	settingsScreen
)

type addStep int

const (
	addURLStep addStep = iota
	addDestinationStep
)

type actionResultMsg struct {
	info string
	err  error
}

type tickMsg time.Time
type splashDoneMsg struct{}

type Option func(*Model)

func WithTheme(name string) Option {
	return func(m *Model) {
		if th, ok := ThemeByName(name); ok {
			m.theme = th
		}
	}
}

func WithThemeOverrides(overrides map[string]string) Option {
	return func(m *Model) {
		m.theme = ApplyThemeOverrides(m.theme, overrides)
	}
}

func WithCleanupOnRemove(enabled bool) Option {
	return func(m *Model) {
		m.cleanupOnRemove = enabled
	}
}

func WithDefaultAddOptions(opts download.Options) Option {
	return func(m *Model) {
		m.defaultAddOptions = opts
	}
}

func WithTickEvery(d time.Duration) Option {
	return func(m *Model) {
		if d > 0 {
			m.tickEvery = d
		}
	}
}

type Model struct {
	ctx context.Context
	mgr *manager.Manager

	items []manager.DownloadRecord
	queue []string

	selected int
	width    int
	height   int

	keys keyMap
	help help.Model

	progress progress.Model

	screen   screen
	showHelp bool

	input textinput.Model
	settingsInput textinput.Model
	step  addStep
	add   addDraft

	message string
	errMsg  string

	messageUntil time.Time
	errUntil     time.Time

	theme  Theme
	styles styles
	now    time.Time

	cleanupOnRemove   bool
	defaultAddOptions download.Options
	tickEvery         time.Duration
	progressMemo      map[string]memoProgress
	homeDir           string
	recentDir         string
	browserDir        string
	browserSelected   int
	browserOffset     int
	browserEntries    []browserEntry

	settingsCursor  int
	settingsEditing bool
	settingsFields  []settingField
}

type memoProgress struct {
	SpeedBps float64
	ETA      time.Duration
	At       time.Time
}

type addDraft struct {
	url string
	dst string
}

type browserEntry struct {
	name  string
	path  string
	isDir bool
}

func NewModel(ctx context.Context, mgr *manager.Manager, opts ...Option) *Model {
	homeDir := resolveBrowserHomeDir()

	in := textinput.New()
	in.Prompt = "> "
	in.Placeholder = "https://example.com/file.iso"
	in.CharLimit = 4096
	in.Blur()

	settingsIn := textinput.New()
	settingsIn.Prompt = "> "
	settingsIn.CharLimit = 256
	settingsIn.Blur()

	m := &Model{
		ctx:  ctx,
		mgr:  mgr,
		keys: defaultKeys(),
		help: help.New(),
		progress: progress.New(
			progress.WithDefaultGradient(),
			progress.WithoutPercentage(),
		),
		screen:          splashScreen,
		input:           in,
		settingsInput:   settingsIn,
		theme:           OceanTheme,
		cleanupOnRemove: true,
		tickEvery:       250 * time.Millisecond,
		now:             time.Now(),
		progressMemo:    make(map[string]memoProgress),
		homeDir:         homeDir,
		recentDir:       homeDir,
		browserDir:      homeDir,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.styles = newStyles(m.theme)
	m.help.ShowAll = false
	m.syncBrowserTo(m.recentDir)
	m.refreshSnapshot()
	return m
}

func Run(ctx context.Context, mgr *manager.Manager, opts ...Option) error {
	m := NewModel(ctx, mgr, opts...)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-mgr.Events():
				if !ok {
					return
				}
				p.Send(event)
			}
		}
	}()

	_, err := p.Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd(m.tickEvery), splashDoneCmd(startupSplashDuration))
}

func (m *Model) refreshSnapshot() {
	m.items = m.mgr.List()
	m.queue = m.mgr.Queue()
	m.applyProgressSmoothing()
	sort.Slice(m.items, func(i, j int) bool {
		return m.items[i].CreatedAt.Before(m.items[j].CreatedAt)
	})
	if len(m.items) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(m.items) {
		m.selected = len(m.items) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *Model) applyProgressSmoothing() {
	now := time.Now()
	for i := range m.items {
		it := &m.items[i]
		memo, ok := m.progressMemo[it.ID]
		if !ok {
			continue
		}
		if now.Sub(memo.At) > 5*time.Second {
			continue
		}

		if it.Progress.SpeedBps > 0 {
			memo.SpeedBps = it.Progress.SpeedBps
			memo.At = now
		}
		if it.Progress.ETA > 0 {
			memo.ETA = it.Progress.ETA
			memo.At = now
		}

		if it.Status == manager.StatusDownloading {
			if it.Progress.SpeedBps <= 0 && memo.SpeedBps > 0 {
				it.Progress.SpeedBps = memo.SpeedBps
			}
			if it.Progress.ETA <= 0 && memo.ETA > 0 {
				it.Progress.ETA = memo.ETA
			}
			if it.Progress.ETA <= 0 && it.Progress.Total > it.Progress.Downloaded && it.Progress.SpeedBps > 0 {
				remaining := float64(it.Progress.Total-it.Progress.Downloaded) / it.Progress.SpeedBps
				if remaining > 0 {
					it.Progress.ETA = time.Duration(remaining * float64(time.Second))
				}
			}
		}

		m.progressMemo[it.ID] = memo
	}
}

func (m *Model) currentItem() (manager.DownloadRecord, bool) {
	if len(m.items) == 0 || m.selected < 0 || m.selected >= len(m.items) {
		return manager.DownloadRecord{}, false
	}
	return m.items[m.selected], true
}

func (m *Model) applyEvent(ev manager.Event) {
	for i := range m.items {
		if m.items[i].ID != ev.ID {
			continue
		}
		m.items[i].Status = ev.Status
		if ev.Progress != nil {
			m.items[i].Progress = *ev.Progress
		}
		if ev.Error != "" {
			m.items[i].Error = ev.Error
		}
		m.items[i].UpdatedAt = ev.At
		m.applyProgressSmoothing()
		return
	}

	// For structural events like add/remove/queue changes, fall back to a full refresh.
	m.refreshSnapshot()
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func statusLabel(s manager.Status) string {
	switch s {
	case manager.StatusQueued:
		return "QUEUED"
	case manager.StatusDownloading:
		return "DOWNLOADING"
	case manager.StatusPaused:
		return "PAUSED"
	case manager.StatusCompleted:
		return "COMPLETED"
	case manager.StatusErrored:
		return "ERROR"
	default:
		return strings.ToUpper(string(s))
	}
}

func progressPercent(p manager.ProgressInfo) string {
	if p.Total <= 0 {
		return "-"
	}
	pct := (float64(p.Downloaded) / float64(p.Total)) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("%.1f%%", pct)
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}

func humanSpeed(bps float64) string {
	if bps <= 0 {
		return "-"
	}
	return fmt.Sprintf("%s/s", humanBytes(int64(bps)))
}

func humanETA(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	if d > 24*time.Hour {
		return ">1d"
	}
	return d.Round(time.Second).String()
}

func humanDuration(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	if d < time.Second {
		return "<1s"
	}
	return d.Round(time.Second).String()
}

func tickCmd(d time.Duration) tea.Cmd {
	if d <= 0 {
		d = 250 * time.Millisecond
	}
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func splashDoneCmd(d time.Duration) tea.Cmd {
	if d <= 0 {
		d = startupSplashDuration
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func resolveBrowserHomeDir() string {
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" {
		if resolvedUser, err := user.Lookup(sudoUser); err == nil && resolvedUser.HomeDir != "" {
			return resolvedUser.HomeDir
		}
	}

	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home
	}

	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		return homeDir
	}

	return "."
}

func suggestedFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "download.bin"
	}
	name := path.Base(parsed.Path)
	if name == "." || name == "/" || name == "" {
		return "download.bin"
	}
	return name
}

func (m *Model) beginDestinationSelection() {
	m.step = addDestinationStep
	m.syncBrowserTo(m.recentDir)
	m.input.SetValue(filepath.Join(m.browserDir, suggestedFilename(m.add.url)))
	m.input.Placeholder = filepath.Join(m.browserDir, suggestedFilename(m.add.url))
	m.errMsg = ""
}
