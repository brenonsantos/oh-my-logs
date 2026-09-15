# Project Decoders & Terminal Sanitization Guide

`oml` supports **Project-Local Payload Decoders** and **Universal Terminal Sanitization**, allowing projects to automatically decode complex telemetry, bitfields, and proprietary payloads without embedding proprietary schemas, keys, or code inside `oh-my-logs`.

---

## 1. Project Configuration (`.oml.yaml`)

`oml` automatically discovers project-level configuration when launched from a project folder or anywhere within a Git repository:

- `.oml.yaml`
- `.oml.yml`
- `.oml/config.yaml`
- `.oml/config.yml`

### Sample `.oml.yaml`
```yaml
profile: Zephyr
baud: 115200

# Optional project-level decoders
decoders:
  # Declarative telemetry formatter
  - match: '^instruments,\s*(?P<cpu>\d+),\s*(?P<usage>\d+),\s*(?P<mem>\d+)'
    format: 'CPU: {cpu} | Usage: {usage}% | Mem: {mem}KB'

  # Procedural worker process
  - match: '^CS:(?P<payload>[A-Za-z0-9+/=()]+)'
    exec: 'python3 .oml/decode.py'
```

---

## 2. Declarative Decoders (Zero-Code)

Declarative decoders match log messages using regular expressions with named capture groups and format the table message using `{field}` placeholders.

```yaml
decoders:
  - match: '^sensors,\s*(?P<temp>\d+\.\d+),\s*(?P<humidity>\d+)'
    format: 'Temp: {temp}°C | Humidity: {humidity}%'
```

### Key Behaviors:
- **Message Enrichment**: Replaces the displayed `message` column with the rendered template.
- **Field Extraction**: Injects all named groups (`temp`, `humidity`) into `Record.Fields`.
- **Instant Filtering**: Filter immediately with `f temp:24` or `f humidity:50`.
- **Raw Preservation**: The original unparsed message is preserved in `_raw_message` and displayed in the inspector drawer.

---

## 3. Procedural Decoders (`exec` Workers)

For encrypted, compressed, or packed binary/Base64 payloads (such as SMP messages or custom protocols), `oml` can delegate decoding to an external worker process (Python, Go, Node, Bash, Rust, etc.).

```yaml
decoders:
  - match: '^[A-Za-z0-9+/]{2}:[A-Za-z0-9+/]{2}/'
    exec: 'python3 .oml/os_msg_decoder.py'
```

### Stdio Worker Protocol:
1. `oml` launches the worker process once and keeps it alive.
2. For each matching log, `oml` writes the payload followed by `\n` to the worker's `stdin`.
3. The worker writes a single JSON line to `stdout`:
   ```json
   {
     "summary": "res_sensors: sensorDataChanged (temp=22.4, hum=48)",
     "fields": {
       "sender": "sensorController",
       "topic": "res_sensors",
       "temp": "22.4",
       "hum": "48"
     }
   }
   ```
4. If a script fails or crashes, `oml` automatically restarts the worker without blocking the UI event loop.

---

## 4. Universal Interactive Terminal & ANSI Sanitizer

Embedded microcontrollers and RTOSes (e.g. Zephyr with `CONFIG_SHELL=y`, FreeRTOS CLI) often share a single UART between an interactive shell and asynchronous logging.

When an async log arrives while the shell prompt (`uart:~$ ` or `device:~$ `) is displayed:
1. The shell sends ANSI cursor repositioning and line-clear codes (`\x1b[9D\x1b[J`, `\x1b[8D\x1b[K`, or `\r\x1b[K`) to erase its prompt.
2. `oml`'s universal terminal cleaner automatically interprets these sequences and removes the erased prompt before passing the clean log line to the parser, ring buffer, and direct-to-disk logging.
3. SGR color codes (`\x1b[32m`) and valid log data remain completely intact.

---

## 5. Inspector Drawer (`Enter` / `v`)

Pressing **`Enter`** or **`v`** on any decoded row opens the Inspector Drawer:
- **`PARSED FIELDS`**: Displays extracted decoder fields alongside standard log fields.
- **`MESSAGE & PAYLOAD`**: Displays the human-readable decoded summary.
- **`ORIGINAL MESSAGE`**: Shows the original, unmodified payload before decoder transformations.
- **`RAW LOG`**: Shows the complete physical line received over the wire.
