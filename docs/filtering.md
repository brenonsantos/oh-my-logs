# Filtering & Search Guide

`oml` features an expressive, low-latency, Logcat-style filtering engine designed specifically for high-throughput embedded logging. All filter evaluations execute concurrently in $O(1)$ ring-buffer dispatch.

---

## 1. Filter Syntax Reference

Filters are entered by pressing **`f`** on any active tab, or when spawning a new tab with **`Ctrl+T`**.

| Expression | Type | Meaning |
|------------|------|---------|
| `motor` | Substring | Any field contains "motor" (case-insensitive). |
| `-motor` | Exclusion | No field contains "motor". |
| `can, over` or `can \| over` | OR Logic | Any field contains "can" **OR** contains "over". |
| `module:CAN` | Field Match | Field `module` equals `CAN`. |
| `-module:CAN` | Negative Field Match | Field `module` does **NOT** equal `CAN`. |
| `level:error,warn` | Field Value Set | Field `level` equals `error` **OR** `warn`. |
| `motor module:CAN` | AND Logic | Contains "motor" **AND** `module` equals `CAN`. |
| `can, over -heartbeat` | Compound | (Contains "can" OR "over") **AND** does NOT contain "heartbeat". |
| `level:err -module:spi power` | Complex Multi-Rule | Level is `err`, module is NOT `spi`, and any field contains `power`. |

---

## 2. Advanced Filtering Behaviors

### Case Insensitivity
All filter operations are case-insensitive by default:
- `level:ERROR` matches `error`, `Error`, and `ERROR`.
- `term:adc` matches `ADC`, `Adc`, and `adc`.

### Exclusions (`-`)
Prefixing any term or field rule with a minus sign (`-`) negates the match:
- `-heartbeat`: Discards high-frequency periodic ping logs.
- `-level:DEBUG`: Hides verbose debug logs.
- `-module:i2c,spi`: Hides all logs originating from either the I2C or SPI driver modules.

### Comma-Separated Values (OR Lists)
Comma-separated arguments inside a field selector evaluate with OR logic:
```text
level:error,fatal,panic
```
Matches records where `level` is either `error`, `fatal`, or `panic`.

---

## 3. Filter Presets Modal (`F`)

Press **`F`** at any time to open the **Filter Presets Modal**.

The modal lets you quickly apply frequently used filters without retyping them:

### Built-in Presets
- **Errors Only**: `level:err,error,fatal`
- **Warnings & Errors**: `level:err,error,fatal,warn,warning`
- **Suppress Noise**: `-heartbeat -tick -ping`
- **CAN Bus Traffic**: `module:can`
- **Power & Battery**: `power, battery, vbat, current`

### Modal Controls
| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate preset list |
| `Enter` | Apply selected preset to active tab |
| `Ctrl+S` | Save the current tab's filter as a new custom preset |
| `d` / `Delete` | Delete selected custom preset |
| `Esc` | Close modal |

### Custom Presets Persistence
Custom presets are saved locally in your user configuration directory under `filters.json`:
- **macOS**: `~/Library/Application Support/oh-my-logs/filters.json`
- **Linux**: `~/.config/oh-my-logs/filters.json`
- **Windows**: `%APPDATA%\oh-my-logs\filters.json`

---

## 4. Filter History Navigation

While inside the filter input prompt (**`f`**):
- Press **`Ctrl+P`** (or **`↑`**) to recall previous filter expressions from history.
- Press **`Ctrl+N`** (or **`↓`**) to navigate forward through history.
- History is saved automatically across restarts in `filters.json`.

---

## 5. In-View Substring Search (`Ctrl+F`)

While filtering determines which rows are visible in a tab, **Search** lets you highlight and navigate specific occurrences within the current view.

| Key | Action |
|-----|--------|
| `Ctrl+F` | Open interactive search prompt |
| `Enter` / `↓` | Jump to next search match |
| `↑` | Jump to previous search match |
| `n` / `N` | Next / previous match (in normal mode) |
| `Esc` | Clear search highlighting and dismiss search cursor |

### Search Highlights
- All matching substrings are highlighted in real-time across the entire log table.
- The active match is highlighted with distinct inverted focus markers (`▶`).
- The status bar displays your current position: `match 3 of 42`.
