package serial

import (
	"testing"
)

func TestListPortsOutput(t *testing.T) {
	ports, err := ListPorts()
	t.Logf("ListPorts err: %v, count: %d", err, len(ports))
	for i, p := range ports {
		t.Logf("Port[%d]: %q", i, p)
	}
}

func TestMatchCandidatePort(t *testing.T) {
	available := []string{
		"/dev/cu.Bluetooth-Incoming-Port",
		"/dev/cu.debug-console",
		"/dev/cu.usbserial-1400",
		"/dev/cu.usbserial-1401",
		"/dev/tty.usbserial-1400",
		"/dev/tty.usbserial-1401",
	}

	// 1. Exact match
	got := MatchCandidatePort("/dev/cu.usbserial-1401", available)
	if got != "/dev/cu.usbserial-1401" {
		t.Errorf("expected exact match, got %q", got)
	}

	// 2. tty to cu normalization
	got = MatchCandidatePort("/dev/tty.usbserial-1401", available)
	if got != "/dev/tty.usbserial-1401" { // exact match found in list
		t.Errorf("expected exact match, got %q", got)
	}

	// 3. Location ID shifted from 1101 to 1401 (channel 1 preserved)
	got = MatchCandidatePort("/dev/cu.usbserial-1101", available)
	if got != "/dev/cu.usbserial-1401" {
		t.Errorf("expected /dev/cu.usbserial-1401, got %q", got)
	}

	// 4. Location ID shifted from 1100 to 1400 (channel 0 preserved)
	got = MatchCandidatePort("/dev/cu.usbserial-1100", available)
	if got != "/dev/cu.usbserial-1400" {
		t.Errorf("expected /dev/cu.usbserial-1400, got %q", got)
	}

	// 5. Linux ttyUSB0 -> ttyUSB1
	linuxPorts := []string{"/dev/ttyUSB1"}
	got = MatchCandidatePort("/dev/ttyUSB0", linuxPorts)
	if got != "/dev/ttyUSB1" {
		t.Errorf("expected /dev/ttyUSB1, got %q", got)
	}

	// 6. Non-existent device family returns empty
	got = MatchCandidatePort("COM1", available)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
