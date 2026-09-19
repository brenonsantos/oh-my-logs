package record_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func BenchmarkBuffer_Add(b *testing.B) {
	buf := record.NewBuffer(record.DefaultCapacity)
	r := record.NewRecord("test log line message")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Add(r)
	}
}

func BenchmarkBuffer_InsertBeforeID(b *testing.B) {
	buf := record.NewBuffer(1000)
	for i := 1; i <= 1000; i++ {
		r := record.NewRecord("record " + strconv.Itoa(i))
		r.ID = uint64(i)
		buf.Add(r)
	}

	targetID := uint64(500)
	marker := record.NewMarkerRecord("checkpoint", time.Now())

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.InsertBeforeID(targetID, marker)
	}
}

func BenchmarkBuffer_All(b *testing.B) {
	buf := record.NewBuffer(10_000)
	for i := 0; i < 10_000; i++ {
		buf.Add(record.NewRecord("record"))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = buf.All()
	}
}
