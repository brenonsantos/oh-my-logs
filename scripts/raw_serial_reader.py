#!/usr/bin/env python3
"""
Raw Serial Reader & Diagnostics Tool
Inspects incoming raw byte streams from serial ports with millisecond timing,
explicit line-ending markers ([CR], [LF]), byte-count tracking, and hex dumps.
Zero external dependencies required (uses standard library termios on macOS/Linux;
falls back to pyserial if available).
"""

import argparse
import glob
import os
import select
import sys
import time

try:
    import termios
    import tty
    HAVE_TERMIOS = True
except ImportError:
    HAVE_TERMIOS = False

try:
    import serial
    HAVE_PYSERIAL = True
except ImportError:
    HAVE_PYSERIAL = False


BAUD_RATES = {
    9600: getattr(termios, "B9600", 9600) if HAVE_TERMIOS else 9600,
    19200: getattr(termios, "B19200", 19200) if HAVE_TERMIOS else 19200,
    38400: getattr(termios, "B38400", 38400) if HAVE_TERMIOS else 38400,
    57600: getattr(termios, "B57600", 57600) if HAVE_TERMIOS else 57600,
    115200: getattr(termios, "B115200", 115200) if HAVE_TERMIOS else 115200,
    230400: getattr(termios, "B230400", 230400) if HAVE_TERMIOS else 230400,
    460800: getattr(termios, "B460800", 460800) if HAVE_TERMIOS else 460800,
    921600: getattr(termios, "B921600", 921600) if HAVE_TERMIOS else 921600,
}


def find_serial_ports():
    """Discover serial ports on macOS and Linux."""
    patterns = [
        "/dev/cu.usbserial*",
        "/dev/cu.usbmodem*",
        "/dev/cu.SLAB_USB*",
        "/dev/ttyUSB*",
        "/dev/ttyACM*",
        "/dev/cu.*",
    ]
    seen = set()
    ports = []
    for pattern in patterns:
        for path in sorted(glob.glob(pattern)):
            if path not in seen and not path.endswith("Bluetooth-Incoming-Port"):
                seen.add(path)
                ports.append(path)
    return ports


