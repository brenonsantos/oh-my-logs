package filter_test

import (
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/record"
)

// Helper to build a record from a map.
func rec(fields map[string]string) record.Record {
	r := record.NewRecord("")
	for k, v := range fields {
		r.Fields[k] = v
	}
	return r
}

// ── ParseFilter ──────────────────────────────────────────────────────────────

func TestParseFilter_Empty(t *testing.T) {
	exprs, err := filter.ParseFilter("")
	if err != nil || exprs != nil {
		t.Fatalf("empty input: got exprs=%v err=%v, want nil,nil", exprs, err)
	}
}

func TestParseFilter_Contains(t *testing.T) {
	exprs, _ := filter.ParseFilter("motor")
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expr, got %d", len(exprs))
	}
	e := exprs[0]
	if e.Kind != filter.MatchContains || len(e.Values) != 1 || e.Values[0] != "motor" || e.Negate {
		t.Errorf("unexpected expr: %+v", e)
	}
}

func TestParseFilter_NegativeContains(t *testing.T) {
	exprs, _ := filter.ParseFilter("-motor")
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expr, got %d", len(exprs))
	}
	e := exprs[0]
	if !e.Negate || e.Kind != filter.MatchContains || len(e.Values) != 1 || e.Values[0] != "motor" {
		t.Errorf("unexpected expr: %+v", e)
	}
}

func TestParseFilter_CommaOR(t *testing.T) {
	exprs, _ := filter.ParseFilter("can, over")
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expr, got %d", len(exprs))
	}
	e := exprs[0]
	if e.Kind != filter.MatchContains || len(e.Values) != 2 || e.Values[0] != "can" || e.Values[1] != "over" {
		t.Errorf("unexpected expr: %+v", e)
	}
}

func TestParseFilter_PipeOR(t *testing.T) {
	exprs, _ := filter.ParseFilter("can | over")
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expr, got %d", len(exprs))
	}
	e := exprs[0]
	if e.Kind != filter.MatchContains || len(e.Values) != 2 || e.Values[0] != "can" || e.Values[1] != "over" {
		t.Errorf("unexpected expr: %+v", e)
	}
}


func TestParseFilter_FieldEqual(t *testing.T) {
	exprs, _ := filter.ParseFilter("module:CAN")
	if len(exprs) != 1 {
		t.Fatalf("expected 1 expr, got %d", len(exprs))
	}
	e := exprs[0]
	if e.Kind != filter.MatchFieldEqual || e.Field != "module" || len(e.Values) != 1 || e.Values[0] != "can" {
		t.Errorf("unexpected expr: %+v", e)
	}
}

func TestParseFilter_NegativeField(t *testing.T) {
	exprs, _ := filter.ParseFilter("-module:CAN")
	e := exprs[0]
	if !e.Negate || e.Kind != filter.MatchFieldEqual {
		t.Errorf("unexpected expr: %+v", e)
	}
}

func TestParseFilter_MultipleValues(t *testing.T) {
	exprs, _ := filter.ParseFilter("level:error,warn")
	e := exprs[0]
	if e.Field != "level" || len(e.Values) != 2 || e.Values[0] != "error" || e.Values[1] != "warn" {
		t.Errorf("unexpected expr: %+v", e)
	}
}

func TestParseFilter_MultipleTokens(t *testing.T) {
	exprs, _ := filter.ParseFilter("motor module:CAN")
	if len(exprs) != 2 {
		t.Fatalf("expected 2 exprs, got %d", len(exprs))
	}
}

// ── Filter.Matches ───────────────────────────────────────────────────────────

func TestFilter_EmptyMatchesAll(t *testing.T) {
	f, _ := filter.New("")
	r := rec(map[string]string{"message": "anything"})
	if !f.Matches(r) {
		t.Error("empty filter should match everything")
	}
	if !f.Empty() {
		t.Error("Empty() should be true")
	}
}

func TestFilter_Contains_Match(t *testing.T) {
	f, _ := filter.New("motor")
	r := rec(map[string]string{"message": "MotorControl started"})
	if !f.Matches(r) {
		t.Error("should match: 'motor' in 'MotorControl started'")
	}
}

