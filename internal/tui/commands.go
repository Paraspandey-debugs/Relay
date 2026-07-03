package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

func addDownloadCmd(mgr manager.Interface, req manager.AddRequest) tea.Cmd {
	return func() tea.Msg {
		id, err := mgr.Add(req)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("added %s", shortID(id))}
	}
}

func pauseCmd(mgr manager.Interface, id string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Pause(id)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("paused %s", shortID(id))}
	}
}

func resumeCmd(mgr manager.Interface, id string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Resume(id)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("resumed %s", shortID(id))}
	}
}

func removeCmd(mgr manager.Interface, id string, cleanup bool) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Remove(id, cleanup)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("removed %s", shortID(id))}
	}
}

func reorderQueueCmd(mgr manager.Interface, ids []string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.ReorderQueue(ids)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: "queue reordered"}
	}
}
