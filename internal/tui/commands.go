package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Paraspandey-debugs/Relay/internal/core/download"
	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

func addDownloadCmd(mgr *manager.Manager, url, dst string, opts download.Options) tea.Cmd {
	return func() tea.Msg {
		id, err := mgr.Add(manager.AddRequest{
			URL:         url,
			Destination: dst,
			Options:     opts,
		})
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("added %s", shortID(id))}
	}
}

func pauseCmd(mgr *manager.Manager, id string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Pause(id)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("paused %s", shortID(id))}
	}
}

func resumeCmd(mgr *manager.Manager, id string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Resume(id)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("resumed %s", shortID(id))}
	}
}

func removeCmd(mgr *manager.Manager, id string, cleanup bool) tea.Cmd {
	return func() tea.Msg {
		err := mgr.Remove(id, cleanup)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: fmt.Sprintf("removed %s", shortID(id))}
	}
}

func reorderQueueCmd(mgr *manager.Manager, queue []string) tea.Cmd {
	return func() tea.Msg {
		err := mgr.ReorderQueue(queue)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{info: "queue reordered"}
	}
}