func TestFilter_Contains_NoMatch(t *testing.T) {
	f, _ := filter.New("motor")
	r := rec(map[string]string{"message": "CAN initialized"})
	if f.Matches(r) {
		t.Error("should not match")
	}
}

func TestFilter_NegativeContains_Excludes(t *testing.T) {
	f, _ := filter.New("-heartbeat")
	r := rec(map[string]string{"message": "heartbeat ping"})
	if f.Matches(r) {
		t.Error("negative filter should exclude matching record")
	}
}

func TestFilter_NegativeContains_Passes(t *testing.T) {
	f, _ := filter.New("-heartbeat")
	r := rec(map[string]string{"message": "motor started"})
	if !f.Matches(r) {
		t.Error("negative filter should pass non-matching record")
	}
}

func TestFilter_FieldEqual_Match(t *testing.T) {
	f, _ := filter.New("module:CAN")
	r := rec(map[string]string{"module": "CAN", "message": "init"})
	if !f.Matches(r) {
		t.Error("field:value should match")
	}
}

func TestFilter_FieldEqual_CaseInsensitive(t *testing.T) {
	f, _ := filter.New("module:can")
	r := rec(map[string]string{"module": "CAN"})
	if !f.Matches(r) {
		t.Error("field match should be case-insensitive")
	}
}

func TestFilter_FieldEqual_MissingField(t *testing.T) {
	f, _ := filter.New("module:CAN")
	r := rec(map[string]string{"message": "no module field"})
	if f.Matches(r) {
		t.Error("missing field should not match")
	}
}

func TestFilter_NegativeField(t *testing.T) {
	f, _ := filter.New("-module:CAN")
	r := rec(map[string]string{"module": "CAN"})
	if f.Matches(r) {
		t.Error("negative field:value should exclude matching record")
	}
}

func TestFilter_FieldOR(t *testing.T) {
	f, _ := filter.New("level:error,warn")
	rErr := rec(map[string]string{"level": "ERROR"})
	rWarn := rec(map[string]string{"level": "WARN"})
	rInfo := rec(map[string]string{"level": "INFO"})
	if !f.Matches(rErr) {
		t.Error("ERROR should match level:error,warn")
	}
	if !f.Matches(rWarn) {
		t.Error("WARN should match level:error,warn")
	}
	if f.Matches(rInfo) {
		t.Error("INFO should not match level:error,warn")
	}
}

func TestFilter_MultipleAND(t *testing.T) {
	f, _ := filter.New("motor module:CAN")
	rBoth := rec(map[string]string{"module": "CAN", "message": "motor started"})
	rOnlyModule := rec(map[string]string{"module": "CAN", "message": "CAN init"})
	if !f.Matches(rBoth) {
		t.Error("record with both conditions should match")
	}
	if f.Matches(rOnlyModule) {
		t.Error("record missing 'motor' should not match")
	}
}

func TestFilter_ContainsOR(t *testing.T) {
	f, err := filter.New("can, over")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rCAN := rec(map[string]string{"module": "CAN", "message": "CAN initialized"})
	rOver := rec(map[string]string{"module": "PDM", "message": "Overcurrent detected"})
	rADC := rec(map[string]string{"module": "ADC", "message": "ADC reading high"})

	if !f.Matches(rCAN) {
		t.Error("'can, over' should match CAN record")
	}
	if !f.Matches(rOver) {
		t.Error("'can, over' should match Overcurrent record")
	}
	if f.Matches(rADC) {
		t.Error("'can, over' should NOT match ADC record")
	}
}

func TestFilter_ContainsOR_WithNegation(t *testing.T) {
	// "can, over -ADC" means (can OR over) AND NOT adc
	f, err := filter.New("can, over -ADC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rCAN := rec(map[string]string{"module": "CAN", "message": "CAN initialized"})
	rBoth := rec(map[string]string{"module": "CAN", "message": "CAN ADC error"})

	if !f.Matches(rCAN) {
		t.Error("should match pure CAN")
	}
	if f.Matches(rBoth) {
		t.Error("should be excluded because it contains ADC")
	}
}

