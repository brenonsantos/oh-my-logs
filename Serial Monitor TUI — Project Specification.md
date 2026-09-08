# Serial Monitor TUI

Build a cross-platform terminal UI (TUI) application in Go for monitoring, filtering, searching, and recording serial output from embedded systems.

The application should be designed primarily for firmware/embedded developers who work with multiple products and architectures where **logging formats differ significantly**.

The application must NOT assume that logs contain standard fields such as timestamp, log level, module, or message.

The core concept is:

> A serial stream is parsed according to a user-selected profile into arbitrary named fields, and the TUI renders those fields as configurable columns.

The application should initially be a TUI only. Do not build a GUI.

---

# 1. Goals

The application should allow a developer to:

- Select a serial port.
- Configure baud rate and serial parameters.
- Connect/disconnect from the serial device.
- Continuously receive serial data.
- Parse incoming data using a configurable parser profile.
- Display parsed records in a table.
- Define arbitrary columns for each architecture/profile.
- Search through received records.
- Filter records.
- Exclude records.
- Pause/follow the live stream.
- Clear the display.
- Save received data to a log file.
- Replay previously captured logs without a serial device.
- Support completely different logging formats without changing the TUI code.

The application should feel similar to tools such as Logcat, but should be much more generic.

---

# 2. Important architectural principle

DO NOT create a fixed structure such as:

```go
type LogEntry struct {
    Timestamp time.Time
    Level     Level
    Module    string
    Message   string
}
```

This is intentionally too restrictive.

Some architectures may have:

```text
timestamp
level
module
message
```

Others may have:

```text
timestamp
cpu
task
event
code
message
```

Others may have:

```text
address
operation
data
result
```

Others may have only:

```text
message
```

Therefore the normalized representation should be generic.

Use something conceptually similar to:

```go
type Record struct {
    Fields map[string]string
    Raw    string
}
```

The parser/profile determines which fields exist.

The UI determines which fields are displayed as columns.

---

# 3. Technology

Use:

- Go
- Bubble Tea for the TUI
- Bubbles for reusable TUI components where appropriate
- Lip Gloss for styling
- `go.bug.st/serial` for serial communication

Avoid unnecessary dependencies.

The application should compile into a single executable.

Target platforms:

- Linux
- Windows
- macOS

Do not use PlatformIO or any embedded-specific build system.

---

# 4. High-level architecture

Use a layered architecture:

```text
                    ┌─────────────────────┐
                    │       Serial        │
                    │       Input         │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │      Framing        │
                    │ line/record reader  │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │       Parser        │
                    │  selected profile   │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │      Record         │
                    │ arbitrary fields    │
                    └──────────┬──────────┘
                               │
                 ┌─────────────┴─────────────┐
                 ▼                           ▼
        ┌─────────────────┐         ┌─────────────────┐
        │    Storage      │         │     Filters     │
        │ ring buffer     │         │ search/filter   │
        └────────┬────────┘         └────────┬────────┘
                 │                           │
                 └─────────────┬─────────────┘
                               ▼
                    ┌─────────────────────┐
                    │        TUI          │
                    │ table + controls    │
                    └─────────────────────┘
```

The serial layer must not know anything about the TUI.

The parser must not know anything about the TUI.

The filtering system must not know anything about serial ports.

The TUI should consume application state/events.

---

# 5. Suggested project structure

Use a structure similar to:

```text
serialmon/
├── cmd/
│   └── serialmon/
│       └── main.go
│
├── internal/
│   ├── serial/
│   │   ├── port.go
│   │   ├── reader.go
│   │   └── config.go
│   │
│   ├── record/
│   │   ├── record.go
│   │   ├── buffer.go
│   │   └── columns.go
│   │
│   ├── parser/
│   │   ├── parser.go
│   │   ├── regex.go
│   │   ├── raw.go
│   │   └── profile.go
│   │
│   ├── filter/
│   │   ├── filter.go
│   │   ├── expression.go
│   │   └── matcher.go
│   │
│   ├── config/
│   │   ├── config.go
│   │   └── loader.go
│   │
│   └── tui/
│       ├── model.go
│       ├── update.go
│       ├── view.go
│       ├── keys.go
│       └── styles.go
│
├── profiles/
│   └── examples/
│
├── testdata/
│
├── go.mod
└── README.md
```

