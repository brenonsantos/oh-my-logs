package decoder

import (
	"fmt"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// Decoder processes a record if its message or raw payload matches a rule.
type Decoder interface {
	Decode(r record.Record) (record.Record, bool)
	Close() error
}

// Pipeline orchestrates multiple decoders, evaluating them in order.
type Pipeline struct {
	decoders []Decoder
}

// NewPipeline initializes a Pipeline from decoder configs.
func NewPipeline(configs []Config) (*Pipeline, error) {
	var decs []Decoder
	for idx, cfg := range configs {
		if cfg.Match == "" {
			continue
		}
		if cfg.Exec != "" {
			d, err := NewExecDecoder(cfg)
			if err != nil {
				return nil, fmt.Errorf("decoder[%d] exec: %w", idx, err)
			}
			decs = append(decs, d)
		} else {
			d, err := NewDeclarativeDecoder(cfg)
			if err != nil {
				return nil, fmt.Errorf("decoder[%d] match: %w", idx, err)
			}
			decs = append(decs, d)
		}
	}
	return &Pipeline{decoders: decs}, nil
}

// Decode runs the pipeline against a Record. The first matching decoder
// transforms the record and returns it.
func (p *Pipeline) Decode(r record.Record) record.Record {
	if p == nil || len(p.decoders) == 0 {
		return r
	}
	for _, d := range p.decoders {
		if transformed, matched := d.Decode(r); matched {
			return transformed
		}
	}
	return r
}

// Close gracefully closes all decoders in the pipeline.
func (p *Pipeline) Close() error {
	if p == nil {
		return nil
	}
	var firstErr error
	for _, d := range p.decoders {
		if err := d.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
