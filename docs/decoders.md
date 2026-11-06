# Payload Decoders & Terminal Sanitization Guide

`oml` supports **Project-Local Payload Decoders** and **Profile-Bundled Companion Decoders**, allowing you to automatically decode complex telemetry, packed bitfields, Base64 frames, and proprietary wire protocols without modifying or recompiling `oh-my-logs`.

---

## 1. Overview & Architecture

Serial log streams frequently interleave human-readable text logs with compact binary or encoded frames (such as telemetry packets, sensor bitfields, or hex dumps).

`oml` allows you to attach **Decoders** either:
1. **At the Project Level (`.oml.yaml`)**: Applied locally when running `oml` inside a specific project or workspace folder.
2. **At the Profile Level (`profiles/<profile>.yaml`)**: Packaged with a profile and distributed across teams via Git repositories or zip bundles. When users switch profiles in `oml` (`P`), the companion decoders activate automatically.

### Decoder Types

| Type | How It Works | Best Used For |
|------|--------------|---------------|
| **Declarative (Zero-Code)** | Uses regex named capture groups with `{field}` template string formatting. | Simple structured strings, comma-separated telemetry, key-value pairs. |
| **Procedural (`exec` Worker)** | Spawns a background process (Python, Go, Node, Bash, Rust) communicating via JSON over standard input/output (`stdin`/`stdout`). | Base64 frames, packed C bitfields, protocol buffers, custom CRC/cryptographic validation. |

---

## 2. Declarative Decoders (Zero-Code)

Declarative decoders match incoming log messages with a regular expression containing named capture groups and dynamically re-format the displayed log line.

### Example in `.oml.yaml` or Profile YAML
```yaml
decoders:
  - match: '^sensors,\s*(?P<temp>\d+\.\d+),\s*(?P<humidity>\d+),\s*(?P<voltage>\d+\.\d+)'
    format: 'Temp: {temp}°C | Humidity: {humidity}% | Battery: {voltage}V'
```

### Key Behaviors:
- **Message Enrichment**: Replaces the displayed `message` column with the rendered template string.
- **Field Extraction**: Injects all named groups (`temp`, `humidity`, `voltage`) into the log record's structured fields dictionary.
- **Instant Filtering**: Filter immediately in the UI using standard field search: `f temp:24` or `f voltage:3.3`.
- **Raw Preservation**: The original unparsed message is preserved in `_raw_message` and can be viewed anytime in the Inspector Drawer (`Enter` or `v`).

---

## 3. Procedural Script Decoders (`exec` Workers)

For binary frames, encoded payloads, or lookup database translations, `oml` delegates decoding to a lightweight subprocess:

```yaml
decoders:
  - match: '^PKT:(?P<payload>[A-Za-z0-9+/=]+)'
    exec: 'python3 packet_decoder.py'
```

### The Stdio Worker Protocol:
1. **Persistent Process**: `oml` launches the worker process once when the profile activates and keeps it alive.
2. **Streaming Requests**: For each matching log line, `oml` writes the line payload followed by `\n` to the worker process's standard input (`stdin`).
3. **JSON Responses**: The worker reads one line from `stdin` and writes exactly one JSON line to `stdout`:
   ```json
   {
     "summary": "telemetry_packet: temp=24.2C, status=OK",
     "fields": {
       "packet_type": "telemetry",
       "temp": "24.2",
       "status": "OK"
     }
   }
   ```
4. **Resiliency**: If your script crashes or exits, `oml` automatically restarts it on the next matching line without dropping logs or freezing the UI event loop.

---

## 4. Step-by-Step Tutorial: Adding Decoders to a Profile

This guide walks through creating and bundling a companion decoder with a custom profile.

### Step 1: Design the Trigger Regular Expression (`match`)
Ensure your `match` regex matches only lines intended for this decoder:
```yaml
# Good: specific prefix anchor
match: '^PKT:[A-Za-z0-9+/=]{10,}'

# Good: structured command/response header
match: '^telemetry:\s*\['
```

---

### Step 2: Organize the Companion Repository / Folder Layout
When publishing a profile for your team, organize your repository using standard bundle layout:

```text
my-firmware-tools/
├── profiles/
│   └── mydevice.yaml
└── decoders/
    └── mydevice/
        ├── packet_decoder.py
        ├── sensor_database.json
        └── helpers.py
```

> [!IMPORTANT]
> The folder name inside `decoders/` **must match the profile name** in lowercase (e.g. profile `MyDevice` corresponds to folder `decoders/mydevice/`). When installed, `oml` stores companion scripts in `~/.config/oh-my-logs/decoders/mydevice/`.

---

### Step 3: Write the Decoder Script

Create your decoder script (e.g. `decoders/mydevice/packet_decoder.py`).

Here is a production-ready Python template:

