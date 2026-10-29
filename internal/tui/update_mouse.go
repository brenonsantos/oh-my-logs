package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/clipboard"
	"github.com/brenoniehues/oh-my-logs/internal/game"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleCopyKey() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		m.message = "No records to copy"
		return m, nil
	}

	text := m.selectedText()
	if text != "" {
		_ = clipboard.Copy(text)
		lines := strings.Split(text, "\n")
		if len(lines) == 1 {
			if m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd {
				m.message = fmt.Sprintf("✓ Copied %d chars to clipboard", len([]rune(text)))
			} else {
				m.message = "✓ Copied row to clipboard"
			}
		} else {
			if d, ok := m.selectionDelta(); ok {
				m.message = fmt.Sprintf("✓ Copied %d rows (Δt: %s) to clipboard", len(lines), timing.FormatDelta(d))
			} else {
				m.message = fmt.Sprintf("✓ Copied %d rows to clipboard", len(lines))
			}
		}
		return m, nil
	}

	// Fallback single row copy
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
	if m.mode == modeGame && m.activeGame != nil {
		originX, originY := m.gameModalBoardOrigin()
		relX := msg.X - originX
		relY := msg.Y - originY
		var cmd tea.Cmd
		m.activeGame, cmd = m.activeGame.Update(game.AimClickMsg{X: relX, Y: relY})
		return m, cmd
	}
	if m.mode != modeNormal && m.mode != modeSearch && m.mode != modeFilter && m.mode != modeTimeJump {
		return m, nil
	}

	// Horizontal scrollbar divider click
	if msg.Y == m.height-hScrollbarBottomOffset {
		m.isDraggingHScroll = true
		return m.handleHScrollbarClick(msg)
	}

	// Tab bar click
	if len(m.tabs) > 1 && msg.Y == tabBarRow {
		return m.handleTabBarMouseClick(msg)
	}

	// Status bar click
	if msg.Y == m.height-statusBarBottomOffset {
		return m.handleStatusBarMouseClick(msg)
	}

	// Key bar click
	if msg.Y == m.height-keyBarBottomOffset {
		return m.handleKeyBarMouseClick(msg)
	}

	// Split mode table click handling
	if m.splitMode != SplitNone {
		return m.handleSplitTableMouseClick(msg)
	}

	// Table rows click
	tableStartY := m.tableDataStartY()
	tableEndY := tableStartY + len(m.visibleRows())

	if msg.Y >= tableStartY && msg.Y < tableEndY {
		// Vertical scrollbar click at right edge
		if msg.X >= m.tableWidth()-2 && len(m.visible) > m.tableHeight && m.tableHeight > 1 {
			m.isDraggingVScroll = true
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
			var col int
			if msg.X >= prefixWidth {
				col = msg.X - prefixWidth + m.scrollX
			} else {
				col = 0
			}
			m.cursorCol = col
			m.charSelStart = -1
			m.charSelEnd = -1
			m.mousePressRow = absIdx
			m.mousePressCol = col
			m.mousePressBlock = msg.Alt || msg.Ctrl

			now := time.Now()
			if m.lastClickRow == absIdx && now.Sub(m.lastClickTime) < doubleClickThreshold {
				// Double click: select word and copy to clipboard
				plainText := m.selectedRowPlainText(absIdx)
				wStart, wEnd, word := findWordAt(plainText, col)
				if word != "" {
					m.charSelStart = wStart
					m.charSelEnd = wEnd
					_ = clipboard.Copy(word)
					m.message = fmt.Sprintf("✓ Copied '%s' to clipboard", word)
				} else {
					_ = clipboard.Copy(m.visible[absIdx].Raw)
					m.message = "✓ Copied row to clipboard"
				}
			}
			m.lastClickTime = now
			m.lastClickRow = absIdx
			m.lastClickCol = col
			m.clampScroll()
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
	m.mousePressRow = -1
	m.mousePressCol = -1
	m.mousePressBlock = false
	m.clampScroll()
	return m, nil
}

// splitTableStartY returns the terminal Y row where viewSplitTable() begins.
func (m Model) splitTableStartY() int {
	if len(m.tabs) > 1 {
		return tabBarRow + tabBarChromeRows
	}
	return tabBarRow
}

// tableDataStartY returns the terminal Y row where log data rows begin in single-table view.
func (m Model) tableDataStartY() int {
	return m.splitTableStartY() + splitPaneHeaderOverhead
}

func (m Model) handleSplitTableMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	splitStartY := m.splitTableStartY()
	totalH := m.tableHeight + splitPaneHeaderOverhead
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

		dataStartY := splitStartY + splitPaneHeaderOverhead
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
				col := msg.X - paneStartX - prefixWidth + m.scrollX
				if col < 0 {
					col = 0
				}
				m.cursorCol = col
				m.charSelStart = -1
				m.charSelEnd = -1
				m.mousePressRow = absIdx
				m.mousePressCol = col
				m.mousePressBlock = msg.Alt || msg.Ctrl

				now := time.Now()
				if m.lastClickRow == absIdx && now.Sub(m.lastClickTime) < doubleClickThreshold {
					plainText := m.selectedRowPlainText(absIdx)
					wStart, wEnd, word := findWordAt(plainText, col)
					if word != "" {
						m.charSelStart = wStart
						m.charSelEnd = wEnd
						_ = clipboard.Copy(word)
						m.message = fmt.Sprintf("✓ Copied '%s' to clipboard", word)
					} else {
						_ = clipboard.Copy(m.visible[absIdx].Raw)
						m.message = "✓ Copied row to clipboard"
					}
				}
				m.lastClickTime = now
				m.lastClickRow = absIdx
				m.lastClickCol = col

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
		dataStartY = splitStartY + splitPaneHeaderOverhead
		dataEndY = splitStartY + topH
	} else {
		dataStartY = dividerY + 1 + splitPaneHeaderOverhead
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
			col := msg.X - prefixWidth + m.scrollX
			if col < 0 {
				col = 0
			}
			m.cursorCol = col
			m.charSelStart = -1
			m.charSelEnd = -1
			m.mousePressRow = absIdx
			m.mousePressCol = col
			m.mousePressBlock = msg.Alt || msg.Ctrl

			now := time.Now()
			if m.lastClickRow == absIdx && now.Sub(m.lastClickTime) < doubleClickThreshold {
				plainText := m.selectedRowPlainText(absIdx)
				wStart, wEnd, word := findWordAt(plainText, col)
				if word != "" {
					m.charSelStart = wStart
					m.charSelEnd = wEnd
					_ = clipboard.Copy(word)
					m.message = fmt.Sprintf("✓ Copied '%s' to clipboard", word)
				} else {
					_ = clipboard.Copy(m.visible[absIdx].Raw)
					m.message = "✓ Copied row to clipboard"
				}
			}
			m.lastClickTime = now
			m.lastClickRow = absIdx
			m.lastClickCol = col

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
	m.toggleFollow()
	return m, nil
}

