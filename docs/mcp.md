# Model Context Protocol (MCP) & AI Integration

`oml` includes a native, headless **Model Context Protocol (MCP)** server (`oml mcp`) with an integrated local IPC bridge. This enables AI assistants like **Gemini CLI**, **Claude Desktop**, **Cursor**, or custom autonomous agents to interact directly with hardware serial devices.

---

## Architecture: TUI & AI Coexistence

On modern operating systems, serial ports (`/dev/tty*`, `/dev/cu*`, `COM*`) are exclusive resources (`EBUSY` when opened concurrently). `oml` solves this with an automatic **local IPC bridge**:

```
                 ┌─────────────────────────────────┐
                 │       Hardware Serial Port       │
                 └────────────────┬────────────────┘
                                  │ (exclusive OS handle)
                                  ▼
               ┌──────────────────────────────────────┐
               │         oml Interactive TUI          │
               │   (Primary hardware owner & display) │
               └──────────────────┬───────────────────┘
                                  │ Local Socket / Named Pipe
                                  │ (~/.config/oh-my-logs/oml.sock)
                                  ▼
┌──────────────┐  stdio JSON-RPC  ┌───────────────────┐
│  AI Client   │ ◄──────────────► │      oml mcp      │
│ (Gemini/etc) │                  │  (Headless Proxy) │
└──────────────┘                  └───────────────────┘
```

1. **When TUI is running**: `oml` runs an IPC server on a local domain socket (`~/.config/oh-my-logs/oml.sock` on macOS/Linux, loopback TCP on Windows). Any AI assistant executing `oml mcp` transparently connects to the socket and queries the live TUI buffer.
2. **When AI runs first**: If `oml mcp` opens the port first, launching the `oml` TUI automatically requests a port handover. `oml mcp` releases the serial port handle; the TUI acquires it, and `oml mcp` seamlessly switches to IPC client mode.
3. **Zero interruption**: You can keep the `oml` TUI open on one screen while your AI assistant analyzes logs and transmits test commands in your terminal or IDE.

---

## Configuration

### Gemini CLI
Add `oml` to `.gemini/settings.json` (project-local) or `~/.gemini/settings.json` (global):

```json
{
  "mcpServers": {
    "oml": {
      "command": "oml",
      "args": ["mcp"]
    }
  }
}
```

### Claude Desktop
Add `oml` to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "oml": {
      "command": "oml",
      "args": ["mcp"]
    }
  }
}
```

### Pre-configuring Initial Connection Flags
You can optionally configure `oml mcp` with initial port, baud rate, or profile flags:

```json
{
  "mcpServers": {
    "oml": {
      "command": "oml",
      "args": ["mcp", "--port", "/dev/ttyACM0", "--baud", "115200", "--profile", "Zephyr"]
    }
  }
}
```

---

## Tool Catalog

The MCP server exposes standard tools discoverable automatically by LLMs:

### Connection & Hardware Management
- `list_ports`: Enumerates hardware serial ports and shows which one is currently connected.
- `connect_port`: Dynamically switches or connects to a serial port, baud rate, and parsing profile.
- `disconnect_port`: Releases the port handle, allowing external flash tools (such as `west flash`, `pyocd`, `openocd`) to flash new firmware without quitting the AI session.
- `get_device_status`: Returns current state (`connected`, `port`, `baud`, `profile`, `buffer_count`, `buffer_cap`, `uptime_sec`).

### Log Inspection & Analysis
- **Formatted Record Metadata**: All log records returned by MCP tools include standardized metadata prefixes containing the unique record ID and host timestamp:
  ```text
  [ID:450 | 14:15:30.125] [  250.125000] <inf> net: connected
  ```
  This allows AI agents to directly reference events by `#ID` or timestamp.
- `get_recent_logs`: Retrieves recent records from the in-memory ring buffer (including pre-existing logs from active `oml` TUI sessions).
  - `count`: Number of lines to return (default: `50`, max: `10000`).
  - `all`: Boolean flag to retrieve all available records from memory history regardless of count limit (default: `false`).
  - `min_level`: Filter by minimum severity level (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`).
  - `module`: Filter by module or tag name substring.
  - `since_id`: Only return records with ID greater than this checkpoint ID.
  - `until_id`: Only return records with ID less than or equal to this upper bound ID.
  - `start_time`: Optional start time or device uptime boundary (e.g. `'250.0'`, `'14:15:00'`).
  - `end_time`: Optional end time or device uptime boundary (e.g. `'255.0'`, `'14:15:30'`).
- `search_logs`: Searches the log buffer history using substring match or regular expression (`is_regex: true`), with optional `start_time` and `end_time` boundaries.
- `get_logs_in_timerange`: Retrieves records falling within a specified time interval or device uptime range.
  - `from_time`: Start boundary (e.g. uptime float `'250.0'`, clock time `'14:15:00'`, or ISO-8601).
  - `to_time`: End boundary (e.g. uptime float `'255.0'`, clock time `'14:15:30'`, or ISO-8601).
  - `query`: Optional text or regex filter within the time range.
  - `min_level`, `module`: Optional level and module filters.
  - `max_results`: Maximum lines to return (default: `100`, max: `2000`).
- `get_logs_around`: Inspects the context window of logs surrounding a specific pivot event. Perfect for seeing what happened immediately before and after an assert, reboot, fault, or uptime milestone.
  - `target_id`: Log record ID to center around (e.g. `450`).
  - `target_time`: Timestamp, clock time, or device uptime to center around (e.g. `'250.'`, `'250.125'`, `'14:15:30.123'`). Finds the closest matching record.
  - `target_query`: Search query string or regex to find the pivot record.
  - `before`: Lines to show before the target event (default: `20`, max: `200`).
  - `after`: Lines to show after the target event (default: `20`, max: `200`).
  - `window`: Total context window size if `before`/`after` are omitted.
- `clear_log_buffer`: Resets in-memory log history for clean test runs.

### Command Execution
- `send_serial_command`: Transmits a command string over serial TX and collects response output during a configurable wait window.
  - `command`: Command string to send (e.g. `kernel uptime`, `help`, `version`).
  - `wait_ms`: Response collection window in milliseconds (default: `500`).
  - `ending`: Line ending (`crlf`, `lf`, `cr`, `none`).

### Watchdogs & Active Monitoring
- `wait_for_log`: Blocks until a specific string or regular expression appears in the logs or timeout occurs (up to 120s). Useful for asserting boot completion or crash absence.
- `watch_session`: Observes incoming logs over a time window (e.g. 10s to 60s) with optional `stop_on_pattern`. Perfect when the AI requests the developer to perform manual hardware operations (e.g. *"Please press the reset button or plug in the USB sensor"*).

---

## Example Prompts for AI Assistants

Once configured, you can prompt your AI assistant naturally:

- *"Check which serial ports are open and connect to our board."*
- *"Are there any ERROR or WARN logs in the device buffer?"*
- *"Show me what happened around uptime 250s (or around log ID 450)."*
- *"Search for all logs between uptime 240.0 and 260.0."*
- *"Find where 'assert failed' occurred and show me 30 lines before and after."*
- *"Send the command 'version' over serial and tell me what firmware is running."*
- *"I'm going to press the user button on the board now; please watch the logs for 15 seconds and summarize what happens."*
- *"Disconnect the serial port so I can flash new firmware with west flash."*
