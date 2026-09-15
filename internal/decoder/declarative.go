package decoder

import (
	"regexp"
	"strings"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// DeclarativeDecoder applies regex named capture groups and formatting templates.
type DeclarativeDecoder struct {
	re     *regexp.Regexp
	format string
}

// NewDeclarativeDecoder compiles and creates a DeclarativeDecoder.
func NewDeclarativeDecoder(cfg Config) (*DeclarativeDecoder, error) {
	re, err := regexp.Compile(cfg.Match)
	if err != nil {
		return nil, err
	}
	return &DeclarativeDecoder{
		re:     re,
		format: cfg.Format,
	}, nil
}

// Decode extracts named capture groups from matching record payloads and formats the message.
func (d *DeclarativeDecoder) Decode(r record.Record) (record.Record, bool) {
	if d.re == nil {
		return r, false
	}

	candidate := r.Get("message")
	if candidate == "" {
		candidate = r.Get("msg")
	}
	if candidate == "" {
		candidate = r.Raw
	}

	matches := d.re.FindStringSubmatch(candidate)
	if matches == nil {
		return r, false
	}

	if r.Fields == nil {
		r.Fields = make(map[string]string)
	}

	// Preserve the original unparsed candidate payload before transformation
	if r.Fields["_raw_message"] == "" {
		r.Fields["_raw_message"] = candidate
	}

	names := d.re.SubexpNames()
	extracted := make(map[string]string)
	for i, name := range names {
		if i > 0 && i < len(matches) && name != "" {
			r.Fields[name] = matches[i]
			extracted[name] = matches[i]
		}
	}

	if d.format != "" {
		formatted := d.format
		for k, v := range extracted {
			token := "{" + k + "}"
			formatted = strings.ReplaceAll(formatted, token, v)
		}
		r.Fields["message"] = formatted
	}

	return r, true
}

// Close is a no-op for declarative decoders.
func (d *DeclarativeDecoder) Close() error {
	return nil
}
