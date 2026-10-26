# Master Keybindings & Interactions Reference

A complete reference of all keyboard shortcuts, mouse gestures, and interactive prompts supported in `oml`.

---

## 1. Virtual Tabs & Panes

| Key | Action | Description |
|-----|--------|-------------|
| `Ctrl+T` | Create New Tab | Prompts for filter expression and creates a concurrent virtual tab. |
| `Tab` | Next Tab / Switch Pane | Cycles to next tab, or toggles pane focus in split view. |
| `Shift+Tab` | Previous Tab | Cycles to previous tab. |
| `1` .. `9` | Jump to Tab N | Jump directly to tab index 1 through 9. |
| `Ctrl+W` | Close Tab | Closes active virtual tab (preserves default tab). |
| `\|` (pipe) | Toggle Vertical Split | Splits terminal into side-by-side Left and Right panes. |
| `_` (underscore) | Toggle Horizontal Split | Splits terminal into stacked Top and Bottom panes. |
| `w` | Switch Pane Focus | Toggles active keyboard/mouse focus between panes. |
| `S` | Toggle Sync Scroll | Toggles Synchronized Chronological Scrolling between panes. |

---

## 2. Navigation, Scrolling & 2D Viewport

| Key | Action | Description |
|-----|--------|-------------|
| `↑` / `k` | Scroll Up | Move selection or scroll up one row. |
| `↓` / `j` | Scroll Down | Move selection or scroll down one row. |
| `←` / `h` | Cursor Left | Move character cursor left; auto-pans viewport when reaching margin. |
| `→` / `l` | Cursor Right | Move character cursor right; auto-pans viewport when reaching margin. |
| `{` / `}` | Pan Viewport Left / Right | Shift 2D horizontal viewport column window left / right. |
| `Wheel` / Trackpad | Smooth Scroll | Smooth vertical scrolling; trackpad tilt / Shift+Wheel pans horizontally. |
| `PgUp` / `PgDn` | Page Up / Down | Scroll one viewport page up / down. |
| `Ctrl+U` / `Ctrl+D` | Half-Page Up / Down | Scroll half viewport page up / down. |
| `g` / `Home` | Jump to Top | Jump to oldest record in buffer (auto-pauses follow). |
| `G` / `End` | Jump to Bottom | Jump to newest record in buffer (resumes follow). |
| `J` | Jump to Timestamp | Open interactive timestamp jump prompt (`-5m`, `+30s`, `14:20:00`). |

---

## 3. Search & Filter

| Key | Action | Description |
|-----|--------|-------------|
| `Ctrl+F` | Open Search Prompt | Search logs with real-time matching and inverted focus highlight (`▶`). |
| `Enter` / `↓` | Next Search Match | Jump to next search match (while search input active). |
| `↑` | Previous Search Match | Jump to previous search match (while search input active). |
| `n` / `N` | Next / Prev Match | Navigate through search matches in normal viewing mode. |
| `f` | Edit Tab Filter | Edit Logcat-style filter for active tab (`field:val -exclude`). |
| `F` / `Ctrl+P` | Filter Presets Modal | Open saved presets picker modal; press `Ctrl+S` to save current filter. |
| `Ctrl+P` / `↑` | Previous Filter History | Recall previous filter query from history (in filter prompt). |
| `Ctrl+N` / `↓` | Next Filter History | Navigate forward in filter query history (in filter prompt). |
| `Ctrl+V` | Paste Clipboard | Paste text from system clipboard into search or filter prompt. |
| `Esc` | Cancel / Clear Search | Clear search highlighting or dismiss active input prompt. |

---

## 4. Selection, Copying & Clipboard