```python
#!/usr/bin/env python3
"""Companion decoder worker for oh-my-logs (oml).

Reads one line per stdin line, and emits one JSON line to stdout.
"""

import sys
import json
import os
from contextlib import redirect_stdout

# Ensure local imports work regardless of current working directory
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
if SCRIPT_DIR not in sys.path:
    sys.path.insert(0, SCRIPT_DIR)

# CRITICAL: Suppress third-party library banner prints during startup so stdout remains pure JSON
with redirect_stdout(sys.stderr):
    # Load your databases, lookup tables, or libraries here
    DB_FILE = os.path.join(SCRIPT_DIR, "sensor_database.json")
    DATABASE = {}
    if os.path.exists(DB_FILE):
        with open(DB_FILE, "r", encoding="utf-8") as f:
            DATABASE = json.load(f)


def decode_packet(line: str) -> dict:
    """Parse and translate payload line into summary and fields."""
    line = line.strip()
    if not line:
        return {"summary": "", "fields": {}}

    # Strip prefix if present (e.g. 'PKT:')
    if line.startswith("PKT:"):
        payload = line[4:].strip()
    else:
        payload = line

    try:
        # Example: parse comma-separated key=value or binary bitfield
        fields = {}
        parts = payload.split(",")
        for part in parts:
            if "=" in part:
                k, v = part.split("=", 1)
                fields[k.strip()] = v.strip()

        # Build readable human summary for table view
        summary = f"Packet [{fields.get('type', 'data')}]: " + ", ".join(
            f"{k}={v}" for k, v in fields.items() if k != "type"
        )

        return {
            "summary": summary,
            "fields": fields,
        }
    except Exception as err:
        # Fallback gracefully on parsing errors
        return {
            "summary": f"[decode error: {err}] {line}",
            "fields": {"_error": str(err)},
        }


def main():
    # Process incoming lines in a loop
    for line in sys.stdin:
        result = decode_packet(line)
        # Emit exactly one JSON object followed by newline
        sys.stdout.write(json.dumps(result) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
```

---

### Step 4: Test the Decoder Standalone

Before running in `oml`, verify that your script reads from `stdin` and outputs clean JSON lines:

```bash
echo "PKT:type=telemetry,temp=24.5,battery=98" | python3 decoders/mydevice/packet_decoder.py
```

**Expected output:**
```json
{"summary": "Packet [telemetry]: temp=24.5, battery=98", "fields": {"type": "telemetry", "temp": "24.5", "battery": "98"}}
```

> [!TIP]
> **Purity Rule**: The worker's `stdout` must receive **only valid JSON lines**. If your script or an imported module prints debug messages, send them to `sys.stderr` (`print("debug info", file=sys.stderr)`).

---

### Step 5: Declare the Decoder in `profiles/mydevice.yaml`

Register the companion script inside your profile definition:

```yaml
name: MyDevice
version: 1.0.0
source: https://github.com/my-org/my-firmware-tools.git

parser:
  type: regex
  patterns:
    - '^\[(?P<uptime>[^\]]+)\]\s+<(?P<level>[A-Z])>\s+(?P<message>.*)$'

columns:
  - field: uptime
    title: Uptime
    width: 14
    style: uptime
  - field: level
    title: Level
    width: 7
    style: level
  - field: message
    title: Message
    width: 0
    style: primary

# Companion decoders bundled with this profile
decoders:
  - match: '^PKT:[A-Za-z0-9+/=,]+'
    exec: 'python3 packet_decoder.py'
```

Notice that `exec: 'python3 packet_decoder.py'` does **not** need an absolute path. `oml` automatically resolves companion decoder scripts relative to the profile's companion folder.

---

### Step 6: Install, Distribute & Update

#### Installing for the First Time
Teammates can install the profile and all companion decoders directly from Git or local directory:

```bash
# Install from Git repository:
oml profile install https://github.com/my-org/my-firmware-tools.git

# Or install from local directory:
oml profile install ./my-firmware-tools/
```

`oml` automatically copies `mydevice.yaml` into global profiles and installs all files in `decoders/mydevice/` into `~/.config/oh-my-logs/decoders/mydevice/`.

#### Updating Decoders Across the Team
When you update decoder logic, add new message schemas, or fix bitfield decoding:
1. Bump the version number in `profiles/mydevice.yaml` (e.g. `version: 1.0.1`).
2. Commit and push the changes to your central Git repository.
3. Teammates run:
   ```bash
   oml profile update mydevice
   ```
   `oml` checks the remote version, downloads the updated profile, and replaces the companion decoder scripts automatically!

---

## 5. Live UI Features for Decoded Logs

Once your decoder is active, `oml` provides rich interactivity for decoded records:

1. **Table View Enrichment**: The `message` column displays the decoder's human-readable `summary`.
2. **Inspector Drawer (`Enter` / `v`)**:
   - Displays all extracted `fields` as individual key-value pairs.
   - Shows the original, unmodified payload in **ORIGINAL MESSAGE**.
   - Shows the physical wire bytes in **RAW LOG**.
3. **Structured Field Filtering**: Filter on any decoded field instantly:
   ```text
   f type:telemetry
   f temp:>25
   ```
4. **Log File Export (`.txt`)**:
   - When saving logs (`s` or direct-to-disk logging), `oml` writes both the raw wire log and an indented continuation line with the decoded message:
     ```text
     [00:01:23.456] <I> PKT:type=telemetry,temp=24.5,battery=98
       ↳ Packet [telemetry]: temp=24.5, battery=98
     ```
5. **System Clipboard Copying (`y`)**:
   - Pressing **`y`** copies the highlighted log row along with its decoded translation directly to your clipboard.

---

## 6. Universal Interactive Terminal & ANSI Sanitizer

Embedded microcontrollers and RTOSes (e.g. Zephyr with `CONFIG_SHELL=y`, FreeRTOS CLI) often share a single UART between an interactive shell and asynchronous logging.

When an async log arrives while a shell prompt (`uart:~$ ` or `device:~$ `) is displayed:
1. The shell sends ANSI cursor repositioning and line-clear codes (`\x1b[9D\x1b[J`, `\x1b[8D\x1b[K`, or `\r\x1b[K`) to erase its prompt.
2. `oml`'s universal terminal cleaner automatically interprets these sequences and removes the erased prompt before passing the clean log line to your decoders and table.
3. SGR color codes (`\x1b[32m`) and valid log data remain completely intact.
