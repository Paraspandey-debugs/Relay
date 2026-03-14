package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

func (m *Model) View() string {
	if m.screen == splashScreen {
		return m.renderSplash()
	}

	if m.screen == addScreen {
		return m.renderAddInput()
	}

	if m.screen == settingsScreen {
		return m.renderSettings()
	}

	var b strings.Builder
	b.WriteString(m.styles.Header.Render("Relay"))
	b.WriteString("\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n")
	b.WriteString(m.renderStatsLine())
	b.WriteString("\n")
	b.WriteString(m.renderSearchLine())
	b.WriteString("\n\n")

	visible := m.visibleItems()
	if len(visible) == 0 {
		if m.searchQuery != "" {
			b.WriteString(m.styles.Muted.Render("No downloads match the current filter."))
		} else {
			b.WriteString(m.styles.Muted.Render("No downloads in this tab. Press 'a' to add one."))
		}
		b.WriteString("\n\n")
	} else {
		for i, item := range visible {
			selected := i == m.selected
			b.WriteString(m.renderDownloadRow(item, selected))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(m.renderQueue())
	b.WriteString("\n")
	if m.showLogPanel {
		b.WriteString("\n")
		b.WriteString(m.renderLogPanel())
		b.WriteString("\n")
	}

	if item, ok := m.currentItem(); ok {
		b.WriteString(m.renderSelected(item))
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString(m.styles.ErrorLine.Render("error: " + m.errMsg))
		b.WriteString("\n")
	} else if m.message != "" {
		b.WriteString(m.styles.InfoLine.Render("info: " + m.message))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.renderHelpBlock())

	main := m.styles.App.Render(b.String())
	if m.removeConfirm {
		return m.renderConfirmOverlay(main)
	}
	return main
}

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

func (m *Model) renderTabs() string {
	queued, active, done := m.tabCounts()

	renderTab := func(label string, count int, tab listTab) string {
		text := fmt.Sprintf(" %s (%d) ", label, count)
		if m.activeTab == tab {
			return m.styles.StatusActive.Render(text)
		}
		return m.styles.Muted.Render(text)
	}

	return strings.Join([]string{
		renderTab("Queued", queued, tabQueued),
		renderTab("Active", active, tabActive),
		renderTab("Done", done, tabDone),
	}, " ")
}

func (m *Model) renderSearchLine() string {
	if m.searchActive {
		return m.searchInput.View()
	}
	if m.searchQuery != "" {
		return m.styles.Subtle.Render("Filter: " + m.searchQuery + "  (press f to clear)")
	}
	return m.styles.Subtle.Render("Press f to filter list")
}

func (m *Model) renderStatsLine() string {
	queued, active, done := m.tabCounts()
	visible := len(m.visibleItems())
	total := len(m.items)
	speed := humanSpeed(m.aggregateSpeedBps())
	return m.styles.Subtle.Render(fmt.Sprintf("items %d/%d  queued %d  active %d  done %d  total speed %s", visible, total, queued, active, done, speed))
}

func (m *Model) renderLogPanel() string {
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Event Log"))
	b.WriteString("\n")
	if len(m.logEntries) == 0 {
		b.WriteString(m.styles.Muted.Render("no log entries yet"))
		return b.String()
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
			b.WriteString(m.styles.InfoLine.Render(line))
		} else {
			b.WriteString(m.styles.Muted.Render(line))
		}
		b.WriteString("\n")
	}
	b.WriteString(m.styles.Subtle.Render("l toggle  up/down scroll  g top  G bottom"))
	return b.String()
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
	var b strings.Builder
	b.WriteString(m.styles.Header.Render("Add Download"))
	b.WriteString("\n")
	b.WriteString(m.styles.Subtle.Render("Enter details and press Enter to continue, Esc to cancel."))
	b.WriteString("\n\n")

	label := "Source URL"
	if m.step == addDestinationStep {
		label = "Destination Path"
	}
	b.WriteString(m.styles.Label.Render(label))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	if m.step == addDestinationStep {
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render(fmt.Sprintf("URL: %s", m.add.url)))
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render(fmt.Sprintf("Recent directory: %s", m.recentDir)))
		b.WriteString("\n\n")
		b.WriteString(m.styles.Label.Render("Directory Tree"))
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render(m.browserPathLabel()))
		b.WriteString("\n")
		for i, entry := range m.visibleBrowserEntries() {
			absoluteIndex := m.browserOffset + i
			prefix := "  "
			if absoluteIndex == m.browserSelected {
				prefix = "> "
			}
			b.WriteString(m.styles.Muted.Render(prefix + entry.name + "/"))
			b.WriteString("\n")
		}
		if len(m.browserEntries) == 0 {
			b.WriteString(m.styles.Muted.Render("(no subdirectories)"))
			b.WriteString("\n")
		} else {
			b.WriteString(m.styles.Subtle.Render(m.browserPaginationLabel()))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(m.styles.Subtle.Render(m.browserHint()))
	}
	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.ErrorLine.Render("error: " + m.errMsg))
	}
	return m.styles.App.Render(b.String())
}

