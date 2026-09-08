# oh-my-logs (`oml`)

A cross-platform terminal UI for monitoring, filtering, searching, and recording serial output from embedded systems.

```
┌ Oh My Logs ──────────────────────────────────────────────────────────────────┐
│  Port: /dev/ttyACM0   Baud: 115200   ● Connected   Profile: Example1        │
├──────────────────────────────────────────────────────────────────────────────┤
│  1: All (1248)  [2: Errors (4)]  3: CAN (312)   Tab: cycle · ^T: new · ^W: close │
├──────────────────────────────────────────────────────────────────────────────┤
│ Time           Level    Module    Message                                     │
│ 15:42:31.102   INFO     SYS       System initialized                         │
│ 15:42:31.254   INFO     CAN       CAN initialized                            │
│ 15:42:32.103   WARN     ADC       Channel 3 reading high                     │
│ 15:42:33.876   ERROR    PDM       Overcurrent detected                       │
├──────────────────────────────────────────────────────────────────────────────┤
│ tab [2/3: Errors]  │  1248 records  │  4 shown  │  filter: level:ERROR  │  FOLLOW │
├──────────────────────────────────────────────────────────────────────────────┤
│ Ctrl+F search   f filter   Tab tab   ^T new tab   Space pause   ? help   q quit  │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Features

- **Virtual Tabs with independent filters & views** — spawn multiple tabs (`Ctrl+T`), each with its own independent logcat filter, visible rows, scroll position, follow/pause mode, and search state. All tabs update concurrently in real-time from the shared serial stream.
- **Profile-driven columns** — define completely different table columns per architecture/product via YAML. No code changes required.
- **Generic data model** — records are `map[string]string`; no hardcoded fields like timestamp, level, or module.
- **Regex parser with named groups** — extract arbitrary fields from any log format.
- **Raw parser** — display unstructured output verbatim.
- **Logcat-style filtering** — `term`, `-term`, `field:value`, `-field:value`, `field:v1,v2`.
- **Ctrl+F search** — navigate matches with `n`/`N` or `↑`/`↓` in search mode, with active focus indicator (`▶`) and in-place substring highlighting.
- **Follow / Pause** — auto-scroll on new records; pause without losing data.
- **Persistent timestamp capture & toggle** — arrival timestamps are always captured on ingest; press `t` anytime to toggle visibility on or off without losing past timestamps.
- **Auto-reconnect on USB hotplug** — automatically detects disconnected USB serial devices and seamlessly re-attaches upon re-plugging (handling port location shifts and cu/tty variations).
- **Context-aware empty states** — clean, informative displays when disconnected, waiting for incoming serial data, or when active filters match 0 logs.
- **Ring buffer** — configurable capacity (default 50,000 records); no unbounded memory growth.
- **Log replay** — replay saved `.log` files with `--file`.
- **Save log** — saves all raw lines to a timestamped file (`s`).
- **Mouse & Clipboard interactions** — click tabs to switch, click `^T`/`^W` to create/close, click rows to select & auto-pause, double-click to copy, click-and-drag multi-row selection with auto-copy on release, and smooth auto-scrolling.
- **Cross-platform clipboard** — dual ANSI OSC 52 sequence support (works over SSH and tmux) + native OS clipboard integration (`pbcopy`, `wl-copy`, `xclip`, `clip.exe`). Paste with `Ctrl+V` into search and filter prompts.
- **Race-free** — all tests pass under `go test -race`.

---

## Installation

You can install `oml` globally so that the `oml` command is available from anywhere in your terminal.

### Option 1: Automated Installers

#### macOS & Linux
Run the install script from the repository:
```bash
./install.sh
```
*Or install directly via curl:*
```bash
curl -fsSL https://raw.githubusercontent.com/brenoniehues/oh-my-logs/main/install.sh | bash
```
> Installs `oml` to `/usr/local/bin` (or `~/.local/bin`) and sets up default profiles in your user config directory.

#### Windows (PowerShell)
Run the install script in PowerShell:
```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```
*Or install directly via web:*
```powershell
irm https://raw.githubusercontent.com/brenoniehues/oh-my-logs/main/install.ps1 | iex
```
> Installs `oml.exe` to `%LOCALAPPDATA%\Programs\oh-my-logs`, adds it to your user `PATH` environment variable permanently, and sets up default profiles in `%APPDATA%\oh-my-logs\profiles\`.

---

### Option 2: Binary Self-Install (`--install`)

If you already downloaded or built the binary:
```bash
./oml --install
```
This automatically detects your operating system, copies the binary into your PATH, and initializes your system profiles directory.

---

### Option 3: Install via Go

```bash
go install github.com/brenoniehues/oh-my-logs/cmd/oml@latest
```
Ensure your `$GOPATH/bin` (typically `~/go/bin`) is in your `$PATH`.

---

### Uninstalling

- **macOS / Linux**: `./install.sh --uninstall` or `oml --uninstall`
- **Windows**: `powershell -ExecutionPolicy Bypass -File install.ps1 -Uninstall` or `oml --uninstall`

---

## Usage

```bash
# Open TUI without a device (explore the UI)
oml

