package serial

import goserial "go.bug.st/serial"

// ListPorts returns the names of all available serial ports on the system.
// If enumeration partially succeeds before encountering an error, the
// successfully enumerated ports are returned along with the error.
func ListPorts() ([]string, error) {
	ports, err := goserial.GetPortsList()
	return ports, err
}
