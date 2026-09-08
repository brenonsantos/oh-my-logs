package record_test

import (
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func makeRecord(msg string) record.Record {
	r := record.NewRecord(msg)
	r.Fields["message"] = msg
	return r
}

func TestBuffer_Empty(t *testing.T) {
	b := record.NewBuffer(10)
	if got := b.Len(); got != 0 {
		t.Fatalf("empty buffer: Len() = %d, want 0", got)
	}
	if got := b.All(); got != nil {
		t.Fatalf("empty buffer: All() = %v, want nil", got)
	}
	if got := b.Cap(); got != 10 {
		t.Fatalf("Cap() = %d, want 10", got)
	}
}

func TestBuffer_PartialFill(t *testing.T) {
	b := record.NewBuffer(10)
	for i := 0; i < 5; i++ {
		b.Add(makeRecord(string(rune('A' + i))))
	}
	if got := b.Len(); got != 5 {
		t.Fatalf("Len() = %d, want 5", got)
	}
	all := b.All()
	if len(all) != 5 {
		t.Fatalf("All() len = %d, want 5", len(all))
	}
	for i, r := range all {
		want := string(rune('A' + i))
		if r.Fields["message"] != want {
			t.Errorf("all[%d].message = %q, want %q", i, r.Fields["message"], want)
		}
	}
}

func TestBuffer_ExactlyFull(t *testing.T) {
	b := record.NewBuffer(5)
	for i := 0; i < 5; i++ {
		b.Add(makeRecord(string(rune('A' + i))))
	}
	if got := b.Len(); got != 5 {
		t.Fatalf("Len() = %d, want 5", got)
	}
}

func TestBuffer_Overflow(t *testing.T) {
	b := record.NewBuffer(3)
	// Add 5 records into a capacity-3 buffer.
	labels := []string{"A", "B", "C", "D", "E"}
	for _, l := range labels {
		b.Add(makeRecord(l))
	}
	// Buffer should hold the last 3: C, D, E
	if got := b.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
	all := b.All()
	want := []string{"C", "D", "E"}
	for i, r := range all {
		if r.Fields["message"] != want[i] {
			t.Errorf("all[%d].message = %q, want %q", i, r.Fields["message"], want[i])
		}
	}
}

func TestBuffer_OrderPreserved(t *testing.T) {
	b := record.NewBuffer(100)
	for i := 0; i < 50; i++ {
		b.Add(makeRecord(string(rune('a' + i%26))))
	}
	all := b.All()
	if len(all) != 50 {
		t.Fatalf("len(All()) = %d, want 50", len(all))
	}
	// Just verify insertion order is maintained.
	for i, r := range all {
		want := string(rune('a' + i%26))
		if r.Fields["message"] != want {
			t.Errorf("all[%d].message = %q, want %q", i, r.Fields["message"], want)
		}
	}
}

func TestBuffer_Clear(t *testing.T) {
	b := record.NewBuffer(10)
	for i := 0; i < 5; i++ {
		b.Add(makeRecord("x"))
	}
	b.Clear()
	if got := b.Len(); got != 0 {
		t.Fatalf("after Clear(), Len() = %d, want 0", got)
	}
	if got := b.All(); got != nil {
		t.Fatalf("after Clear(), All() = %v, want nil", got)
	}
}

func TestBuffer_AddAfterClear(t *testing.T) {
	b := record.NewBuffer(5)
	b.Add(makeRecord("old"))
	b.Clear()
	b.Add(makeRecord("new"))
	all := b.All()
	if len(all) != 1 || all[0].Fields["message"] != "new" {
		t.Fatalf("after Clear()+Add, unexpected state: %v", all)
	}
}
