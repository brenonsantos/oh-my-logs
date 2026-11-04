package encoder

import "encoding/json"

// Config defines a command encoder / autocomplete provider rule.
type Config struct {
	// Name is a human-readable identifier for this encoder (e.g. "CAN Frame", "Protocol Builder").
	Name string `yaml:"name"`

	// Trigger is an optional prefix or keyword that engages this encoder automatically (e.g. "can ", "msg ").
	// If empty, the encoder can be activated via shortcut toggle.
	Trigger string `yaml:"trigger,omitempty"`

	// Triggers is an optional list of prefixes that engage this encoder automatically.
	Triggers []string `yaml:"triggers,omitempty"`

	// Description is an optional description shown in UI menus or tooltips.
	Description string `yaml:"description,omitempty"`

	// Exec is the command line to run as a persistent worker process.
	// Incoming requests are sent to stdin as JSON lines; responses are read from stdout.
	Exec string `yaml:"exec"`

	// ExtraPaths contains additional search directories to prepend to PATH for the worker.
	ExtraPaths []string `yaml:"extra_paths,omitempty"`
}

// Suggestion represents an autocomplete item returned by an encoder.
type Suggestion struct {
	Value       string `json:"value"`                 // The text to insert/complete
	Description string `json:"description,omitempty"` // Helpful description (e.g. "Topic: State Update")
}

// UnmarshalJSON supports both "description" and legacy "desc" JSON field keys.
func (s *Suggestion) UnmarshalJSON(data []byte) error {
	type Alias Suggestion
	aux := struct {
		*Alias
		AltDesc string `json:"desc,omitempty"`
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if s.Description == "" && aux.AltDesc != "" {
		s.Description = aux.AltDesc
	}
	return nil
}

// EncodeResult represents the result of encoding a human-readable command into wire bytes.
type EncodeResult struct {
	Payload string `json:"payload"`           // The actual string/bytes to transmit over serial UART
	Display string `json:"display,omitempty"` // Optional formatted text to record in TX log history
}
