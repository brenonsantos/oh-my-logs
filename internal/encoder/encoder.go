package encoder

import (
	"fmt"
	"strings"
)

// Encoder transforms human-readable commands into wire-level payloads and provides autocomplete suggestions.
type Encoder interface {
	Name() string
	Trigger() string
	Description() string
	Suggest(query string, cursor int) ([]Suggestion, error)
	Encode(text string) (EncodeResult, error)
	Close() error
}

// Previewer is an optional interface implemented by encoders that can provide
// a live decoded or human-readable translation of the currently typed command.
type Previewer interface {
	LastPreview() string
}

// Pipeline orchestrates one or more encoders configured in a profile or project.
type Pipeline struct {
	encoders []Encoder
}

// NewPipeline initializes a Pipeline from encoder configs.
func NewPipeline(configs []Config) (*Pipeline, error) {
	var encs []Encoder
	for idx, cfg := range configs {
		if cfg.Exec == "" {
			continue
		}
		e, err := NewWorkerEncoder(cfg)
		if err != nil {
			return nil, fmt.Errorf("encoder[%d] %q: %w", idx, cfg.Name, err)
		}
		encs = append(encs, e)
	}
	return &Pipeline{encoders: encs}, nil
}

// NewPipelineWithEncoders creates a pipeline with existing Encoder implementations (useful for tests or custom wrappers).
func NewPipelineWithEncoders(encoders []Encoder) *Pipeline {
	return &Pipeline{encoders: encoders}
}

// Encoders returns all active encoders in the pipeline.
func (p *Pipeline) Encoders() []Encoder {
	if p == nil {
		return nil
	}
	return p.encoders
}

// Match inspects input and returns the matching encoder if input starts with a registered Trigger.
// It also returns the trimmed query string after the trigger.
func (p *Pipeline) Match(input string) (Encoder, string) {
	if p == nil || len(p.encoders) == 0 {
		return nil, input
	}

	var best Encoder
	bestTriggerLen := -1
	bestTrigger := ""

	for _, e := range p.encoders {
		rawTrigger := e.Trigger()
		for _, trig := range strings.Split(rawTrigger, ",") {
			trig = strings.Trim(trig, "\r\n\t")
			if trig != "" && strings.HasPrefix(input, trig) {
				if len(trig) > bestTriggerLen {
					best = e
					bestTriggerLen = len(trig)
					bestTrigger = trig
				}
			}
		}
	}

	if best != nil {
		rem := strings.TrimPrefix(input, bestTrigger)
		return best, rem
	}

	return nil, input
}

// FindByName retrieves an encoder by its Name identifier.
func (p *Pipeline) FindByName(name string) Encoder {
	if p == nil {
		return nil
	}
	for _, e := range p.encoders {
		if strings.EqualFold(e.Name(), name) {
			return e
		}
	}
	return nil
}

// Close gracefully terminates all background worker processes.
func (p *Pipeline) Close() error {
	if p == nil {
		return nil
	}
	var firstErr error
	for _, e := range p.encoders {
		if err := e.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