Adjust this structure if a better Go architecture is appropriate, but preserve the separation of responsibilities.

---

# 6. Record model

The fundamental data object should be generic.

For example:

```go
type Record struct {
    Fields map[string]string
    Raw    string
}
```

Do not impose predefined semantic fields.

A parser may produce:

```text
Fields:
    time    = "15:42:31.102"
    level   = "WARN"
    module  = "ADC"
    message = "Channel 3 reading high"
```

Another parser may produce:

```text
Fields:
    timestamp = "15:42:31"
    cpu       = "1"
    task      = "MotorControl"
    code      = "0x123"
    text      = "Overcurrent detected"
```

Another:

```text
Fields:
    address = "0x08001234"
    operation = "WRITE"
    data = "01 02 03 04"
```

The TUI must handle all three.

---

# 7. Profiles

A profile describes how a particular architecture/product's serial output should be parsed and displayed.

Profiles should eventually be external configuration files rather than hardcoded Go code.

Use YAML or TOML. Prefer YAML unless there is a strong reason otherwise.

Example:

```yaml
name: STM32-PDM

parser:
  type: regex
  pattern: '^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$'

columns:
  - field: time
    title: Time
    width: 14

  - field: level
    title: Level
    width: 8

  - field: module
    title: Module
    width: 10

  - field: message
    title: Message
    width: 0
```

Another profile can define completely different columns:

```yaml
name: Legacy-ECU

parser:
  type: regex
  pattern: '^(?P<cpu>\d+);(?P<task>[^;]+);(?P<code>[^;]+);(?P<text>.*)$'

columns:
  - field: cpu
    title: CPU
    width: 5

  - field: task
    title: Task
    width: 18

  - field: code
    title: Code
    width: 10

  - field: text
    title: Text
    width: 0
```

There should be no assumption that a profile contains `level`.

---

# 8. Parser abstraction

Create an interface similar to:

```go
type Parser interface {
    Parse(input string) (Record, error)
}
```

A parser profile should determine how a line becomes a Record.

Initially implement:

1. Raw parser
2. Regex parser

Raw parser:

```text
Incoming line:
Hello world
```

produces:

```text
Fields:
    message = "Hello world"
```

Regex parser should support named capture groups.

For example:

```regex
^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$
```

becomes:

```text
time    = ...
level   = ...
module  = ...
message = ...
```

If a line cannot be parsed, do not crash.

The application should continue receiving data.

Provide a configurable behavior for unparsed lines. Initially, preserve them as a record containing:

```text
_raw = original line
```

or equivalent.

---

# 9. Columns

Columns are defined by the profile.

A column should contain at minimum:

```go
type Column struct {
    Field string
    Title string
    Width int
}
```

Possible future fields:

```text
Alignment
Formatter
Color
Visibility
```

Do not implement unnecessary features in the first version.

If `Width == 0`, treat it as flexible/remaining width.

The TUI must render the columns in the order defined by the profile.

If a record does not contain a requested field, render an empty cell.

---

# 10. Search

Implement Ctrl+F.

Search should operate on visible/current records and search across the textual field values.

Search UI:

```text
Search: motor█
```

Behavior:

- Ctrl+F opens search.
- Enter confirms.
- Esc closes search.
- Next match can be selected with Enter or a dedicated key.
- Previous match should also be supported.
- Search should highlight the matching text where practical.

Search is different from filtering.

Search should navigate through matches without permanently removing non-matching records.

---

