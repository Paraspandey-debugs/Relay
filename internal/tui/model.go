package tui

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

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

type listTab int

const (
	tabQueued listTab = iota
	tabActive
	tabDone
)

func matchesTab(status manager.Status, tab listTab) bool {
	switch tab {
	case tabQueued:
		return status == manager.StatusQueued || status == manager.StatusPaused || status == manager.StatusErrored
	case tabActive:
		return status == manager.StatusDownloading
	case tabDone:
		return status == manager.StatusCompleted
	default:
		return true
	}
}

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

	// items, queue, selected removed – now delegated to jobsList
	width  int
	height int

	keys keyMap
	help help.Model

	progress progress.Model

	screen   screen
	showHelp bool

	input         textinput.Model
	searchInput   textinput.Model
	settingsInput textinput.Model
	step          addStep
	add           addDraft
	searchActive  bool
	searchQuery   string

	message string
	errMsg  string

	messageUntil time.Time
	errUntil     time.Time

	theme  Theme
	styles styles
	now    time.Time

	// progress smoothing state
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

	showLogPanel    bool
	logCursor       int
	logEntries      []string
	removeConfirm   bool
	pendingRemoveID string

	jobsList JobsListComponent
	details  DetailComponent
	stats    StatsComponent
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

	searchIn := textinput.New()
	searchIn.Prompt = "search> "
	searchIn.Placeholder = "type to filter by URL/path/ID"
	searchIn.CharLimit = 256
	searchIn.Blur()

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
		searchInput:     searchIn,
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
	m.appendLog("relay started")
	for _, opt := range opts {
		opt(m)
	}
	m.styles = newStyles(m.theme)
	m.help.ShowAll = false
	m.jobsList = NewJobsList(m.theme, m.styles)
	m.details = NewDetailComponent(m.theme, m.styles)
	m.stats = NewStatsComponent(m.theme, m.styles)
	m.syncBrowserTo(m.recentDir)

	// Load initial state into jobs list component
	m.jobsList, _ = m.jobsList.Update(updateFullStateMsg{items: mgr.List(), queue: mgr.Queue()})
	m.jobsList.SetTab(tabQueued)
	m.syncStats()

	// Apply explicit backgrounds to text inputs to prevent terminal default bleed
	inBg := lipgloss.Color(m.theme.Background)
	inFg := lipgloss.Color(m.theme.Foreground)
	inAccent := lipgloss.Color(m.theme.Accent)

	inStyle := lipgloss.NewStyle().Foreground(inFg).Background(inBg)
	promptStyle := lipgloss.NewStyle().Foreground(inAccent).Background(inBg)

	m.input.TextStyle, m.input.PromptStyle, m.input.Cursor.Style = inStyle, promptStyle, promptStyle
	m.searchInput.TextStyle, m.searchInput.PromptStyle, m.searchInput.Cursor.Style = inStyle, promptStyle, promptStyle
	m.settingsInput.TextStyle, m.settingsInput.PromptStyle, m.settingsInput.Cursor.Style = inStyle, promptStyle, promptStyle
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

// New helper to update stats from the jobs list component
func (m *Model) syncStats() {
	total := m.jobsList.GetTotal()
	queued := m.jobsList.GetQueued()
	active := m.jobsList.GetActive()
	done := m.jobsList.GetDone()
	speed := m.jobsList.GetAggregateSpeed()
	m.stats.UpdateStats(total, queued, active, done, speed)
}

func (m *Model) currentItem() (manager.DownloadRecord, bool) {
	job := m.jobsList.SelectedJob()
	if job == nil {
		return manager.DownloadRecord{}, false
	}
	return *job, true
}

func (m *Model) appendLog(entry string) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return
	}
	line := time.Now().Format("15:04:05") + "  " + entry
	m.logEntries = append(m.logEntries, line)
	if len(m.logEntries) > 200 {
		m.logEntries = m.logEntries[len(m.logEntries)-200:]
	}
	m.logCursor = len(m.logEntries) - 1
	if m.logCursor < 0 {
		m.logCursor = 0
	}
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
