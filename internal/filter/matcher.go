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

// matchContains checks whether any field value contains one of e.Values.
// If e.Values has multiple entries, ANY match satisfies the condition (OR semantics).
func matchContains(r record.Record, e Expr) bool {
	matched := false
outer:
	for _, val := range e.Values {
		if r.Raw != "" && containsFold(r.Raw, val) {
			matched = true
			break outer
		}
		for _, v := range r.Fields {
			if containsFold(v, val) {
				matched = true
				break outer
			}
		}
	}

	if e.Negate {
		return !matched
	}
	return matched
}

// matchFieldEqual checks whether the named field contains or equals any of the target values.
func matchFieldEqual(r record.Record, e Expr) bool {
	rawVal, ok := r.Fields[e.Field]
	if !ok {
		return e.Negate
	}
	matched := false
	for _, v := range e.Values {
		if containsFold(rawVal, v) {
			matched = true
			break
		}
	}
	if e.Negate {
		return !matched
	}
	return matched
}

// containsFold performs a fast, zero-allocation case-insensitive substring search.
// substrLower must already be lowercase.
func containsFold(s, substrLower string) bool {
	n := len(substrLower)
	if n == 0 {
		return true
	}
	if len(s) < n {
		return false
	}
	maxStart := len(s) - n
	for i := 0; i <= maxStart; i++ {
		match := true
		for j := 0; j < n; j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != substrLower[j] {
				if c >= 0x80 || substrLower[j] >= 0x80 {
					return strings.Contains(strings.ToLower(s), substrLower)
				}
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