| Key / Gesture | Action | Description |
|---------------|--------|-------------|
| `Click` | Select Row | Selects single row, places character cursor, and pauses follow. |
| `2x Click` | Copy Word | Double-clicking copies the clicked word / token to clipboard. |
| `Horizontal Drag` | Select Characters | Dragging horizontally on same row selects characters; auto-copies on release. |
| `Vertical Drag` | Select Full Rows | Dragging across rows without modifier selects entire lines; copy with `Ctrl+C` or `y`. |
| `Option+Drag` (Mac)<br>`Alt+Drag` / `Ctrl+Drag` | Rectangular Block Select | Dragging with modifier selects a rectangular column/square block across rows; auto-copies on release! |
| `Shift+←` / `Shift+→` | Select Characters | Extend character selection left or right with keyboard. |
| `Shift+↑` / `Shift+↓` (`K` / `J`)| Extend Line Selection | Expand multi-line selection up or down; calculates live $\Delta t$ on status bar. |
| `Ctrl+A` | Select All Records | Selects all records in current view; ready to copy with `y` or `Ctrl+C`. |
| `y` / `Ctrl+C` | Copy Selection | Copies active character span, rectangular block, or selected lines to clipboard. |
| `Y` | Copy Raw Record | Copies raw unformatted log record under cursor. |
| `Esc` | Clear Selection | First press clears character selection; second press clears row selection. |

---

## 5. Timing, Delta-Time (`Δt`) & Bench Markers

| Key | Action | Description |
|-----|--------|-------------|
| `t` | Cycle Timestamp Mode | Cycle: `Clock` $\rightarrow$ `Δt` $\rightarrow$ `Both` $\rightarrow$ `OFF`. |
| `m` / `M` | Add Bench Marker | Inject custom timestamped checkpoint note into stream (`📌 MARKER`). |
| `Shift+↑/↓` or `Drag` | Stopwatch Delta | Real-time interval calculation across selection (`• 7 rows selected · Δt: +160.0ms`). |

---

## 6. Bookmarks & Log Pinning

| Key | Action | Description |
|-----|--------|-------------|
| `b` | Toggle Bookmark | Pin/unpin focused row or entire multi-selected batch. |
| `]` | Next Bookmark | Jump chronologically to next pinned record. |
| `[` | Previous Bookmark | Jump chronologically to previous pinned record. |
| `B` (Shift+b) | Bookmarked-Only View | Toggle filter showing only pinned records. |
| `Alt+B` | Clear All Bookmarks | Remove all pinned bookmarks across the buffer. |

---

## 7. Device, Profiles & Modals

| Key | Action | Description |
|-----|--------|-------------|
| `Space` | Pause / Follow Stream | Toggle auto-scroll follow mode on new incoming logs. |
| `Enter` / `v` | Inspect Row Detail | Open modal showing all fields, formatted JSON/XML/YAML, and hex dump. |
| `i` / `:` | Single-line Serial TX | Open command send prompt with line endings (`Ctrl+E`) and history. |
| `I` / `Ctrl+I` | Shell Passthrough | Enter interactive raw shell mode (`<Tab>` completion, arrows, `^C`). |
| `x` / `X` | Switch Display Format | Toggle row rendering: `Parsed` $\rightarrow$ `Raw` $\rightarrow$ `Hex Dump` $\rightarrow$ `Binary Bits`. |
| `o` / `ctrl+o` | Column Customization | Open interactive modal to toggle column visibility and adjust widths. |
| `p` | Serial Port & Baud Picker| Open port picker modal to change device or baud rate. |
| `P` (Shift+p) | Profile Switcher | Open profile switcher modal to re-parse logs under different schema. |
| `r` | Reconnect Port / Restart| Re-open serial port connection or restart custom process stream. |
| `D` | Disconnect Port | Release serial port handle (freeing UART for `west flash` or `pyocd`). |
| `,` / `C` | Settings & Preferences | Open settings modal (themes, direct-to-disk recording, buffer capacity). |
| `s` | Save Log | Save entire buffer to timestamped `.log` file on disk. |
| `c` | Clear Buffer | Reset in-memory log buffer across all tabs. |
| `?` | Help Modal | Open quick reference cheat sheet modal. |
| `Ctrl+G` | Mini-Games | Embedded retro games (Snake, Pong, Aim Trainer Blitz, etc.) during long compilation/flashes. |
| `q` | Quit | Cleanly exit `oml`. |
