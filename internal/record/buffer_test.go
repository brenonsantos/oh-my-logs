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

func TestBuffer_Resize(t *testing.T) {
	// 1. Grow capacity from 3 to 6
	b := record.NewBuffer(3)
	b.Add(makeRecord("A"))
	b.Add(makeRecord("B"))
	b.Add(makeRecord("C"))

	b.Resize(6)
	if b.Cap() != 6 {
		t.Fatalf("expected cap 6, got %d", b.Cap())
	}
	if b.Len() != 3 {
		t.Fatalf("expected len 3 after grow, got %d", b.Len())
	}
	all := b.All()
	if len(all) != 3 || all[0].Fields["message"] != "A" || all[2].Fields["message"] != "C" {
		t.Fatalf("unexpected records after grow: %v", all)
	}

	// Add more records to verify write operations continue properly
	b.Add(makeRecord("D"))
	b.Add(makeRecord("E"))
	if b.Len() != 5 {
		t.Fatalf("expected len 5 after adds, got %d", b.Len())
	}

	// 2. Shrink capacity from 6 to 3 (should truncate oldest: keeps C, D, E)
	b.Resize(3)
	if b.Cap() != 3 {
		t.Fatalf("expected cap 3, got %d", b.Cap())
	}
	if b.Len() != 3 {
		t.Fatalf("expected len 3 after shrink, got %d", b.Len())
	}
	all = b.All()
	if len(all) != 3 || all[0].Fields["message"] != "C" || all[1].Fields["message"] != "D" || all[2].Fields["message"] != "E" {
		t.Fatalf("unexpected records after shrink: %v", all)
	}

	// 3. Invalid capacity ignored
	b.Resize(0)
	if b.Cap() != 3 {
		t.Fatalf("expected cap 3 after invalid resize, got %d", b.Cap())
	}

	// 4. Same capacity is no-op
	b.Resize(3)
	if b.Cap() != 3 {
		t.Fatalf("expected cap 3, got %d", b.Cap())
	}
}

func TestBuffer_InsertBeforeID(t *testing.T) {
	b := record.NewBuffer(5)
	r1 := record.Record{ID: 10, Raw: "first"}
	r2 := record.Record{ID: 20, Raw: "second"}
	r3 := record.Record{ID: 30, Raw: "third"}
	b.Add(r1)
	b.Add(r2)
	b.Add(r3)

	// Insert before r2 (ID: 20)
	marker := record.Record{ID: 15, Raw: "marker"}
	b.InsertBeforeID(20, marker)

	all := b.All()
	if len(all) != 4 {
		t.Fatalf("expected 4 records, got %d", len(all))
	}
	if all[0].ID != 10 || all[1].ID != 15 || all[2].ID != 20 || all[3].ID != 30 {
		t.Fatalf("unexpected order after InsertBeforeID: %v", all)
	}

	// Insert with nonexistent target appends
	extra := record.Record{ID: 40, Raw: "extra"}
	b.InsertBeforeID(999, extra)
	all = b.All()
	if len(all) != 5 || all[4].ID != 40 {
		t.Fatalf("expected extra to append when target not found, got: %v", all)
	}
}

func TestBuffer_RemoveByID(t *testing.T) {
	b := record.NewBuffer(5)

	// 1. Remove from empty buffer
	if b.RemoveByID(10) {
		t.Errorf("expected RemoveByID to return false on empty buffer")
	}

	// 2. Add records [10, 20, 30, 40]
	b.Add(record.Record{ID: 10, Raw: "ten"})
	b.Add(record.Record{ID: 20, Raw: "twenty"})
	b.Add(record.Record{ID: 30, Raw: "thirty"})
	b.Add(record.Record{ID: 40, Raw: "forty"})

	// 3. Remove non-existent ID
	if b.RemoveByID(999) {
		t.Errorf("expected RemoveByID to return false for nonexistent ID")
	}
	if b.Len() != 4 {
		t.Errorf("expected len 4, got %d", b.Len())
	}

	// 4. Remove middle element (ID 20)
	if !b.RemoveByID(20) {
		t.Fatalf("expected RemoveByID(20) to return true")
	}
	if b.Len() != 3 {
		t.Fatalf("expected len 3, got %d", b.Len())
	}
	all := b.All()
	if len(all) != 3 || all[0].ID != 10 || all[1].ID != 30 || all[2].ID != 40 {
		t.Fatalf("unexpected records after removing 20: %v", all)
	}

	// 5. Remove first element (ID 10)
	if !b.RemoveByID(10) {
		t.Fatalf("expected RemoveByID(10) to return true")
	}
	all = b.All()
	if len(all) != 2 || all[0].ID != 30 || all[1].ID != 40 {
		t.Fatalf("unexpected records after removing 10: %v", all)
	}

	// 6. Remove last element (ID 40)
	if !b.RemoveByID(40) {
		t.Fatalf("expected RemoveByID(40) to return true")
	}
	all = b.All()
	if len(all) != 1 || all[0].ID != 30 {
		t.Fatalf("unexpected records after removing 40: %v", all)
	}

	// 7. Remove the only remaining element (ID 30)
	if !b.RemoveByID(30) {
		t.Fatalf("expected RemoveByID(30) to return true")
	}
	if b.Len() != 0 || len(b.All()) != 0 {
		t.Fatalf("expected buffer to be empty, len=%d", b.Len())
	}
}
