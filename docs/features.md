# Features

## Implemented Features

- **Log Streaming & Follow**: Stream serial ports and files with auto-follow and pause (`Space`).
- **Ring Buffer**: Configurable circular buffer (default 50,000 records) to bound memory usage.
- **Filtering**: Filter by keywords, exclusions (`-term`), fields (`level:err`), or comma-separated OR syntax.
- **Input Editing**: Cursor movement (`←`/`→`, `Home`/`End`, word jumps) and paste (`Ctrl+V`) in all prompts.
- **Filter Presets & History**: Save and load filter presets (`F`) and navigate history with `↑`/`↓`.
- **Search**: Search logs with `Ctrl+F` and jump between matches with `n`/`N`.
- **Virtual Tabs**: Multiple tabs (`Ctrl+T`, `Tab`) with independent filters, display formats, and scroll offsets.
- **Split View**: Vertical (`|`) and horizontal (`_`) pane splitting with optional synchronized scrolling (`S`).
- **2D Viewport Panning**: Pan horizontal viewport (`{`/`}`, Shift+Wheel, trackpad tilt).
- **Character Cursor & Substring Copy**: Move character cursor (`←`/`h`, `→`/`l`), select text (`Shift+←`/`→`), and copy to clipboard (`y`).
- **Interactive Scrollbars**: Clickable and draggable vertical and horizontal scrollbars. Horizontal bar only appears when lines exceed window width, bounded to line length.
- **Multi-Format Display**: Toggle between parsed, raw, hex dump, and binary bit views (`x`).
- **Delta Timing (`Δt`)**: Show elapsed time between logs (`t`) and calculate total delta across row selections.
- **Serial Send (TX)**: Send text commands (`i`) with selectable line endings and input history.
- **TX Echo**: Echo sent commands into the log stream for filtering and bookmarking.
- **Bookmarking**: Pin rows (`b`), jump between pins (`[`/`]`), and filter to pinned rows (`B`).
- **Parsing Profiles**: Define column layouts and regex patterns via YAML profiles.
- **Zephyr RTOS Profile**: Built-in parser for Zephyr uptime, log levels, module tags, and messages.
- **Port & Baud Picker**: List serial ports and set baud rates (`p`).
- **Profile Switcher**: Switch profiles on the fly (`P`) and re-parse buffered logs.
- **Row Selection & Copy**: Select rows with mouse or `Shift+↑`/`↓` and copy to clipboard (`y`/`Y`).
- **Auto-Reconnect**: Automatically reconnects when serial devices drop, with manual retry (`r`).
- **Port Disconnect**: Release serial port (`D`) to flash firmware without quitting `oml`.
- **Settings Modal**: Configure buffer size, themes, serial defaults, and display format (`,` or `C`).
- **Themes**: Multiple color schemes (Dark Slate, Monokai, Nord, Gruvbox, Tokyo Night, High Contrast).
- **In-Place Self-Updater**: Update binary directly (`oml --update`) and uninstall (`oml --uninstall`).
- **Direct-to-Disk Logging**: Stream incoming raw logs directly to disk without ring-buffer limits, with top-bar `🔴 REC` indicator, directory/file picker in Settings, and CLI flags (`--tee <path>`, `--direct-to-disk`).
- **Row Detail Inspector**: View all parsed fields, pretty-printed structured payloads (automatic detection of JSON, XML, YAML, and Logfmt with syntax colorization), word-wrapped raw text, and hex preview in an interactive modal (`Enter` or `v`), with vertical scrolling (`↑`/`↓`, `PgUp`/`PgDn`), row hopping (`←`/`→`, `[`/`]`), quick bookmarking (`b`), and clipboard copy (`y`/`Y`).
- **Mini-Games**: Built-in games (Snake, Tetris, 2048, etc.) accessible with `Ctrl+G`.

---

## Planned Features

- **Periodic TX Pings & Macros**: Send heartbeat pings on a timer and assign quick macros (`F1`–`F8`).
- **Regex Triggers & Responses**: Auto-reply to specific patterns (e.g. `PING` → `PONG`) and pause on crash strings.
- **Modbus & NMEA Profiles**: Parsers for Modbus RTU frames and NMEA 0183 GPS sentences.
- **Column Customization**: Toggle column visibility and adjust widths interactively.
- **Multi-Port Support**: Open different serial ports across tabs.
- **Network Sources**: Connect to TCP sockets (Telnet/Wi-Fi serial) and UDP syslog.
- **Structured Export**: Export logs to CSV, JSON Lines, or Markdown.
- **Throughput Metrics**: Rolling logs/sec throughput display.
- **Scripting Hooks**: User scripts for custom CRC checks and decoding.
