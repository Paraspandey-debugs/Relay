package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

func (m *Model) View() string {
	if m.screen == splashScreen {
		return lipgloss.NewStyle().Background(lipgloss.Color(m.theme.Background)).Foreground(lipgloss.Color(m.theme.Foreground)).Width(m.width).Height(m.height).Render(m.renderSplash())
	}

	var content string
	if m.screen == addScreen {
		content = m.renderAddInput()
	} else if m.screen == settingsScreen {
		content = m.renderSettings()
	} else {

		// Route selection to Detail component via message
		sel := m.jobsList.SelectedJob()
		m.details, _ = m.details.Update(JobSelectedMsg(sel))

		// Top pane / Header
		header := m.stats.HeaderView()
		if m.searchActive {
			header += m.searchInput.View() + "\n"
		} else if m.jobsList.searchQuery != "" {
			header += m.styles.Subtle.Render("Filter: "+m.jobsList.searchQuery+"  (press f to clear)") + "\n"
		}

		// Bottom Stats
		m.stats.UpdateStats(len(m.jobsList.jobs), len(m.jobsList.queue), m.stats.active, m.stats.done, m.aggregateSpeedBps())
		footer := m.stats.View()

		// Add Log Panel if requested
		if m.showLogPanel {
			logView := m.renderLogPanel()
			footer = logView + "\n" + footer
		}

		appPaddingVert := 2 // m.styles.App has Padding(1, 2)
		// account for horizontal padding (left + right). Padding(1,2) => 2 each side = 4 total
		appPaddingHoriz := 4
		usedHeight := lipgloss.Height(header) + lipgloss.Height(footer) + appPaddingVert
		if m.errMsg != "" {
			usedHeight += 2
		} else if m.message != "" {
			usedHeight += 2
		}
		availHeight := m.height - usedHeight
		if availHeight < 5 {
			availHeight = 5
		}

		// Compute inner width available to the two columns after App horizontal padding
		innerWidth := m.width - appPaddingHoriz
		if innerWidth <= 0 {
			innerWidth = m.width
		}
		leftOuterWidth := innerWidth / 2
		rightOuterWidth := innerWidth - leftOuterWidth

		// LeftPane/RightPane each have border + horizontal padding = 4 columns.
		// Components receive content width; styles add the chrome.
		const paneChromeHoriz = 4
		leftContentWidth := leftOuterWidth - paneChromeHoriz
		rightContentWidth := rightOuterWidth - paneChromeHoriz
		if leftContentWidth < 12 {
			leftContentWidth = 12
		}
		if rightContentWidth < 12 {
			rightContentWidth = 12
		}

		m.jobsList.SetSize(leftContentWidth, availHeight)
		// send window size to detail component so it updates its internal width/height
		m.details, _ = m.details.Update(tea.WindowSizeMsg{Width: rightContentWidth, Height: availHeight})

		// Dash Layout Main
		mainSplit := lipgloss.JoinHorizontal(lipgloss.Top,
			m.jobsList.View(),
			m.details.View(),
		)

		// Wrap the two panes in a card-area background so there are no uncolored gaps.
		// Width keeps any odd leftover column painted with the card color.
		mainBody := lipgloss.NewStyle().
			Background(lipgloss.Color(m.theme.Card)).
			Width(innerWidth).
			Height(availHeight).
			Render(mainSplit)

		mainContent := lipgloss.JoinVertical(lipgloss.Left, header, mainBody, footer)

		if m.errMsg != "" {
			mainContent += "\n" + m.styles.ErrorLine.Render("error: "+m.errMsg)
		} else if m.message != "" {
			mainContent += "\n" + m.styles.InfoLine.Render("info: "+m.message)
		}
		content = mainContent
	}

	main := m.styles.App.Width(m.width).Height(m.height).Render(content)
	if m.removeConfirm {
		return m.renderConfirmOverlay(main)
	}
	return main
}

// removed writeln helper — use explicit slices and lipgloss.JoinVertical in renderers

func (m *Model) renderHelpBlock() string {
	availableWidth := 72
	if m.width > 0 {
		availableWidth = m.width - 8
	}
	if availableWidth < 28 {
		availableWidth = 28
	}

	entries := m.shortGuideEntries()
	if m.showHelp {
		entries = m.fullGuideEntries()
	}

	guideText := m.renderGuideEntries(entries, availableWidth-4)
	body := m.styles.FooterTitle.Render("Guide") + "\n" + guideText
	return m.styles.FooterCard.MaxWidth(availableWidth).Render(body)
}
func (m *Model) renderHeader() string {
	return m.styles.Header.Render("Relay")
}
func (m *Model) shortGuideEntries() []string {
	return []string{
		"1/2/3 tab", "tab next", "f filter", "l log", "a add", "p pause", "r resume", "x remove", "ctrl+q quit",
	}
}

func (m *Model) fullGuideEntries() []string {
	return []string{
		"1 queued", "2 active", "3 done", "tab next", "f filter", "l log",
		"j/k move", "p pause", "r resume", "x remove", "y confirm", "n cancel",
		"K/J queue", "R refresh", "g/G log top/bottom", "s settings", "? hide guide", "ctrl+q quit",
	}
}

