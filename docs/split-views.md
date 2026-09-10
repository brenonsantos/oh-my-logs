# Dual-Pane Split Views & Chronological Sync Guide

Embedded systems debugging often requires monitoring two different aspects of a system simultaneously—such as observing sensor interrupts in one view while tracking high-level network state in another, or comparing an error filter against the full raw stream.

`oml` provides native **Dual-Pane Split Views** with optional **Synchronized Chronological Scrolling**.

---

## 1. Opening & Toggling Split Views

| Key | Action | Description |
|-----|--------|-------------|
| `\|` (pipe) | Toggle Vertical Split | Splits the terminal into side-by-side Left and Right panes. Press again to collapse back to single view. |
| `_` (underscore) | Toggle Horizontal Split | Splits the terminal into Top and Bottom panes. Press again to collapse back to single view. |
| `w` | Switch Focused Pane | Toggles active keyboard/mouse focus between Pane 1 and Pane 2. |
| `Tab` | Switch Tab / Pane | Cycles tabs, or shifts pane focus when multi-pane is active. |

---

## 2. Independent Tab Assignment

Each pane displays an independent Virtual Tab:
- **Left / Top Pane (P1)**: Marked with `[P1]` in the tab bar.
- **Right / Bottom Pane (P2)**: Marked with `[P2]` in the tab bar.

### Assigning Tabs to Panes
1. Press **`w`** to focus the desired pane.
2. Press **`Tab`**, **`]`**, or number keys (`1`..`9`) to select which tab should display in that pane.

### Common Multi-Pane Configurations
- **Raw vs. Errors**: Left pane displays the unfiltered raw stream (`Tab 1: All`), right pane displays critical faults (`Tab 2: level:err`).
- **Subsystem Correlation**: Left pane displays MCU state (`module:mcu`), right pane displays Bluetooth/Wi-Fi modem telemetry (`module:radio`).
- **Noise Analysis**: Left pane displays normal traffic, right pane displays heartbeats and ping latency.

---

## 3. Synchronized Chronological Scrolling (`S`)

When inspecting past anomalies, manually scrolling two separate panes to the exact same point in time is tedious. 

Press **`S`** to toggle **Synchronized Chronological Scrolling**:

```text
⟷ SYNC ON
```

### How Chronological Sync Works
1. Select a row or scroll in the active pane.
2. The inactive pane automatically looks up the timestamp of the focused row and **jumps to the chronologically closest log record** in its own view.
3. If logs arrive at different rates across filters, binary search matching finds the exact nearest point in time.

Press **`S`** again to disable synchronization (`⟷ SYNC OFF`) and scroll panes independently.

---

## 4. Status Bar Indicators

When split view is active, the bottom status bar provides continuous context:

```text
SPLIT [VERT]  │  ⟷ SYNC ON  │  focus: Left [All]  │  1420 records  │  ⏱ BOTH  │  FOLLOW
```

- **Split Mode**: `SPLIT [VERT]` or `SPLIT [HORIZ]`.
- **Sync Status**: `⟷ SYNC ON` (green) or `⟷ SYNC OFF` (muted).
- **Focus**: `focus: Left [TabName]` or `focus: Right [TabName]`.
