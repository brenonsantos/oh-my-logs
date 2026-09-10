# Master Keybindings & Interactions Reference

A complete reference of all keyboard shortcuts, mouse gestures, and interactive prompts supported in `oml`.

---

## 1. Virtual Tabs & Panes

| Key | Action |
|-----|--------|
| `Ctrl+T` | Create a new virtual tab (prompts for filter expression). |
| `Tab` | Switch to next tab (or switch focused pane in split view). |
| `Shift+Tab` | Switch to previous tab. |
| `1` .. `9` | Jump directly to tab index 1 through 9. |
| `Ctrl+W` | Close active virtual tab (preserves default tab). |
| `\|` (pipe) | Toggle Vertical Dual-Pane Split view (side-by-side). |
| `_` (underscore) | Toggle Horizontal Dual-Pane Split view (stacked). |
| `w` | Switch focus between Left/Right (or Top/Bottom) pane. |
| `S` | Toggle Synchronized Chronological Scrolling between panes. |

---

## 2. Navigation & Scrolling

| Key | Action |
|-----|--------|
| `↑` / `k` | Scroll up one row. |
| `↓` / `j` | Scroll down one row. |
| `Wheel` / Trackpad | Smooth vertical scrolling. |
| `PgUp` / `PgDn` | Scroll one page up / down. |
| `Ctrl+U` / `Ctrl+D` | Scroll half page up / down. |
| `g` / `Home` | Jump to top (oldest record in buffer, auto-pauses follow). |
| `G` / `End` | Jump to bottom (newest record in buffer, resumes follow). |

---

## 3. Search & Filter

| Key | Action |
|-----|--------|
| `Ctrl+F` | Open in-view substring search prompt. |
| `Enter` / `↓` | Next search match (in search prompt). |
| `↑` | Previous search match (in search prompt). |
| `n` / `N` | Next / previous search match (in normal mode). |
| `f` | Edit filter for current tab (auto-names tab to filter). |
| `F` (Shift+f) | Open Filter Presets modal. |
| `Ctrl+P` / `↑` | Previous filter from history (in filter prompt). |
| `Ctrl+N` / `↓` | Next filter from history (in filter prompt). |
| `Ctrl+V` | Paste clipboard text directly into search or filter prompt. |
| `Esc` | Clear search highlighting or cancel input prompt. |

---

## 4. Timing & Delta-Time (`Δt`)

| Key | Action |
|-----|--------|
| `t` | Cycle timestamp display: `Clock` $\rightarrow$ `Δt` $\rightarrow$ `Both` $\rightarrow$ `OFF`. |
| `Mouse Drag` | Select batch of rows; displays live elapsed $\Delta t$ on bottom bar. |
| `Shift+↑` / `Shift+↓` | Expand selection across rows; displays live elapsed $\Delta t$ on bottom bar. |

---

## 5. Selection, Copy & Bookmarks

| Key | Action |
|-----|--------|
| `Click` | Select row and auto-pause follow. |
| `Double Click` | Instant copy of raw row to system clipboard. |
| `y` | Yank (copy) selected row or multi-row range to clipboard. |
| `Y` | Yank raw unformatted log record. |
| `b` | Toggle bookmark (pin) on focused row or selected batch. |
| `]` | Jump to next bookmarked record. |
| `[` | Jump to previous bookmarked record. |
| `B` (Shift+b) | Toggle "Bookmarked Only" view filter. |
| `Alt+B` | Clear all bookmarks across the buffer. |

---

## 6. Device, Replay & Actions

| Key | Action |
|-----|--------|
| `Space` | Pause / resume stream follow. |
| `i` | Open interactive Serial TX send prompt. |
| `Ctrl+E` | Cycle serial line ending (`CRLF`, `LF`, `CR`, `None`) in TX prompt. |
| `c` | Clear in-memory log buffer across all tabs. |
| `s` | Save all raw buffer records to a timestamped `.log` file. |
| `p` | Open Serial Port & Baud rate picker modal. |
| `P` (Shift+p) | Open Profile switcher modal (re-parses buffer). |
| `r` | Reconnect current serial port. |
| `?` | Open Help & Keyboard Shortcuts popup. |
| `q` / `Ctrl+C` | Clean exit. |

---

## 7. Easter Egg Mini-Game (`G`)

| Key | Action |
|-----|--------|
| `G` (Shift+g) | Launch embedded Pong mini-game during long firmware builds/flashes. |
| `↑` / `↓` or `W` / `S` | Move paddle. |
| `Esc` / `q` | Exit game and return to logs. Background serial data remains fully buffered. |
