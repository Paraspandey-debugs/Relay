package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

// JobsListComponent manages the left pane
type JobsListComponent struct {
	width, height int
	jobs          map[string]*manager.DownloadRecord
	order         []string // maintains order
	queue         []string
	selected      int
	activeTab     listTab
	searchQuery   string
	searchActive  bool
	progressModel progress.Model

	theme  Theme
	styles styles
}

func NewJobsList(th Theme, st styles) JobsListComponent {
	return JobsListComponent{
		jobs:          make(map[string]*manager.DownloadRecord),
		order:         make([]string, 0),
		queue:         make([]string, 0),
		theme:         th,
		styles:        st,
		progressModel: progress.New(progress.WithDefaultGradient(), progress.WithoutPercentage()),
	}
}

func (m JobsListComponent) Init() tea.Cmd {
	return nil
}

func (m JobsListComponent) Update(msg tea.Msg) (JobsListComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case manager.Event:
		// Diff-based state update!
		if job, exists := m.jobs[msg.ID]; exists {
			job.Status = msg.Status
			if msg.Progress != nil {
				job.Progress = *msg.Progress
			}
			if msg.Error != "" {
				job.Error = msg.Error
			}
			job.UpdatedAt = msg.At
		} else {
			// New job
			newJob := manager.DownloadRecord{
				ID:        msg.ID,
				Status:    msg.Status,
				UpdatedAt: msg.At,
			}
			if msg.Progress != nil {
				newJob.Progress = *msg.Progress
			}
			m.jobs[msg.ID] = &newJob
			m.order = append(m.order, msg.ID)
		}
		// ensure selection is valid
		m.ensureSelection()

	case updateFullStateMsg: // custom internal message for full resync sync
		m.order = nil
		for _, item := range msg.items {
			itemCopy := item
			m.jobs[item.ID] = &itemCopy
			m.order = append(m.order, item.ID)
		}
		m.queue = msg.queue
		m.ensureSelection()
	}
	return m, nil
}

func (m *JobsListComponent) MoveSelection(delta int) {
	if delta < 0 && m.selected > 0 {
		m.selected--
	} else if delta > 0 && m.selected < len(m.visibleItems())-1 {
		m.selected++
	}
	m.ensureSelection()
}

func (m *JobsListComponent) SetTab(t listTab) {
	m.activeTab = t
	m.ensureSelection()
}

func (m *JobsListComponent) NextTab() {
	m.activeTab = (m.activeTab + 1) % 3
	m.ensureSelection()
}

