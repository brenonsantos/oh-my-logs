# Aim Trainer Mini-Game (10-Second Blitz) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a fast-paced, mouse-driven 10-second aim training mini-game inside `oml` (`Ctrl+G`), with 3 simultaneous targets centered with the `🎯` emoji, instant respawning, reaction time tracking, combo streaks, and end-of-round performance statistics.

**Architecture:** A new `AimTrainer` game struct implementing `game.MiniGame` in `internal/game/aim_trainer.go`. Target hit detection uses 6x3 bounding boxes with a centered `🎯` emoji. Screen mouse clicks in `m.mode == modeGame` are translated to playfield coordinates and dispatched to the active game.

**Tech Stack:** Go 1.22+, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`).

**Spec:** User requirements aligned during brainstorming: 10-second blitz mode, 3 targets constantly on screen, mouse-driven clicking, centered `🎯` emoji, reaction time & accuracy statistics.

## Global Constraints
- Zero proprietary words (`zcore`, `whirlpool`, `smp`).
- Build must be 100% warning-free.
- All tests must pass with `go test -count=1 -race ./...`.

## Review Focus
- Mouse coordinate offset translation when modal is centered in terminal.
- Multi-byte emoji width rendering: `🎯` is 2 terminal cells wide; centered in a 6-cell box (`│ 🎯 │`).
- Non-overlapping target generation ensuring 3 active targets are always separated.
- 10.0-second timer countdown and sub-second precision display.
- Clean restart on `R` / `Space` / Click without leaking goroutines or state.

---

### Task 1: Aim Trainer Core Game Engine (`internal/game/aim_trainer.go`)

**Files:**
- Create: `internal/game/aim_trainer.go`
- Test: `internal/game/aim_trainer_test.go`

- [ ] **Step 1: Write unit tests in `internal/game/aim_trainer_test.go`**:
  - Test `NewAimTrainer()` initial state: 3 active non-overlapping targets, 10.0s time remaining, score 0, combo 1.
  - Test `HandleClick(x, y)`:
    - Click inside target bounding box: returns `true`, updates score, records reaction time, spawns replacement target.
    - Click outside target: returns `false`, resets combo to 1, increments misses.
  - Test timer countdown via `TickMsg` until game over.
  - Test game over stats: accuracy %, average reaction time, rank evaluation.
- [ ] **Step 2: Run test to verify it fails before implementation**:
  - `go test -v ./internal/game -run TestAimTrainer`
- [ ] **Step 3: Implement `AimTrainer` in `internal/game/aim_trainer.go`**:
  - Target representation: `Target` with `X`, `Y`, `SpawnTime time.Time`.
  - Box dimensions: width 6 (`╭────╮`), height 3 (`╰────╯`), center row `│ 🎯 │`.
  - Playfield grid: 52 columns wide, 12 rows high.
  - Non-overlapping target generation.
  - Hit & miss handling, reaction times, combo multipliers.
  - `View(width int)`: render live header (timer, score, combo, acc), playfield grid with targets and hit flashes, and end-of-round summary card.
- [ ] **Step 4: Run tests to verify they pass**:
  - `go test -v ./internal/game -run TestAimTrainer`
- [ ] **Step 5: Commit**:
  - `git add internal/game/ && git commit -m "feat(game): implement core 10-second aim trainer game engine"`

---

### Task 2: Registry & TUI Mouse Dispatch Integration

**Files:**
- Modify: `internal/game/game.go` (register `NewAimTrainer`)
- Modify: `internal/tui/update_mouse.go` (route mouse clicks when `modeGame`)
- Modify: `internal/tui/view_modals.go` (compute board origin helper)
- Test: `internal/tui/game_test.go`

- [ ] **Step 1: Register `NewAimTrainer` in `internal/game/game.go`**:
  - Add `NewAimTrainer` to `Registry = []func() MiniGame{ ... }`.
- [ ] **Step 2: Add board coordinate translation in `internal/tui/`**:
  - In `internal/tui/view_modals.go`, calculate the exact `(originX, originY)` where the game canvas starts.
  - In `internal/tui/update_mouse.go`, when `m.mode == modeGame && msg.Action == tea.MouseActionPress`, translate `(msg.X - originX, msg.Y - originY)` and pass `AimClickMsg` to `m.activeGame.Update(...)`.
- [ ] **Step 3: Add unit tests in `internal/tui/game_test.go`**:
  - Test mouse click on game modal routes to aim trainer.
- [ ] **Step 4: Run all tests**:
  - `go test -count=1 -race ./internal/game/... ./internal/tui/...`
- [ ] **Step 5: Commit**:
  - `git add internal/tui/ internal/game/ && git commit -m "feat(tui): wire mouse click routing for aim trainer mini-game"`

---

### Task 3: Documentation & Verification

**Files:**
- Modify: `docs/features.md`
- Modify: `docs/keybindings.md`
- Modify: `plan.md`
- Create: `PR_AIM_TRAINER_MINI_GAME.md`

- [ ] **Step 1: Update documentation files**:
  - Add Aim Trainer Blitz to `docs/features.md` and `docs/keybindings.md`.
  - Mark Phase 102 complete in `plan.md`.
- [ ] **Step 2: Generate PR message**:
  - Create `PR_AIM_TRAINER_MINI_GAME.md`.
- [ ] **Step 3: Run full repo test suite with `-race`**:
  - `go test -count=1 -race ./...`
- [ ] **Step 4: Compile & verify binary install**:
  - `go build -o ~/.local/bin/oml ./cmd/oml`
- [ ] **Step 5: Commit**:
  - `git add docs/ plan.md && git commit -m "docs: document aim trainer blitz mini-game"`
