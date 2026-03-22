package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Paraspandey-debugs/Relay/internal/manager"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Route to components
	m.jobsList, cmd = m.jobsList.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.details, cmd = m.details.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Sync the active selection to the details component on every update
	m.details, _ = m.details.Update(JobSelectedMsg(m.jobsList.SelectedJob()))

	m.stats, cmd = m.stats.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		if m.screen == splashScreen {
			if key.Matches(msg, m.keys.Quit) {
				return m, tea.Quit
			}
			return m, tea.Batch(cmds...)
		}
		if m.screen == addScreen {
			return m.handleAddInput(msg)
		}
		if m.screen == settingsScreen {
			return m.handleSettingsKeys(msg)
		}
		return m.handleListKeys(msg)
	case splashDoneMsg:
		if m.screen == splashScreen {
			m.screen = listScreen
		}
		return m, nil
	case manager.Event:
		if msg.Progress != nil {
			memo := m.progressMemo[msg.ID]
			if msg.Progress.SpeedBps > 0 {
				memo.SpeedBps = msg.Progress.SpeedBps
				memo.At = time.Now()
			}
			if msg.Progress.ETA > 0 {
				memo.ETA = msg.Progress.ETA
				memo.At = time.Now()
			}
			m.progressMemo[msg.ID] = memo
		}
		if msg.Type == manager.EventProgress {
			m.applyEvent(msg)
		} else {
			m.refreshSnapshot()
			m.notifyInfo(fmt.Sprintf("event: %s (%s)", msg.Type, shortID(msg.ID)))
		}
		if msg.Error != "" {
			m.notifyError(msg.Error)
		}
		return m, nil
	case actionResultMsg:
		if msg.err != nil {
			m.notifyError(msg.err.Error())
			return m, nil
		}
		m.notifyInfo(msg.info)
		m.errMsg = ""
		m.refreshSnapshot()
		return m, nil
	case tickMsg:
		m.now = time.Time(msg)
		if m.errMsg != "" && !m.errUntil.IsZero() && m.now.After(m.errUntil) {
			m.errMsg = ""
			m.errUntil = time.Time{}
		}
		if m.message != "" && !m.messageUntil.IsZero() && m.now.After(m.messageUntil) {
			m.message = ""
			m.messageUntil = time.Time{}
		}

		for id, memo := range m.progressMemo {
			if m.now.Sub(memo.At) > 15*time.Second {
				delete(m.progressMemo, id)
			}
		}
		return m, tickCmd(m.tickEvery)
	default:
		if m.screen == addScreen || (m.screen == settingsScreen && m.settingsEditing) {
			var inputCmd tea.Cmd
			if m.screen == addScreen {
				m.input, inputCmd = m.input.Update(msg)
			} else {
				m.settingsInput, inputCmd = m.settingsInput.Update(msg)
			}
			if inputCmd != nil {
				cmds = append(cmds, inputCmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Map the search Active state to jobsList
		m.jobsList.searchActive = m.searchActive
		m.jobsList.searchQuery = m.searchQuery

		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil
	}
}

func (m *Model) handleAddInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.input.Blur()
		m.screen = listScreen
		m.step = addURLStep
		m.add = addDraft{}
		m.notifyInfo("add cancelled")
		return m, nil
	case m.step == addDestinationStep && key.Matches(msg, m.keys.Up):
		m.moveBrowser(-1)
		return m, nil
	case m.step == addDestinationStep && key.Matches(msg, m.keys.Down):
		m.moveBrowser(1)
		return m, nil
	case m.step == addDestinationStep && msg.String() == "right":
		if err := m.browserEnterSelected(); err != nil {
			m.notifyError(err.Error())
		}
		return m, nil
	case m.step == addDestinationStep && msg.String() == "left":
		m.browserParent()
		return m, nil
	case m.step == addDestinationStep && msg.String() == "tab":
		m.applyBrowserSelectionToInput()
		return m, nil
	case msg.String() == "enter":
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.notifyError("field cannot be empty")
			return m, nil
		}
		if m.step == addURLStep {
			m.add.url = value
			m.beginDestinationSelection()
			return m, nil
		}

		m.add.dst = value
		m.recentDir = filepath.Dir(value)
		if existing, ok := m.mgr.FindDuplicate(m.add.url, m.add.dst); ok {
			m.notifyError(fmt.Sprintf("duplicate exists: %s", shortID(existing.ID)))
			return m, nil
		}
		m.input.Blur()
		m.screen = listScreen
		m.step = addURLStep
		m.input.SetValue("")
		m.input.Placeholder = "https://example.com/file.iso"
		return m, addDownloadCmd(m.mgr, m.add.url, m.add.dst, m.defaultAddOptions)
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m *Model) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.removeConfirm {
		switch {
		case key.Matches(msg, m.keys.Confirm):
			id := m.pendingRemoveID
			m.pendingRemoveID = ""
			m.removeConfirm = false
			if id == "" {
				return m, nil
			}
			return m, removeCmd(m.mgr, id, m.cleanupOnRemove)
		case key.Matches(msg, m.keys.Cancel):
			m.pendingRemoveID = ""
			m.removeConfirm = false
			m.notifyInfo("remove cancelled")
			return m, nil
		default:
			return m, nil
		}
	}

	if m.searchActive {
		switch msg.String() {
		case "esc":
			m.searchActive = false
			m.searchInput.Blur()
			m.searchQuery = ""
			m.searchInput.SetValue("")
			return m, nil
		case "enter":
			m.searchActive = false
			m.searchInput.Blur()
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchQuery = strings.TrimSpace(m.searchInput.Value())
			return m, cmd
		}
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Log):
		m.showLogPanel = !m.showLogPanel
		if m.showLogPanel {
			m.logCursor = len(m.logEntries) - 1
			if m.logCursor < 0 {
				m.logCursor = 0
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.LogTop):
		m.logCursor = 0
		return m, nil
	case key.Matches(msg, m.keys.LogBottom):
		m.logCursor = len(m.logEntries) - 1
		if m.logCursor < 0 {
			m.logCursor = 0
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.showLogPanel {
			if m.logCursor > 0 {
				m.logCursor--
			}
			return m, nil
		}
		m.jobsList.MoveSelection(-1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.showLogPanel {
			if m.logCursor < len(m.logEntries)-1 {
				m.logCursor++
			}
			return m, nil
		}
		m.jobsList.MoveSelection(1)
		return m, nil
	case key.Matches(msg, m.keys.TabQueued):
		m.jobsList.SetTab(tabQueued)
		return m, nil
	case key.Matches(msg, m.keys.TabActive):
		m.jobsList.SetTab(tabActive)
		return m, nil
	case key.Matches(msg, m.keys.TabDone):
		m.jobsList.SetTab(tabDone)
		return m, nil
	case key.Matches(msg, m.keys.NextTab):
		m.jobsList.NextTab()
		return m, nil
	case key.Matches(msg, m.keys.Search):
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			m.searchActive = false
			return m, nil
		}
		m.searchActive = true
		m.searchInput.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil
	case key.Matches(msg, m.keys.Add):
		m.screen = addScreen
		m.step = addURLStep
		m.add = addDraft{}
		m.input.SetValue("")
		m.input.Placeholder = "https://example.com/file.iso"
		m.input.Focus()
		return m, nil
	case key.Matches(msg, m.keys.Pause):
		if item, ok := m.currentItem(); ok {
			return m, pauseCmd(m.mgr, item.ID)
		}
		return m, nil
	case key.Matches(msg, m.keys.Resume):
		if item, ok := m.currentItem(); ok {
			return m, resumeCmd(m.mgr, item.ID)
		}
		return m, nil
	case key.Matches(msg, m.keys.Remove):
		if item, ok := m.currentItem(); ok {
			m.pendingRemoveID = item.ID
			m.removeConfirm = true
			return m, nil
		}
		return m, nil
	case key.Matches(msg, m.keys.MoveQueueUp):
		return m.moveSelectedQueue(-1)
	case key.Matches(msg, m.keys.MoveQueueDown):
		return m.moveSelectedQueue(1)
	case key.Matches(msg, m.keys.Settings):
		m.openSettings()
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		m.refreshSnapshot()
		m.notifyInfo("refreshed")
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) moveSelectedQueue(delta int) (tea.Model, tea.Cmd) {
	item, ok := m.currentItem()
	if !ok {
		return m, nil
	}
	idx := -1
	for i, id := range m.queue {
		if id == item.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.notifyError("selected item is not queued")
		return m, nil
	}
	newIdx := idx + delta
	if newIdx < 0 || newIdx >= len(m.queue) {
		return m, nil
	}
	q := append([]string(nil), m.queue...)
	q[idx], q[newIdx] = q[newIdx], q[idx]
	m.queue = q
	return m, reorderQueueCmd(m.mgr, q)
}

func (m *Model) notifyError(err string) {
	m.errMsg = err
	m.errUntil = time.Now().Add(8 * time.Second)
	m.appendLog("error: " + err)
}

func (m *Model) notifyInfo(info string) {
	m.message = info
	m.messageUntil = time.Now().Add(4 * time.Second)
	m.appendLog(info)
}