# 11. Filtering

Implement a simple Logcat-like filtering language.

Initial syntax:

```text
motor
```

means:

```text
contains "motor"
```

Negative:

```text
-motor
```

means:

```text
does not contain "motor"
```

Field-specific:

```text
module:CAN
```

Multiple expressions:

```text
motor module:CAN
```

means AND:

```text
contains motor
AND module == CAN
```

Negative field:

```text
-module:CAN
```

Multiple possible values:

```text
level:error,warn
```

should mean:

```text
level == error OR level == warn
```

However, the filtering system must remain generic.

`level:error` is just shorthand for:

```text
field "level" equals "error"
```

It must also work with arbitrary fields:

```text
task:MotorControl
cpu:1
code:0x123
operation:WRITE
```

Do not implement level-specific filtering logic.

---

# 12. Filter UI

The right-side panel can contain:

```text
FILTER

> motor -heartbeat module:CAN

Clear
```

Eventually support saved filter presets.

For the MVP, one active filter expression is sufficient.

---

# 13. TUI layout

The initial visual design should be a dark, technical, developer-oriented interface.

Conceptually:

```text
┌ Serial Monitor ────────────────────────────────────────────────────────────┐
│ Port: /dev/ttyACM0   Baud: 115200   ● Connected      Profile: STM32-PDM   │
├───────────────────────────────────────────────────────────────┬────────────┤
│ Time          Level   Module   Message                        │ Search     │
├───────────────────────────────────────────────────────────────┤            │
│ 15:42:31.102  INFO    SYS      System initialized             │ Filter     │
│ 15:42:31.254  INFO    CAN      CAN initialized                │            │
│ 15:42:32.103  WARN    ADC      Reading high                  │ Columns    │
│ 15:42:33.876  ERROR   PDM      Overcurrent detected           │            │
│ 15:42:34.112  INFO    CAN      Message received               │ Profile    │
│                                                               │            │
│                                                               │ Serial     │
├───────────────────────────────────────────────────────────────┴────────────┤
│ 1248 messages │ 32 filtered │ ERROR: 3 │ WARN: 29 │ FOLLOW                │
├────────────────────────────────────────────────────────────────────────────┤
│ Ctrl+F Search   f Filter   c Clear   Space Pause   ↑↓ Scroll   q Quit      │
└────────────────────────────────────────────────────────────────────────────┘
```

The right-side configuration panel is optional if terminal width is insufficient.

The application must remain usable on smaller terminals.

---

# 14. Profile-specific columns

This is one of the most important features.

When the user selects a different profile, the table should change automatically.

Example profile A:

```text
TIME          LEVEL   MODULE   MESSAGE
15:42:31.102  INFO    CAN      Initialized
15:42:31.552  WARN    ADC      Reading high
```

Profile B:

```text
CPU   TASK           CODE     TEXT
1     MotorControl   0x123    Started
2     CAN            0x455    RX timeout
```

Profile C:

```text
ADDRESS      OPERATION   DATA
0x08001234   WRITE       01 02 03 04
0x08001238   VERIFY      OK
```

The TUI itself should not contain code such as:

```go
if profile == "STM32" {
    ...
}
```

Profiles should drive the UI.

---

# 15. Serial configuration

Support at minimum:

- Port
- Baud rate
- Data bits
- Stop bits
- Parity

Common baud-rate presets:

```text
9600
19200
38400
57600
115200
230400
460800
921600
```

The user should be able to select the serial port from the TUI.

Initially prioritize:

```text
Port
Baud
8N1
```

but design the configuration so additional settings are easy to add.

---

# 16. Connection state

Display connection status prominently:

```text
● Connected
○ Disconnected
⚠ Error
```

Connection errors should be visible without crashing the application.

Support reconnecting.

---

# 17. Follow mode

When new records arrive, the table should normally stay at the bottom.

This is "Follow" mode.

If the user scrolls upward, automatically disable follow.

