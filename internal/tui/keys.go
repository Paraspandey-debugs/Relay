package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Add           key.Binding
	Pause         key.Binding
	Resume        key.Binding
	Remove        key.Binding
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
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "move down"),
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
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Add, k.Pause, k.Resume, k.Remove, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Add, k.Pause, k.Resume, k.Remove},
		{k.MoveQueueUp, k.MoveQueueDown, k.Refresh, k.Help, k.Quit},
	}
}
