package serial

import (
	"strings"
	"unicode"

	goserial "go.bug.st/serial"
)

// ListPorts returns the names of all available serial ports on the system.
// If enumeration partially succeeds before encountering an error, the
// successfully enumerated ports are returned along with the error.
func ListPorts() ([]string, error) {
	ports, err := goserial.GetPortsList()
	return ports, err
}

// MatchCandidatePort finds the best matching serial port from available ports
// for a previously used target port.
// It prioritizes:
// 1. Exact match
// 2. macOS cu / tty alternate (preferring cu)
// 3. Same device family with identical channel / sub-interface suffix
// 4. Same device family fallback candidate
func MatchCandidatePort(target string, available []string) string {
	if target == "" || len(available) == 0 {
		return ""
	}

	// 1. Exact match
	for _, p := range available {
		if p == target {
			return p
		}
	}

	// 2. macOS cu / tty alternate
	if strings.HasPrefix(target, "/dev/cu.") {
		ttyAlt := strings.Replace(target, "/dev/cu.", "/dev/tty.", 1)
		for _, p := range available {
			if p == ttyAlt {
				return p
			}
		}
	} else if strings.HasPrefix(target, "/dev/tty.") {
		cuAlt := strings.Replace(target, "/dev/tty.", "/dev/cu.", 1)
		for _, p := range available {
			if p == cuAlt {
				return p
			}
		}
	}

	// 3. Family match
	family := detectPortFamily(target)
	if family == "" {
		return ""
	}

	var candidates []string
	for _, p := range available {
		if isIgnoredSystemPort(p) {
			continue
		}
		if detectPortFamily(p) == family {
			// On macOS, prefer cu.* over tty.*
			if strings.HasPrefix(p, "/dev/cu.") || !hasCuEquivalent(p, available) {
				candidates = append(candidates, p)
			}
		}
	}

	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	// Multi-candidate: try matching channel / sub-interface suffix (e.g. channel 1 vs 0)
	targetChan := extractChannelSuffix(target)
	if targetChan != "" {
		for _, c := range candidates {
			if extractChannelSuffix(c) == targetChan {
				return c
			}
		}
	}

	return candidates[0]
}

func detectPortFamily(port string) string {
	lower := strings.ToLower(port)
	switch {
	case strings.Contains(lower, "wchusbserial"):
		return "wchusbserial"
	case strings.Contains(lower, "slab_usbtouart"):
		return "slab"
	case strings.Contains(lower, "usbserial"):
		return "usbserial"
	case strings.Contains(lower, "usbmodem"):
		return "usbmodem"
	case strings.Contains(lower, "ttyusb"):
		return "ttyusb"
	case strings.Contains(lower, "ttyacm"):
		return "ttyacm"
	case strings.Contains(lower, "com"):
		return "com"
	default:
		return ""
	}
}

func isIgnoredSystemPort(port string) bool {
	lower := strings.ToLower(port)
	return strings.Contains(lower, "bluetooth") ||
		strings.Contains(lower, "debug-console") ||
		strings.Contains(lower, "incoming-port")
}

func hasCuEquivalent(ttyPort string, available []string) bool {
	if !strings.HasPrefix(ttyPort, "/dev/tty.") {
		return false
	}
	cuPort := strings.Replace(ttyPort, "/dev/tty.", "/dev/cu.", 1)
	for _, p := range available {
		if p == cuPort {
			return true
		}
	}
	return false
}

func extractChannelSuffix(port string) string {
	if len(port) == 0 {
		return ""
	}
	last := rune(port[len(port)-1])
	if unicode.IsDigit(last) {
		return string(last)
	}
	return ""
}
