package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

func safeWidth(width int) int {
	if width < 1 {
		return 1
	}
	return width
}

func (m *Model) contentWidth() int {
	return safeWidth(m.width - 4)
}

func (m *Model) fullWidthLine(style lipgloss.Style, text string) string {
	return style.Copy().Width(m.contentWidth()).Render(text)
}

func (m *Model) withAppBackground(content string) string {
	return lipgloss.NewStyle().
		Width(m.contentWidth()).
		Background(lipgloss.Color(m.theme.Background)).
		Render(content)
}

func (m *Model) View() string {
	if m.screen == splashScreen {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			m.renderSplash(),
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color(m.theme.Foreground)),
			lipgloss.WithWhitespaceBackground(lipgloss.Color(m.theme.Background)),
		)
	}

	var content string
	if m.screen == addScreen {
		content = m.renderAddInput()
	} else if m.screen == settingsScreen {
		content = m.renderSettings()
	} else {

		// Top pane / Header
		headerLines := []string{m.stats.HeaderView()}
		if m.searchActive {
			headerLines = append(headerLines, m.searchInput.View())
		} else if m.jobsList.searchQuery != "" {
			headerLines = append(headerLines, m.fullWidthLine(m.styles.Subtle, "Filter: "+m.jobsList.searchQuery+"  (press f to clear)"))
		}
		header := lipgloss.JoinVertical(lipgloss.Left, headerLines...)

		// Bottom Stats
		m.stats.UpdateStats(m.jobsList.GetTotal(), m.jobsList.GetQueued(), m.jobsList.GetActive(), m.jobsList.GetDone(), m.jobsList.GetAggregateSpeed())
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

		hideRightPane := false
		verticalLayout := false
		
		var leftOuterWidth, rightOuterWidth int
		if innerWidth < 90 {
			// Terminal is too narrow for side-by-side.
			leftOuterWidth = innerWidth
			rightOuterWidth = innerWidth
			if availHeight >= 20 {
				verticalLayout = true
			} else {
				hideRightPane = true
			}
		} else {
			leftOuterWidth = (innerWidth * 55) / 100 // 55% for list
			rightOuterWidth = innerWidth - leftOuterWidth
		}

		var mainSplit string
		leftTitle := m.styles.Label.Render(" Downloads ")
		rightTitle := m.styles.Label.Render(" Details ")

		if hideRightPane {
			m.jobsList.SetSize(leftOuterWidth - BorderFrameWidth, availHeight - BorderFrameHeight)
			leftBox := RenderBtopBox(leftTitle, "", m.jobsList.View(), leftOuterWidth, availHeight, m.theme.Muted)
			mainSplit = leftBox
		} else if verticalLayout {
			listHeight := availHeight / 2
			detailHeight := availHeight - listHeight
			
			m.jobsList.SetSize(leftOuterWidth - BorderFrameWidth, listHeight - BorderFrameHeight)
			m.details, _ = m.details.Update(tea.WindowSizeMsg{Width: rightOuterWidth - BorderFrameWidth, Height: detailHeight - BorderFrameHeight})
			
			var rightPane string
			if m.removeConfirm {
				rightPane = m.renderConfirmOverlay(rightOuterWidth - BorderFrameWidth, detailHeight - BorderFrameHeight)
			} else {
				rightPane = m.details.View()
			}

			leftBox := RenderBtopBox(leftTitle, "", m.jobsList.View(), leftOuterWidth, listHeight, m.theme.Muted)
			rightBox := RenderBtopBox(rightTitle, "", rightPane, rightOuterWidth, detailHeight, m.theme.Muted)
			
			mainSplit = lipgloss.JoinVertical(lipgloss.Left, leftBox, rightBox)
		} else {
			m.jobsList.SetSize(leftOuterWidth - BorderFrameWidth, availHeight - BorderFrameHeight)
			
			graphHeight := availHeight / 3
			if graphHeight < 8 {
				graphHeight = 8
			}
			
			var rightCol string
			
			if availHeight < 20 {
				m.details, _ = m.details.Update(tea.WindowSizeMsg{Width: rightOuterWidth - BorderFrameWidth, Height: availHeight - BorderFrameHeight})
				var rightPane string
				if m.removeConfirm {
					rightPane = m.renderConfirmOverlay(rightOuterWidth - BorderFrameWidth, availHeight - BorderFrameHeight)
				} else {
					rightPane = m.details.View()
				}
				rightCol = RenderBtopBox(rightTitle, "", rightPane, rightOuterWidth, availHeight, m.theme.Muted)
			} else {
				detailHeight := availHeight - graphHeight
				
				totalDownloaded := int64(0)
				for _, rec := range m.mgr.List() {
					totalDownloaded += rec.Progress.Downloaded
				}

				graphBox := renderGraphBox(rightOuterWidth, graphHeight, m.speedHistory, m.theme, m.styles, totalDownloaded)
				
				m.details, _ = m.details.Update(tea.WindowSizeMsg{Width: rightOuterWidth - BorderFrameWidth, Height: detailHeight - BorderFrameHeight})
				var rightPane string
				if m.removeConfirm {
					rightPane = m.renderConfirmOverlay(rightOuterWidth - BorderFrameWidth, detailHeight - BorderFrameHeight)
				} else {
					rightPane = m.details.View()
				}
				detailBox := RenderBtopBox(rightTitle, "", rightPane, rightOuterWidth, detailHeight, m.theme.Muted)
				
				rightCol = lipgloss.JoinVertical(lipgloss.Left, graphBox, detailBox)
			}

			leftBox := RenderBtopBox(leftTitle, "", m.jobsList.View(), leftOuterWidth, availHeight, m.theme.Muted)

			mainSplit = lipgloss.JoinHorizontal(lipgloss.Top,
				leftBox,
				rightCol,
			)
		}

		// Wrap the two panes in a card-area background so there are no uncolored gaps.
		// Width keeps any odd leftover column painted with the card color.
		mainBody := lipgloss.Place(
			innerWidth,
			availHeight,
			lipgloss.Left,
			lipgloss.Top,
			mainSplit,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceBackground(lipgloss.Color(m.theme.Card)),
		)

		mainLines := []string{header, mainBody, footer}
		if m.errMsg != "" {
			mainLines = append(mainLines, m.fullWidthLine(m.styles.ErrorLine, "error: "+m.errMsg))
		} else if m.message != "" {
			mainLines = append(mainLines, m.fullWidthLine(m.styles.InfoLine, "info: "+m.message))
		}
		content = lipgloss.JoinVertical(lipgloss.Left, mainLines...)
	}

	innerW := m.width - 4
	innerH := m.height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	placed := lipgloss.Place(
		innerW,
		innerH,
		lipgloss.Left,
		lipgloss.Top,
		content,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(m.theme.Foreground)),
		lipgloss.WithWhitespaceBackground(lipgloss.Color(m.theme.Background)),
	)
	main := m.styles.App.Width(m.width).Height(m.height).Render(placed)
	return main
}

