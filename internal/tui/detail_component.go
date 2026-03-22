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
	if m.job == nil {
		return m.styles.RightPane.Width(m.width).Height(m.height).Render(m.styles.CardMuted.Render("No active selection."))
	}

	item := *m.job

	wrap := lipgloss.NewStyle().Width(m.width - 4)

	title := m.styles.CardLabel.Render("Detailed Overview")
	id := m.styles.CardMuted.Render("ID:   " + item.ID)
	url := m.styles.CardMuted.Render(wrap.Render("URL:  " + item.URL))
	path := m.styles.CardMuted.Render(wrap.Render("Path: " + item.Destination))
	status := "\n" + m.styles.CardMuted.Render("Status: ") + statusPill(item.Status, m.styles) + "\n"

	var details string
	if item.Status == manager.StatusDownloading {
		details = lipgloss.JoinVertical(lipgloss.Left,
			m.styles.CardMuted.Render(fmt.Sprintf("Progress: %s / %s (%s)", humanBytes(item.Progress.Downloaded), totalLabel(item.Progress.Total), progressPercent(item.Progress))),
			m.styles.CardMuted.Render(fmt.Sprintf("Speed:    %s", humanSpeed(item.Progress.SpeedBps))),
			m.styles.CardMuted.Render(fmt.Sprintf("ETA:      %s", humanETA(item.Progress.ETA))),
		)
	} else if item.Status == manager.StatusCompleted {
		details = lipgloss.JoinVertical(lipgloss.Left,
			m.styles.CardMuted.Render(fmt.Sprintf("Completed:  %s", totalLabel(item.Progress.Total))),
			m.styles.CardMuted.Render(fmt.Sprintf("Time taken: %s", humanDuration(completedDuration(item)))),
		)
	}

	var errStr string
	if item.Error != "" {
		errStr = "\n" + m.styles.CardError.Render(wrap.Render("Error: "+item.Error))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title, id, url, path, status, details, errStr,
	)

	return m.styles.RightPane.Width(m.width).Height(m.height).Render(
		lipgloss.NewStyle().Padding(1).Render(content),
	)
}
