package sim

import (
	"fmt"
	"strings"
	"time"
)

// ProfileKind identifies a simulated embedded OS/firmware personality.
type ProfileKind string

const (
	ProfileZephyr ProfileKind = "zephyr"
	ProfileESPIDF ProfileKind = "esp-idf"
	ProfileLinux  ProfileKind = "linux"
)

// EraseStyle controls how the simulated shell clears its active prompt before
// printing an asynchronous log line.
type EraseStyle uint8

const (
	// EraseCursorBack uses ESC[<n>D ESC[J (Zephyr CONFIG_SHELL style).
	EraseCursorBack EraseStyle = iota
	// EraseCarriageReturn uses \r ESC[K (ESP-IDF / Linux / FreeRTOS style).
	EraseCarriageReturn
)

// Profile defines the prompt, log formatting, colors, and boot banner for a
// simulated embedded target.
type Profile struct {
	Kind       ProfileKind
	Prompt     string
	EraseStyle EraseStyle
	UseColor   bool
	Modules    []string
	Banner     []string
}

// ParseProfile returns a built-in Profile by name. Empty name defaults to zephyr.
func ParseProfile(name string) (Profile, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "zephyr":
		return Profile{
			Kind:       ProfileZephyr,
			Prompt:     "uart:~$ ",
			EraseStyle: EraseCursorBack,
			UseColor:   true,
			Modules:    []string{"kernel", "net", "wifi", "ble", "mqtt", "sensor", "fs_nvs", "pm"},
			Banner: []string{
				"*** Booting Zephyr OS build v3.7.0 (simulated) ***",
				"[00:00:00.012,100] <inf> kernel: CPU: ARM Cortex-M4 @ 120 MHz, RAM: 256 KB",
				"[00:00:00.045,820] <inf> fs_nvs: 16 Sectors of 4096 bytes",
				"[00:00:00.118,400] <inf> net: Network interface up (192.168.1.42)",
			},
		}, nil
	case "esp-idf", "espidf", "esp32":
		return Profile{
			Kind:       ProfileESPIDF,
			Prompt:     "esp32> ",
			EraseStyle: EraseCarriageReturn,
			UseColor:   true,
			Modules:    []string{"wifi", "mqtt_client", "ble_gap", "adc", "nvs", "ota", "heap"},
			Banner: []string{
				"ESP-ROM:esp32s3-20210327 (simulated)",
				"I (12) boot: ESP-IDF v5.2.1 2nd stage bootloader",
				"I (48) cpu_start: Pro cpu start user code",
				"I (110) wifi: mode : sta (aa:bb:cc:dd:ee:ff)",
			},
		}, nil
	case "linux":
		return Profile{
			Kind:       ProfileLinux,
			Prompt:     "root@board# ",
			EraseStyle: EraseCarriageReturn,
			UseColor:   false,
			Modules:    []string{"kernel", "eth0", "systemd", "dbus", "i2c-core", "mmc0", "sshd"},
			Banner: []string{
				"Linux version 6.6.32-embedded (gcc 13.2.0) #1 SMP PREEMPT (simulated)",
				"[    0.412891] mmc0: new high speed SDHC card at address aaaa",
				"[    1.204512] eth0: Link is Up - 1Gbps/Full - flow control rx/tx",
				"Welcome to Embedded Linux 6.6 (ttyS0)",
			},
		}, nil
	default:
		return Profile{}, fmt.Errorf("sim: unknown profile %q (supported: zephyr, esp-idf, linux)", name)
	}
}

// Level represents a generic log severity.
type Level string

const (
	LevelDebug Level = "dbg"
	LevelInfo  Level = "inf"
	LevelWarn  Level = "wrn"
	LevelError Level = "err"
)

// FormatLog formats a single log line according to the profile's conventions.
func (p Profile) FormatLog(uptime time.Duration, level Level, module, message string) string {
	switch p.Kind {
	case ProfileESPIDF:
		return p.formatESPIDF(uptime, level, module, message)
	case ProfileLinux:
		return p.formatLinux(uptime, level, module, message)
	default:
		return p.formatZephyr(uptime, level, module, message)
	}
}

func (p Profile) formatZephyr(uptime time.Duration, level Level, module, message string) string {
	totalSec := int64(uptime / time.Second)
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	us := int64(uptime%time.Second) / 1000
	ms := us / 1000
	remUs := us % 1000
	ts := fmt.Sprintf("[%02d:%02d:%02d.%03d,%03d]", h, m, s, ms, remUs)

	line := fmt.Sprintf("%s <%s> %s: %s", ts, string(level), module, message)
	if !p.UseColor {
		return line
	}
	switch level {
	case LevelError:
		return "\x1b[1;31m" + line + "\x1b[0m"
	case LevelWarn:
		return "\x1b[1;33m" + line + "\x1b[0m"
	case LevelDebug:
		return "\x1b[90m" + line + "\x1b[0m"
	default:
		return line
	}
}

func (p Profile) formatESPIDF(uptime time.Duration, level Level, module, message string) string {
	ms := uptime.Milliseconds()
	var tag, color string
	switch level {
	case LevelError:
		tag, color = "E", "\x1b[0;31m"
	case LevelWarn:
		tag, color = "W", "\x1b[0;33m"
	case LevelDebug:
		tag, color = "D", "\x1b[0;36m"
	default:
		tag, color = "I", "\x1b[0;32m"
	}
	line := fmt.Sprintf("%s (%d) %s: %s", tag, ms, module, message)
	if !p.UseColor {
		return line
	}
	return color + line + "\x1b[0m"
}

func (p Profile) formatLinux(uptime time.Duration, level Level, module, message string) string {
	sec := uptime.Seconds()
	lvl := ""
	switch level {
	case LevelError:
		lvl = "ERROR "
	case LevelWarn:
		lvl = "WARN "
	case LevelDebug:
		lvl = "DEBUG "
	}
	return fmt.Sprintf("[%12.6f] %s: %s%s", sec, module, lvl, message)
}

// EraseSequence returns the VT100 escape sequence that erases a prompt line of
// display width visibleLen.
func (p Profile) EraseSequence(visibleLen int) string {
	if p.EraseStyle == EraseCursorBack && visibleLen > 0 {
		return fmt.Sprintf("\x1b[%dD\x1b[J", visibleLen)
	}
	return "\r\x1b[K"
}
