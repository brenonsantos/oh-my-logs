package tui

import (
	"fmt"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
)

// SplitMode defines the dual-pane view state.
type SplitMode int

const (
	SplitNone       SplitMode = iota // Standard single-pane view
	SplitVertical                    // Side-by-side split (left & right)
	SplitHorizontal                  // Stacked split (top & bottom)
)

// toggleSplit switches between SplitNone and the requested split mode (SplitVertical or SplitHorizontal).
func (m *Model) toggleSplit(mode SplitMode) {
	if m.splitMode == mode {
		// Close split mode — resume single tab with the currently focused tab
		m.syncActiveTabToModel()
		m.activeTab = m.activeTabIdx()
		m.splitMode = SplitNone
		m.activePane = 0
		m.syncModelToActiveTab()
		m.recalcLayout()
		m.clampScroll()
		m.message = "Split view closed (single tab)"
		return
	}

	m.syncActiveTabToModel()
	if len(m.tabs) == 1 {
		// Automatically create a second tab if only 1 exists
		initFilter, _ := filter.New("")
		m.tabs = append(m.tabs, Tab{
			Name:      "Tab 2",
			FilterRaw: "",
			Filter:    initFilter,
			ViewportState: ViewportState{
				Visible:      m.buffer.All(),
				Follow:       true,
				SelectedRow:  -1,
				CursorCol:    -1,
				CharSelStart: -1,
				CharSelEnd:   -1,
			},
		})
		m.splitLeftTab = 0
		m.splitRightTab = 1
	} else {
		m.splitLeftTab = m.activeTab
		if m.splitRightTab == m.splitLeftTab || m.splitRightTab < 0 || m.splitRightTab >= len(m.tabs) {
			m.splitRightTab = (m.splitLeftTab + 1) % len(m.tabs)
		}
	}

	m.splitMode = mode
	m.activePane = 0
	m.syncScroll = true // default to chronological sync on split
	m.recalcLayout()

	// Ensure viewports and follow offsets match the split pane heights
	for p := 0; p < 2; p++ {
		t := m.currentTabForPane(p)
		if t != nil {
			h := m.paneDataHeight(p)
			if t.Follow && len(t.Visible) > h {
				t.ScrollOffset = len(t.Visible) - h
			} else {
				maxO := len(t.Visible) - h
				if maxO < 0 {
					maxO = 0
				}
				if t.ScrollOffset > maxO {
					t.ScrollOffset = maxO
				}
			}
		}
	}

	m.syncModelToActiveTab()
	m.syncOtherPaneChronologically()
	m.clampScroll()
	if mode == SplitVertical {
		m.message = "Split view: Vertical (side-by-side) · [S] Sync ON · [w] Switch pane"
	} else {
		m.message = "Split view: Horizontal (stacked) · [S] Sync ON · [w] Switch pane"
	}
}

// switchPaneFocus toggles focus between primary pane (0) and secondary pane (1) without moving tabs.
func (m *Model) switchPaneFocus() {
	if m.splitMode == SplitNone {
		return
	}
	oldInsp := m.isInspectorActive()
	m.syncActiveTabToModel()
	if m.activePane == 0 {
		m.activePane = 1
	} else {
		m.activePane = 0
	}
	m.syncModelToActiveTab()
	if m.isInspectorActive() != oldInsp {
		m.recalcLayout()
	}
	m.clampScroll()

	paneName := "Left"
	if m.splitMode == SplitHorizontal {
		paneName = "Top"
		if m.activePane == 1 {
			paneName = "Bottom"
		}
	} else if m.activePane == 1 {
		paneName = "Right"
	}
	cur := m.currentTab()
	m.message = fmt.Sprintf("Focus: %s Pane [%s]", paneName, cur.DisplayName(m.activeTabIdx()+1))
}

// toggleSyncScroll toggles chronological time-locked scrolling on/off.
func (m *Model) toggleSyncScroll() {
	m.syncScroll = !m.syncScroll
	if m.syncScroll {
		m.message = "Chronological Sync: ON"
		m.syncOtherPaneChronologically()
	} else {
		m.message = "Chronological Sync: OFF (Independent scrolling)"
	}
}

// currentTabForPane returns the tab assigned to the given pane index (0 = left/top, 1 = right/bottom).
func (m *Model) currentTabForPane(pane int) *Tab {
	idx := m.paneTabIdx(pane)
	if idx < 0 || idx >= len(m.tabs) {
		return m.currentTab()
	}
	return &m.tabs[idx]
}

// paneTabIdx returns the tab index in m.tabs assigned to pane 0 or 1.
// Positions are frozen: pane 0 is always splitLeftTab, pane 1 is always splitRightTab.
func (m *Model) paneTabIdx(pane int) int {
	if pane == 1 {
		if m.splitRightTab >= 0 && m.splitRightTab < len(m.tabs) {
			return m.splitRightTab
		}
		if len(m.tabs) > 1 {
			return 1
		}
		return 0
	}
	if m.splitLeftTab >= 0 && m.splitLeftTab < len(m.tabs) {
		return m.splitLeftTab
	}
	return 0
}

// paneDataHeight returns the number of visible log data rows for the given pane (0 or 1).
func (m *Model) paneDataHeight(pane int) int {
	if m.splitMode != SplitHorizontal {
		h := m.tableHeight
		if h < 1 {
			return 1
		}
		return h
	}

	totalH := m.tableHeight + 2
	availH := totalH - 1
	topH := availH / 2
	if topH < 3 {
		topH = 3
	}
	bottomH := availH - topH
	if bottomH < 3 {
		bottomH = 3
	}

	if pane == 1 {
		h := bottomH - 2
		if h < 1 {
			return 1
		}
		return h
	}
	h := topH - 2
	if h < 1 {
		return 1
	}
	return h
}

// activeDataHeight returns the number of visible log data rows in the currently focused pane.
func (m *Model) activeDataHeight() int {
	return m.paneDataHeight(m.activePane)
}
