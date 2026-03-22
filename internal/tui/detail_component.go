package tui

import (
	"fmt"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JobSelectedMsg should be dispatched by the parent component when selection changes.
type JobSelectedMsg *manager.DownloadRecord

// DetailComponent manages the right pane detailed view
type DetailComponent struct {
	width, height int
	job           *manager.DownloadRecord // Passed down dynamically by Parent or self-managed
	theme         Theme
	styles        styles
}

func NewDetailComponent(th Theme, st styles) DetailComponent {
	return DetailComponent{
		theme:  th,
		styles: st,
	}
}

func (m DetailComponent) Init() tea.Cmd {
	return nil
}

func (m DetailComponent) Update(msg tea.Msg) (DetailComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case JobSelectedMsg:
		m.job = msg
	}
	return m, nil
}

func (m DetailComponent) View() string {
	lineWidth := m.width - 2
	if lineWidth < 1 {
		lineWidth = 1
	}

	if m.job == nil {
		empty := m.styles.CardMuted.Copy().Width(lineWidth).Render("No active selection.")
		return m.styles.RightPane.Width(m.width).Height(m.height).Render(empty)
	}

	item := *m.job

	wrapWidth := lineWidth
	if wrapWidth < 12 {
		wrapWidth = 12
	}
	wrap := lipgloss.NewStyle().Width(wrapWidth)

	title := m.styles.CardLabel.Copy().Width(lineWidth).Render("Detailed Overview")
	id := m.styles.CardMuted.Copy().Width(lineWidth).Render("ID:   " + item.ID)
	url := m.styles.CardMuted.Copy().Width(lineWidth).Render(wrap.Render("URL:  " + item.URL))
	path := m.styles.CardMuted.Copy().Width(lineWidth).Render(wrap.Render("Path: " + item.Destination))
	statusLine := lipgloss.JoinHorizontal(lipgloss.Top, m.styles.CardMuted.Render("Status: "), statusPill(item.Status, m.styles))
	status := m.styles.CardMuted.Copy().Width(lineWidth).Render(statusLine)

	var detailLines []string
	if item.Status == manager.StatusDownloading {
		detailLines = []string{
			m.styles.CardMuted.Copy().Width(lineWidth).Render(fmt.Sprintf("Progress: %s / %s (%s)", humanBytes(item.Progress.Downloaded), totalLabel(item.Progress.Total), progressPercent(item.Progress))),
			m.styles.CardMuted.Copy().Width(lineWidth).Render(fmt.Sprintf("Speed:    %s", humanSpeed(item.Progress.SpeedBps))),
			m.styles.CardMuted.Copy().Width(lineWidth).Render(fmt.Sprintf("ETA:      %s", humanETA(item.Progress.ETA))),
		}
	} else if item.Status == manager.StatusCompleted {
		detailLines = []string{
			m.styles.CardMuted.Copy().Width(lineWidth).Render(fmt.Sprintf("Completed:  %s", totalLabel(item.Progress.Total))),
			m.styles.CardMuted.Copy().Width(lineWidth).Render(fmt.Sprintf("Time taken: %s", humanDuration(completedDuration(item)))),
		}
	}

	lines := []string{title, id, url, path, status}
	lines = append(lines, detailLines...)

	if item.Error != "" {
		lines = append(lines, m.styles.CardError.Copy().Width(lineWidth).Render(wrap.Render("Error: "+item.Error)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return m.styles.RightPane.Width(m.width).Height(m.height).Render(content)
}
