package record

// Record is the generic, profile-agnostic data object produced by a parser.
// Fields contains named values extracted from a raw serial line.
// Raw preserves the original unmodified line.
type Record struct {
	Fields map[string]string
	Raw    string
}

// NewRecord creates an empty Record with the given raw line.
func NewRecord(raw string) Record {
	return Record{
		Fields: make(map[string]string),
		Raw:    raw,
	}
}

// Get returns the value of a field, or an empty string if the field is absent.
func (r Record) Get(field string) string {
	return r.Fields[field]
}

// Column describes a single display column in the TUI table.
type Column struct {
	Field  string            // the Record.Fields key to render
	Title  string            // the column header label
	Width  int               // fixed column width; 0 means flexible (fill remaining space)
	Style  string            // semantic style: "timestamp", "level", "identifier", "primary", "muted"
	Colors map[string]string // optional value -> color mapping (e.g. "ERROR" -> "red")
}

