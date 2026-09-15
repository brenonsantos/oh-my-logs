package decoder

// Config defines a payload decoder rule.
type Config struct {
	// Match is the regex pattern to match against the record message or raw line.
	Match string `yaml:"match"`

	// Format is an optional template string for declarative decoding.
	// Named capture groups from Match can be referenced like "{field}".
	// Example: "CPU: {cpu}, Usage: {usage}%, Mem: {mem}KB"
	Format string `yaml:"format,omitempty"`

	// Exec is an optional shell command to run as a persistent worker process.
	// Incoming matching payloads are sent to stdin; decoded JSON lines are read from stdout.
	// Example: "python3 .oml/smp_decoder.py"
	Exec string `yaml:"exec,omitempty"`
}