# Connect to a serial port at default baud (115200)
oml --port /dev/ttyACM0

# Specify baud rate
oml --port /dev/ttyACM0 --baud 921600

# Use a named profile
oml --port /dev/ttyACM0 --baud 115200 --profile Example1

# Use a profile by file path
oml --port COM3 --profile ./profiles/examples/example1.yaml

# Replay a saved log file
oml --file ./oml-2026-09-08T10-00-00.log --profile Example1

# Profile management
oml --list-profiles                     # List all available profiles & locations
oml --import-profile ./my-device.yaml   # Install into global OS profiles directory
oml --export-profile Zephyr > custom.yaml # Export profile template to customize
oml --profiles-dir                      # Print global profiles directory path

# Version
oml --version
```

---

## Keyboard Controls

### Virtual Tabs

| Key | Action |
|-----|--------|
| `Ctrl+T` | Create a new virtual tab (prompts for filter) |
| `Tab` / `]` | Switch to next tab |
| `Shift+Tab` / `[` | Switch to previous tab |
| `1` .. `9` | Jump directly to tab index 1..9 |
| `Ctrl+W` | Close active virtual tab (preserves at least one tab) |
| `f` | Edit filter for the current active tab |

### Navigation & Scrolling

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Scroll one row |
| `Wheel` / Trackpad | Smooth scroll rows |
| `PgUp` / `PgDn` | Scroll one page (`Ctrl+U` / `Ctrl+D` half page) |
| `g` / `Home` | Jump to top (oldest record, pauses follow) |
| `G` / `End` | Jump to bottom (newest record & resumes follow) |

### Search & Filter

| Key | Action |
|-----|--------|
| `Ctrl+F` | Open search prompt |
| `Enter` / `↓` | Next search match (centers match in view) |
| `↑` | Previous search match |
| `n` / `N` | Next / previous search match (in normal mode) |
| `f` | Edit filter for current tab (auto-names tab to filter) |
| `Esc` | Cancel input / clear search highlight |

### Actions & Device Controls

| Key | Action |
|-----|--------|
| `Space` | Pause / resume stream follow |
| `c` | Clear log buffer across all tabs |
| `t` | Toggle timestamp column visibility (`⏱ ON` / `⏱ OFF`) |
| `s` | Save all raw buffer lines to a timestamped file |
| `p` | Open Serial Port & Baud rate picker modal |
| `P` | Open Profile switcher modal (re-parses buffer) |
| `r` | Reconnect current serial port |
| `?` | Toggle Help modal popup |
| `q` / `Ctrl+C` | Clean exit |

**While in Search or Filter input:**

| Key | Action |
|-----|--------|
| `Enter` | Apply filter / search |
| `Esc` | Cancel input |
| `Backspace` | Delete character |

---

## Profiles

Profiles live in your OS config directory under `oh-my-logs/profiles/`:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/oh-my-logs/profiles/` |
| Linux | `~/.config/oh-my-logs/profiles/` |
| Windows | `%APPDATA%\oh-my-logs\profiles\` |

You can also pass a direct `.yaml` path via `--profile`.

### Profile Format

```yaml
name: Example1

parser:
  type: regex
  pattern: '^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$'

columns:
  - field: time
    title: Time
    width: 14        # fixed width in characters; 0 = flexible/fill
    style: timestamp # semantic style: timestamp, level, identifier, primary, muted

  - field: level
    title: Level
    width: 8
    style: level
    colors:          # optional custom color mapping
      ERROR: red
      WARN: yellow
      INFO: blue

  - field: module
    title: Module
    width: 10
    style: identifier

  - field: message
    title: Message
    width: 0         # flexible: fills remaining space
    style: primary
```


**Parser types:**
- `regex` — named capture groups (`(?P<name>...)`) become record fields
- `raw` — entire line placed in `message` field

**If a line doesn't match**, it is stored with `_raw = <original line>` and the stream continues uninterrupted.

### Example: Delimited format without level (Example2)

```yaml
name: Example2

parser:
  type: regex
  pattern: '^(?P<timestamp>\d+);(?P<cpu>\d+);(?P<task>[^;]+);(?P<code>[^;]+);(?P<message>.*)$'

columns:
  - field: timestamp
    title: Timestamp
    width: 12
  - field: cpu
    title: CPU
    width: 5
  - field: task
    title: Task
    width: 18
  - field: code
    title: Code
    width: 10
  - field: message
    title: Message
    width: 0
```

Input: `1693000000;1;MotorControl;0x123;Overcurrent detected`

Output:
```
Timestamp     CPU   Task               Code        Message
1693000000    1     MotorControl       0x123       Overcurrent detected
```

### Importing, Exporting & Managing Profiles

`oml` provides built-in commands to import, export, and list profiles:

#### 1. CLI Profile Operations

- **List all discovered profiles**:
  ```bash
  oml --list-profiles
  ```
  Displays all profiles, indicating whether they are `[global]` (system-wide) or `[local]` (workspace examples), their parser type, and their path on disk.

- **Import a profile**:
  ```bash
  oml --import-profile ./my-device.yaml
  ```
  Validates the YAML syntax and regex capture groups, then installs the profile into your system's global OS profiles directory. Once imported, you can run `oml --profile <Name>` from any working directory.

- **Export a profile**:
  ```bash
  # Output YAML to stdout (pipe or redirect):
  oml --export-profile Zephyr > my-custom-zephyr.yaml

  # Or save directly to a target file:
  oml --export-profile Zephyr --out ./my-custom-zephyr.yaml
  ```

- **Check profiles directory path**:
  ```bash
  oml --profiles-dir
  ```

#### 2. Keeping Proprietary Profiles Out of Git

If you work with proprietary firmware or confidential log formats, save your profile in your OS user configuration directory:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/oh-my-logs/profiles/` |
| Linux | `~/.config/oh-my-logs/profiles/` |
| Windows | `%APPDATA%\oh-my-logs\profiles\` |

Profiles stored here are automatically discovered by `oml` system-wide, but remain outside the git workspace so they will never be accidentally committed.

#### 3. In-App Profile Switcher (`P`)

Press `P` at any time while running `oml` to open the profile switcher modal. Global profiles are marked with `[global]`. Selecting a profile instantly re-parses the in-memory buffer without dropping incoming serial logs.

---

## Filtering

The filter syntax supports substring matching, exclusions, field-specific equality, and boolean logic:

| Expression | Meaning |
|-----------|---------|
| `motor` | any field contains "motor" (case-insensitive) |
| `-motor` | no field contains "motor" |
| `can, over` or `can \| over` | any field contains "can" **OR** contains "over" |
| `module:CAN` | field `module` equals `CAN` |
| `-module:CAN` | field `module` does NOT equal `CAN` |
| `level:error,warn` | field `level` equals `error` **OR** `warn` |
| `motor module:CAN` | contains "motor" **AND** module is CAN |
| `can, over -heartbeat` | (contains "can" **OR** "over") **AND** does NOT contain "heartbeat" |

---

## Virtual Tabs

Embedded systems frequently produce high-volume logs from multiple concurrent subsystems (e.g., RTOS scheduler ticks, CAN communications, motor control loops, and sensor reads). Virtual Tabs let you monitor several filtered views simultaneously without losing the main stream:

- **Independent views** — Each tab maintains its own filter, visible rows, follow/pause status, and scroll position.
- **Real-time multi-tab ingest** — Incoming serial lines are dispatched to all tabs concurrently in $O(1)$ time without lag.
- **Workflow example**:
  - `Tab 1 (All)`: Full raw stream, following newest logs.
  - `Tab 2 (Errors)`: Filtered to `level:ERROR,WARN` to quickly spot faults.
  - `Tab 3 (CAN)`: Filtered to `module:CAN` while debugging bus traffic.
  - `Tab 4 (Clean)`: Filtered to `-heartbeat -tick` to suppress high-frequency background noise.

### Tab Shortcuts:
- **`Ctrl+T`**: Create a new tab and immediately enter a filter expression.
- **`Tab` / `]`**: Cycle to the next tab.
- **`Shift+Tab` / `[`**: Cycle to the previous tab.
- **`1` .. `9`**: Jump directly to tab 1 through 9.
- **`f`**: Update the active tab's filter (auto-renames the tab).
- **`Ctrl+W`**: Close the active tab (the default tab cannot be closed).

---

## Mouse & Clipboard Support

`oml` provides full mouse interactions alongside cross-platform system clipboard integration (supporting ANSI OSC 52 sequences across SSH/tmux as well as native OS tools like `pbcopy`, `wl-copy`, `xclip`, and `clip.exe`):

### Mouse Interactions
- **Clicking Virtual Tabs**: Click directly on any tab pill in the tab bar to switch to it. Click `^T: new` or `^W: close` on the right side of the tab bar to create or close tabs.
- **Row Focus & Auto-Pause**: Click any log row to focus it with a `▶` indicator. Ingest follow mode is automatically paused so streaming serial lines will not shift or displace your view.
- **Double-Click to Copy**: Rapidly double-clicking any log line copies the raw record directly to your system clipboard with a status confirmation.
- **Multi-Row Drag Selection**: Click and drag down or up across log rows to highlight a range of records. Releasing the mouse button sets the selection range without premature auto-copying (press `y` to copy).
- **Keyboard Multi-Row Selection**: Press **`Shift+↑`** / **`Shift+↓`** (or **`K`** / **`J`**) while focused on a row to expand or contract selection across multiple lines.
- **Auto-Scrolling while Dragging**: Dragging the mouse past the top or bottom of the table viewport automatically scrolls up or down.
- **Status Bar Toggle**: Clicking the status bar or paused badge toggles between streaming `FOLLOW` and `PAUSED`.

### Clipboard & Keyboard Shortcuts
- **`y`**: Yank (copy) currently selected row (or highlighted multi-row range) to clipboard.
- **`Y`**: Yank raw unformatted log record.
- **`Shift+↑` / `Shift+↓`** (or `K` / `J`): Expand or contract row selection.
- **`Ctrl+V`**: Paste clipboard text directly into Search (`Ctrl+F`) or Filter (`f`) input prompts.
- **`Esc`**: Clear row selection and search highlights.

> [!TIP]
> **Native Terminal Selection**: Terminal emulators capture mouse events while TUI cell motion is enabled. If you want to select arbitrary characters natively using your terminal's built-in text selection, hold **`Option`** (macOS) or **`Shift`** (Linux/Windows) while dragging.


## Architecture

```
Serial / File Input
        │
        ▼
    Framing (bufio.Scanner, line-based)
        │
        ▼
    Parser (selected profile)
        │
        ▼
    Record { Fields map[string]string, Raw string }
        │
   ┌────┴────┐
   ▼         ▼
Ring Buffer  Filter
   │         │
   └────┬────┘
        ▼
       TUI (Bubble Tea)
```

- `internal/record` — generic data model + ring buffer
- `internal/parser` — `Parser` interface, `RawParser`, `RegexParser`, YAML profile loader
- `internal/filter` — filter expression parser + matcher
- `internal/serial` — `Source` interface, `SerialSource` (live port), `FileSource` (replay)
- `internal/config` — OS config dir, profile file discovery
- `internal/tui` — Bubble Tea model, update, view, styles, key bindings
- `cmd/oml` — CLI entry point

---

## Running Tests

```bash
go test ./...
go test -race ./...
```