func (m *Model) renderGuideEntries(entries []string, width int) string {
	if width < 20 {
		width = 20
	}

	var rawLines []string
	line := ""
	for _, entry := range entries {
		part := "[" + entry + "]"
		plainLen := len(entry) + 2

		currentLen := len(line)
		if line == "" {
			line = part
			continue
		}
		if currentLen+2+plainLen > width {
			rawLines = append(rawLines, line)
			line = part
			continue
		}
		line += "  " + part
	}
	if line != "" {
		rawLines = append(rawLines, line)
	}

	lines := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		lines = append(lines, m.styles.Subtle.Render(raw))
	}

	return strings.Join(lines, "\n")
}

// Dead legacy rendering helpers removed: tabs/search/stats are handled by components.

func (m *Model) renderLogPanel() string {
	var lines []string
	lines = append(lines, m.styles.Label.Render("Event Log"))

	if len(m.logEntries) == 0 {
		lines = append(lines, m.styles.Muted.Render("no log entries yet"))
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	maxLines := 8
	if m.height > 0 {
		if h := m.height / 5; h > maxLines {
			maxLines = h
		}
		if maxLines > 12 {
			maxLines = 12
		}
	}
	if maxLines < 4 {
		maxLines = 4
	}

	start := m.logCursor - maxLines + 1
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > len(m.logEntries) {
		end = len(m.logEntries)
	}

	for i := start; i < end; i++ {
		prefix := "  "
		if i == m.logCursor {
			prefix = "> "
		}
		line := prefix + m.logEntries[i]
		if i == m.logCursor {
			lines = append(lines, m.styles.InfoLine.Render(line))
		} else {
			lines = append(lines, m.styles.Muted.Render(line))
		}
	}
	lines = append(lines, m.styles.Subtle.Render("l toggle  up/down scroll  g top  G bottom"))
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *Model) renderConfirmOverlay(content string) string {
	msg := "Remove selected download?\n"
	msg += "This can delete partial files if cleanup is enabled.\n\n"
	msg += "y/enter confirm   n/esc cancel"

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(m.theme.Warning)).
		Background(lipgloss.Color(m.theme.Card)).
		Padding(1, 2).
		Render(msg)

	if m.width > 0 && m.height > 0 {
		overlay := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
		return content + "\n" + overlay
	}
	return content + "\n\n" + box
}

func (m *Model) renderSplash() string {
	banner := m.styles.Header.Render(strings.TrimSpace(relayStartupASCII))
	subtitle := m.styles.Subtle.Render("download manager")
	content := lipgloss.JoinVertical(lipgloss.Center, banner, "", subtitle)

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	return content
}

func (m *Model) renderAddInput() string {
	var lines []string
	lines = append(lines,
		m.styles.Header.Render("Add Download"),
		m.styles.Subtle.Render("Enter details and press Enter to continue, Esc to cancel."),
		"",
	)

	label := "Source URL"
	if m.step == addDestinationStep {
		label = "Destination Path"
	}
	lines = append(lines, m.styles.Label.Render(label), m.input.View())

	if m.step == addDestinationStep {
		lines = append(lines,
			"",
			m.styles.Muted.Render(fmt.Sprintf("URL: %s", m.add.url)),
			m.styles.Muted.Render(fmt.Sprintf("Recent directory: %s", m.recentDir)),
			"",
			m.styles.Label.Render("Directory Tree"),
			m.styles.Muted.Render(m.browserPathLabel()),
		)
		for i, entry := range m.visibleBrowserEntries() {
			absoluteIndex := m.browserOffset + i
			prefix := "  "
			if absoluteIndex == m.browserSelected {
				prefix = "> "
			}
			lines = append(lines, m.styles.Muted.Render(prefix+entry.name+"/"))
		}
		if len(m.browserEntries) == 0 {
			lines = append(lines, m.styles.Muted.Render("(no subdirectories)"))
		} else {
			lines = append(lines, m.styles.Subtle.Render(m.browserPaginationLabel()))
		}
		lines = append(lines, "", m.styles.Subtle.Render(m.browserHint()))
	}
	if m.errMsg != "" {
		lines = append(lines, "", m.styles.ErrorLine.Render("error: "+m.errMsg))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// Legacy per-row/detail/queue rendering removed — JobsListComponent and DetailComponent handle these.

func completedDuration(item manager.DownloadRecord) time.Duration {
	if item.ActiveFor > 0 {
		return item.ActiveFor
	}
	if !item.CompletedAt.IsZero() && !item.CreatedAt.IsZero() {
		d := item.CompletedAt.Sub(item.CreatedAt)
		if d > 0 {
			return d
		}
	}
	if !item.UpdatedAt.IsZero() && !item.CreatedAt.IsZero() {
		d := item.UpdatedAt.Sub(item.CreatedAt)
		if d > 0 {
			return d
		}
	}
	return 0
}

// statusPill removed from view.go; components use their own rendering helpers.

func progressRatio(p manager.ProgressInfo) float64 {
	if p.Total <= 0 {
		return 0
	}
	r := float64(p.Downloaded) / float64(p.Total)
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

func totalLabel(total int64) string {
	if total <= 0 {
		return "unknown"
	}
	return humanBytes(total)
}
