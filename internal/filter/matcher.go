package filter

import (
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// Match reports whether r satisfies all expressions (AND logic).
// An empty exprs slice always returns true.
func Match(r record.Record, exprs []Expr) bool {
	for _, e := range exprs {
		if !matchOne(r, e) {
			return false
		}
	}
	return true
}

func matchOne(r record.Record, e Expr) bool {
	switch e.Kind {
	case MatchContains:
		return matchContains(r, e)
	case MatchFieldEqual:
		return matchFieldEqual(r, e)
	}
	return true
}

// matchContains joins all field values and checks for a substring.
func matchContains(r record.Record, e Expr) bool {
	// Build a combined string of all field values for global search.
	var parts []string
	for _, v := range r.Fields {
		parts = append(parts, v)
	}
	combined := strings.ToLower(strings.Join(parts, " "))
	found := strings.Contains(combined, e.Text)
	if e.Negate {
		return !found
	}
	return found
}

// matchFieldEqual checks whether the named field equals any of the target values.
func matchFieldEqual(r record.Record, e Expr) bool {
	fieldVal := strings.ToLower(r.Fields[e.Field])
	matched := false
	for _, v := range e.Values {
		if fieldVal == v {
			matched = true
			break
		}
	}
	if e.Negate {
		return !matched
	}
	return matched
}