Example:

```text
FOLLOW
```

When disabled:

```text
PAUSED
```

or:

```text
FOLLOW OFF
```

Provide a shortcut to return to the newest record.

Suggested:

```text
g
```

or:

```text
End
```

---

# 18. Pause

Pause should stop updating the visual viewport but should NOT necessarily stop reading the serial port.

Prefer:

```text
Serial
   ↓
Reader
   ↓
Buffer
   ↓
TUI paused
```

rather than stopping the serial reader.

This means logs aren't lost merely because the user temporarily pauses the UI.

The ring buffer should continue receiving records.

---

# 19. Ring buffer

Do not store an unlimited number of records by default.

Implement a configurable ring buffer.

Default:

```text
50,000 records
```

Possible future configuration:

```text
10,000
50,000
100,000
1,000,000
```

The buffer should efficiently discard the oldest records when full.

---

# 20. Recording

Implement the ability to save received data.

At minimum:

```text
Save Log
```

should save the raw serial lines.

Prefer preserving the original raw data rather than only saving parsed fields.

This allows the log to be replayed with a different parser later.

---

# 21. Replay

Implement a future-friendly architecture for replaying files.

The source of records should be abstracted from the beginning.

Conceptually:

```go
type Source interface {
    Read() (...)
}
```

Potential sources:

```text
SerialSource
FileSource
```

This allows:

```text
Serial device → parser → records → TUI
```

and:

```text
Log file → parser → records → TUI
```

to use the same processing pipeline.

Replay does not need sophisticated timing control in the first MVP.

A simple "open log file and parse it" mode is sufficient.

---

# 22. Statistics

Display basic statistics in the footer.

Do NOT assume levels exist.

If a profile contains a `level` field, optionally show counts:

```text
ERROR: 3
WARN: 29
INFO: 1184
```

If it does not contain a level field, do not display level statistics.

The application must continue to work normally for profiles with no level field.

This is an important example of the UI being driven by profile capabilities rather than hardcoded assumptions.

---

# 23. Configuration directory

Use an OS-appropriate configuration directory.

Store:

```text
profiles/
settings/
saved filters/
```

Keep the exact implementation idiomatic to Go and the target operating systems.

Profiles should be easy for users to edit manually.

---

# 24. CLI

Support basic startup options:

```text
serialmon
serialmon --port COM3
serialmon --port /dev/ttyACM0
serialmon --baud 115200
serialmon --profile STM32-PDM
serialmon --file test.log
```

Support:

```text
--help
--version
```

If both a serial port and file are specified, reject the configuration with a clear error.

---

# 25. Keyboard controls

Initial controls:

```text
Ctrl+F     Search
f          Filter
c          Clear
Space      Pause/resume
↑ / ↓      Scroll
PageUp     Scroll page up
PageDown   Scroll page down
g          Go to newest
p          Serial port
r          Reconnect
s          Save log
q          Quit
Ctrl+C     Quit
```

Do not require mouse support for the MVP.

Mouse support can be added later if Bubble Tea makes it easy.

---

# 26. Threading/concurrency

Serial reading must not block the TUI.

Use a dedicated goroutine or equivalent asynchronous mechanism.

Conceptually:

```text
serial goroutine
      │
      │ records/events
      ▼
application event channel
      │
      ▼
Bubble Tea Update()
```

Protect shared state appropriately.

Avoid data races.

The application must pass:

```bash
go test -race ./...
```

---

# 27. Error handling

The application should be resilient.

Malformed input must not crash the application.

Examples:

- Invalid UTF-8
- Partial serial lines
- Parser mismatch
- Serial disconnect
- Port disappearing
- Invalid profile
- Invalid regex
- Invalid filter

Errors should be presented to the user in the TUI where appropriate.

A malformed record should not terminate the serial reader.

---

# 28. Testing

Write unit tests for:

### Parser

Test:

