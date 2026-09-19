package filter_test

import (
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/filter"
	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func BenchmarkFilter_Match_Contains(b *testing.B) {
	f, err := filter.New("motor")
	if err != nil {
		b.Fatalf("filter.New error: %v", err)
	}

	r := record.NewRecord("[15:42:31.102][INFO][SYS] MotorControl loop running at 1000Hz")
	r.Fields["time"] = "15:42:31.102"
	r.Fields["level"] = "INFO"
	r.Fields["module"] = "SYS"
	r.Fields["message"] = "MotorControl loop running at 1000Hz"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.Matches(r)
	}
}

func BenchmarkFilter_Match_FieldEqual(b *testing.B) {
	f, err := filter.New("module:SYS")
	if err != nil {
		b.Fatalf("filter.New error: %v", err)
	}

	r := record.NewRecord("[15:42:31.102][INFO][SYS] MotorControl loop running at 1000Hz")
	r.Fields["time"] = "15:42:31.102"
	r.Fields["level"] = "INFO"
	r.Fields["module"] = "SYS"
	r.Fields["message"] = "MotorControl loop running at 1000Hz"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.Matches(r)
	}
}

func BenchmarkFilter_Match_Complex(b *testing.B) {
	f, err := filter.New("level:info,warn module:SYS motor -fault")
	if err != nil {
		b.Fatalf("filter.New error: %v", err)
	}

	r := record.NewRecord("[15:42:31.102][INFO][SYS] MotorControl loop running at 1000Hz")
	r.Fields["time"] = "15:42:31.102"
	r.Fields["level"] = "INFO"
	r.Fields["module"] = "SYS"
	r.Fields["message"] = "MotorControl loop running at 1000Hz"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = f.Matches(r)
	}
}
