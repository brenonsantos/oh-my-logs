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
- **Select All & Precision Mouse Selection**:
  - `Ctrl+A`: Select all records in active view for instant batch copying.
  - Double-click: Instant copy of clicked word or token to OS clipboard.
  - Horizontal drag: Real-time substring character selection on the same row with auto-copy on release.
  - Normal vertical drag: Editor-style whole-line selection (`row1\nrow2\nrow3`) ready to copy with `Ctrl+C` or `y`.
  - Modifier drag (`Option/Alt` or `Ctrl` + Drag): Rectangular column / square block selection with auto-copy on release.
- **Interactive Scrollbars**: Clickable and draggable vertical and horizontal scrollbars. Horizontal bar only appears when lines exceed window width, bounded to line length.
- **Multi-Format Display**: Toggle between parsed, raw, hex dump, and binary bit views (`x`).
- **Column Customization Modal**: Interactively toggle column visibility and adjust column widths on the fly (`o`).
- **Timestamp Jump & Time Seeking**: Jump directly to relative offsets (`-5m`, `+30s`) or absolute wall-clock times (`14:20:00`) with interactive input and chronological search (`J`).
- **Live Bench Markers**: Inject annotated milestone checkpoint notes directly into the stream timeline (`m`).
- **Delta Timing (`Δt`)**: Show elapsed time between logs (`t`) and calculate total delta across row selections.
- **Serial Send (TX)**: Send text commands (`i`) with selectable line endings (`Ctrl+E`) and input history.
- **Interactive Shell Passthrough ("minicom-inside-oml")**: Bidirectional raw interactive terminal mode (`I` / `Ctrl+I`) supporting full VT100 escape sequences, `<Tab>` completion, arrow history, and `^C` interrupt forwarding.
- **TX Echo**: Echo sent commands into the log stream for filtering and bookmarking.
- **Bookmarking**: Pin rows (`b`), jump between pins (`[`/`]`), and filter to pinned rows (`B`).
- **Parsing Profiles & Subcommands**: Define column layouts and regex patterns via YAML profiles. First-class `oml profile` CLI (`list`, `install`, `update`, `uninstall`, `show`, `export`, `path`) with numeric IDs (`#1`), versioning, and upstream upgrades from URLs or Git repositories.
- **Zephyr RTOS Profile**: Built-in parser for Zephyr uptime, log levels, module tags, and messages.
- **Dual-Engine Payload Decoders**: Declarative regex/template formatters and procedural worker processes (`exec`) with persistent stdio streaming, auto-restart, and timeout protection.
- **Companion Decoder Bundles**: Install profiles with companion scripts/databases into isolated namespaces (`~/.config/oml/decoders/<profile>/`) with automatic `$PATH` and interpreter script resolution.
- **Universal Interactive Terminal Cleaner**: Automatically strips shell prompt noise (`uart:~$ `, `device:~$ `) and terminal cursor erasure sequences (`\x1b[9D\x1b[J`) emitted by RTOS shells without corrupting colors.
- **Port & Baud Picker**: List serial ports and set baud rates (`p`).
- **Profile Switcher**: Switch profiles on the fly (`P`) and re-parse buffered logs.
- **Row Selection & Copy**: Select rows with mouse or `Shift+↑`/`↓` and copy to clipboard (`y`/`Y`/`Ctrl+C`).
- **Auto-Reconnect**: Automatically reconnects when serial devices drop, with manual retry (`r`).
- **Port Disconnect**: Release serial port (`D`) to flash firmware without quitting `oml`.
- **Settings Modal**: Configure buffer size, themes, serial defaults, and display format (`,` or `C`).
- **Themes**: Multiple color schemes (Dark Slate, Monokai, Nord, Gruvbox, Tokyo Night, High Contrast).
- **In-Place Self-Updater**: Update binary directly (`oml --update`) and uninstall (`oml --uninstall`).
- **Direct-to-Disk Logging**: Stream incoming raw logs directly to disk without ring-buffer limits, with top-bar `🔴 REC` indicator, directory/file picker in Settings, and CLI flags (`--tee <path>`, `--direct-to-disk`).
- **Custom Process Streams & Stdin Pipeline**: Stream logs directly from external commands (`oml --cmd "adb logcat -v threadtime"` / `--exec`) or piped standard input (`adb logcat | oml`, `cat file.log | oml`) with terminal redirection for full keyboard/mouse TUI interactivity, bi-directional stdin TX (`i`), and hot process restart (`r`).
- **Row Detail Inspector**: View all parsed fields, pretty-printed structured payloads (automatic detection of JSON, XML, YAML, and Logfmt with syntax colorization), word-wrapped raw text, and hex preview in an interactive modal (`Enter` or `v`), with vertical scrolling (`↑`/`↓`, `PgUp`/`PgDn`), row hopping (`←`/`→`, `[`/`]`), quick bookmarking (`b`), and clipboard copy (`y`/`Y`).
- **Native Model Context Protocol (MCP) Server**: Headless `oml mcp` server exposing full hardware port control, log querying, watchdog triggers, and serial transmission over local IPC to AI coding assistants (Gemini CLI, Claude Desktop, Cursor).
- **Mini-Games**: Built-in games (Snake, Pong, Tetris, 2048, and the 10-second mouse-driven **Aim Trainer Blitz**) accessible with `Ctrl+G` or `g` on empty buffer.

---

## Planned Features

- **Periodic TX Pings & Macros**: Send heartbeat pings on a timer and assign quick macros (`F1`–`F8`).
- **Regex Triggers & Responses**: Auto-reply to specific patterns (e.g. `PING` → `PONG`) and pause on crash strings.
- **Modbus & NMEA Profiles**: Parsers for Modbus RTU frames and NMEA 0183 GPS sentences.
- **Multi-Port Support**: Open different serial ports across tabs.
- **Network Sources**: Connect to TCP sockets (Telnet/Wi-Fi serial) and UDP syslog.
- **Structured Export**: Export logs to CSV, JSON Lines, or Markdown.
- **Throughput Metrics**: Rolling logs/sec throughput display.
- **Scripting Hooks**: User scripts for custom CRC checks and decoding.
