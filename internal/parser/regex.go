package parser

import (
	"fmt"
	"regexp"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// RegexParser parses lines using a regular expression with named capture groups.
// Example pattern:  `^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\s+(?P<message>.*)$`
type RegexParser struct {
	re *regexp.Regexp
}

// NewRegexParser compiles the given pattern and returns a RegexParser.
// Returns an error if the pattern is not a valid regular expression.
func NewRegexParser(pattern string) (*RegexParser, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("parser: invalid regex pattern: %w", err)
	}
	return &RegexParser{re: re}, nil
}

// Parse implements Parser. Named capture groups become record fields.
// If the line does not match the pattern, a record with Fields["_raw"] is
// returned alongside a descriptive error (the caller decides how to handle it).
func (p *RegexParser) Parse(line string) (record.Record, error) {
	r := record.NewRecord(line)

	match := p.re.FindStringSubmatch(line)
	if match == nil {
		r.Fields["_raw"] = line
		r.Fields["message"] = line
		return r, fmt.Errorf("parser: line did not match pattern: %q", line)
	}

	names := p.re.SubexpNames()
	for i, name := range names {
		if i == 0 || name == "" {
			continue // skip full match and unnamed groups
		}
		r.Fields[name] = match[i]
	}
	return r, nil
}