// removed writeln helper — use explicit slices and lipgloss.JoinVertical in renderers



// Dead legacy rendering helpers removed: tabs/search/stats are handled by components.

func (m *Model) renderLogPanel() string {
	var lines []string
	lines = append(lines, m.fullWidthLine(m.styles.Label, "Event Log"))

	if len(m.logEntries) == 0 {
		lines = append(lines, m.fullWidthLine(m.styles.Muted, "no log entries yet"))
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
			lines = append(lines, m.fullWidthLine(m.styles.InfoLine, line))
		} else {
			lines = append(lines, m.fullWidthLine(m.styles.Muted, line))
		}
	}
	lines = append(lines, m.fullWidthLine(m.styles.Subtle, "l toggle  up/down scroll  g top  G bottom"))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.withAppBackground(content)
}

func (m *Model) renderConfirmOverlay(width, height int) string {
	msg := "Remove selected download?\n"
	msg += "This can delete partial files if cleanup is enabled.\n\n"
	msg += "y/enter confirm   n/esc cancel"

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(m.theme.Warning)).
		Background(lipgloss.Color(m.theme.Card)).
		Padding(1, 2).
		Render(msg)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

func (m *Model) renderSplash() string {
	banner := m.styles.Header.Render(strings.TrimSpace(relayStartupASCII))
	subtitle := m.styles.Subtle.Render("download manager")

	blockWidth := lipgloss.Width(banner)
	if w := lipgloss.Width(subtitle); w > blockWidth {
		blockWidth = w
	}
	if blockWidth < 1 {
		blockWidth = 1
	}

	fill := lipgloss.NewStyle().
		Width(blockWidth).
		Background(lipgloss.Color(m.theme.Background))

	bannerLine := fill.Render(banner)
	spacerLine := fill.Render("")
	subtitleLine := fill.Render(subtitle)

	return lipgloss.JoinVertical(lipgloss.Left, bannerLine, spacerLine, subtitleLine)
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
	m.input.Width = m.width - 4
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
		lines = append(lines, "", m.fullWidthLine(m.styles.ErrorLine, "error: "+m.errMsg))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.withAppBackground(content)
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
