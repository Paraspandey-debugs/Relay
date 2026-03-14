package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
)

type settingField struct {
	label string
	value string // current string value (shown/edited)
	kind  string // "int", "duration", "bytes", "bool"
	hint  string
}

// defaultedOpts fills zero-values in opts with the library defaults so the
// settings screen always shows a sensible baseline.
func defaultedOpts(opts download.Options) download.Options {
	def := download.DefaultOptions()
	if opts.Workers == 0 {
		opts.Workers = def.Workers
	}
	if opts.MinChunkSize == 0 {
		opts.MinChunkSize = def.MinChunkSize
	}
	if opts.MaxChunkSize == 0 {
		opts.MaxChunkSize = def.MaxChunkSize
	}
	if opts.Timeout == 0 {
		opts.Timeout = def.Timeout
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = def.MaxRetries
	}
	if opts.BaseBackoff == 0 {
		opts.BaseBackoff = def.BaseBackoff
	}
	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = def.MaxBackoff
	}
	return opts
}

func buildSettingsFields(opts download.Options) []settingField {
	opts = defaultedOpts(opts)
	return []settingField{
		{
			label: "Workers",
			value: strconv.Itoa(opts.Workers),
			kind:  "int",
			hint:  "parallel chunk workers per download (0 = library default)",
		},
		{
			label: "Min chunk size",
			value: humanBytes(opts.MinChunkSize),
			kind:  "bytes",
			hint:  "minimum HTTP range chunk size  (e.g. 2.0MB, 524KB)",
		},
		{
			label: "Max chunk size",
			value: humanBytes(opts.MaxChunkSize),
			kind:  "bytes",
			hint:  "maximum HTTP range chunk size  (e.g. 8.0MB)",
		},
		{
			label: "Timeout",
			value: opts.Timeout.String(),
			kind:  "duration",
			hint:  "per-request HTTP timeout  (e.g. 30s, 1m30s)",
		},
		{
			label: "Max retries",
			value: strconv.Itoa(opts.MaxRetries),
			kind:  "int",
			hint:  "retry attempts on failure",
		},
		{
			label: "Base backoff",
			value: opts.BaseBackoff.String(),
			kind:  "duration",
			hint:  "initial delay between retries  (e.g. 500ms)",
		},
		{
			label: "Max backoff",
			value: opts.MaxBackoff.String(),
			kind:  "duration",
			hint:  "maximum delay between retries  (e.g. 20s)",
		},
		{
			label: "Force single",
			value: strconv.FormatBool(opts.ForceSingle),
			kind:  "bool",
			hint:  "disable parallel chunk downloading  (true / false)",
		},
		{
			label: "No resume",
			value: strconv.FormatBool(opts.NoResume),
			kind:  "bool",
			hint:  "always restart downloads from scratch  (true / false)",
		},
	}
}

// fieldsToOpts reconstructs a download.Options from the edited settings fields.
func fieldsToOpts(fields []settingField) download.Options {
	opts := download.DefaultOptions()
	for _, f := range fields {
		switch f.label {
		case "Workers":
			if v, err := strconv.Atoi(strings.TrimSpace(f.value)); err == nil {
				opts.Workers = v
			}
		case "Min chunk size":
			if v, err := parseBytes(f.value); err == nil && v > 0 {
				opts.MinChunkSize = v
			}
		case "Max chunk size":
			if v, err := parseBytes(f.value); err == nil && v > 0 {
				opts.MaxChunkSize = v
			}
		case "Timeout":
			if v, err := time.ParseDuration(strings.TrimSpace(f.value)); err == nil && v > 0 {
				opts.Timeout = v
			}
		case "Max retries":
			if v, err := strconv.Atoi(strings.TrimSpace(f.value)); err == nil {
				opts.MaxRetries = v
			}
		case "Base backoff":
			if v, err := time.ParseDuration(strings.TrimSpace(f.value)); err == nil && v > 0 {
				opts.BaseBackoff = v
			}
		case "Max backoff":
			if v, err := time.ParseDuration(strings.TrimSpace(f.value)); err == nil && v > 0 {
				opts.MaxBackoff = v
			}
		case "Force single":
			if v, err := strconv.ParseBool(strings.TrimSpace(f.value)); err == nil {
				opts.ForceSingle = v
			}
		case "No resume":
			if v, err := strconv.ParseBool(strings.TrimSpace(f.value)); err == nil {
				opts.NoResume = v
			}
		}
	}
	return opts
}

// parseBytes parses human-readable byte strings like "2.0MB", "512KB", or
// plain integer strings.
func parseBytes(s string) (int64, error) {
	upper := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1 << 40},
		{"GB", 1 << 30},
		{"MB", 1 << 20},
		{"KB", 1 << 10},
		{"B", 1},
	}
	for _, entry := range suffixes {
		if strings.HasSuffix(upper, entry.suffix) {
			numStr := strings.TrimSuffix(upper, entry.suffix)
			n, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size %q", s)
			}
			return int64(n * float64(entry.mult)), nil
		}
	}
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

