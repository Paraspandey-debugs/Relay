package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	TabQueued     key.Binding
	TabActive     key.Binding
	TabDone       key.Binding
	NextTab       key.Binding
	Search        key.Binding
	Log           key.Binding
	Confirm       key.Binding
	Cancel        key.Binding
	LogTop        key.Binding
	LogBottom     key.Binding
	Add           key.Binding
	Pause         key.Binding
	Resume        key.Binding
	Remove        key.Binding
	Settings      key.Binding
	MoveQueueUp   key.Binding
	MoveQueueDown key.Binding
	Refresh       key.Binding
	Help          key.Binding
	Quit          key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("", ""),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "move down"),
		),
		TabQueued: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "queued tab"),
		),
		TabActive: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "active tab"),
		),
		TabDone: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "done tab"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		Search: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "search"),
		),
		Log: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "toggle log"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y", "enter"),
			key.WithHelp("y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n", "cancel"),
		),
		LogTop: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "log top"),
		),
		LogBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "log bottom"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add"),
		),
		Pause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause"),
		),
		Resume: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "resume"),
		),
		Remove: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "remove"),
		),
		Settings: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "settings"),
		),
		MoveQueueUp: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "queue up"),
		),
		MoveQueueDown: key.NewBinding(
			key.WithKeys("J"),
			key.WithHelp("J", "queue down"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "h"),
			key.WithHelp("?", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+q", "ctrl+c"),
			key.WithHelp("ctrl+q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.TabQueued, k.TabActive, k.TabDone, k.Search, k.Log, k.Add, k.Pause, k.Remove, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.TabQueued, k.TabActive, k.TabDone, k.NextTab, k.Search, k.Log, k.Add},
		{k.Up, k.Down, k.Pause, k.Resume, k.Remove, k.Settings},
		{k.MoveQueueUp, k.MoveQueueDown, k.Refresh, k.LogTop, k.LogBottom, k.Help, k.Quit},
	}
}
