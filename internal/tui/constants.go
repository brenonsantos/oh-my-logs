package tui

import "time"

// Layout & Screen Chrome Constants
const (
	// fixedChromeRows represents the non-data rows in single-tab view:
	// 1 title bar + 1 divider + 1 column header + 1 divider + 1 horizontal scrollbar/divider + 1 status bar + 1 key hints bar.
	fixedChromeRows = 7

	// tabBarChromeRows is the additional height added when multiple tabs exist (1 tab bar + 1 divider).
	tabBarChromeRows = 2

	// splitPaneHeaderOverhead is the row overhead for each split pane (1 pane header + 1 pane divider).
	splitPaneHeaderOverhead = 2

	// minTableHeight is the minimum rows allocated to the scrollable table body.
	minTableHeight = 3

	// minDrawerAvailHeight is the minimum total available space required before allocating the inspector drawer.
	minDrawerAvailHeight = 10

	// drawerHeightPercent is the percentage of available height allocated to the inspector drawer.
	drawerHeightPercent = 35

	// minDrawerHeight is the minimum height (in terminal rows) of the inspector drawer.
	minDrawerHeight = 4

	// maxDrawerHeight is the maximum height (in terminal rows) of the inspector drawer.
	maxDrawerHeight = 10

	// inspectorDividerRows is the row height of the inspector metadata header/divider.
	inspectorDividerRows = 1
)

// Column Layout & Sizing Constants
const (
	// prefixWidth is the visual character width of the row indicator prefix:
	// cursor indicator (1) + bookmark indicator (1) + spacing gap (1).
	prefixWidth = 3

	// colGap is the spacing between adjacent columns in the table ("  ").
	colGap = 2

	// minFlexColWidth is the minimum width allocated to an auto-flex column (e.g. message or raw).
	minFlexColWidth = 8

	// narrowDeficitThreshold is the available flex space below which hex/bin columns will shrink.
	narrowDeficitThreshold = 15

	// minHexBinWidth is the lower bound width when shrinking hex or binary columns in narrow viewports.
	minHexBinWidth = 18

	// timestampColWidth is the default column width for the arrival timestamp.
	timestampColWidth = 14

	// deltaColWidth is the default column width for the delta time column.
	deltaColWidth = 10

	// rawLenColWidth is the column width for the byte length column in hex/binary views.
	rawLenColWidth = 6

	// hexDumpColWidth is the standard column width for 16-byte hex dump pairs.
	hexDumpColWidth = 48

	// binaryBitsColWidth is the standard column width for binary bits view.
	binaryBitsColWidth = 36
)

// Modal Dialog Dimension Constants
const (
	// modalWidthPortPicker is the width for the serial port & baud selection modal.
	modalWidthPortPicker = 68

	// modalWidthSettings is the width for the configuration & settings dialog.
	modalWidthSettings = 52

	// modalWidthFilters is the width for the filter presets dialog.
	modalWidthFilters = 56

	// modalWidthHelp is the width for the keyboard shortcuts help dialog.
	modalWidthHelp = 89

	// minSavePresetInputWidth is the minimum visual width for the preset name input prompt.
	minSavePresetInputWidth = 10

	// savePresetPromptLabelWidth is the character offset reserved for "Preset Name: ".
	savePresetPromptLabelWidth = 17
)

// Scrollbar & Interaction Constants
const (
	// minScrollTrackWidth is the minimum track width required to display a horizontal scrollbar.
	minScrollTrackWidth = 10

	// minScrollThumbWidth is the minimum character width of a scrollbar thumb.
	minScrollThumbWidth = 3

	// doubleClickThreshold is the maximum duration between two clicks on the same row to register a double-click.
	doubleClickThreshold = 400 * time.Millisecond
)