func (m Model) handleMouseMotion(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal && m.mode != modeSearch && m.mode != modeFilter && m.mode != modeTimeJump {
		return m, nil
	}

	if msg.Button == tea.MouseButtonNone {
		m.isDraggingVScroll = false
		m.isDraggingHScroll = false
	}

	// Dragging vertical scrollbar
	if m.isDraggingVScroll && len(m.visible) > m.tableHeight && m.tableHeight > 1 {
		dataStartY := m.tableDataStartY()
		if m.splitMode != SplitNone {
			dataStartY = m.splitTableStartY() + splitPaneHeaderOverhead
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
	if m.isDraggingHScroll {
		return m.handleHScrollbarClick(msg)
	}

	if m.selectionStart < 0 || len(m.visible) == 0 {
		return m, nil
	}

	dataStartY := m.tableDataStartY()
	dataEndY := dataStartY + m.tableHeight
	paneStartX := 0
	if m.splitMode == SplitVertical {
		dataStartY = m.splitTableStartY() + splitPaneHeaderOverhead
		dataEndY = m.splitTableStartY() + m.tableHeight + splitPaneHeaderOverhead
		splitX := (m.width - 1) / 2
		if m.activePane == 1 {
			paneStartX = splitX + 1
		}
	} else if m.splitMode == SplitHorizontal {
		splitStartY := m.splitTableStartY()
		totalH := m.tableHeight + splitPaneHeaderOverhead
		availH := totalH - 1
		topH := availH / 2
		dividerY := splitStartY + topH
		if m.activePane == 0 {
			dataStartY = splitStartY + splitPaneHeaderOverhead
			dataEndY = splitStartY + topH
		} else {
			dataStartY = dividerY + 1 + splitPaneHeaderOverhead
			dataEndY = splitStartY + totalH
		}
	}

	col := msg.X - paneStartX - prefixWidth + m.scrollX
	if col < 0 {
		col = 0
	}

	isBlock := m.mousePressBlock || msg.Alt || msg.Ctrl

	if msg.Y < dataStartY {
		// Dragging above table viewport: auto-scroll up
		if m.scrollOffset > 0 {
			m.scrollOffset--
			m.clampScroll()
		}
		m.selectionEnd = m.scrollOffset
		m.cursorCol = col
		m.follow = false
		if isBlock && m.mousePressCol >= 0 {
			m.charSelStart = m.mousePressCol
			m.charSelEnd = col
			top, bot := m.selectionStart, m.selectionEnd
			if top > bot {
				top, bot = bot, top
			}
			m.message = fmt.Sprintf("%d rows block selected (release to copy)", bot-top+1)
		} else {
			m.charSelStart = -1
			m.charSelEnd = -1
			top, bot := m.selectionStart, m.selectionEnd
			if top > bot {
				top, bot = bot, top
			}
			m.message = fmt.Sprintf("%d rows selected (press y or Ctrl+C to copy)", bot-top+1)
		}
		return m, nil
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
		m.cursorCol = col
		m.follow = false
		if isBlock && m.mousePressCol >= 0 {
			m.charSelStart = m.mousePressCol
			m.charSelEnd = col
			top, bot := m.selectionStart, m.selectionEnd
			if top > bot {
				top, bot = bot, top
			}
			m.message = fmt.Sprintf("%d rows block selected (release to copy)", bot-top+1)
		} else {
			m.charSelStart = -1
			m.charSelEnd = -1
			top, bot := m.selectionStart, m.selectionEnd
			if top > bot {
				top, bot = bot, top
			}
			m.message = fmt.Sprintf("%d rows selected (press y or Ctrl+C to copy)", bot-top+1)
		}
		return m, nil
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
		m.cursorCol = col
		m.follow = false

		if absIdx == m.mousePressRow && m.mousePressCol >= 0 {
			// Horizontal drag on the same row: character range selection
			m.selectionStart = absIdx
			m.selectionEnd = absIdx
			m.selectedRow = absIdx
			m.cursorCol = col
			m.charSelStart = m.mousePressCol
			m.charSelEnd = col
			minSel, maxSel := m.charSelStart, m.charSelEnd
			if minSel > maxSel {
				minSel, maxSel = maxSel, minSel
			}
			m.message = fmt.Sprintf("%d chars selected", maxSel-minSel)
			return m, nil
		}

		if isBlock && m.mousePressCol >= 0 {
			// Drag across rows with modifier (Option/Alt or Ctrl): rectangular block selection
			m.selectedRow = absIdx
			m.selectionStart = m.mousePressRow
			m.charSelStart = m.mousePressCol
			m.charSelEnd = col
			top := m.selectionStart
			bot := m.selectionEnd
			if top > bot {
				top, bot = bot, top
			}
			m.message = fmt.Sprintf("%d rows block selected (release to copy)", bot-top+1)
			return m, nil
		}

		// Drag across rows without modifier: select entire rows like a normal text editor
		m.selectedRow = absIdx
		m.selectionStart = m.mousePressRow
		m.selectionEnd = absIdx
		m.charSelStart = -1
		m.charSelEnd = -1
		top := m.selectionStart
		bot := m.selectionEnd
		if top > bot {
			top, bot = bot, top
		}
		m.message = fmt.Sprintf("%d rows selected (press y or Ctrl+C to copy)", bot-top+1)
		return m, nil
	}
	if start, end := m.selectionRange(); start >= 0 && end >= 0 {
		m.message = m.selectionMessage(end-start+1, "press y or Ctrl+C to copy")
	}
	return m, nil
}

func (m Model) handleMouseRelease(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.isDraggingVScroll = false
	m.isDraggingHScroll = false
	if m.mode != modeNormal && m.mode != modeSearch && m.mode != modeFilter && m.mode != modeTimeJump {
		return m, nil
	}

	// 1. Single row or multi-row block character selection: auto-copy on release
	if m.charSelStart >= 0 && m.charSelEnd >= 0 && m.charSelStart != m.charSelEnd {
		text := m.selectedText()
		if text != "" {
			_ = clipboard.Copy(text)
			lines := strings.Split(text, "\n")
			if len(lines) == 1 {
				m.message = fmt.Sprintf("✓ Copied '%s' to clipboard", text)
			} else {
				m.message = fmt.Sprintf("✓ Copied %d rows block to clipboard", len(lines))
			}
		}
	} else if start, end := m.selectionRange(); start >= 0 && end >= 0 {
		m.selectedRow = m.selectionEnd
		m.message = m.selectionMessage(end-start+1, "press y or Ctrl+C to copy")
	} else {
		m.selectionStart = -1
		m.selectionEnd = -1
		m.charSelStart = -1
		m.charSelEnd = -1
	}

	m.mousePressRow = -1
	m.mousePressCol = -1
	m.mousePressBlock = false
	m.clampScroll()
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
	}
	m.clampScroll()

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
	}
	m.clampScroll()

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

func (m Model) handleSelectAll() (tea.Model, tea.Cmd) {
	if len(m.visible) == 0 {
		return m, nil
	}
	m.follow = false
	m.selectionStart = 0
	m.selectionEnd = len(m.visible) - 1
	m.selectedRow = m.selectionEnd
	m.charSelStart = -1
	m.charSelEnd = -1
	m.mousePressRow = -1
	m.mousePressCol = -1
	m.mousePressBlock = false
	count := len(m.visible)
	if count > 1 {
		m.message = m.selectionMessage(count, "press y or Ctrl+C to copy")
	} else {
		m.message = "1 row selected (press y or Ctrl+C to copy)"
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
