package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleCopyKey() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		m.message = "No records to copy"
		return m, nil
	}

	// 0. If character selection active on a row, copy substring
	if m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd {
		targetIdx := m.selectedRow
		if targetIdx < 0 || targetIdx >= len(m.visible) {
			targetIdx = m.scrollOffset
		}
		if targetIdx >= 0 && targetIdx < len(m.visible) {
			plainText := m.selectedRowPlainText(targetIdx)
			runes := []rune(plainText)
			minSel, maxSel := m.charSelStart, m.charSelEnd
			if minSel > maxSel {
				minSel, maxSel = maxSel, minSel
			}
			if minSel < 0 {
				minSel = 0
			}
			if minSel < len(runes) {
				if maxSel > len(runes) {
					maxSel = len(runes)
				}
				sub := string(runes[minSel:maxSel])
				_ = clipboard.Copy(sub)
				m.message = fmt.Sprintf("✓ Copied %d chars to clipboard", len([]rune(sub)))
				return m, nil
			}
		}
	}

	// 1. If multi-row selection active, copy range
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
		var b strings.Builder
		count := 0
		for i := start; i <= end; i++ {
			b.WriteString(m.visible[i].Raw)
			b.WriteByte('\n')
			count++
		}
		_ = clipboard.Copy(strings.TrimRight(b.String(), "\n"))
		if d, ok := m.selectionDelta(); ok {
			m.message = fmt.Sprintf("✓ Copied %d rows (Δt: %s) to clipboard", count, timing.FormatDelta(d))
		} else {
			m.message = fmt.Sprintf("✓ Copied %d rows to clipboard", count)
		}
		return m, nil
	}

	// 2. If single row selected, copy it
	targetIdx := m.selectedRow
	if targetIdx < 0 || targetIdx >= len(m.visible) {
		if m.follow && len(m.visible) > 0 {
			targetIdx = len(m.visible) - 1
		} else {
			targetIdx = m.scrollOffset
			if targetIdx >= len(m.visible) {
				targetIdx = len(m.visible) - 1
			}
		}
	}

	if targetIdx >= 0 && targetIdx < len(m.visible) {
		_ = clipboard.Copy(m.visible[targetIdx].Raw)
		m.message = "✓ Copied row to clipboard"
	}
	return m, nil
}

func (m Model) handleMousePress(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeHelp {
		m.mode = modeNormal
		return m, nil
	}
	if m.mode == modePortPicker || m.mode == modeProfilePicker || m.mode == modeFilterPresets || m.mode == modeSavePresetPrompt || m.mode == modeGame || m.mode == modeTXInput {
		return m, nil
	}

	// Horizontal scrollbar divider click (Row m.height - 3)
	if msg.Y == m.height-3 {
		return m.handleHScrollbarClick(msg)
	}

	// Tab bar click (Row 2 when len(m.tabs) > 1)
	if len(m.tabs) > 1 && msg.Y == 2 {
		return m.handleTabBarMouseClick(msg)
	}

	// Status bar click (Row m.height - 2)
	if msg.Y == m.height-2 {
		return m.handleStatusBarMouseClick(msg)
	}

	// Key bar click (Row m.height - 1)
	if msg.Y == m.height-1 {
		return m.handleKeyBarMouseClick(msg)
	}

	// Split mode table click handling
	if m.splitMode != SplitNone {
		return m.handleSplitTableMouseClick(msg)
	}

	// Table rows click
	tableStartY := 4
	if len(m.tabs) > 1 {
		tableStartY = 6
	}
	tableEndY := tableStartY + len(m.visibleRows())

	if msg.Y >= tableStartY && msg.Y < tableEndY {
		// Vertical scrollbar click at right edge
		if msg.X >= m.tableWidth()-2 && len(m.visible) > m.tableHeight && m.tableHeight > 1 {
			clickRow := msg.Y - tableStartY
			maxOffset := len(m.visible) - m.tableHeight
			if maxOffset > 0 {
				m.follow = false
				m.scrollOffset = clickRow * maxOffset / (m.tableHeight - 1)
				m.clampScroll()
			}
			return m, nil
		}

		rowOffset := msg.Y - tableStartY
		absIdx := m.scrollOffset + rowOffset
		if absIdx >= 0 && absIdx < len(m.visible) {
			m.follow = false
			m.selectedRow = absIdx
			m.selectionStart = absIdx
			m.selectionEnd = absIdx
			if msg.X >= 3 {
				m.cursorCol = msg.X - 3 + m.scrollX
			} else {
				m.cursorCol = 0
			}
			m.charSelStart = -1
			m.charSelEnd = -1

			now := time.Now()
			if m.lastClickRow == absIdx && now.Sub(m.lastClickTime) < 400*time.Millisecond {
				// Double click: copy row to clipboard
				_ = clipboard.Copy(m.visible[absIdx].Raw)
				m.message = "✓ Copied row to clipboard"
			}
			m.lastClickTime = now
			m.lastClickRow = absIdx
		}
		return m, nil
	}

	// Click outside table or on empty state: clear selection
	m.selectedRow = -1
	m.selectionStart = -1
	m.selectionEnd = -1
	m.cursorCol = -1
	m.charSelStart = -1
	m.charSelEnd = -1
	return m, nil
}

