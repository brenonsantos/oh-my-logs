# oh-my-logs (`oml`)

> **Release v1.2.0 — Taubateano**  
> A high-performance, cross-platform terminal UI for monitoring, filtering, measuring latency, and transmitting serial data for embedded systems.

<img width="926" height="676" alt="oh-my-logs main interface" src="https://github.com/user-attachments/assets/beaa2cf3-4b70-4240-a166-a40532f9edb1" />
<img width="926" height="676" alt="oh-my-logs dual pane split view" src="https://github.com/user-attachments/assets/3de5dc14-fcd1-4407-93f6-94f6884aa519" />
<img width="926" height="676" alt="oh-my-logs virtual tabs" src="https://github.com/user-attachments/assets/06956e71-8f40-4830-9ebc-0043e9a4f227" />

---

## Highlights

- **⚡ Virtual Tabs with Independent Filters** — Spawn concurrent tabs (`Ctrl+T`), each with its own logcat filter, visible rows, follow/pause status, and scroll position. Ingests in $O(1)$ across all tabs simultaneously.
- **⏱️ Delta-Time Mode (`Δt`) & Inter-Log Latency** — Adaptive Exponential Moving Average (EMA) baseline engine detects rapid bursts, normal cadence, hiccups, and communication timeouts with dynamic color-coding. Cycle modes with `t`.
- **📏 Batch Selection Stopwatch** — Click-drag or `Shift+↓` across a batch of lines to immediately see the elapsed time span on the bottom bar (`7 selected (Δt: +160.0ms)`).
- **🪟 Dual-Pane Split Views (`|` and `_`)** — Split your terminal vertically or horizontally to compare different subsystems side-by-side, with optional **Synchronized Chronological Scrolling (`S`)**.
- **📡 Interactive Serial TX (Send)** — Send commands directly over serial (`i`), cycle line endings (`Ctrl+E`: CRLF, LF, CR, None), browse command history, and track sent commands highlighted with a distinct Maple Orange `TX` badge.
- **📑 Profile-Driven Columns** — Custom YAML profiles transform unstructured serial output into structured columns using regex capture groups (`Zephyr`, `STM32`, `FreeRTOS`, `CAN Bus`).
- **🔖 Bookmarks & Log Pinning** — Pin critical records with `b`, jump chronologically with `[` and `]`, or isolate marked logs with Bookmarked-Only view (`B`).
- **🔍 Substring Search & Presets** — In-view substring search (`Ctrl+F`) with real-time match highlights, and a quick Filter Presets modal (`F`) with history recall (`Ctrl+P`/`Ctrl+N`).
- **🖱️ Mouse & Native Clipboard** — Double-click to copy, click-and-drag multi-row selection, smooth trackpad scrolling, and cross-platform clipboard support (OSC 52 over SSH/tmux + native OS tools).
- **🔌 Robust Auto-Reconnect** — Automatically detects USB hotplug disconnections and re-attaches seamlessly when hardware is re-plugged.

---

## Quick Installation

### Automated Install Script (Latest Stable Release)

#### macOS & Linux
```bash
curl -fsSL https://raw.githubusercontent.com/brenonsantos/oh-my-logs/main/install.sh | bash
```

#### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/brenonsantos/oh-my-logs/main/install.ps1 | iex
```

---

### Nightly Builds
To install the latest rolling build built automatically from `main`:

```bash
# macOS & Linux
curl -fsSL https://raw.githubusercontent.com/brenonsantos/oh-my-logs/main/install.sh | bash -s -- --nightly

# Windows (PowerShell)
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/brenonsantos/oh-my-logs/main/install.ps1))) -Nightly
```

---

### In-Place Self-Updating & Uninstall
Once installed, you can update or remove `oml` directly from your terminal:

```bash
oml --update           # Check for and install the latest stable release
oml --update --nightly # Update to the latest rolling nightly build
oml --uninstall        # Remove oml binary from system PATH
```

---

### Alternative Installation Options

#### Install Specific Release Tag
```bash
# macOS & Linux
curl -fsSL https://raw.githubusercontent.com/brenonsantos/oh-my-logs/main/install.sh | bash -s -- --version v1.2.0

