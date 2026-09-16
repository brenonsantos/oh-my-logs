# Profile Configuration Guide

In `oml`, **Profiles** define how serial lines from a specific microcontroller, RTOS, or communication protocol are parsed, mapped into columns, and displayed in the terminal UI.

Profiles are defined as YAML files and require zero Go compilation or code modification.

---

## 1. Quick Example

```yaml
name: Zephyr

# Zephyr RTOS structured log format
# Example: [00:00:03.165,977] <err> ext_log_system: critical level log
parser:
  type: regex
  pattern: '^\[\s*(?P<uptime>[^\]]+?)\s*\]\s+<(?P<level>[a-zA-Z]+)>\s+(?:(?P<module>[a-zA-Z0-9_.-]+):\s+)?(?P<message>.*)$'

columns:
  - field: uptime
    title: Uptime
    width: 19
    style: uptime

  - field: level
    title: Level
    width: 7
    style: level
    colors:
      err: red
      wrn: yellow
      inf: cyan
      dbg: gray

  - field: module
    title: Module
    width: 16
    style: identifier

  - field: message
    title: Message
    width: 0 # 0 = flexible width; expands to fill terminal space
    style: primary
```

---

## 2. Profile Structure Reference

### Top-Level Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | **Required.** Display name of the profile (e.g. `Zephyr`, `STM32`, `CAN`). |
| `version` | `string` | Optional semantic version string (e.g. `1.2.0`). Used by `oml profile update`. |
| `source` | `string` | Optional origin URL or Git repository where the profile was downloaded from. |
| `parser` | `object` | Parser configuration (see below). |
| `columns` | `list` | Ordered list of table column definitions. |
| `ingest` | `object` | Optional pipeline ingest settings (automatic timestamping). |
| `timing` | `object` | Optional latency tracking thresholds and ratios. |

---

### Parser Configuration (`parser`)

| Field | Type | Description |
|-------|------|-------------|
| `type` | `string` | `"regex"` or `"raw"`. Defaults to `"raw"`. |
| `pattern` | `string` | Regular expression with named capture groups `(?P<name>...)`. Required when `type: regex`. |

#### How Regex Parsing Works
- Each named capture group `(?P<field_name>...)` extracts the matching text into the record's field dictionary under `field_name`.
- Any line that fails to match the regular expression is stored safely with `_raw = line` and `message = line`. The TUI displays it seamlessly without crashing or dropping logs.

---

### Column Configuration (`columns`)

Each item in `columns` defines a display column in the main log table:

| Field | Type | Description |
|-------|------|-------------|
| `field` | `string` | Name of the extracted capture group (e.g. `uptime`, `level`, `module`, `message`). |
| `title` | `string` | Header text displayed at the top of the table. |
| `width` | `int` | Fixed column width in characters. Set to `0` for flexible width (expands to fill remaining terminal width). |
| `style` | `string` | Semantic color style: `timestamp`, `uptime`, `level`, `identifier`, `primary`, or `muted`. |
| `colors` | `map` | Value-to-color mapping (e.g. `ERROR: red`, `WARN: yellow`, `DEBUG: gray`). |

#### Supported Semantic Styles
- `primary`: Default bold white/high-contrast text (best for log messages).
- `muted`: Subdued slate/gray text (best for auxiliary metadata or secondary IDs).
- `identifier`: Subtle lavender/cyan accent (best for subsystem/module names).
- `level`: Dynamically color-coded based on log severity (red for error, yellow for warning, cyan for info, gray for debug).
- `timestamp`: Formatted clock arrival time.
- `uptime`: Embedded device monotonic uptime counter.

#### Supported Colors in `colors` Map
- `red`, `yellow`, `green`, `cyan`, `blue`, `magenta`, `gray`, `white`, `orange`, or hex strings `#RRGGBB`.

---

### Ingest Injections (`ingest`)

When working with microcontrollers that do not output timestamps, you can configure `oml` to stamp arrival time when the line is received:

```yaml
ingest:
  timestamp:
    enabled: true
    field: "_ts"            # Field name to inject (defaults to "_ts")
    format: "15:04:05.000"  # Go time layout (defaults to millisecond resolution)
```