// splitTableStartY returns the terminal Y row where viewSplitTable() begins.
func (m Model) splitTableStartY() int {
	if len(m.tabs) > 1 {
		return 4
	}
	return 2
}

func (m Model) handleSplitTableMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	splitStartY := m.splitTableStartY()
	totalH := m.tableHeight + 2
	splitEndY := splitStartY + totalH

	if msg.Y < splitStartY || msg.Y >= splitEndY {
		return m, nil
	}

	if m.splitMode == SplitVertical {
		splitX := (m.width - 1) / 2
		clickedPane := 0
		if msg.X > splitX {
			clickedPane = 1
		}
		if m.activePane != clickedPane {
			m.switchPaneFocus()
		}

		dataStartY := splitStartY + 2
		if msg.Y >= dataStartY && msg.Y < splitEndY {
			rowOffset := msg.Y - dataStartY
			absIdx := m.scrollOffset + rowOffset
			if absIdx >= 0 && absIdx < len(m.visible) {
				m.follow = false
				m.selectedRow = absIdx
				m.selectionStart = absIdx
				m.selectionEnd = absIdx
				paneStartX := 0
				if clickedPane == 1 {
					paneStartX = splitX + 1
				}
				col := msg.X - paneStartX - 3 + m.scrollX
				if col < 0 {
					col = 0
				}
				m.cursorCol = col
				m.charSelStart = -1
				m.charSelEnd = -1

				now := time.Now()
				if m.lastClickRow == absIdx && now.Sub(m.lastClickTime) < 400*time.Millisecond {
					_ = clipboard.Copy(m.visible[absIdx].Raw)
					m.message = "✓ Copied row to clipboard"
				}
				m.lastClickTime = now
				m.lastClickRow = absIdx

				if m.syncScroll {
					m.syncOtherPaneChronologically()
				}
			}
		}
		return m, nil
	}

	// SplitHorizontal
	availH := totalH - 1
	topH := availH / 2
	dividerY := splitStartY + topH
	clickedPane := 0
	if msg.Y > dividerY {
		clickedPane = 1
	}
	if m.activePane != clickedPane {
		m.switchPaneFocus()
	}

	var dataStartY, dataEndY int
	if clickedPane == 0 {
		dataStartY = splitStartY + 2
		dataEndY = splitStartY + topH
	} else {
		dataStartY = dividerY + 1 + 2
		dataEndY = splitEndY
	}
	if msg.Y >= dataStartY && msg.Y < dataEndY {
		rowOffset := msg.Y - dataStartY
		absIdx := m.scrollOffset + rowOffset
		if absIdx >= 0 && absIdx < len(m.visible) {
			m.follow = false
			m.selectedRow = absIdx
			m.selectionStart = absIdx
			m.selectionEnd = absIdx
			col := msg.X - 3 + m.scrollX
			if col < 0 {
				col = 0
			}
			m.cursorCol = col
			m.charSelStart = -1
			m.charSelEnd = -1

			now := time.Now()
			if m.lastClickRow == absIdx && now.Sub(m.lastClickTime) < 400*time.Millisecond {
				_ = clipboard.Copy(m.visible[absIdx].Raw)
				m.message = "✓ Copied row to clipboard"
			}
			m.lastClickTime = now
			m.lastClickRow = absIdx

			if m.syncScroll {
				m.syncOtherPaneChronologically()
			}
		}
	}
	return m, nil
}