func (m *JobsListComponent) ensureSelection() {
	vis := m.visibleItems()
	if len(vis) == 0 {
		m.selected = 0
		return
	}
	if m.selected >= len(vis) {
		m.selected = len(vis) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m JobsListComponent) visibleItems() []manager.DownloadRecord {
	var out []manager.DownloadRecord
	q := strings.ToLower(strings.TrimSpace(m.searchQuery))
	for _, id := range m.order {
		if job, ok := m.jobs[id]; ok {
			if !matchesTab(job.Status, m.activeTab) {
				continue
			}
			if q != "" && !strings.Contains(strings.ToLower(job.ID+" "+job.URL+" "+job.Destination), q) {
				continue
			}
			out = append(out, *job)
		}
	}
	return out
}

func (m JobsListComponent) SelectedID() string {
	vis := m.visibleItems()
	if len(vis) == 0 || m.selected < 0 || m.selected >= len(vis) {
		return ""
	}
	return vis[m.selected].ID
}

func (m JobsListComponent) SelectedJob() *manager.DownloadRecord {
	id := m.SelectedID()
	if id == "" {
		return nil
	}
	return m.jobs[id]
}

func (m JobsListComponent) View() string {
	vis := m.visibleItems()

	var b strings.Builder

	// Tabs
	tabsView := m.renderTabs() + "\n\n"
	queueView := m.renderQueue()
	b.WriteString(tabsView)

	// Pagination logic
	// Calculate available height for items
	// m.height is OUTER height of LeftPane. LeftPane has Border=2, Padding(1,1)=2 -> Total 4
	innerAvail := m.height - 4
	used := lipgloss.Height(tabsView) + lipgloss.Height(queueView) + 1 // +1 for spacing
	itemsHeightAvail := innerAvail - used
	if itemsHeightAvail < 5 {
		itemsHeightAvail = 5
	} // Fallback to avoid panic

	// Each row is approximately 4 lines tall (1 top border, 1 text, 1 prog, 1 bottom margin/border)
	itemHeight := 4
	maxItems := itemsHeightAvail / itemHeight
	if maxItems < 1 {
		maxItems = 1
	}

	if len(vis) == 0 {
		b.WriteString(m.styles.CardMuted.Render("No downloads in this tab. Press 'a' to add."))
	} else {
		// Calculate slice bounds to keep selected in view
		start := 0
		if len(vis) > maxItems {
			start = m.selected - (maxItems / 2)
			if start < 0 {
				start = 0
			}
			if start+maxItems > len(vis) {
				start = len(vis) - maxItems
			}
		}
		end := start + maxItems
		if end > len(vis) {
			end = len(vis)
		}

		for i := start; i < end; i++ {
			selected := i == m.selected
			b.WriteString(m.renderRow(vis[i], selected))
			b.WriteString("\n")
		}

		if start > 0 {
			b.WriteString(m.styles.CardMuted.Render(fmt.Sprintf("...and %d above", start)))
			b.WriteString("\n")
		}
		if end < len(vis) {
			b.WriteString(m.styles.CardMuted.Render(fmt.Sprintf("...and %d below", len(vis)-end)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(queueView)

	out := m.styles.LeftPane.Width(m.width).Height(m.height).Render(b.String())
	return out
}

func (m *JobsListComponent) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m JobsListComponent) renderTabs() string {
	renderTab := func(label string, count int, tab listTab) string {
		text := fmt.Sprintf(" %s (%d) ", label, count)
		if m.activeTab == tab {
			return m.styles.StatusActive.Copy().MarginRight(1).Render(text)
		}
		inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Foreground)).Background(lipgloss.Color(m.theme.SelectedCard)).Padding(0, 1).MarginRight(1)
		return inactiveStyle.Render(text)
	}

	queued, active, done := 0, 0, 0
	for _, j := range m.jobs {
		switch {
		case matchesTab(j.Status, tabQueued):
			queued++
		case matchesTab(j.Status, tabActive):
			active++
		case matchesTab(j.Status, tabDone):
			done++
		}
	}
	row := renderTab("Queued", queued, tabQueued) + renderTab("Active", active, tabActive) + renderTab("Done", done, tabDone)
	lineWidth := m.width - 4
	if lineWidth < 1 {
		lineWidth = 1
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(m.theme.Card)).Width(lineWidth).Render(row)
}

func (m JobsListComponent) renderQueue() string {
	lineWidth := m.width - 4
	if lineWidth < 1 {
		lineWidth = 1
	}
	if len(m.queue) == 0 {
		return m.styles.CardMuted.Copy().Width(lineWidth).Render("Queue: empty")
	}
	parts := make([]string, 0, len(m.queue))
	for i, id := range m.queue {
		parts = append(parts, fmt.Sprintf("%d:%s", i+1, shortID(id)))
	}
	label := m.styles.CardLabel.Copy().Width(lineWidth).Render("Queue")
	body := m.styles.CardMuted.Copy().Width(lineWidth).Render(strings.Join(parts, "  "))
	return lipgloss.JoinVertical(lipgloss.Left, label, body)
}

func (m JobsListComponent) renderRow(item manager.DownloadRecord, selected bool) string {
	width := m.width - 8
	if width < 10 {
		width = 10
	}
	filename := item.Destination
	if filename == "" {
		filename = item.URL
	}
	status := statusPill(item.Status, m.styles)

	var metrics string
	if item.Status == manager.StatusCompleted {
		metrics = fmt.Sprintf("%s / %s", humanBytes(item.Progress.Downloaded), totalLabel(item.Progress.Total))
	} else {
		metrics = fmt.Sprintf("%s / %s • %s • ETA %s", humanBytes(item.Progress.Downloaded), totalLabel(item.Progress.Total), humanSpeed(item.Progress.SpeedBps), humanETA(item.Progress.ETA))
	}

	var titleStyle, mutedStyle lipgloss.Style
	spacerBg := lipgloss.Color(m.theme.Card)
	if selected {
		titleStyle = m.styles.SelectedCardTitle.Copy().Width(width - lipgloss.Width(status) - 4)
		mutedStyle = m.styles.SelectedCardMuted
		spacerBg = lipgloss.Color(m.theme.SelectedCard)
	} else {
		titleStyle = m.styles.CardTitle.Copy().Width(width - lipgloss.Width(status) - 4)
		mutedStyle = m.styles.CardMuted
	}
	left := titleStyle.Render(filename)
	right := status

	spacerWidth := width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	spacer := lipgloss.NewStyle().Background(spacerBg).Width(spacerWidth).Render("")
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)

	metricsRendered := mutedStyle.Render(metrics)
	progWidth := width - lipgloss.Width(metricsRendered) - 4
	if progWidth < 10 {
		progWidth = 10
	}

	m.progressModel.Width = progWidth
	bar := m.progressModel.ViewAs(progressRatio(item.Progress))

	spacer2Width := width - lipgloss.Width(bar) - lipgloss.Width(metricsRendered)
	if spacer2Width < 1 {
		spacer2Width = 1
	}
	spacer2 := lipgloss.NewStyle().Background(spacerBg).Width(spacer2Width).Render("")
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, bar, spacer2, metricsRendered)

	card := lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)

	// Use themed card styles so background and borders match the overall theme
	if selected {
		// SelectedCard already contains Background and ThickBorder
		return m.styles.SelectedCard.MarginBottom(1).Render(card)
	}
	// DownloadCard contains RoundedBorder, Background and padding
	return m.styles.DownloadCard.MarginBottom(1).Render(card)
}

