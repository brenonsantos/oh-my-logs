package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	} else if m.mode == modeFilterPresets {
		sb.WriteString(m.viewFilterPresetsModal())
	} else if m.mode == modeSavePresetPrompt {
		sb.WriteString(m.viewSavePresetModal())
	} else if m.mode == modeSettings {
		sb.WriteString(m.viewSettingsModal())
	} else if m.mode == modeColumnModal {
		sb.WriteString(m.viewColumnModal())
	} else if m.mode == modeThemeModal {
		sb.WriteString(m.viewThemeModal())
	} else if m.mode == modeFilePicker {
		sb.WriteString(m.viewFilePickerModal())
	} else if m.mode == modeHelp {
		sb.WriteString(m.viewHelpModal())
	} else if m.mode == modeRowDetail {
		sb.WriteString(m.viewRowDetailModal())
	} else if m.mode == modeGame {
		sb.WriteString(m.viewGameModal())
	} else if len(m.visible) == 0 && m.splitMode == SplitNone {
		sb.WriteString(m.viewEmptyState())
		if m.mode == modeMarkerPrompt {
			sb.WriteByte('\n')
			sb.WriteString(m.viewMarkerDrawer())
		}
	} else if m.splitMode != SplitNone {
		sb.WriteString(m.viewSplitTable())
		if m.isInspectorActive() && m.inspectorHeight > 0 {
			sb.WriteByte('\n')
			sb.WriteString(m.viewInspectorDivider())
			sb.WriteByte('\n')
			sb.WriteString(m.viewInspectorDrawer())
		}
		if m.mode == modeMarkerPrompt {
			sb.WriteByte('\n')
			sb.WriteString(m.viewMarkerDrawer())
		}
	} else {
		// Table Header
		sb.WriteString(m.viewTableHeader())
		sb.WriteByte('\n')

		// Header Divider
		sb.WriteString(m.viewDivider())
		sb.WriteByte('\n')

		// Table Rows
		sb.WriteString(m.viewTable())

		// Wireshark-style Bottom Inspector Drawer
		if m.isInspectorActive() && m.inspectorHeight > 0 {
			sb.WriteByte('\n')
			sb.WriteString(m.viewInspectorDivider())
			sb.WriteByte('\n')
			sb.WriteString(m.viewInspectorDrawer())
		}

		// Marker Note Bottom Drawer
		if m.mode == modeMarkerPrompt {
			sb.WriteByte('\n')
			sb.WriteString(m.viewMarkerDrawer())
		}
	}

	sb.WriteByte('\n')

	// 4. Divider before status bar (with horizontal scrollbar)
	sb.WriteString(m.viewHorizontalScrollbarDivider())
	sb.WriteByte('\n')

	// 5. Status bar
	sb.WriteString(m.viewStatusBar())
	sb.WriteByte('\n')

	// 6. Key hints / input bar
	sb.WriteString(m.viewKeyBar())

	rendered := sb.String()
	if m.height <= 0 {
		return rendered
	}

	// Hard layout clamp: Guarantee exact m.height lines and visual width <= m.width
	lines := strings.Split(rendered, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	for i, l := range lines {
		if lipgloss.Width(l) > m.width {
			lines[i] = ansiCut(l, 0, m.width)
		}
	}
	return strings.Join(lines, "\n")
}
