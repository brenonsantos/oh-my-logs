# Bookmarks & Log Pinning Guide

During long-running embedded test runs or complex boot sequences, finding and tracking specific milestones across tens of thousands of log records can be challenging.

`oml` provides a complete **Bookmarks & Log Pinning System**.

---

## 1. Pinning Logs (`b`)

### Single Row Pinning
1. Click any log line or navigate using arrow keys (`↑` / `↓`).
2. Press **`b`** to toggle its pinned status.
3. A distinct yellow star (**`★`**) appears in the gutter prefix.

### Multi-Row Batch Pinning
If you have selected a range of lines using mouse dragging or **`Shift+↑`** / **`Shift+↓`**:
- Press **`b`** to toggle bookmarks across the **entire selected batch** at once.

---

## 2. Navigating Between Bookmarks (`[` and `]`)

You can instantly jump through marked events in chronological order without scrolling:

- Press **`]`** to jump forward to the next pinned log.
- Press **`[`** to jump backward to the previous pinned log.
- Navigation automatically wraps around the beginning and end of the buffer and centers the bookmarked record in view.

---

## 3. Filter to Bookmarked-Only View (`B`)

Press **`Shift+B`** (**`B`**) to isolate all pinned records:
- The table viewport instantly collapses to display **only** bookmarked rows.
- The status bar badge updates:
  ```text
  ★ 4 pinned (only)
  ```
- Press **`B`** again to return to the normal stream view.

---

## 4. Clearing All Bookmarks (`Alt+B`)

Press **`Alt+B`** (or `Option+B` on macOS) to remove all bookmarks across the buffer and reset the bookmark index.

---

## 5. Keyboard Reference

| Key | Action |
|-----|--------|
| `b` | Toggle bookmark on focused row or selected batch |
| `]` | Jump to next bookmarked record |
| `[` | Jump to previous bookmarked record |
| `B` (Shift+b) | Toggle "Bookmarked Only" view filter |
| `Alt+B` | Clear all bookmarks across the buffer |
