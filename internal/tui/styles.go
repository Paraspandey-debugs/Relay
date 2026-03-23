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
	Background:   "#0D1117",
	Foreground:   "#E6EDF3",
	Accent:       "#58A6FF",
	Secondary:    "#1F6FEB",
	Success:      "#3FB950",
	Warning:      "#D29922",
	Error:        "#F85149",
	Muted:        "#8B949E",
	Header:       "#58A6FF",
	Card:         "#161B22",
	SelectedCard: "#21262D",
}

var SunsetTheme = Theme{
	Name:         "sunset",
	Background:   "#1C1210",
	Foreground:   "#F2D7D0",
	Accent:       "#FF7A59",
	Secondary:    "#D95A40",
	Success:      "#52C463",
	Warning:      "#E6923C",
	Error:        "#E84A5F",
	Muted:        "#9E827B",
	Header:       "#FFA28B",
	Card:         "#2B1C19",
	SelectedCard: "#3D2A26",
}

var MonoTheme = Theme{
	Name:         "mono",
	Background:   "#0A0A0A",
	Foreground:   "#FAFAFA",
	Accent:       "#E0E0E0",
	Secondary:    "#737373",
	Success:      "#A3E635",
	Warning:      "#FBBF24",
	Error:        "#F87171",
	Muted:        "#525252",
	Header:       "#FFFFFF",
	Card:         "#171717",
	SelectedCard: "#262626",
}

var SurgeTheme = Theme{
	Name:         "surge",
	Background:   "#1A1B26",
	Foreground:   "#C0CAF5",
	Accent:       "#7AA2F7",
	Secondary:    "#414868",
	Success:      "#9ECE6A",
	Warning:      "#E0AF68",
	Error:        "#F7768E",
	Muted:        "#565F89",
	Header:       "#BB9AF7",
	Card:         "#24283B",
	SelectedCard: "#2E3247",
}

func ThemeByName(name string) (Theme, bool) {
	switch name {
	case "ocean":
		return OceanTheme, true
	case "sunset":
		return SunsetTheme, true
	case "mono":
		return MonoTheme, true
	case "surge":
		return SurgeTheme, true
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
	App               lipgloss.Style
	Header            lipgloss.Style
	Subtle            lipgloss.Style
	Label             lipgloss.Style
	Muted             lipgloss.Style
	CardLabel         lipgloss.Style
	CardMuted         lipgloss.Style
	CardInfo          lipgloss.Style
	CardError         lipgloss.Style
	FooterCard        lipgloss.Style
	FooterTitle       lipgloss.Style
	DownloadCard      lipgloss.Style
	SelectedCard      lipgloss.Style
	CardTitle         lipgloss.Style
	SelectedCardTitle lipgloss.Style
	SelectedCardMuted lipgloss.Style
	InfoLine          lipgloss.Style
	ErrorLine         lipgloss.Style
	StatusDone        lipgloss.Style
	StatusActive      lipgloss.Style
	StatusPaused      lipgloss.Style
	StatusError       lipgloss.Style
	StatusQueued      lipgloss.Style
	StatusStarting    lipgloss.Style
	StatusStopping    lipgloss.Style
	StatusVerifying   lipgloss.Style
	LeftPane          lipgloss.Style
	RightPane         lipgloss.Style
}

func newStyles(t Theme) styles {
	bg := lipgloss.Color(t.Background)
	cardBg := lipgloss.Color(t.Card)
	selCardBg := lipgloss.Color(t.SelectedCard)

	return styles{
		App: lipgloss.NewStyle().
			Background(lipgloss.Color(t.Background)).
			Foreground(lipgloss.Color(t.Foreground)).
			Padding(1, 2),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Header)).
			Background(bg).
			Bold(true),
		Subtle: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)).Background(bg),
		Label: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Accent)).
			Background(bg).
			Bold(true),
		Muted: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)).Background(bg),

		// Card-context styles (for use inside LeftPane/RightPane)
		CardLabel: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Background(cardBg).Bold(true),
		CardMuted: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)).Background(cardBg),
		CardInfo:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success)).Background(cardBg),
		CardError: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Background(cardBg).Bold(true),

		FooterCard: lipgloss.NewStyle().
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Muted)).
			PaddingTop(1),
		FooterTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Accent)).
			Bold(true),
		DownloadCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(t.Secondary)).
			BorderBackground(lipgloss.Color(t.Card)).
			Background(lipgloss.Color(t.Card)).
			Padding(0, 1),
		SelectedCard: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(t.Accent)).
			BorderBackground(lipgloss.Color(t.SelectedCard)).
			Background(lipgloss.Color(t.SelectedCard)).
			Padding(0, 0),
		CardTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Foreground)).
			Background(cardBg).
			Bold(true),
		SelectedCardTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Accent)).
			Background(selCardBg).
			Bold(true),
		SelectedCardMuted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Muted)).
			Background(selCardBg),
		InfoLine: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Success)).Background(bg),
		ErrorLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Error)).
			Background(bg).
			Bold(true),
		StatusDone: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Success)).
			Bold(true).
			Padding(0, 1),
		StatusActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Accent)).
			Bold(true).
			Padding(0, 1),
		StatusPaused: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Warning)).
			Bold(true).
			Padding(0, 1),
		StatusError: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Error)).
			Bold(true).
			Padding(0, 1),
		StatusQueued: lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Background)).
			Background(lipgloss.Color(t.Secondary)).
			Bold(true).
			Padding(0, 1),
		LeftPane: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Muted)).
			BorderBackground(lipgloss.Color(t.Card)).
			Background(lipgloss.Color(t.Card)).
			Padding(1, 1),
		RightPane: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(t.Muted)).
			BorderBackground(lipgloss.Color(t.Card)).
			Background(lipgloss.Color(t.Card)).
			Padding(1, 1),
	}
}