func (m Model) handleTabBarMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	w := m.tableWidth()
	curX := 1 // leading space in viewTabBar
	for i := range m.tabs {
		t := &m.tabs[i]
		displayName := t.DisplayName(i + 1)
		countStr := fmt.Sprintf("%d", len(t.Visible))
		label := fmt.Sprintf("%d: %s (%s)", i+1, displayName, countStr)
		// TabActive / TabInactive have Padding(0, 1) -> width is len(label) + 2
		pillWidth := len([]rune(label)) + 2
		if msg.X >= curX && msg.X < curX+pillWidth {
			m.switchTab(i)
			return m, nil
		}
		curX += pillWidth + 1
	}

	// Check right-side hints: "Tab: cycle · ^T: new · ^W: close  "
	hints := "Tab: cycle · ^T: new · ^W: close  "
	hintsStartX := w - len(hints)
	if msg.X >= hintsStartX {
		relX := msg.X - hintsStartX
		if relX < 14 {
			if len(m.tabs) > 1 {
				m.switchTab((m.activeTab + 1) % len(m.tabs))
			}
			return m, nil
		} else if relX >= 14 && relX < 25 {
			return m.handleCreateNewTab()
		} else {
			return m.handleCloseActiveTab()
		}
	}
	return m, nil
}

func (m Model) handleKeyBarMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// If only 1 tab exists and user clicks "^T new tab"
	if len(m.tabs) == 1 && msg.X > 20 && msg.X < 36 {
		return m.handleCreateNewTab()
	}
	return m, nil
}

func (m Model) handleStatusBarMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Toggle follow mode when status bar is clicked
	m.follow = !m.follow
	if m.follow {
		m.scrollToBottom()
		m.selectedRow = -1
		m.selectionStart = -1
		m.selectionEnd = -1
		m.message = "Resumed stream (FOLLOW)"
	} else {
		m.message = "Stream paused"
	}
	return m, nil
}

func (m Model) handleMouseMotion(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Dragging vertical scrollbar
	if msg.X >= m.tableWidth()-2 && len(m.visible) > m.tableHeight && m.tableHeight > 1 {
		dataStartY := 4
		if len(m.tabs) > 1 {
			dataStartY = 6
		}
		if m.splitMode != SplitNone {
			dataStartY = m.splitTableStartY() + 2
		}
		clickRow := msg.Y - dataStartY
		if clickRow < 0 {
			clickRow = 0
		}
		if clickRow >= m.tableHeight {
			clickRow = m.tableHeight - 1
		}
		maxOffset := len(m.visible) - m.tableHeight
		if maxOffset > 0 {
			m.follow = false
			m.scrollOffset = clickRow * maxOffset / (m.tableHeight - 1)
			m.clampScroll()
		}
		return m, nil
	}

	// Dragging horizontal scrollbar
	if msg.Y == m.height-3 {
		return m.handleHScrollbarClick(msg)
	}

	if m.selectionStart < 0 || len(m.visible) == 0 {
		return m, nil
	}

	dataStartY := 4
	if len(m.tabs) > 1 {
		dataStartY = 6
	}
	dataEndY := dataStartY + m.tableHeight
	if m.splitMode == SplitVertical {
		dataStartY = m.splitTableStartY() + 2
		dataEndY = m.splitTableStartY() + m.tableHeight + 2
	} else if m.splitMode == SplitHorizontal {
		splitStartY := m.splitTableStartY()
		totalH := m.tableHeight + 2
		availH := totalH - 1
		topH := availH / 2
		dividerY := splitStartY + topH
		if m.activePane == 0 {
			dataStartY = splitStartY + 2
			dataEndY = splitStartY + topH
		} else {
			dataStartY = dividerY + 3
			dataEndY = splitStartY + totalH
		}
	}

	if msg.Y < dataStartY {
		// Dragging above table viewport: auto-scroll up
		if m.scrollOffset > 0 {
			m.scrollOffset--
			m.clampScroll()
		}
		m.selectionEnd = m.scrollOffset
		m.follow = false
	} else if msg.Y >= dataEndY {
		// Dragging below table viewport: auto-scroll down
		m.scrollOffset++
		m.clampScroll()
		h := m.activeDataHeight()
		endIdx := m.scrollOffset + h - 1
		if endIdx >= len(m.visible) {
			endIdx = len(m.visible) - 1
		}
		m.selectionEnd = endIdx
		m.follow = false
	} else {
		rowOffset := msg.Y - dataStartY
		absIdx := m.scrollOffset + rowOffset
		if absIdx >= len(m.visible) {
			absIdx = len(m.visible) - 1
		}
		if absIdx < 0 {
			absIdx = 0
		}
		m.selectionEnd = absIdx
		m.follow = false
	}
	if start, end := m.selectionRange(); start >= 0 && end >= 0 {
		m.message = m.selectionMessage(end-start+1, "press y to copy")
	}
	return m, nil
}

