package serial

import (
	goserial "go.bug.st/serial"
)

// Config holds serial port connection parameters.
type Config struct {
	Port     string
	Baud     int
	DataBits int
	StopBits StopBits
	Parity   Parity
}

// DefaultConfig returns sensible defaults: 115200 8N1.
func DefaultConfig() Config {
	return Config{
		Baud:     115200,
		DataBits: 8,
		StopBits: StopBits1,
		Parity:   ParityNone,
	}
}

// CommonBaudRates returns the standard baud rates available for selection.
func CommonBaudRates() []int {
	return []int{9600, 19200, 38400, 57600, 115200, 230400, 460800, 921600}
}

// StopBits selects the number of stop bits.
type StopBits int

const (
	StopBits1  StopBits = iota // 1 stop bit
	StopBits15                 // 1.5 stop bits
	StopBits2                  // 2 stop bits
)

// Parity selects the parity mode.
type Parity int

const (
	ParityNone  Parity = iota
	ParityOdd
	ParityEven
	ParityMark
	ParitySpace
)

// toSerialMode converts our Config to the go.bug.st/serial.Mode struct.
func toSerialMode(cfg Config) goserial.Mode {
	m := goserial.Mode{
		BaudRate: cfg.Baud,
		DataBits: cfg.DataBits,
	}

	switch cfg.StopBits {
	case StopBits1:
		m.StopBits = goserial.OneStopBit
	case StopBits15:
		m.StopBits = goserial.OnePointFiveStopBits
	case StopBits2:
		m.StopBits = goserial.TwoStopBits
	default:
		m.StopBits = goserial.OneStopBit
	}

	switch cfg.Parity {
	case ParityNone:
		m.Parity = goserial.NoParity
	case ParityOdd:
		m.Parity = goserial.OddParity
	case ParityEven:
		m.Parity = goserial.EvenParity
	case ParityMark:
		m.Parity = goserial.MarkParity
	case ParitySpace:
		m.Parity = goserial.SpaceParity
	default:
		m.Parity = goserial.NoParity
	}

	return m
}