func statusPill(s manager.Status, st styles) string {
	text := statusLabel(s)
	switch s {
	case manager.StatusCompleted:
		return st.StatusDone.Render(text)
	case manager.StatusDownloading:
		return st.StatusActive.Render(text)
	case manager.StatusPaused:
		return st.StatusPaused.Render(text)
	case manager.StatusErrored:
		return st.StatusError.Render(text)
	default:
		return st.StatusQueued.Render(text)
	}
}

// custom internal message for full resync
type updateFullStateMsg struct {
	items []manager.DownloadRecord
	queue []string
}

// GetTotal returns the total number of jobs.
func (m JobsListComponent) GetTotal() int {
	return len(m.jobs)
}

// GetQueued returns the number of jobs that are queued, paused, or errored.
func (m JobsListComponent) GetQueued() int {
	count := 0
	for _, job := range m.jobs {
		if matchesTab(job.Status, tabQueued) {
			count++
		}
	}
	return count
}

// GetActive returns the number of jobs that are downloading.
func (m JobsListComponent) GetActive() int {
	count := 0
	for _, job := range m.jobs {
		if matchesTab(job.Status, tabActive) {
			count++
		}
	}
	return count
}

// GetDone returns the number of completed jobs.
func (m JobsListComponent) GetDone() int {
	count := 0
	for _, job := range m.jobs {
		if matchesTab(job.Status, tabDone) {
			count++
		}
	}
	return count
}

// GetAggregateSpeed returns the sum of download speeds of active jobs.
func (m JobsListComponent) GetAggregateSpeed() float64 {
	var total float64
	for _, job := range m.jobs {
		if job.Status == manager.StatusDownloading && job.Progress.SpeedBps > 0 {
			total += job.Progress.SpeedBps
		}
	}
	return total
}

// Queue returns the current queue order.
func (m JobsListComponent) Queue() []string {
	return m.queue
}
