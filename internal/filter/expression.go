package filter

import (
	"strings"
)

// MatchKind specifies the type of matching an expression performs.
type MatchKind int

const (
	// MatchContains performs a global case-insensitive substring match across
	// all field values of a record (OR semantics across Values).
	MatchContains MatchKind = iota
	// MatchFieldEqual checks whether a specific field equals one of the given
	// values (case-insensitive, OR semantics across Values).
	MatchFieldEqual
)

// Expr is a single parsed filter token.
type Expr struct {
	Negate bool
	Kind   MatchKind
	Field  string   // for MatchFieldEqual: the field key
	Values []string // accepted values (OR semantics) for either kind
}

// ParseFilter parses a filter expression string into a slice of Expr values.
// Tokens separated by whitespace are ANDed together.
// Values separated by comma or pipe within a token are ORed together.
//
// Syntax:
//
//	can, over     – any field contains "can" OR "over"
//	can | over    – any field contains "can" OR "over"
//	motor         – any field contains "motor"
//	-motor        – no field contains "motor"
//	module:CAN    – field equals value
//	module:CAN,SYS– field equals CAN OR SYS
//	motor -ping   – contains motor AND does NOT contain ping
func ParseFilter(input string) ([]Expr, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	// Normalize: replace pipe '|' with ','
	input = strings.ReplaceAll(input, "|", ",")

	// Clean whitespace around commas so "can, over" becomes "can,over"
	// and "module:CAN, PDM" becomes "module:CAN,PDM"
	var sb strings.Builder
	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == ',' {
			// Strip preceding spaces in builder
			str := sb.String()
			sb.Reset()
			sb.WriteString(strings.TrimRight(str, " \t"))
			sb.WriteRune(',')
			// Skip trailing spaces
			for i+1 < len(runes) && (runes[i+1] == ' ' || runes[i+1] == '\t') {
				i++
			}
		} else {
			sb.WriteRune(r)
		}
	}
	cleaned := sb.String()

	tokens := strings.Fields(cleaned)
	exprs := make([]Expr, 0, len(tokens))

	for _, tok := range tokens {
		var e Expr

		if strings.HasPrefix(tok, "-") {
			e.Negate = true
			tok = tok[1:]
		}
		if tok == "" {
			continue
		}

		if idx := strings.IndexByte(tok, ':'); idx >= 0 {
			e.Kind = MatchFieldEqual
			e.Field = tok[:idx]
			rawValues := tok[idx+1:]
			for _, v := range strings.Split(rawValues, ",") {
				v = strings.ToLower(strings.TrimSpace(v))
				if v != "" {
					e.Values = append(e.Values, v)
				}
			}
		} else {
			e.Kind = MatchContains
			for _, v := range strings.Split(tok, ",") {
				v = strings.ToLower(strings.TrimSpace(v))
				if v != "" {
					e.Values = append(e.Values, v)
				}
			}
		}

		if len(e.Values) > 0 {
			exprs = append(exprs, e)
		}
	}

	return exprs, nil
}

