# Interactive Serial TX (Send) Guide

Many embedded devices, cellular modems, and microcontroller bootloaders feature interactive command shells (e.g. AT commands, MicroPython REPL, Zephyr shell, or custom debug CLI).

`oml` allows you to send commands directly over the serial interface without exiting or switching terminal windows.

---

## 1. Opening the TX Prompt (`i`)

Press **`i`** (input/inject) to open the interactive TX prompt at the bottom of the screen:

```text
TX [CRLF] > _
```

Type your command and press **`Enter`** to transmit the raw bytes directly over the serial port.

---

## 2. Line Ending Selector (`Ctrl+E`)

Different microcontrollers and modems expect different line endings.

While inside the TX prompt, press **`Ctrl+E`** to cycle through supported line endings:

1. **`CRLF` (`\r\n`)**: Standard for AT commands, Windows serial shells, and modems.
2. **`LF` (`\n`)**: Standard for Unix/Linux embedded consoles and Zephyr shell.
3. **`CR` (`\r`)**: Legacy carriage-return terminals.
4. **`None`**: Transmits raw text without any appended terminator.

Your selected line ending is persisted across restarts in `~/.config/oml/settings.json`.

---

## 3. Command History & Draft Preservation

- Press **`↑`** (or **`Ctrl+P`**) to browse previous commands sent during the session or in earlier runs.
- Press **`↓`** (or **`Ctrl+N`**) to browse forward.
- **Draft Preservation**: If you start typing a command and press `↑` to inspect previous commands, your unsent draft is saved and restored when you press `↓` back to the bottom.
- Command history is persisted across sessions in `settings.json`.

---

## 4. Maple Orange TX Echo & Log Stream Integration

When a command is transmitted, `oml` immediately echoes it into the in-memory log buffer so you have an accurate timeline of actions and device responses:

- **Level Badge**: `TX` highlighted in warm Maple Orange.
- **Payload**: Highlighted in Maple Orange so transmitted commands stand out distinctly from received device output.
- **Filtering**:
  - Filter sent commands: `level:TX`
  - Suppress sent commands from view: `-level:TX`
  - Group commands with responses: `level:TX,ERR`

---

## 5. Interactive Shell Passthrough Mode ("minicom-inside-oml") (`I` / `Ctrl+I`)

While the one-line TX prompt (`i`) is ideal for discrete AT commands or single shell commands, embedded systems frequently run complex interactive consoles (e.g. Zephyr shell with `<Tab>` autocomplete, MicroPython REPL, or custom menu systems).

Press **`I`** or **`Ctrl+I`** to enter **Interactive Shell Passthrough** mode:
- **Raw Byte Passthrough**: Keyboard input is forwarded directly to the serial port in real time, supporting arrow navigation, backspace, `<Tab>` completion, and escape sequences.
- **Signal Forwarding**: Pressing `Ctrl+C` sends the `0x03` interrupt byte to the device to abort running commands or stop loops without quitting `oml`.
- **Exit Passthrough**: Press **`Ctrl+D`** or **`Ctrl+]`** (or `Esc`) to exit passthrough mode and return to normal log inspection.

---

## 6. Keyboard Reference

| Key | Action |
|-----|--------|
| `i` / `:` | Open interactive serial send prompt |
| `Enter` | Transmit command with active line ending (or apply autocomplete if suggestions open) |
| `Ctrl+E` / `Tab` | Cycle line ending (`CRLF` $\rightarrow$ `LF` $\rightarrow$ `CR` $\rightarrow$ `None`) |
| `Ctrl+T` / `Alt+E` | Cycle encoder mode (`[RAW]` $\rightarrow$ `[Encoder]` $\rightarrow$ `[RAW]`) |
| `↑` / `↓` | Navigate autocomplete suggestions (or history when no suggestions open) |
| `Tab` / `Enter` | Complete selected suggestion |
| `Ctrl+P` / `Ctrl+N` | Previous / Next command in history |
| `Ctrl+V` | Paste text from system clipboard into prompt |
| `I` / `Ctrl+I` | Enter **Interactive Shell Passthrough** mode (`<Tab>` completion, arrows, `^C`) |
| `Ctrl+D` / `Ctrl+]` / `Esc` | Exit shell passthrough mode |
| `Esc` | Close suggestions drawer / Cancel and close single-line TX prompt |

---

## 7. Command Encoders & Interactive Autocomplete

When interfacing with modern firmware, commands are often formatted in complex binary, base64-framed, or structured protocols (e.g. CAN bus, binary management packets, custom RPCs). Manually typing raw encoded bytes is tedious and error-prone.

`oml` provides a generic, decoupled **Encoder Worker Architecture**:

### Profile & Project Configuration
Define encoders in your profile YAML or `.oml/project.yaml`:

```yaml
encoders:
  - name: "ManagementProtocol"
    trigger: "mgmt "
    description: "Encode management protocol RPC commands to base64 wire frames"
    exec: "python3 ./encoders/mgmt_codec.py"
    extra_paths:
      - "./encoders"
```

### Dynamic Autocomplete Drawer
- As you type into the `TX` bar, `oml` passes input queries asynchronously to the encoder subprocess.
- Autocomplete suggestions appear docked directly above the TX input bar in an interactive drawer with descriptions.
- Use **`↑` / `↓`** to navigate suggestions and **`Tab`** or **`Enter`** to apply.
- Use **`Alt+E`** to cycle between `[RAW]` mode and available encoders at any time.
- When an encoder is selected, transmitting transforms the human-readable text into the wire payload for the serial port, while logging the readable command with an encoder badge in the history buffer.
