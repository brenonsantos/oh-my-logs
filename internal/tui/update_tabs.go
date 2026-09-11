package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	tea "github.com/charmbracelet/bubbletea"
)

// rebuildVisible refilters the buffer and updates m.visible and the active tab.
func (m *Model) rebuildVisible() {
	all := m.buffer.All()
	var base []record.Record
	if m.activeFilter == nil || m.activeFilter.Empty() {
		base = all
	} else {
		base = make([]record.Record, 0, len(all))
		for _, r := range all {
			if m.activeFilter.Matches(r) {
				base = append(base, r)
			}
		}
	}

	if m.bookmarkedOnly {
		out := make([]record.Record, 0, len(base))
		for _, r := range base {
			if _, ok := m.bookmarks[r.ID]; ok {
				out = append(out, r)
			}
		}
		m.visible = out
	} else {
		m.visible = base
	}

	if len(m.tabs) > 0 {
		cur := m.currentTab()
		cur.Visible = m.visible
		cur.BookmarkedOnly = m.bookmarkedOnly
	}
}

// rebuildAllTabs refilters the buffer across all tabs (e.g. on profile reload).
func (m *Model) rebuildAllTabs() {
	all := m.buffer.All()
	for i := range m.tabs {
		t := &m.tabs[i]
		var base []record.Record
		if t.Filter == nil || t.Filter.Empty() {
			base = all
		} else {
			base = make([]record.Record, 0, len(all))
			for _, r := range all {
				if t.Filter.Matches(r) {
					base = append(base, r)
				}
			}
		}

		if t.BookmarkedOnly {
			out := make([]record.Record, 0, len(base))
			for _, r := range base {
				if _, ok := m.bookmarks[r.ID]; ok {
					out = append(out, r)
				}
			}
			t.Visible = out
		} else {
			t.Visible = base
		}
	}
	m.syncModelToActiveTab()
	m.clampScroll()
}

func (m Model) handleCreateNewTab() (tea.Model, tea.Cmd) {
	m.syncActiveTabToModel()
	newIdx := len(m.tabs)
	name := fmt.Sprintf("Tab %d", newIdx+1)
	newTab := Tab{
		Name: name,
		ViewportState: ViewportState{
			Follow:       true,
			Visible:      m.buffer.All(),
			SelectedRow:  -1,
			CursorCol:    -1,
			CharSelStart: -1,
			CharSelEnd:   -1,
		},
	}
	if len(newTab.Visible) > m.activeDataHeight() {
		newTab.ScrollOffset = len(newTab.Visible) - m.activeDataHeight()
	}
	m.tabs = append(m.tabs, newTab)
	m.activeTab = newIdx
	m.syncModelToActiveTab()
	m.recalcLayout()
	m.mode = modeFilter
	m.filterInput = ""
	m.filterCursor = 0
	m.message = fmt.Sprintf("Created %s — enter filter (or Enter/Esc for all)", name)
	return m, nil
}

func (m Model) handleCloseActiveTab() (tea.Model, tea.Cmd) {
	if len(m.tabs) <= 1 {
		m.message = "Cannot close the only tab"
		return m, nil
	}
	closedName := m.tabs[m.activeTab].DisplayName(m.activeTab + 1)
	m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
	if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	}
	m.syncModelToActiveTab()
	m.recalcLayout()
	m.message = fmt.Sprintf("Closed %s", closedName)
	return m, nil
}

func (m *Model) scrollToMatch(idx int) {
	m.follow = false
	// Center the match in the viewport
	half := m.activeDataHeight() / 2
	m.scrollOffset = idx - half
	m.clampScroll()
}

// runSearch finds all visible rows matching the search string.
func (m *Model) runSearch() {
	m.searchMatches = nil
	q := strings.ToLower(m.searchInput)
	if q == "" {
		return
	}
	for i, r := range m.visible {
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				m.searchMatches = append(m.searchMatches, i)
			}
		}
	}
	m.searchCursor = 0
	if len(m.searchMatches) > 0 {
		m.scrollToMatch(m.searchMatches[0])
	}
}

func (m *Model) nextSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor + 1) % len(m.searchMatches)
	m.scrollToMatch(m.searchMatches[m.searchCursor])
}

func (m *Model) prevSearchMatch() {
	if len(m.searchMatches) == 0 {
		return
	}
	m.searchCursor = (m.searchCursor - 1 + len(m.searchMatches)) % len(m.searchMatches)
	m.scrollToMatch(m.searchMatches[m.searchCursor])
}

func (m *Model) scrollToBottom() {
	h := m.activeDataHeight()
	if len(m.visible) > h {
		m.scrollOffset = len(m.visible) - h
	} else {
		m.scrollOffset = 0
	}
}

func (m *Model) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	h := m.activeDataHeight()
	maxOffset := len(m.visible) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if len(m.tabs) > 0 {
		cur := m.currentTab()
		cur.ScrollOffset = m.scrollOffset
		cur.Follow = m.follow
	}
}

func (m *Model) clampScrollX() {
	if m.scrollX < 0 {
		m.scrollX = 0
	}
	containerW := m.tableWidth()
	curTab := m.currentTab()
	if m.splitMode == SplitVertical {
		splitX := (containerW - 1) / 2
		if m.activePane == 0 {
			containerW = splitX
		} else {
			containerW = containerW - splitX - 1
		}
	}
	maxW := m.maxContentWidthForTab(curTab, containerW)
	maxScroll := maxW - containerW
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollX > maxScroll {
		m.scrollX = maxScroll
	}
	if curTab != nil {
		curTab.ScrollX = m.scrollX
	}
}

func (m *Model) recalcLayout() {
	if m.height <= 0 {
		return
	}
	// Fixed rows: 1 title + 1 divider + 1 header + 1 divider + 1 divider + 1 status + 1 keys = 7 fixed rows.
	// If more than 1 tab is present, tab bar adds 2 rows (1 row tab bar + 1 row divider).
	fixed := 7
	if len(m.tabs) > 1 {
		fixed += 2
	}
	avail := m.height - fixed
	if avail < 1 {
		avail = 1
	}

	if m.isInspectorActive() && avail >= 10 {
		// Allocate drawer: ~35% of avail height (min 4 rows, max 10 rows)
		drawerH := avail * 35 / 100
		if drawerH < 4 {
			drawerH = 4
		} else if drawerH > 10 {
			drawerH = 10
		}
		m.inspectorHeight = drawerH
		m.tableHeight = avail - drawerH - 1 // 1 row for inspector header divider
		if m.tableHeight < 3 {
			m.tableHeight = 3
		}
	} else {
		m.inspectorHeight = 0
		m.tableHeight = avail
	}
}

// cmdSaveLog saves all raw lines in the buffer to a timestamped file.
func (m *Model) cmdSaveLog() tea.Cmd {
	return func() tea.Msg {
		all := m.buffer.All()
		name := fmt.Sprintf("oml-%s.log", time.Now().Format("2006-01-02T15-04-05"))
		path := filepath.Join(".", name)
		f, err := os.Create(path)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("save log: %w", err)}
		}
		defer f.Close()
		for _, r := range all {
			fmt.Fprintln(f, r.Raw)
		}
		return ErrorMsg{Err: fmt.Errorf("saved %d records to %s", len(all), path)}
	}
}
