package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleToggleBookmark toggles bookmark status on the selected row or batch across multi-selection.
func (m Model) handleToggleBookmark() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	if m.bookmarks == nil {
		m.bookmarks = make(map[uint64]struct{})
	}

	// 1. Multi-row selection active (via mouse drag or Shift+Up/Down)
	if m.selectionStart >= 0 && m.selectionEnd >= 0 && m.selectionStart != m.selectionEnd {
		start := m.selectionStart
		end := m.selectionEnd
		if start > end {
			start, end = end, start
		}
		if start < 0 {
			start = 0
		}
		if end >= len(m.visible) {
			end = len(m.visible) - 1
		}
		if start <= end {
			allBookmarked := true
			for i := start; i <= end; i++ {
				rec := m.visible[i]
				if rec.ID == 0 {
					continue
				}
				if _, ok := m.bookmarks[rec.ID]; !ok {
					allBookmarked = false
					break
				}
			}

			count := end - start + 1
			if allBookmarked {
				for i := start; i <= end; i++ {
					delete(m.bookmarks, m.visible[i].ID)
				}
				m.message = fmt.Sprintf("Unpinned %d rows (%d-%d)", count, start+1, end+1)
			} else {
				for i := start; i <= end; i++ {
					rec := m.visible[i]
					if rec.ID == 0 {
						m.nextRecordID++
						rec.ID = m.nextRecordID
						m.visible[i] = rec
					}
					m.bookmarks[rec.ID] = struct{}{}
				}
				m.message = fmt.Sprintf("★ Pinned %d rows (%d-%d)", count, start+1, end+1)
			}

			if m.bookmarkedOnly {
				m.rebuildVisible()
				m.selectionStart = -1
				m.selectionEnd = -1
				m.selectedRow = -1
				m.clampScroll()
			}
			return m, nil
		}
	}

	// 2. Single row toggle
	rowIdx := m.selectedRow
	if rowIdx < 0 || rowIdx >= len(m.visible) {
		rowIdx = m.scrollOffset
		if rowIdx >= len(m.visible) {
			rowIdx = len(m.visible) - 1
		}
		m.selectedRow = rowIdx
	}
	if rowIdx >= 0 && rowIdx < len(m.visible) {
		rec := m.visible[rowIdx]
		if rec.ID == 0 {
			m.nextRecordID++
			rec.ID = m.nextRecordID
			m.visible[rowIdx] = rec
		}
		if _, exists := m.bookmarks[rec.ID]; exists {
			delete(m.bookmarks, rec.ID)
			m.message = fmt.Sprintf("Unpinned row %d", rowIdx+1)
		} else {
			m.bookmarks[rec.ID] = struct{}{}
			m.message = fmt.Sprintf("★ Pinned row %d", rowIdx+1)
		}
		if m.bookmarkedOnly {
			m.rebuildVisible()
			if m.selectedRow >= len(m.visible) {
				m.selectedRow = len(m.visible) - 1
			}
			m.clampScroll()
		}
	}
	return m, nil
}

// handleNextBookmark jumps to the next bookmarked record.
func (m Model) handleNextBookmark() (tea.Model, tea.Cmd) {
	if len(m.bookmarks) == 0 || len(m.visible) == 0 {
		m.message = "No bookmarks"
		return m, nil
	}
	start := m.selectedRow
	if start < 0 {
		start = m.scrollOffset - 1
	}
	found := -1
	for i := start + 1; i < len(m.visible); i++ {
		if _, ok := m.bookmarks[m.visible[i].ID]; ok {
			found = i
			break
		}
	}
	if found < 0 {
		for i := 0; i <= start && i < len(m.visible); i++ {
			if _, ok := m.bookmarks[m.visible[i].ID]; ok {
				found = i
				break
			}
		}
	}
	if found >= 0 {
		m.selectedRow = found
		m.follow = false
		h := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset || m.selectedRow >= m.scrollOffset+h {
			m.scrollOffset = m.selectedRow - h/2
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		m.message = fmt.Sprintf("★ Bookmark at row %d", m.selectedRow+1)
	} else {
		m.message = "No bookmarks in visible view"
	}
	return m, nil
}

// handlePrevBookmark jumps to the previous bookmarked record.
func (m Model) handlePrevBookmark() (tea.Model, tea.Cmd) {
	if len(m.bookmarks) == 0 || len(m.visible) == 0 {
		m.message = "No bookmarks"
		return m, nil
	}
	start := m.selectedRow
	if start < 0 {
		start = m.scrollOffset
	}
	found := -1
	for i := start - 1; i >= 0; i-- {
		if _, ok := m.bookmarks[m.visible[i].ID]; ok {
			found = i
			break
		}
	}
	if found < 0 {
		for i := len(m.visible) - 1; i >= start && i >= 0; i-- {
			if _, ok := m.bookmarks[m.visible[i].ID]; ok {
				found = i
				break
			}
		}
	}
	if found >= 0 {
		m.selectedRow = found
		m.follow = false
		h := m.activeDataHeight()
		if m.selectedRow < m.scrollOffset || m.selectedRow >= m.scrollOffset+h {
			m.scrollOffset = m.selectedRow - h/2
		}
		m.clampScroll()
		if m.splitMode != SplitNone && m.syncScroll {
			m.syncOtherPaneChronologically()
		}
		m.message = fmt.Sprintf("★ Bookmark at row %d", m.selectedRow+1)
	} else {
		m.message = "No bookmarks in visible view"
	}
	return m, nil
}

// handleToggleBookmarksOnly toggles filtering the table to only bookmarked records.
func (m Model) handleToggleBookmarksOnly() (tea.Model, tea.Cmd) {
	m.bookmarkedOnly = !m.bookmarkedOnly
	m.rebuildVisible()
	m.selectedRow = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	m.scrollOffset = 0
	m.clampScroll()
	if m.bookmarkedOnly {
		m.message = fmt.Sprintf("★ Showing %d bookmarked row(s)", len(m.visible))
	} else {
		m.message = "Showing all rows"
	}
	return m, nil
}
