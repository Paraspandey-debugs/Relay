package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name         string
	Background   string
	Foreground   string
	Accent       string
	Secondary    string
	Success      string
	Warning      string
	Error        string
	Muted        string
	Header       string
	Card         string
	SelectedCard string
}

var OceanTheme = Theme{
	Name:         "ocean",
	Background:   "#0A0E1A",
	Foreground:   "#D9E2F2",
	Accent:       "#5BD1D7",
	Secondary:    "#4F8EF7",
	Success:      "#3DDC97",
	Warning:      "#F4C95D",
	Error:        "#FF5D73",
	Muted:        "#8FA7C7",
	Header:       "#7FD1FF",
	Card:         "#111A2E",
	SelectedCard: "#1C2E52",
}

var SunsetTheme = Theme{
	Name:         "sunset",
	Background:   "#1A0F0A",
	Foreground:   "#FFE8D6",
	Accent:       "#F39C6B",
	Secondary:    "#FFD166",
	Success:      "#7AE582",
	Warning:      "#F6BD60",
	Error:        "#FF6B6B",
	Muted:        "#C3A995",
	Header:       "#FFAF87",
	Card:         "#2A1912",
	SelectedCard: "#3B2419",
}

var MonoTheme = Theme{
	Name:         "mono",
	Background:   "#121212",
	Foreground:   "#F5F5F5",
	Accent:       "#B0BEC5",
	Secondary:    "#90A4AE",
	Success:      "#A5D6A7",
	Warning:      "#FFE082",
	Error:        "#EF9A9A",
	Muted:        "#BDBDBD",
	Header:       "#ECEFF1",
	Card:         "#1E1E1E",
	SelectedCard: "#2A2A2A",
}

func ThemeByName(name string) (Theme, bool) {
	switch name {
	case "ocean":
		return OceanTheme, true
	case "sunset":
		return SunsetTheme, true
	case "mono":
		return MonoTheme, true
	default:
		return Theme{}, false
	}
}

func ApplyThemeOverrides(base Theme, overrides map[string]string) Theme {
	if len(overrides) == 0 {
		return base
	}

	for key, value := range overrides {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}

		switch strings.ToLower(strings.TrimSpace(key)) {
		case "background":
			base.Background = v
		case "foreground":
			base.Foreground = v
		case "accent":
			base.Accent = v
		case "secondary":
			base.Secondary = v
		case "success":
			base.Success = v
		case "warning":
			base.Warning = v
		case "error":
			base.Error = v
		case "muted":
			base.Muted = v
		case "header":
			base.Header = v
		case "card":
			base.Card = v
		case "selectedcard", "selected-card":
			base.SelectedCard = v
		}
	}

	return base
}

type styles struct {
	App          lipgloss.Style
	Header       lipgloss.Style
	Subtle       lipgloss.Style
	Label        lipgloss.Style
	Muted        lipgloss.Style
	FooterCard   lipgloss.Style
	FooterTitle  lipgloss.Style
	DownloadCard lipgloss.Style
	SelectedCard lipgloss.Style
	CardTitle    lipgloss.Style
	InfoLine     lipgloss.Style
	ErrorLine    lipgloss.Style
	StatusDone   lipgloss.Style
	StatusActive lipgloss.Style
	StatusPaused lipgloss.Style
	StatusError  lipgloss.Style
	StatusQueued lipgloss.Style
}

func newStyles(t Theme) styles {
	return styles{
		App: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Background)).
			Foreground(lipgloss.Color(t.Foreground)).
			Padding(1, 2),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Header)).
			Bold(true),
		Subtle: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Accent)).
			Bold(true),
		Muted: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)),
		FooterCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Secondary)).
			Background(lipgloss.Color(t.Card)).
			Padding(0, 1),
		FooterTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Accent)).
			Bold(true),
		DownloadCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Secondary)).
			Background(lipgloss.Color(t.Card)).
			Padding(0, 1),
		SelectedCard: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(t.Accent)).
			Background(lipgloss.Color(t.SelectedCard)).
			Padding(0, 0),
		CardTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Foreground)).
			Bold(true),
		InfoLine: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success)),
		ErrorLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Error)).
			Bold(true),
		StatusDone: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Success)).
			Bold(true),
		StatusActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Accent)).
			Bold(true),
		StatusPaused: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Warning)).
			Bold(true),
		StatusError: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Error)).
			Bold(true),
		StatusQueued: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Secondary)).
			Bold(true),
	}
}