func (m *Model) renderDownloadRow(item manager.DownloadRecord, selected bool) string {
	bar := m.progress.ViewAs(progressRatio(item.Progress))
	status := m.statusPill(item.Status)
	meta := ""
	if item.Status == manager.StatusCompleted {
		meta = fmt.Sprintf(
			"%s  %s  %s/%s  completed in %s",
			shortID(item.ID),
			status,
			humanBytes(item.Progress.Downloaded),
			totalLabel(item.Progress.Total),
			humanDuration(completedDuration(item)),
		)
	} else {
		meta = fmt.Sprintf(
			"%s  %s  %s/%s  %s  ETA %s",
			shortID(item.ID),
			status,
			humanBytes(item.Progress.Downloaded),
			totalLabel(item.Progress.Total),
			humanSpeed(item.Progress.SpeedBps),
			humanETA(item.Progress.ETA),
		)
	}

	fileLine := item.Destination
	if item.Destination == "" {
		fileLine = item.URL
	}

	block := m.styles.DownloadCard.Render(
		m.styles.CardTitle.Render(fileLine) + "\n" +
			m.styles.Muted.Render(meta) + "\n" +
			bar,
	)
	if selected {
		return m.styles.SelectedCard.Render(block)
	}
	return block
}

func (m *Model) renderQueue() string {
	if len(m.queue) == 0 {
		return m.styles.Muted.Render("Queue: empty")
	}
	parts := make([]string, 0, len(m.queue))
	for i, id := range m.queue {
		parts = append(parts, fmt.Sprintf("%d:%s", i+1, shortID(id)))
	}
	return m.styles.Label.Render("Queue") + "\n" + strings.Join(parts, "  ")
}

func (m *Model) renderSelected(item manager.DownloadRecord) string {
	var b strings.Builder
	b.WriteString(m.styles.Label.Render("Selected"))
	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("ID: "+item.ID) + "\n")
	b.WriteString(m.styles.Muted.Render("URL: "+item.URL) + "\n")
	b.WriteString(m.styles.Muted.Render("Path: " + item.Destination))
	if item.Status == manager.StatusCompleted {
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render("Completed in: " + humanDuration(completedDuration(item))))
	}
	if item.Error != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.ErrorLine.Render("Last error: " + item.Error))
	}
	return b.String()
}

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

func (m *Model) statusPill(s manager.Status) string {
	text := " " + statusLabel(s) + " "
	switch s {
	case manager.StatusCompleted:
		return m.styles.StatusDone.Render(text)
	case manager.StatusDownloading:
		return m.styles.StatusActive.Render(text)
	case manager.StatusPaused:
		return m.styles.StatusPaused.Render(text)
	case manager.StatusErrored:
		return m.styles.StatusError.Render(text)
	default:
		return m.styles.StatusQueued.Render(text)
	}
}

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