class NativeSerial:
    """POSIX termios serial port wrapper with zero external dependencies."""

    def __init__(self, port, baudrate=115200):
        self.port = port
        self.baudrate = baudrate
        self.fd = None

    def open(self):
        self.fd = os.open(self.port, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
        tty.setraw(self.fd)

        attrs = termios.tcgetattr(self.fd)
        speed = BAUD_RATES.get(self.baudrate, termios.B115200)
        attrs[4] = speed  # ispeed
        attrs[5] = speed  # ospeed
        # 8N1
        attrs[2] = (attrs[2] & ~(termios.CSIZE | termios.PARENB | termios.CSTOPB)) | termios.CS8 | termios.CREAD | termios.CLOCAL
        termios.tcsetattr(self.fd, termios.TCSANOW, attrs)

    def read(self, size=1024, timeout=0.1):
        if not self.fd:
            return b""
        r, _, _ = select.select([self.fd], [], [], timeout)
        if self.fd in r:
            return os.read(self.fd, size)
        return b""

    def close(self):
        if self.fd is not None:
            try:
                os.close(self.fd)
            except Exception:
                pass
            self.fd = None


class PySerialAdapter:
    """Wrapper around pyserial if installed."""

    def __init__(self, port, baudrate=115200):
        self.port = port
        self.baudrate = baudrate
        self.ser = None

    def open(self):
        self.ser = serial.Serial(self.port, self.baudrate, timeout=0.1)

    def read(self, size=1024, timeout=0.1):
        if not self.ser:
            return b""
        return self.ser.read(self.ser.in_waiting or size)

    def close(self):
        if self.ser and self.ser.is_open:
            self.ser.close()


def open_serial_device(port, baudrate):
    if HAVE_TERMIOS:
        dev = NativeSerial(port, baudrate)
        dev.open()
        return dev
    elif HAVE_PYSERIAL:
        dev = PySerialAdapter(port, baudrate)
        dev.open()
        return dev
    else:
        raise RuntimeError("Neither termios (POSIX standard library) nor pyserial is available.")


# ANSI styling
RESET = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[2m"
CYAN = "\033[36m"
MAGENTA = "\033[35m"
YELLOW = "\033[33m"
GREEN = "\033[32m"
RED = "\033[31m"


def format_hex_chunk(offset, data):
    """Format bytes into standard hexdump format."""
    lines = []
    for i in range(0, len(data), 16):
        chunk = data[i:i + 16]
        hex_str = " ".join(f"{b:02x}" for b in chunk)
        ascii_str = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
        lines.append(f"{DIM}{offset + i:08x}{RESET}  {hex_str:<48}  {CYAN}|{ascii_str}|{RESET}")
    return "\n".join(lines)


def run_visual_mode(dev, save_file=None):
    """
    Streams characters as they arrive, making CR (\r) and LF (\n) visible
    and highlighting total byte length whenever a line break occurs.
    """
    print(f"{BOLD}{GREEN}Streaming in Visual Mode (Ctrl+C to stop)...{RESET}")
    print(f"{DIM}Legend: {CYAN}[CR:0x0D]{RESET} = carriage return, "
          f"{MAGENTA}[LF:0x0A (len=N)]{RESET} = newline with byte count.\n")

    line_byte_count = 0
    total_bytes = 0

    try:
        while True:
            chunk = dev.read(1024, timeout=0.05)
            if not chunk:
                continue

            total_bytes += len(chunk)
            if save_file:
                save_file.write(chunk)
                save_file.flush()

            out = []
            for b in chunk:
                line_byte_count += 1
                if b == 0x0D:  # \r
                    out.append(f"{CYAN}[CR]{RESET}")
                elif b == 0x0A:  # \n
                    # Highlight common boundary values (e.g. 80 cols, 64 buffer bytes)
                    color = YELLOW if line_byte_count in (64, 80) else MAGENTA
                    out.append(f"{color}[LF: len={line_byte_count}]{RESET}\n")
                    line_byte_count = 0
                elif 32 <= b < 127:
                    out.append(chr(b))
                elif b == 0x1B:  # ANSI escape
                    out.append(f"{DIM}\\e{RESET}")
                elif b == 0x09:  # \t
                    out.append("\t")
                else:
                    out.append(f"{YELLOW}\\x{b:02x}{RESET}")

            sys.stdout.write("".join(out))
            sys.stdout.flush()

    except KeyboardInterrupt:
        print(f"\n{BOLD}{YELLOW}Stopped by user. Total bytes received: {total_bytes}{RESET}")


def run_lines_mode(dev, save_file=None):
    """
    Buffers input until \n (0x0A) and prints each completed line with its exact
    length and timestamp, noting if it terminated with \r\n or just \n.
    """
    print(f"{BOLD}{GREEN}Streaming in Lines Mode (Ctrl+C to stop)...{RESET}")
    print(f"{DIM}Shows completed lines with exact byte count and timestamp.{RESET}\n")

    buf = bytearray()
    line_number = 0
    start_time = time.time()

    try:
        while True:
            chunk = dev.read(1024, timeout=0.05)
            if not chunk:
                continue

            if save_file:
                save_file.write(chunk)
                save_file.flush()

            buf.extend(chunk)

            while b"\n" in buf:
                line_number += 1
                idx = buf.index(b"\n")
                raw_line = bytes(buf[:idx + 1])
                del buf[:idx + 1]

                elapsed = time.time() - start_time
                length = len(raw_line)
                has_cr = raw_line.endswith(b"\r\n")

                tag_color = YELLOW if length in (64, 80, 81) else CYAN
                cr_tag = f"{DIM}+CR{RESET}" if has_cr else f"{RED}no-CR{RESET}"

                # Strip trailing line breaks for cleaner display of body
                body_bytes = raw_line[:-2] if has_cr else raw_line[:-1]
                body_text = body_bytes.decode("utf-8", errors="replace")

                print(f"[{elapsed:8.3f}s | line {line_number:04d} | {tag_color}len={length:3d}B{RESET} {cr_tag}]: {body_text}")

    except KeyboardInterrupt:
        print(f"\n{BOLD}{YELLOW}Stopped by user. Total lines received: {line_number}{RESET}")


def run_hex_mode(dev, save_file=None):
    """
    Displays full hex dump with byte offsets and ASCII side-by-side.
    """
    print(f"{BOLD}{GREEN}Streaming in Hex Dump Mode (Ctrl+C to stop)...{RESET}\n")
    offset = 0

    try:
        while True:
            chunk = dev.read(128, timeout=0.05)
            if not chunk:
                continue

            if save_file:
                save_file.write(chunk)
                save_file.flush()

            print(format_hex_chunk(offset, chunk))
            offset += len(chunk)

    except KeyboardInterrupt:
        print(f"\n{BOLD}{YELLOW}Stopped by user. Total bytes received: {offset}{RESET}")


def run_raw_mode(dev, save_file=None):
    """
    Passes raw bytes directly to stdout without modification (pipe-friendly).
    """
    try:
        while True:
            chunk = dev.read(1024, timeout=0.05)
            if not chunk:
                continue
            if save_file:
                save_file.write(chunk)
                save_file.flush()
            sys.stdout.buffer.write(chunk)
            sys.stdout.buffer.flush()
    except KeyboardInterrupt:
        pass


def main():
    parser = argparse.ArgumentParser(
        description="Raw Serial Diagnostics & Line-Break Inspector",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Auto-detect port and view live visual stream with [CR] and [LF (len=...)] markers:
  python3 scripts/raw_serial_reader.py

  # Specify port and baud rate:
  python3 scripts/raw_serial_reader.py -p /dev/cu.usbserial-2142300 -b 115200

  # Line-by-line mode with exact byte count per line:
  python3 scripts/raw_serial_reader.py --mode lines

  # Hex dump mode:
  python3 scripts/raw_serial_reader.py --mode hex

  # Save raw byte stream to file while viewing:
  python3 scripts/raw_serial_reader.py --mode visual --save capture.bin
        """,
    )
    parser.add_argument("-p", "--port", help="Serial port device path (e.g. /dev/cu.usbserial-2142300)")
    parser.add_argument("-b", "--baud", type=int, default=115200, help="Baud rate (default: 115200)")
    parser.add_argument(
        "-m", "--mode",
        choices=["visual", "lines", "hex", "raw"],
        default="visual",
        help="Display mode: 'visual' (highlight [CR]/[LF] & lengths), 'lines' (buffered line timestamps), 'hex' (hexdump), 'raw' (pure bytes). Default: visual",
    )
    parser.add_argument("-s", "--save", help="Optional file path to record raw incoming bytes")
    parser.add_argument("-l", "--list", action="store_true", help="List discovered serial ports and exit")

    args = parser.parse_args()

    available_ports = find_serial_ports()

    if args.list:
        print("Available serial ports:")
        if not available_ports:
            print("  (No serial devices detected)")
        for p in available_ports:
            print(f"  - {p}")
        return 0

    target_port = args.port
    if not target_port:
        # Filter for likely USB serial interfaces
        usb_ports = [p for p in available_ports if "usb" in p.lower() or "slab" in p.lower()]
        if len(usb_ports) == 1:
            target_port = usb_ports[0]
            print(f"{DIM}Auto-selected port: {target_port}{RESET}")
        elif len(usb_ports) > 1:
            print(f"{BOLD}Multiple USB serial ports detected:{RESET}")
            for i, p in enumerate(usb_ports, 1):
                print(f"  [{i}] {p}")
            choice = input(f"Select port [1-{len(usb_ports)}]: ").strip()
            try:
                idx = int(choice) - 1
                if 0 <= idx < len(usb_ports):
                    target_port = usb_ports[idx]
                else:
                    print(f"{RED}Invalid selection.{RESET}")
                    return 1
            except ValueError:
                print(f"{RED}Invalid selection.{RESET}")
                return 1
        elif available_ports:
            target_port = available_ports[0]
            print(f"{DIM}Auto-selected port: {target_port}{RESET}")
        else:
            print(f"{RED}Error: No serial ports detected. Connect your device or specify -p /dev/...{RESET}")
            return 1

    print(f"{BOLD}Opening {target_port} at {args.baud} baud...{RESET}")
    save_file = open(args.save, "wb") if args.save else None

    try:
        dev = open_serial_device(target_port, args.baud)
    except Exception as e:
        print(f"{RED}Failed to open {target_port}: {e}{RESET}")
        if save_file:
            save_file.close()
        return 1

    try:
        if args.mode == "visual":
            run_visual_mode(dev, save_file)
        elif args.mode == "lines":
            run_lines_mode(dev, save_file)
        elif args.mode == "hex":
            run_hex_mode(dev, save_file)
        elif args.mode == "raw":
            run_raw_mode(dev, save_file)
    finally:
        dev.close()
        if save_file:
            save_file.close()
            print(f"{GREEN}Raw stream saved to {args.save}{RESET}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
