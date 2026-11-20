#!/usr/bin/env python3
"""Fake generic shell+log UART on a pty (clean-room, Zephyr-style behaviour).

Usage:  python3 scripts/fake_uart.py [/tmp/oml-fake-tty]
Then:   go run ./cmd/oml --port /tmp/oml-fake-tty
        go run ./scripts/linedisc_probe /tmp/oml-fake-tty   (headless dump)

Behaviour: prompt "uart:~$ ", char echo, Tab completes "he"->"help", Enter runs
a command, and every ~1.5s an async log arrives that erases the prompt first
(ESC[nD ESC[J), pauses 80ms MID-LINE, then redraws prompt + typed input.
Also emits a \r progress bar and a short "OK" line once in a while.
"""
import os, pty, sys, time, threading, tty

link = sys.argv[1] if len(sys.argv) > 1 else "/tmp/oml-fake-tty"
master, slave = pty.openpty()
tty.setraw(slave)
name = os.ttyname(slave)
if os.path.islink(link) or os.path.exists(link):
    os.remove(link)
os.symlink(name, link)
print(f"fake UART on {name} -> {link}", flush=True)

PROMPT = "uart:~$ "
buf = ""
lock = threading.Lock()
w = lambda s: os.write(master, s.encode())

def async_log(msg):
    with lock:
        erase = len(PROMPT) + len(buf)
        head, tail = msg[: len(msg) // 2], msg[len(msg) // 2:]
        w(f"\x1b[{erase}D\x1b[J" + head)
        time.sleep(0.08)  # in-flight pause: old 25ms timeout would split here
        w(tail + "\r\n" + PROMPT + buf)

def logger():
    n = 0
    t0 = time.time()
    while True:
        time.sleep(1.5)
        n += 1
        async_log(f"[{time.time()-t0:10.3f}] <inf> net: Connected, seq={n}")
        if n % 3 == 0:
            with lock:
                w("\x1b[%dD\x1b[J" % (len(PROMPT) + len(buf)))
                for p in (10, 40, 80, 100):
                    w(f"progress {p}%\r"); time.sleep(0.03)
                w("\r\nOK\r\n" + PROMPT + buf)

threading.Thread(target=logger, daemon=True).start()
w(PROMPT)
while True:
    try:
        data = os.read(master, 256).decode(errors="ignore")
    except OSError:
        time.sleep(0.1); continue
    for ch in data:
        with lock:
            if ch in "\r\n":
                cmd, buf = buf, ""
                w("\r\n")
                if cmd == "help":
                    w("Available commands:\r\n  help  : show help\r\n  kernel: kernel cmds\r\n")
                elif cmd:
                    w(f"{cmd}: command not found\r\n")
                w(PROMPT)
            elif ch == "\t":
                if "help".startswith(buf) and buf:
                    w("help"[len(buf):]); buf = "help"
            elif ch in "\x7f\b":
                if buf:
                    buf = buf[:-1]; w("\b \b")
            else:
                buf += ch; w(ch)
