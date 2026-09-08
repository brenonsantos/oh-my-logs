# oh-my-logs (`oml`) — Implementation Plan

## Project: `oh-my-logs` (binary: `oml`)
**Language:** Go | **TUI:** Bubble Tea + Lip Gloss | **Serial:** go.bug.st/serial

---

## Phase 1 — Project skeleton & core data model ✅
- [x] Initialize Go module (`go mod init github.com/brenoniehues/oh-my-logs`)
- [x] Install dependencies (bubbletea, bubbles, lipgloss, go.bug.st/serial, yaml.v3)
- [x] `internal/record/record.go` — generic `Record{Fields, Raw}` and `Column{Field, Title, Width}`
- [x] `internal/record/buffer.go` — ring buffer (default 50,000 records)
- [x] `internal/record/buffer_test.go` — unit tests

## Phase 2 — Parser layer ✅
- [x] `internal/parser/parser.go` — `Parser` interface
- [x] `internal/parser/raw.go` — raw line → `{message: line}`
- [x] `internal/parser/regex.go` — named capture group regex parser
- [x] `internal/parser/profile.go` — YAML profile loader (`name`, `parser`, `columns`)
- [x] `internal/parser/parser_test.go` — unit tests

## Phase 3 — Filter layer ✅
- [x] `internal/filter/expression.go` — parse filter string into expressions
- [x] `internal/filter/matcher.go` — evaluate expressions against a `Record`
- [x] `internal/filter/filter.go` — `Filter` entry point
- [x] `internal/filter/filter_test.go` — unit tests

## Phase 4 — Serial & source abstraction ✅
- [x] `internal/serial/config.go` — `Config{Port, Baud, DataBits, StopBits, Parity}` + mode mapping
- [x] `internal/serial/port.go` — port enumeration
- [x] `internal/serial/reader.go` — `Source` interface, `SerialSource`, `FileSource`

## Phase 5 — App config ✅
- [x] `internal/config/config.go` — OS config dir, profiles dir, logs dir, profile enumeration

## Phase 6 — TUI ✅
- [x] `internal/tui/styles.go` — Lip Gloss colour palette + style definitions
- [x] `internal/tui/keys.go` — key bindings (bubbles/key)
- [x] `internal/tui/model.go` — Bubble Tea `Model` struct
- [x] `internal/tui/update.go` — `Update()` + all message/key handling
- [x] `internal/tui/view.go` — `View()` rendering (title bar, dynamic columns, status, key hints)

## Phase 7 — Entrypoint & profiles ✅
- [x] `cmd/oml/main.go` — CLI flags, wires all layers
- [x] `profiles/examples/stm32-pdm.yaml`
- [x] `profiles/examples/legacy-ecu.yaml` (no level field)
- [x] `profiles/examples/raw.yaml`

## Phase 8 — Tests ✅
- [x] `internal/record/buffer_test.go`
- [x] `internal/parser/parser_test.go`
- [x] `internal/filter/filter_test.go`

## Phase 9 — Docs ✅
- [x] `README.md`

---

## MVP Checklist (spec §29)
- [x] Serial port enumeration
- [x] Serial connection + baud config
- [x] Line-based serial reading
- [x] Generic `Record`
- [x] Raw parser
- [x] Regex parser with named capture groups
- [x] YAML profiles with profile-defined columns
- [x] Scrollable log table
- [x] Follow mode / Pause mode
- [x] Ring buffer (50,000 default)
- [x] Ctrl+F search with highlighting + n/N navigation
- [x] Logcat-style filtering (`term`, `-term`, `field:val`, `field:v1,v2`, AND)
- [x] Clear
- [x] Save raw log
- [x] Clean exit
- [x] Unit tests (buffer, parser, filter)
- [x] README
- [x] `go build ./...` — clean ✅
- [x] `go test -race ./...` — all pass ✅

---

## Possible next steps
- [ ] Port selection dialog inside TUI (interactive port picker)
- [ ] Reconnect logic (automatic retry on disconnect)
- [ ] Profile switcher inside TUI
- [ ] Saved filter presets
- [ ] Column visibility toggle
- [ ] Statistics panel (field value counts)
- [ ] Mouse support