func (m Model) handleMouseRelease(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if start, end := m.selectionRange(); start >= 0 && end >= 0 {
		m.selectedRow = m.selectionEnd
		m.message = m.selectionMessage(end-start+1, "press y to copy")
	}
	return m, nil
}

func (m Model) handleSelectUp() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	m.follow = false

	if m.selectionStart < 0 || m.selectionEnd < 0 {
		anchor := m.selectedRow
		if anchor < 0 {
			h := m.activeDataHeight()
			if m.scrollOffset+h < len(m.visible) {
				anchor = m.scrollOffset + h - 1
			} else {
				anchor = len(m.visible) - 1
			}
		}
		m.selectionStart = anchor
		m.selectionEnd = anchor
	}

	if m.selectionEnd > 0 {
		m.selectionEnd--
	}
	m.selectedRow = m.selectionEnd

	if m.selectionEnd < m.scrollOffset {
		m.scrollOffset = m.selectionEnd
		m.clampScroll()
	}

	minS, maxS := m.selectionStart, m.selectionEnd
	if minS > maxS {
		minS, maxS = maxS, minS
	}
	count := maxS - minS + 1
	if count > 1 {
		m.message = m.selectionMessage(count, "press y to copy")
	} else {
		m.message = "1 row selected"
	}

	return m, nil
}

func (m Model) handleSelectDown() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	m.follow = false

	if m.selectionStart < 0 || m.selectionEnd < 0 {
		anchor := m.selectedRow
		if anchor < 0 {
			anchor = m.scrollOffset
		}
		m.selectionStart = anchor
		m.selectionEnd = anchor
	}

	if m.selectionEnd < len(m.visible)-1 {
		m.selectionEnd++
	}
	m.selectedRow = m.selectionEnd

	h := m.activeDataHeight()
	if m.selectionEnd >= m.scrollOffset+h {
		m.scrollOffset = m.selectionEnd - h + 1
		m.clampScroll()
	}

	minS, maxS := m.selectionStart, m.selectionEnd
	if minS > maxS {
		minS, maxS = maxS, minS
	}
	count := maxS - minS + 1
	if count > 1 {
		m.message = m.selectionMessage(count, "press y to copy")
	} else {
		m.message = "1 row selected"
	}

	return m, nil
}

func (m Model) handleHScrollbarClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	w := m.tableWidth()
	if w <= 0 {
		return m, nil
	}

	if m.splitMode == SplitVertical {
		splitX := (w - 1) / 2
		if msg.X < splitX {
			var leftTab *Tab
			if m.splitLeftTab >= 0 && m.splitLeftTab < len(m.tabs) {
				leftTab = &m.tabs[m.splitLeftTab]
			}
			maxW := m.maxContentWidthForTab(leftTab, splitX)
			maxScroll := maxW - splitX
			if maxScroll > 0 && splitX > 0 {
				newScroll := msg.X * maxScroll / splitX
				if newScroll < 0 {
					newScroll = 0
				}
				if newScroll > maxScroll {
					newScroll = maxScroll
				}
				if leftTab != nil {
					leftTab.ScrollX = newScroll
					if m.activePane == 0 {
						m.scrollX = newScroll
					}
				}
			}
		} else if msg.X > splitX {
			rightW := w - splitX - 1
			clickX := msg.X - splitX - 1
			var rightTab *Tab
			if m.splitRightTab >= 0 && m.splitRightTab < len(m.tabs) {
				rightTab = &m.tabs[m.splitRightTab]
			}
			maxW := m.maxContentWidthForTab(rightTab, rightW)
			maxScroll := maxW - rightW
			if maxScroll > 0 && rightW > 0 {
				newScroll := clickX * maxScroll / rightW
				if newScroll < 0 {
					newScroll = 0
				}
				if newScroll > maxScroll {
					newScroll = maxScroll
				}
				if rightTab != nil {
					rightTab.ScrollX = newScroll
					if m.activePane == 1 {
						m.scrollX = newScroll
					}
				}
			}
		}
		return m, nil
	}

	maxW := m.maxContentWidth()
	maxScroll := maxW - w
	if maxScroll > 0 {
		newScroll := msg.X * maxScroll / w
		if newScroll < 0 {
			newScroll = 0
		}
		if newScroll > maxScroll {
			newScroll = maxScroll
		}
		m.scrollX = newScroll
		if len(m.tabs) > 0 {
			m.currentTab().ScrollX = newScroll
		}
	}
	return m, nil
}
