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
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(m.styles.Muted.Render("No downloads yet. Press 'a' to add one."))
		b.WriteString("\n\n")
	} else {
		for i, item := range m.items {
			selected := i == m.selected
			b.WriteString(m.renderDownloadRow(item, selected))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(m.renderQueue())
	b.WriteString("\n")

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
	if m.showHelp {
		b.WriteString(m.help.View(m.keys))
	} else {
		b.WriteString(m.help.ShortHelpView(m.keys.ShortHelp()))
	}

	return m.styles.App.Render(b.String())
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
