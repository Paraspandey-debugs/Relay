package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (m *Model) syncBrowserTo(dir string) {
	clean := filepath.Clean(dir)
	if clean == "." || clean == "" {
		clean = m.homeDir
	}
	if !strings.HasPrefix(clean, m.homeDir) {
		clean = m.homeDir
	}
	m.browserDir = clean
	m.browserSelected = 0
	m.browserOffset = 0
	_ = m.loadBrowserEntries()
}

func (m *Model) loadBrowserEntries() error {
	entries, err := os.ReadDir(m.browserDir)
	if err != nil {
		m.browserEntries = nil
		return err
	}

	out := make([]browserEntry, 0, len(entries)+1)
	if m.browserDir != m.homeDir {
		out = append(out, browserEntry{
			name:  "..",
			path:  filepath.Dir(m.browserDir),
			isDir: true,
		})
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		out = append(out, browserEntry{
			name:  entry.Name(),
			path:  filepath.Join(m.browserDir, entry.Name()),
			isDir: true,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	m.browserEntries = out
	if m.browserSelected >= len(m.browserEntries) {
		m.browserSelected = len(m.browserEntries) - 1
	}
	if m.browserSelected < 0 {
		m.browserSelected = 0
	}
	m.ensureBrowserSelectionVisible()
	return nil
}

func (m *Model) moveBrowser(delta int) {
	if len(m.browserEntries) == 0 {
		return
	}
	m.browserSelected += delta
	if m.browserSelected < 0 {
		m.browserSelected = 0
	}
	if m.browserSelected >= len(m.browserEntries) {
		m.browserSelected = len(m.browserEntries) - 1
	}
	m.ensureBrowserSelectionVisible()
}

func (m *Model) browserEnterSelected() error {
	entry, ok := m.selectedBrowserEntry()
	if !ok {
		return nil
	}
	m.syncBrowserTo(entry.path)
	return nil
}

func (m *Model) browserParent() {
	if m.browserDir == m.homeDir {
		return
	}
	m.syncBrowserTo(filepath.Dir(m.browserDir))
}

func (m *Model) applyBrowserSelectionToInput() {
	target := m.browserDir
	if entry, ok := m.selectedBrowserEntry(); ok && entry.isDir {
		target = entry.path
	}
	m.input.SetValue(filepath.Join(target, suggestedFilename(m.add.url)))
	m.input.CursorEnd()
}

func (m *Model) selectedBrowserEntry() (browserEntry, bool) {
	if len(m.browserEntries) == 0 || m.browserSelected < 0 || m.browserSelected >= len(m.browserEntries) {
		return browserEntry{}, false
	}
	return m.browserEntries[m.browserSelected], true
}

func (m *Model) browserHint() string {
	return "up/down move  right enter dir  left parent  tab use directory  enter confirm path"
}

func (m *Model) browserPathLabel() string {
	return fmt.Sprintf("Browsing from home: %s", m.browserDir)
}

func (m *Model) browserPageSize() int {
	if m.height <= 0 {
		return 10
	}
	pageSize := m.height - 20
	if pageSize < 5 {
		return 5
	}
	if pageSize > 20 {
		return 20
	}
	return pageSize
}

func (m *Model) ensureBrowserSelectionVisible() {
	pageSize := m.browserPageSize()
	if m.browserSelected < m.browserOffset {
		m.browserOffset = m.browserSelected
	}
	if m.browserSelected >= m.browserOffset+pageSize {
		m.browserOffset = m.browserSelected - pageSize + 1
	}
	if m.browserOffset < 0 {
		m.browserOffset = 0
	}
	maxOffset := len(m.browserEntries) - pageSize
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.browserOffset > maxOffset {
		m.browserOffset = maxOffset
	}
}

func (m *Model) visibleBrowserEntries() []browserEntry {
	pageSize := m.browserPageSize()
	if len(m.browserEntries) <= pageSize {
		return m.browserEntries
	}
	start := m.browserOffset
	end := start + pageSize
	if end > len(m.browserEntries) {
		end = len(m.browserEntries)
	}
	return m.browserEntries[start:end]
}

func (m *Model) browserPaginationLabel() string {
	if len(m.browserEntries) == 0 {
		return "0/0"
	}
	start := m.browserOffset + 1
	end := m.browserOffset + len(m.visibleBrowserEntries())
	return fmt.Sprintf("showing %d-%d of %d", start, end, len(m.browserEntries))
}
