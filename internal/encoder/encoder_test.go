package encoder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPipeline_MatchAndFindByName(t *testing.T) {
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "mock_worker.sh")
	scriptContent := `#!/usr/bin/env sh
while read -r line; do
  action=$(echo "$line" | grep -o '"action":"[^"]*"' | cut -d':' -f2 | tr -d '"')
  if [ "$action" = "suggest" ]; then
    echo '{"suggestions":[{"value":"cmd1","desc":"first command"},{"value":"cmd2","desc":"second command"}]}'
  elif [ "$action" = "encode" ]; then
    text=$(echo "$line" | grep -o '"text":"[^"]*"' | cut -d':' -f2 | tr -d '"')
    echo "{\"payload\":\"RAW_${text}\",\"display\":\"Sent ${text}\"}"
  else
    echo '{"error":"unknown action"}'
  fi
done
`
	if err := os.WriteFile(workerScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("failed to write worker script: %v", err)
	}

	configs := []Config{
		{
			Name:        "CAN Encoder",
			Trigger:     "can ",
			Description: "CAN bus frame builder",
			Exec:        "sh mock_worker.sh",
			ExtraPaths:  []string{tmpDir},
		},
		{
			Name:        "Protocol Encoder",
			Trigger:     "msg ",
			Description: "Protocol message encoder",
			Exec:        "sh mock_worker.sh",
			ExtraPaths:  []string{tmpDir},
		},
		{
			Name:        "Direct Encoder",
			Description: "Encoder without trigger",
			Exec:        "sh mock_worker.sh",
			ExtraPaths:  []string{tmpDir},
		},
	}

	pipeline, err := NewPipeline(configs)
	if err != nil {
		t.Fatalf("NewPipeline failed: %v", err)
	}
	defer func() {
		_ = pipeline.Close()
	}()

	if len(pipeline.Encoders()) != 3 {
		t.Fatalf("expected 3 encoders, got %d", len(pipeline.Encoders()))
	}

	// 1. Test Match with triggers
	enc, rem := pipeline.Match("can send 0x123")
	if enc == nil {
		t.Fatalf("expected match for 'can '")
	}
	if enc.Name() != "CAN Encoder" {
		t.Errorf("matched encoder got %q, want %q", enc.Name(), "CAN Encoder")
	}
	if rem != "send 0x123" {
		t.Errorf("remainder got %q, want %q", rem, "send 0x123")
	}

	enc2, rem2 := pipeline.Match("msg UI:StateUpdate")
	if enc2 == nil {
		t.Fatalf("expected match for 'msg '")
	}
	if enc2.Name() != "Protocol Encoder" {
		t.Errorf("matched encoder got %q, want %q", enc2.Name(), "Protocol Encoder")
	}
	if rem2 != "UI:StateUpdate" {
		t.Errorf("remainder got %q, want %q", rem2, "UI:StateUpdate")
	}

	// 2. Test unmatched command
	encNone, remNone := pipeline.Match("wifi status")
	if encNone != nil {
		t.Errorf("expected no match for 'wifi status', got %v", encNone.Name())
	}
	if remNone != "wifi status" {
		t.Errorf("remainder for unmatched got %q, want %q", remNone, "wifi status")
	}

	// 3. Test FindByName
	direct := pipeline.FindByName("Direct Encoder")
	if direct == nil {
		t.Fatalf("expected to find 'Direct Encoder'")
	}
	if direct.Trigger() != "" {
		t.Errorf("expected empty trigger for Direct Encoder, got %q", direct.Trigger())
	}

	// 4. Test Suggest
	suggestions, err := enc.Suggest("test", 4)
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].Value != "cmd1" || suggestions[0].Description != "first command" {
		t.Errorf("unexpected suggestion[0]: %+v", suggestions[0])
	}

	// 5. Test Encode
	res, err := enc.Encode("hello")
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	if !strings.Contains(res.Payload, "RAW_hello") {
		t.Errorf("expected payload to contain RAW_hello, got %q", res.Payload)
	}
	if !strings.Contains(res.Display, "Sent hello") {
		t.Errorf("expected display to contain 'Sent hello', got %q", res.Display)
	}
}

func TestWorker_PreviewAndDescriptionUnmarshal(t *testing.T) {
	tmpDir := t.TempDir()
	workerScript := filepath.Join(tmpDir, "preview_worker.sh")
	scriptContent := `#!/usr/bin/env sh
while read -r line; do
  action=$(echo "$line" | grep -o '"action":"[^"]*"' | cut -d':' -f2 | tr -d '"')
  if [ "$action" = "suggest" ]; then
    echo '{"preview":"decoded preview (status: ok)","suggestions":[{"value":"token1","description":"Full description tag"},{"value":"token2","desc":"Legacy desc tag"}]}'
  else
    echo '{"error":"unknown"}'
  fi
done
`
	if err := os.WriteFile(workerScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("failed to write worker script: %v", err)
	}

	w, err := NewWorkerEncoder(Config{
		Name: "TestPreview",
		Exec: "sh preview_worker.sh",
		ExtraPaths: []string{tmpDir},
	})
	if err != nil {
		t.Fatalf("failed to create worker: %v", err)
	}
	defer func() {
		_ = w.Close()
	}()

	suggs, err := w.Suggest("query", 5)
	if err != nil {
		t.Fatalf("Suggest failed: %v", err)
	}
	if len(suggs) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggs))
	}
	if suggs[0].Description != "Full description tag" {
		t.Errorf("expected 'Full description tag' from 'description' json tag, got %q", suggs[0].Description)
	}
	if suggs[1].Description != "Legacy desc tag" {
		t.Errorf("expected 'Legacy desc tag' from 'desc' json tag, got %q", suggs[1].Description)
	}

	if prev := w.LastPreview(); prev != "decoded preview (status: ok)" {
		t.Errorf("expected preview 'decoded preview (status: ok)', got %q", prev)
	}
}
