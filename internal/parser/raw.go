package parser

import "github.com/brenoniehues/oh-my-logs/internal/record"

// RawParser is a trivial parser that places the entire line into a
// "message" field. It is useful as a fallback when no structured format
// is configured.
type RawParser struct{}

// NewRawParser returns a new RawParser.
func NewRawParser() *RawParser { return &RawParser{} }

// Parse implements Parser. It always succeeds: the whole line becomes
// Fields["message"]. Raw is set to the original line.
func (p *RawParser) Parse(line string) (record.Record, error) {
	r := record.NewRecord(line)
	r.Fields["message"] = line
	return r, nil
}