# Windows (PowerShell)
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/brenonsantos/oh-my-logs/main/install.ps1))) -Version v1.2.0
```

#### Binary Self-Install (`--install`)
If you downloaded or built the binary locally:
```bash
./oml --install
```
Automatically detects your OS, installs `oml` into your `PATH`, and sets up default example profiles.

#### Install via Go
```bash
go install github.com/brenoniehues/oh-my-logs/cmd/oml@latest
```

---

## Quick Start

```bash
# Launch TUI without a connected device (explore interface)
oml

# Connect to a serial port at 115200 baud
oml --port /dev/ttyACM0

# Connect with custom baud rate
oml --port /dev/ttyUSB0 --baud 921600

# Connect using a named profile
oml --port /dev/ttyACM0 --profile Zephyr

# Replay an offline saved log file
oml --file demo.log --profile Zephyr

# Manage profiles
oml --list-profiles                      # List all global & local profiles
oml --import-profile ./my-device.yaml    # Install into global profiles directory
oml --export-profile Zephyr > copy.yaml  # Export profile template to customize
```

---

## Essential Keybindings

| Key | Action |
|-----|--------|
| `Ctrl+T` | Create new virtual tab |
| `Tab` / `]` | Next tab (or switch pane focus in split view) |
| `Shift+Tab` / `[` | Previous tab |
| `f` | Edit filter for active tab |
| `F` | Open Filter Presets modal |
| `Ctrl+F` | Open search prompt |
| `Space` | Pause / resume follow mode |
| `t` | Cycle timestamp display: `Clock` $\rightarrow$ `Δt` $\rightarrow$ `Both` $\rightarrow$ `OFF` |
| `\|` / `_` | Toggle Vertical / Horizontal split view |
| `S` | Toggle Synchronized Chronological Scrolling in split view |
| `i` | Open interactive Serial TX send prompt (`Ctrl+E` cycles line ending) |
| `b` | Pin / bookmark focused record or selected batch |
| `B` | Toggle Bookmarked-Only view filter |
| `y` | Yank (copy) selected row(s) to system clipboard |
| `p` / `P` | Open Serial Port picker (`p`) / Profile switcher (`P`) modal |
| `?` | Help & keyboard shortcuts modal |
| `q` | Clean exit |

---

## Documentation Directory

Explore the dedicated documentation guides for in-depth tutorials and configuration options:

| Guide | Description |
|-------|-------------|
| [**Profile Configuration**](docs/profiles.md) | YAML schema, regex capture groups, semantic column styles, custom colors, timing thresholds, and global profile management. |
| [**Filtering & Search**](docs/filtering.md) | Logcat syntax reference, substring queries, exclusions, multi-condition logic, filter presets modal, and history navigation. |
| [**Delta-Time & Latency**](docs/timing-and-latency.md) | Moving average (EMA) cadence baseline, burst/hiccup/anomaly classification, 4-state cycling, and batch selection stopwatch. |
| [**Dual-Pane Split Views**](docs/split-views.md) | Vertical and horizontal split layouts, pane focus switching, and synchronized chronological scrolling (`S`). |
| [**Serial TX (Send)**](docs/serial-tx.md) | Interactive serial send prompt, line ending selection (`Ctrl+E`), command history, draft recovery, and Maple Orange echo. |
| [**Bookmarks & Log Pinning**](docs/bookmarks.md) | Pinning milestones, batch bookmarking, chronological jumping (`[` / `]`), and Bookmarked-Only filtering. |
| [**Master Keybindings Reference**](docs/keybindings.md) | Complete categorized cheat sheet of all keyboard shortcuts, mouse gestures, input prompts, and easter eggs. |

---

## Architecture Overview

```
Serial Port / File Replay
          │
          ▼
   Line Framing (bufio)
          │
          ▼
   Profile Parser (regex / raw)
          │
          ▼
   Record Data Model (Fields, Raw, Timestamp, Delta)
          │
     ┌────┴────┐
     ▼         ▼
Ring Buffer   Filter Engine (concurrent evaluation)
     │         │
     └────┬────┘
          ▼
    Bubble Tea TUI (Virtual Tabs, Split Panes, Lip Gloss styles)
```

---

## Testing & Quality

All tests are race-condition free and verified under `-race`:
```bash
go test -race -count=1 ./...
```
