# oh-my-logs (`oml`)

A cross-platform terminal UI for monitoring, filtering, searching, and recording serial output from embedded systems.

```
┌ Oh My Logs ──────────────────────────────────────────────────────────────────┐
│  Port: /dev/ttyACM0   Baud: 115200   ● Connected   Profile: STM32-PDM       │
├──────────────────────────────────────────────────────────────────────────────┤
│ Time           Level    Module    Message                                     │
│ 15:42:31.102   INFO     SYS       System initialized                         │
│ 15:42:31.254   INFO     CAN       CAN initialized                            │
│ 15:42:32.103   WARN     ADC       Channel 3 reading high                     │
│ 15:42:33.876   ERROR    PDM       Overcurrent detected                       │
├──────────────────────────────────────────────────────────────────────────────┤
│ 1248 records  │  312 shown  │  filter: module:CAN  │  FOLLOW                │
├──────────────────────────────────────────────────────────────────────────────┤
│ Ctrl+F search   f filter   c clear   Space pause   s save   r reconnect   q quit │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Features

- **Profile-driven columns** — define completely different table columns per architecture/product via YAML. No code changes required.
- **Generic data model** — records are `map[string]string`; no hardcoded fields like timestamp, level, or module.
- **Regex parser with named groups** — extract arbitrary fields from any log format.
- **Raw parser** — display unstructured output verbatim.
- **Logcat-style filtering** — `term`, `-term`, `field:value`, `-field:value`, `field:v1,v2`.
- **Ctrl+F search** — navigate matches with `n`/`N`, highlighted in-place.
- **Follow / Pause** — auto-scroll on new records; pause without losing data.
- **Ring buffer** — configurable capacity (default 50,000 records); no unbounded memory growth.
- **Log replay** — replay saved `.log` files with `--file`.
- **Save log** — saves all raw lines to a timestamped file.
- **Race-free** — all tests pass under `go test -race`.

---

## Installation

```bash
git clone https://github.com/brenoniehues/oh-my-logs
cd oh-my-logs
go build -o oml ./cmd/oml
```

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
oml --port /dev/ttyACM0 --baud 115200 --profile STM32-PDM

# Use a profile by file path
oml --port COM3 --profile ./profiles/examples/stm32-pdm.yaml

# Replay a saved log file
oml --file ./oml-2026-09-08T10-00-00.log --profile STM32-PDM

# Version
oml --version
```

---

## Keyboard Controls

| Key | Action |
|-----|--------|
| `Ctrl+F` | Open search |
| `f` | Open filter |
| `c` | Clear all records |
| `Space` | Pause / resume |
| `↑` / `↓` or `k` / `j` | Scroll one row |
| `PgUp` / `PgDn` | Scroll one page |
| `g` / `End` | Go to newest record (re-enable follow) |
| `G` / `Home` | Go to oldest record |
| `n` / `N` | Next / previous search match |
| `p` | Open serial port picker |
| `P` | Open profile switcher (re-interprets buffer) |
| `t` | Toggle auto-timestamp injection on/off |
| `s` | Save raw log to file |
| `r` | Reconnect |
| `q` / `Ctrl+C` | Quit |


**While in Search or Filter input:**

| Key | Action |
|-----|--------|
| `Enter` | Confirm |
| `Esc` | Cancel |
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
name: STM32-PDM

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

### Example: No level field (Legacy ECU)

```yaml
name: Legacy-ECU

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
