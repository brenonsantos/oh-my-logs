package filter

import (
	"strings"
)

// MatchKind specifies the type of matching an expression performs.
type MatchKind int

const (
	// MatchContains performs a global case-insensitive substring match across
	// all field values of a record.
	MatchContains MatchKind = iota
	// MatchFieldEqual checks whether a specific field equals one of the given
	// values (case-insensitive, OR semantics across values).
	MatchFieldEqual
)

// Expr is a single parsed filter token.
type Expr struct {
	Negate bool      // if true, the match result is inverted
	Kind   MatchKind
	Text   string   // for MatchContains: the substring to search
	Field  string   // for MatchFieldEqual: the field key
	Values []string // for MatchFieldEqual: accepted values (OR semantics)
}

// ParseFilter parses a filter expression string into a slice of Expr values.
// Tokens are separated by whitespace; all tokens must match (AND semantics).
// Empty input returns nil, nil.
//
// Syntax:
//
//	term          – any field contains "term" (case-insensitive)
//	-term         – no field contains "term"
//	field:value   – field equals value (case-insensitive)
//	-field:value  – field does NOT equal value
//	field:v1,v2   – field equals v1 OR v2
func ParseFilter(input string) ([]Expr, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	tokens := strings.Fields(input)
	exprs := make([]Expr, 0, len(tokens))

	for _, tok := range tokens {
		var e Expr

		if strings.HasPrefix(tok, "-") {
			e.Negate = true
			tok = tok[1:]
		}

		if idx := strings.IndexByte(tok, ':'); idx >= 0 {
			e.Kind = MatchFieldEqual
			e.Field = tok[:idx]
			rawValues := tok[idx+1:]
			for _, v := range strings.Split(rawValues, ",") {
				e.Values = append(e.Values, strings.ToLower(strings.TrimSpace(v)))
			}
		} else {
			e.Kind = MatchContains
			e.Text = strings.ToLower(tok)
		}

		exprs = append(exprs, e)
	}

	return exprs, nil
}
