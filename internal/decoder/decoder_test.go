package decoder

import (
	"bufio"
	"fmt"
	"os"
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/record"
)

func TestDeclarativeDecoder_PositionalTelemetry(t *testing.T) {
	cfg := Config{
		Match:  `^instruments,\s*(?P<cpu>\d+),\s*(?P<usage>\d+),\s*(?P<mem>\d+)`,
		Format: "CPU: {cpu}, Usage: {usage}%, Mem: {mem}KB",
	}

	d, err := NewDeclarativeDecoder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := record.NewRecord("[     18.220] <inf> instruments, 1, 42, 512")
	rec.Fields["message"] = "instruments, 1, 42, 512"

	transformed, matched := d.Decode(rec)
	if !matched {
		t.Fatalf("expected record to match decoder")
	}

	if transformed.Fields["message"] != "CPU: 1, Usage: 42%, Mem: 512KB" {
		t.Errorf("expected formatted message, got %q", transformed.Fields["message"])
	}
	if transformed.Fields["cpu"] != "1" {
		t.Errorf("expected cpu=1, got %q", transformed.Fields["cpu"])
	}
	if transformed.Fields["usage"] != "42" {
		t.Errorf("expected usage=42, got %q", transformed.Fields["usage"])
	}
	if transformed.Fields["mem"] != "512" {
		t.Errorf("expected mem=512, got %q", transformed.Fields["mem"])
	}
	if transformed.Fields["_raw_message"] != "instruments, 1, 42, 512" {
		t.Errorf("expected _raw_message to be preserved, got %q", transformed.Fields["_raw_message"])
	}
}

func TestDeclarativeDecoder_NoMatch(t *testing.T) {
	cfg := Config{
		Match:  `^instruments,\s*(?P<cpu>\d+)`,
		Format: "CPU: {cpu}",
	}

	d, err := NewDeclarativeDecoder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec := record.NewRecord("regular log message")
	rec.Fields["message"] = "regular log message"

	_, matched := d.Decode(rec)
	if matched {
		t.Fatalf("expected no match")
	}
}

func TestDeclarativeDecoder_RawFallback(t *testing.T) {
	cfg := Config{
		Match:  `^instruments:\s*(?P<stack>\d+)`,
		Format: "Stack: {stack}B",
	}

	d, err := NewDeclarativeDecoder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No "message" field, candidate is Raw
	rec := record.NewRecord("instruments: 1024")

	transformed, matched := d.Decode(rec)
	if !matched {
		t.Fatalf("expected record to match")
	}
	if transformed.Fields["message"] != "Stack: 1024B" {
		t.Errorf("expected formatted message, got %q", transformed.Fields["message"])
	}
	if transformed.Fields["_raw_message"] != "instruments: 1024" {
		t.Errorf("expected raw message preserved, got %q", transformed.Fields["_raw_message"])
	}
}

func TestPipeline_MultipleDecoders(t *testing.T) {
	configs := []Config{
		{
			Match:  `^sensorA:\s*(?P<val>\d+)`,
			Format: "Sensor A Value: {val}",
		},
		{
			Match:  `^sensorB:\s*(?P<val>\d+)`,
			Format: "Sensor B Value: {val}",
		},
	}

	pipeline, err := NewPipeline(configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer pipeline.Close()

	recA := record.NewRecord("sensorA: 42")
	recA.Fields["message"] = "sensorA: 42"
	resA := pipeline.Decode(recA)
	if resA.Fields["message"] != "Sensor A Value: 42" {
		t.Errorf("expected Sensor A formatted, got %q", resA.Fields["message"])
	}

	recB := record.NewRecord("sensorB: 99")
	recB.Fields["message"] = "sensorB: 99"
	resB := pipeline.Decode(recB)
	if resB.Fields["message"] != "Sensor B Value: 99" {
		t.Errorf("expected Sensor B formatted, got %q", resB.Fields["message"])
	}

	recC := record.NewRecord("sensorC: 123")
	recC.Fields["message"] = "sensorC: 123"
	resC := pipeline.Decode(recC)
	if resC.Fields["message"] != "sensorC: 123" {
		t.Errorf("expected sensorC to remain unmutated, got %q", resC.Fields["message"])
	}
}

// TestHelperProcess is used as a self-contained mock worker process for ExecDecoder tests.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_DECODER_HELPER_PROCESS") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "FAIL" {
			os.Exit(1)
		}
		// Return JSON response line
		fmt.Printf(`{"summary": "Decoded: %s", "fields": {"topic": "Cycle", "count": 5, "ok": true}}`+"\n", line)
	}
	os.Exit(0)
}

func TestExecDecoder_WorkerProcess(t *testing.T) {
	// Build command to launch this same test binary in helper process mode
	cmd := fmt.Sprintf("%s -test.run=TestHelperProcess", os.Args[0])

	cfg := Config{
		Match: `^smp:\s*(?P<payload>.*)`,
		Exec:  cmd,
	}

	d, err := NewExecDecoder(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer d.Close()

	// Pass env var to child process
	os.Setenv("GO_WANT_DECODER_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_WANT_DECODER_HELPER_PROCESS")

	rec := record.NewRecord("smp: 12345")
	rec.Fields["message"] = "smp: 12345"

	transformed, matched := d.Decode(rec)
	if !matched {
		t.Fatalf("expected record to match exec decoder")
	}

	if transformed.Fields["message"] != "Decoded: smp: 12345" {
		t.Errorf("expected summary message from worker, got %q", transformed.Fields["message"])
	}
	if transformed.Fields["topic"] != "Cycle" {
		t.Errorf("expected topic=Cycle, got %q", transformed.Fields["topic"])
	}
	if transformed.Fields["count"] != "5" {
		t.Errorf("expected count=5, got %q", transformed.Fields["count"])
	}
	if transformed.Fields["ok"] != "true" {
		t.Errorf("expected ok=true, got %q", transformed.Fields["ok"])
	}
	if transformed.Fields["_raw_message"] != "smp: 12345" {
		t.Errorf("expected _raw_message preserved, got %q", transformed.Fields["_raw_message"])
	}
}
