package parser

import "github.com/brenoniehues/oh-my-logs/internal/record"

// Parser transforms a raw serial line into a Record.
// If the line cannot be fully parsed, a parser should still return a Record
// (typically with a "_raw" field) and a non-nil error — it must not crash.
type Parser interface {
	Parse(line string) (record.Record, error)
}