---

### Timing & Delta-Time Overrides (`timing`)

Configure custom inter-log latency thresholds or ratios for this specific architecture/protocol:

```yaml
timing:
  warn_threshold: 50ms   # Fixed duration for warning level (hiccup)
  alert_threshold: 200ms # Fixed duration for alert level (anomaly)
  warn_ratio: 3.0        # Multiplier of baseline EMA (default: 2.5x)
  alert_ratio: 6.0       # Multiplier of baseline EMA (default: 5.0x)
```

When set, fixed thresholds override dynamic moving-average ratios, making it ideal for strict communication protocols (e.g. CAN bus, Modbus, or UART heartbeats).

---

## 3. Global vs. Local Profiles

`oml` discovers profiles from two distinct locations:

1. **System Profiles (Global)**:
   Available globally from any directory across your machine.
   - **macOS**: `~/Library/Application Support/oh-my-logs/profiles/`
   - **Linux**: `~/.config/oh-my-logs/profiles/`
   - **Windows**: `%APPDATA%\oh-my-logs\profiles\`
2. **Workspace Profiles (Local)**:
   Profiles placed in `./examples/profiles/` relative to your current project. Marked with `[local]` in the UI.

> [!TIP]
> **Keeping Proprietary Profiles Out of Git**:
> If your hardware logs contain confidential or internal company fields, save your YAML profile in your global OS profiles directory (`oml --import-profile ./confidential.yaml`). It will be available anywhere on your machine without needing to be committed to the repository.

---

## 4. First-Class CLI Profile Management (`oml profile`)

`oml` provides a powerful `oml profile` subcommand family to list, inspect, install, and uninstall profiles and companion decoders:

### List Profiles with Numeric IDs
```bash
oml profile list
# or legacy flag: oml --list-profiles
```
Displays a numbered table with profile `# ID`, name, scope (`[global]`, `[local]`, `[builtin]`), parser type, decoders count (including bundles), and location.

### Install Profiles & Bundles
Install from local files, folders, remote HTTP/HTTPS URLs, or Git repositories:
```bash
# Local YAML file:
oml profile install ./my-device.yaml

# Folder containing profiles & companion decoders:
oml profile install ./tools/firmware-profiles/

# Remote URL (supports public or authenticated raw links):
oml profile install https://raw.githubusercontent.com/org/repo/main/profile.yaml

# Git repository (clones using your local SSH/Git credentials):
oml profile install git@github.com:my-org/firmware-tools.git

# Force overwrite existing profile:
oml profile install -f ./updated-profile.yaml
```

### Update Profiles (`oml profile update`)
Keep installed profiles and companion decoders up to date with central upstream repositories:
```bash
# Check for available updates without applying them (dry run):
oml profile update --check

# Update all installed profiles that have a remote source:
oml profile update

# Update a specific profile by ID or name:
oml profile update 1
oml profile update appliance

# Force re-download even if version matches:
oml profile update appliance --force
```

### Inspect Profile Details
Inspect parser configuration, column widths, styles, and registered decoders without opening the file:
```bash
oml profile show 1          # by numeric ID
oml profile show zephyr     # by name
```

### Uninstall Profiles
Remove globally installed profiles and their companion decoders:
```bash
oml profile uninstall 1          # by numeric ID
oml profile uninstall zephyr     # by name
```

### Export a Profile
```bash
# Print YAML to stdout:
oml profile export 1
oml profile export zephyr

# Save directly to a file:
oml profile export zephyr --out ./custom-zephyr.yaml
```

### Show Global Storage Directories
```bash
oml profile path
# or legacy flag: oml --profiles-dir
```

---

## 5. In-App Profile Switcher (`P`)

Press **`P`** at any time while running `oml` to open the interactive Profile Switcher modal:
- Use `↑` / `↓` (or mouse wheel) to select a profile.
- Press `Enter` to switch.
- The in-memory buffer is **re-parsed instantly** against the new profile, immediately updating columns without dropping any incoming serial logs.
- Press `Esc` to cancel.
