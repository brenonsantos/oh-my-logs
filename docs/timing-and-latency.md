# Delta-Time Mode (`Δt`) & Inter-Log Latency Guide

In embedded systems and firmware development, **inter-log latency** (the elapsed time between consecutive log events) is often more revealing than the message text itself. Latency spikes indicate CPU starvation, bus arbitration contention, blocking I/O, garbage collection pauses, or missed RTOS deadlines.

`oml` features a real-time **Adaptive Cadence Engine** and a multi-mode **Delta-Time (`Δt`)** column.

---

## 1. The Adaptive Cadence Engine

### Why Static Thresholds Fail
Hardcoded thresholds (e.g., "highlight anything over 100ms") break down when monitoring different embedded domains:
- **1 kHz High-Speed Bus**: A 20ms pause is a severe system failure ($20\times$ normal cadence).
- **0.2 Hz Low-Power Cellular Telemetry**: A 2-second interval between packets is perfectly normal.

### Exponential Moving Average (EMA)
`oml` maintains an adaptive baseline for the active stream using a thread-safe Exponential Moving Average (EMA):

$$\text{EMA}_t = \alpha \cdot \Delta t + (1 - \alpha) \cdot \text{EMA}_{t-1}$$

As streaming rates shift, the moving average smoothly self-calibrates without user intervention.

---

## 2. Dynamic Latency Levels & Styling

Each row's delta $R = \frac{\Delta t}{\text{EMA}}$ is dynamically classified into four semantic tiers:

| Level | Ratio / Condition | Visual Style | Meaning |
|-------|-------------------|--------------|---------|
| **Burst** | $R < 0.3\times$ | Cool Cyan | Rapid burst of packets (e.g. DMA transfer, interrupt burst, or buffer flush). |
| **Cadence** | $0.3\times \le R \le 2.5\times$ | Muted Slate | Normal, expected periodic stream cadence. |
| **Hiccup** | $2.5\times < R \le 5.0\times$ | Amber Warning | Minor processing delay, lock contention, or bus retry. |
| **Anomaly** | $R > 5.0\times$ or $\ge \text{Alert}$ | Bold Maple Red | Severe delay, watchdog timeout, hardware stall, or link drop. |

---

## 3. Timestamp & Delta-Time Cycling (`t`)

Pressing **`t`** cycles through four distinct display modes:

$$\text{Clock } (15:04:05.000) \;\longrightarrow\; \Delta t \;(+14.2\text{ms}) \;\longrightarrow\; \text{Both (Clock + } \Delta t) \;\longrightarrow\; \text{OFF}$$

1. **Clock Mode (`⏱ CLOCK`)**: Displays the absolute arrival time or parsed device timestamp (e.g. `14:23:01.020`).
2. **Delta-Time Mode (`⏱ Δt`)**: Replaces the clock column with a compact elapsed duration relative to the previous visible record:
   - `+142µs`
   - `+2.45ms`
   - `+45.2ms`
   - `+1.45s`
   - `+2m04s`
3. **Both Mode (`⏱ BOTH`)**: Renders both the absolute `Time` column and the relative `Δt` column side-by-side.
4. **Off Mode (`⏱ OFF`)**: Hides timestamp columns to maximize horizontal width for log payloads.

Your active mode is persisted across sessions in `~/.config/oml/settings.json`.

---

## 4. Batch Selection Range Delta (Terminal Stopwatch)

You can measure the exact elapsed time between **any two events** or a sequence of logs directly in the terminal:

1. Click and drag across a batch of rows with your mouse, or navigate with **`Shift+↑`** / **`Shift+↓`** (or **`K`** / **`J`**).
2. Look at the bottom status bar:
   ```text
   41 records  │  7 selected (Δt: +160.0ms)  │  ⏱ BOTH  │  FOLLOW
   ```
3. The action message bar continuously reports the live interval:
   ```text
   • 7 rows selected · Δt: +160.0ms (press y to copy)
   ```
4. Pressing **`y`** copies the selected lines to your clipboard and confirms the measured duration:
   ```text
   • ✓ Copied 7 rows (Δt: +160.0ms) to clipboard
   ```

This feature functions like a digital oscilloscope cursor / logic analyzer marker directly inside your terminal log viewer.

---

## 5. Offline Log Replay (`--file`)

When replaying saved log files via `oml --file <file.log>`, `oml` automatically extracts timestamps or device uptimes from the raw records:
- Supported formats: `15:04:05.000`, `15:04:05,000`, `2006-01-02 15:04:05`, `RFC3339`, and float uptimes like `[ 5.182]`.
- Deliberately replicates real-world delta calculations even when lines are loaded into memory instantly.

Try testing this with the included demo file:
```bash
./oml --file examples/logs/zephyr-rtos.log --profile profiles/examples/zephyr.yaml
```

---

## 6. Profile Timing Customization

Profiles can override dynamic EMA ratios with fixed threshold durations or custom ratios:

```yaml
name: CAN_Bus_250k

timing:
  warn_threshold: 20ms   # Durations >= 20ms flagged as Hiccups (Amber)
  alert_threshold: 100ms # Durations >= 100ms flagged as Anomalies (Maple Red)
  warn_ratio: 3.0        # Ratio override for warning
  alert_ratio: 6.0       # Ratio override for alert
```

When fixed thresholds are defined, they take precedence over moving-average ratios.
