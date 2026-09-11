package parser

import (
	"fmt"
	"regexp"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// RegexParser parses lines using one or more regular expressions with named capture groups.
// When multiple patterns are provided, they are evaluated in order; the first matching pattern
// extracts the record's fields.
type RegexParser struct {
	regexes []*regexp.Regexp
}

// NewRegexParser compiles the given patterns and returns a RegexParser.
// Returns an error if no patterns are provided or if any pattern is not a valid regular expression.
func NewRegexParser(patterns ...string) (*RegexParser, error) {
	if len(patterns) == 0 {
		return nil, fmt.Errorf("parser: regex parser requires at least one pattern")
	}
	regexes := make([]*regexp.Regexp, 0, len(patterns))
	for _, pat := range patterns {
		if pat == "" {
			continue
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("parser: invalid regex pattern: %w", err)
		}
		regexes = append(regexes, re)
	}
	if len(regexes) == 0 {
		return nil, fmt.Errorf("parser: regex parser requires at least one non-empty pattern")
	}
	return &RegexParser{regexes: regexes}, nil
}

// Parse implements Parser. Named capture groups from the first matching pattern become record fields.
// If the line does not match any pattern, a record with Fields["_raw"] and Fields["message"] is
// returned alongside a descriptive error (the caller decides how to handle it).
func (p *RegexParser) Parse(line string) (record.Record, error) {
	r := record.NewRecord(line)

	for _, re := range p.regexes {
		match := re.FindStringSubmatch(line)
		if match != nil {
			names := re.SubexpNames()
			for i, name := range names {
				if i == 0 || name == "" {
					continue // skip full match and unnamed groups
				}
				r.Fields[name] = match[i]
			}
			return r, nil
		}
	}

	r.Fields["_raw"] = line
	r.Fields["message"] = line
	return r, fmt.Errorf("parser: line did not match pattern: %q", line)
}
