package parser_test

import (
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/parser"
)

func BenchmarkParser_Regex(b *testing.B) {
	const pattern = `^\[(?P<time>[^\]]+)\]\[(?P<level>[^\]]+)\]\[(?P<module>[^\]]+)\]\s+(?P<message>.*)$`
	p, err := parser.NewRegexParser(pattern)
	if err != nil {
		b.Fatalf("NewRegexParser error: %v", err)
	}

	line := "[15:42:31.102][INFO][SYS] System initialized successfully on core 0"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = p.Parse(line)
	}
}

func BenchmarkParser_Raw(b *testing.B) {
	p := parser.NewRawParser()
	line := "[15:42:31.102][INFO][SYS] System initialized successfully on core 0"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = p.Parse(line)
	}
}
