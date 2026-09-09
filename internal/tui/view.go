package tui

import (
	"strings"
)

// View renders the full terminal UI.
func (m Model) View() string {
	if m.width == 0 {
		return "Initializing…"
	}

	var sb strings.Builder

	// 1. Title bar
	sb.WriteString(m.viewTitleBar())
	sb.WriteByte('\n')

	// 2. Divider line under title bar
	sb.WriteString(m.viewDivider())
	sb.WriteByte('\n')

	// 2b. Virtual tab bar (when multiple tabs exist)
	if len(m.tabs) > 1 {
		sb.WriteString(m.viewTabBar())
		sb.WriteByte('\n')
		sb.WriteString(m.viewDivider())
		sb.WriteByte('\n')
	}

	// 3. Middle area: Modals or Table
	if m.mode == modePortPicker {
		sb.WriteString(m.viewPortPickerModal())
	} else if m.mode == modeProfilePicker {
		sb.WriteString(m.viewProfilePickerModal())
	} else if m.mode == modeHelp {
		sb.WriteString(m.viewHelpModal())
	} else if m.mode == modeGame {
		sb.WriteString(m.viewGameModal())
	} else if len(m.visible) == 0 && m.splitMode == SplitNone {
		sb.WriteString(m.viewEmptyState())
	} else if m.splitMode != SplitNone {
		sb.WriteString(m.viewSplitTable())
	} else {
		// Table Header
		sb.WriteString(m.viewTableHeader())
		sb.WriteByte('\n')

		// Header Divider
		sb.WriteString(m.viewDivider())
		sb.WriteByte('\n')

		// Table Rows
		sb.WriteString(m.viewTable())
	}

	sb.WriteByte('\n')

	// 4. Divider before status bar
	sb.WriteString(m.viewDivider())
	sb.WriteByte('\n')

	// 5. Status bar
	sb.WriteString(m.viewStatusBar())
	sb.WriteByte('\n')

	// 6. Key hints / input bar
	sb.WriteString(m.viewKeyBar())

	return sb.String()
}
