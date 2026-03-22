package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatsComponent struct {
	width  int
	height int
	theme  Theme
	styles styles

	queued int
	active int
	done   int
	total  int
	speed  float64
}

func NewStatsComponent(th Theme, st styles) StatsComponent {
	return StatsComponent{
		theme:  th,
		styles: st,
	}
}

func (m StatsComponent) Init() tea.Cmd {
	return nil
}

func (m StatsComponent) Update(msg tea.Msg) (StatsComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *StatsComponent) UpdateStats(total, queued, active, done int, aggSpeed float64) {
	m.total = total
	m.queued = queued
	m.active = active
	m.done = done
	m.speed = aggSpeed
}

func (m StatsComponent) View() string {
	muted := m.styles.Muted.Render
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Accent)).Render

	stats := fmt.Sprintf("%s %d   %s %d   %s %d   %s %d   %s %s",
		muted("items"), m.total,
		muted("queued"), m.queued,
		muted("active"), m.active,
		muted("done"), m.done,
		muted("throughput"), accent(humanSpeed(m.speed)))

	return m.styles.FooterCard.Width(m.width).Render(stats)
}

func (m StatsComponent) HeaderView() string {
	left := m.styles.Header.Render("Relay Dashboard")
	subtle := m.styles.Subtle.Render("Production Mode")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(subtle)
	if gap < 0 {
		gap = 0
	}
	spacer := strings.Repeat(" ", gap)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, subtle) + "\n"
}