func (m *Model) validateSettingValue(idx int, val string) error {
	f := m.settingsFields[idx]
	switch f.kind {
	case "int":
		_, err := strconv.Atoi(strings.TrimSpace(val))
		return err
	case "duration":
		d, err := time.ParseDuration(strings.TrimSpace(val))
		if err != nil {
			return err
		}
		if d < 0 {
			return fmt.Errorf("duration must be positive")
		}
		return nil
	case "bytes":
		v, err := parseBytes(val)
		if err != nil {
			return err
		}
		if v <= 0 {
			return fmt.Errorf("size must be positive")
		}
		return nil
	case "bool":
		_, err := strconv.ParseBool(strings.TrimSpace(val))
		return err
	}
	return nil
}

// openSettings transitions to the settings screen.
func (m *Model) openSettings() {
	m.settingsFields = buildSettingsFields(m.defaultAddOptions)
	m.settingsCursor = 0
	m.settingsEditing = false
	m.input.Blur()
	m.settingsInput.SetValue("")
	m.settingsInput.Placeholder = ""
	m.settingsInput.Blur()
	m.errMsg = ""
	m.screen = settingsScreen
}

// renderSettings draws the settings screen.
func (m *Model) renderSettings() string {
	var b strings.Builder
	b.WriteString(m.styles.Header.Render("Settings"))
	b.WriteString("\n")
	b.WriteString(m.styles.Subtle.Render("↑/↓ navigate · Enter/e edit field · Esc back to list"))
	b.WriteString("\n\n")

	const labelW = 16
	for i, f := range m.settingsFields {
		selected := i == m.settingsCursor
		prefix := "  "
		if selected {
			prefix = "> "
		}
		label := fmt.Sprintf("%-*s", labelW, f.label)

		if selected && m.settingsEditing {
			b.WriteString(prefix + m.styles.Label.Render(label) + "  " + m.settingsInput.View())
		} else {
			var styledVal string
			if selected {
				styledVal = m.styles.InfoLine.Render(f.value)
			} else {
				styledVal = m.styles.Muted.Render(f.value)
			}
			b.WriteString(prefix + m.styles.Label.Render(label) + "  " + styledVal)
		}
		b.WriteString("\n")

		if selected {
			b.WriteString(m.styles.Subtle.Render("    " + f.hint))
			b.WriteString("\n")
		}
	}

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.ErrorLine.Render("error: " + m.errMsg))
		b.WriteString("\n")
	}

	return m.styles.App.Render(b.String())
}

// handleSettingsKeys processes key events on the settings screen.
func (m *Model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsEditing {
		switch msg.String() {
		case "esc":
			// Cancel edit — restore the original value visible in the field.
			m.settingsEditing = false
			m.settingsInput.SetValue("")
			m.settingsInput.Placeholder = ""
			m.settingsInput.Blur()
			m.errMsg = ""
			return m, nil

		case "enter":
			newVal := strings.TrimSpace(m.settingsInput.Value())
			if err := m.validateSettingValue(m.settingsCursor, newVal); err != nil {
				m.errMsg = err.Error()
				m.errUntil = time.Now().Add(5 * time.Second)
				return m, nil
			}
			m.settingsFields[m.settingsCursor].value = newVal
			m.settingsEditing = false
			m.settingsInput.SetValue("")
			m.settingsInput.Placeholder = ""
			m.settingsInput.Blur()
			m.errMsg = ""
			// Apply the full field set immediately so new downloads pick it up.
			m.defaultAddOptions = fieldsToOpts(m.settingsFields)
			return m, nil

		default:
			var cmd tea.Cmd
			m.settingsInput, cmd = m.settingsInput.Update(msg)
			return m, cmd
		}
	}

	// Navigation / exit when not editing.
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.settingsCursor < len(m.settingsFields)-1 {
			m.settingsCursor++
		}
		return m, nil

	case msg.String() == "enter" || msg.String() == "e":
		m.settingsEditing = true
		m.settingsInput.SetValue(m.settingsFields[m.settingsCursor].value)
		m.settingsInput.Placeholder = m.settingsFields[m.settingsCursor].value
		m.settingsInput.Focus()
		m.errMsg = ""
		return m, nil

	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case msg.String() == "esc":
		m.screen = listScreen
		m.settingsCursor = 0
		m.settingsEditing = false
		m.settingsInput.SetValue("")
		m.settingsInput.Placeholder = ""
		m.settingsInput.Blur()
		m.errMsg = ""
		return m, nil
	}

	return m, nil
}
