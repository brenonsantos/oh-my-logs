package record

import "sync"

// DefaultCapacity is the default ring buffer size.
const DefaultCapacity = 50_000

// Buffer is a fixed-capacity, thread-safe ring buffer for Records.
// When full, the oldest record is silently overwritten.
type Buffer struct {
	mu     sync.Mutex
	data   []Record
	cap    int
	head   int // index of the next write position
	size   int // number of valid records currently stored
	nextID uint64
}

// NewBuffer creates a new ring buffer with the given capacity.
// Panics if capacity < 1.
func NewBuffer(capacity int) *Buffer {
	if capacity < 1 {
		panic("record.Buffer: capacity must be >= 1")
	}
	return &Buffer{
		data: make([]Record, capacity),
		cap:  capacity,
	}
}

// Add inserts a record into the buffer. If the buffer is full, the oldest
// record is overwritten.
func (b *Buffer) Add(r Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.ID == 0 {
		b.nextID++
		r.ID = b.nextID
	}
	b.data[b.head] = r
	b.head = (b.head + 1) % b.cap
	if b.size < b.cap {
		b.size++
	}
}

// All returns a slice of all records in insertion order (oldest first).
// The returned slice is a copy; modifications do not affect the buffer.
func (b *Buffer) All() []Record {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size == 0 {
		return nil
	}
	out := make([]Record, b.size)
	// The oldest record is at (head - size + cap) % cap.
	start := (b.head - b.size + b.cap) % b.cap
	for i := 0; i < b.size; i++ {
		idx := (start + i) % b.cap
		if b.data[idx].ID == 0 {
			b.nextID++
			b.data[idx].ID = b.nextID
		}
		out[i] = b.data[idx]
	}
	return out
}

// Len returns the number of records currently in the buffer.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

// Cap returns the maximum capacity of the buffer.
func (b *Buffer) Cap() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cap
}

// Clear removes all records from the buffer.
func (b *Buffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.head = 0
	b.size = 0
}

// Transform applies fn to each record in the buffer in place.
// The original Record.ID is preserved if the transformed record has ID 0.
func (b *Buffer) Transform(fn func(r Record) Record) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size == 0 {
		return
	}
	start := (b.head - b.size + b.cap) % b.cap
	for i := 0; i < b.size; i++ {
		idx := (start + i) % b.cap
		oldID := b.data[idx].ID
		transformed := fn(b.data[idx])
		if transformed.ID == 0 {
			if oldID != 0 {
				transformed.ID = oldID
			} else {
				b.nextID++
				transformed.ID = b.nextID
			}
		}
		b.data[idx] = transformed
	}
}
