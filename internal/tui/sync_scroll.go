package tui

import (
	"sort"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// findLastRecordBeforeOrAtID searches sorted records for the index of the last
// record whose Record.ID <= targetID.
// If targetID < records[0].ID, returns 0 (earliest available record).
// If targetID >= records[n-1].ID, returns n - 1 (latest available record).
func findLastRecordBeforeOrAtID(records []record.Record, targetID uint64) int {
	n := len(records)
	if n == 0 {
		return -1
	}
	if targetID < records[0].ID {
		return 0
	}
	if targetID >= records[n-1].ID {
		return n - 1
	}

	// sort.Search finds smallest index i where records[i].ID > targetID
	idx := sort.Search(n, func(i int) bool {
		return records[i].ID > targetID
	})

	if idx > 0 {
		return idx - 1
	}
	return 0
}

// findClosestRecordIndex searches sorted records for the index of the record
// chronologically aligned with targetID (last visible record with ID <= targetID).
func findClosestRecordIndex(records []record.Record, targetID uint64) int {
	return findLastRecordBeforeOrAtID(records, targetID)
}

// syncOtherPaneChronologically aligns the non-active pane's viewport so it
// displays records chronologically matching the active pane's view.
func (m *Model) syncOtherPaneChronologically() {
	if !m.syncScroll || m.splitMode == SplitNone || len(m.tabs) < 2 {
		return
	}

	m.syncActiveTabToModel()

	srcTab := m.currentTab()
	if len(srcTab.Visible) == 0 {
		return
	}

	dstPane := 1
	if m.activePane == 1 {
		dstPane = 0
	}
	dstTab := m.currentTabForPane(dstPane)
	if dstTab == nil || len(dstTab.Visible) == 0 {
		return
	}

	// Determine chronological anchor record in srcTab
	anchorIdx := srcTab.SelectedRow
	if anchorIdx < 0 || anchorIdx >= len(srcTab.Visible) {
		anchorIdx = srcTab.ScrollOffset
		if anchorIdx >= len(srcTab.Visible) {
			anchorIdx = len(srcTab.Visible) - 1
		}
	}
	anchorID := srcTab.Visible[anchorIdx].ID

	// Find the last visible record in dstTab whose ID <= anchorID
	targetIdx := findLastRecordBeforeOrAtID(dstTab.Visible, anchorID)
	if targetIdx < 0 {
		return
	}

	paneHeight := m.paneDataHeight(dstPane)

	// Always place cursor on the last visible record on that tab
	dstTab.SelectedRow = targetIdx
	dstTab.Follow = srcTab.Follow

	// Only scroll dstTab if targetIdx moves outside its visible viewport:
	// "The idea was to let Tab B stay still unless Tab A goes past that Tab B Id."
	maxOffset := len(dstTab.Visible) - paneHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	if targetIdx < dstTab.ScrollOffset {
		dstTab.ScrollOffset = targetIdx
	} else if targetIdx >= dstTab.ScrollOffset+paneHeight {
		dstTab.ScrollOffset = targetIdx - paneHeight + 1
	}

	if dstTab.ScrollOffset > maxOffset {
		dstTab.ScrollOffset = maxOffset
	}
	if dstTab.ScrollOffset < 0 {
		dstTab.ScrollOffset = 0
	}
}