```text
valid line
invalid line
missing field
empty field
special characters
```

### Filters

Test:

```text
contains
exclude
field:value
negative field
multiple AND expressions
comma-separated values
case sensitivity
```

### Ring buffer

Test:

```text
empty
partially filled
full
overflow
ordering
```

### Profile loading

Test:

```text
valid profile
missing fields
invalid regex
invalid YAML
```

### Serial layer

Mock the serial source rather than requiring physical hardware for tests.

---

# 29. MVP definition

Do not attempt to implement every possible feature immediately.

The first working version must provide:

1. Go project setup.
2. Bubble Tea TUI.
3. Serial port enumeration.
4. Serial connection.
5. Baud-rate selection.
6. Line-based serial reading.
7. Generic `Record`.
8. Raw parser.
9. Regex parser with named capture groups.
10. YAML profiles.
11. Profile-defined columns.
12. Scrollable log table.
13. Follow mode.
14. Pause mode.
15. Ring buffer.
16. Ctrl+F search.
17. Basic filtering.
18. Clear.
19. Save raw log.
20. Clean exit/reconnect.
21. Unit tests.
22. README with examples.

Do not implement graphs, dashboards, binary protocol decoding, plugins, scripting, or a GUI in the MVP.

---

# 30. Example end-to-end profile

Create an example profile:

```yaml
name: STM32-PDM

parser:
  type: regex
  pattern: '^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$'

columns:
  - field: time
    title: Time
    width: 14

  - field: level
    title: Level
    width: 8

  - field: module
    title: Module
    width: 10

  - field: message
    title: Message
    width: 0
```

Example input:

```text
[15:42:31.102][INFO][SYS] System initialized
[15:42:31.254][INFO][CAN] CAN initialized
[15:42:32.103][WARN][ADC] Channel 3 reading high
[15:42:33.876][ERROR][PDM] Overcurrent detected
```

Expected table:

```text
Time          Level    Module    Message
15:42:31.102  INFO     SYS       System initialized
15:42:31.254  INFO     CAN       CAN initialized
15:42:32.103  WARN     ADC       Channel 3 reading high
15:42:33.876  ERROR    PDM       Overcurrent detected
```

---

# 31. Example architecture without log levels

Create another profile specifically demonstrating that no log level is required:

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

The application must render this correctly without attempting to invent a Level column.

---

# 32. Design philosophy

Prioritize:

1. Generic data model.
2. Clean separation between serial, parsing, filtering, storage, and UI.
3. Profile-driven behavior.
4. Simple configuration.
5. Fast rendering.
6. Low memory usage.
7. Good keyboard UX.
8. Robustness with malformed serial data.

Avoid:

- Hardcoded architecture-specific logic.
- Assuming every record has a level.
- Assuming every record has a timestamp.
- Assuming every record has a message.
- Putting parser logic inside Bubble Tea models.
- Blocking the UI on serial reads.
- Overengineering the configuration language.
- Building GUI functionality before the TUI/core architecture is stable.

---

# 33. Definition of done

The project is considered complete for MVP when I can run:

```bash
serialmon --port /dev/ttyACM0 --baud 115200 --profile STM32-PDM
```

and:

1. Select/connect to the serial port.
2. See incoming records in a scrollable table.
3. See columns defined by the selected profile.
4. Use Ctrl+F to search.
5. Use `f` to filter.
6. Exclude records with `-text`.
7. Filter arbitrary fields with `field:value`.
8. Pause/follow the stream.
9. Clear records.
10. Save the raw serial output.
11. Switch to a profile with completely different columns.
12. Use a profile that contains no `level` field.
13. Continue operating when individual lines fail to parse.
14. Run all unit tests successfully.
15. Run successfully under the Go race detector.

The implementation should be idiomatic Go, reasonably documented, and structured so that a future GUI frontend could reuse the core serial/parser/filter/storage packages without rewriting them.