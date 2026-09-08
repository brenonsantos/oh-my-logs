package filter

import "github.com/brenoniehues/oh-my-logs/internal/record"

// Filter holds a parsed filter expression and applies it to records.
type Filter struct {
	Raw   string // the original input string (for display)
	exprs []Expr
}

// New parses the filter string and returns a Filter.
// An empty string produces an empty filter that matches everything.
func New(input string) (*Filter, error) {
	exprs, err := ParseFilter(input)
	if err != nil {
		return nil, err
	}
	return &Filter{Raw: input, exprs: exprs}, nil
}

// Matches reports whether the record passes all filter expressions.
func (f *Filter) Matches(r record.Record) bool {
	return Match(r, f.exprs)
}

// Empty returns true when no filter expressions are active (matches everything).
func (f *Filter) Empty() bool {
	return len(f.exprs) == 0
}
