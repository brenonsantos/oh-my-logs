package record

import (
	"testing"
	"time"
)

func TestDecodedMessage(t *testing.T) {
	// Standard unmodified record
	r1 := NewRecord("raw serial line")
	if _, ok := r1.DecodedMessage(); ok {
		t.Fatalf("expected r1 to not be decoded")
	}

	// Record with message parsed, but no decoder transformation
	r2 := NewRecord("raw serial line")
	r2.Fields["message"] = "candidate payload"
	if _, ok := r2.DecodedMessage(); ok {
		t.Fatalf("expected r2 to not be decoded")
	}

	// Record where _raw_message matches message (no transformation)
	r3 := NewRecord("raw serial line")
	r3.Fields["_raw_message"] = "candidate payload"
	r3.Fields["message"] = "candidate payload"
	if _, ok := r3.DecodedMessage(); ok {
		t.Fatalf("expected r3 to not be decoded when message equals _raw_message")
	}

	// Record transformed by decoder
	r4 := NewRecord("10:00:00 [INF] 01 02 03")
	r4.Fields["_raw_message"] = "01 02 03"
	r4.Fields["message"] = "Sensor reading: temp=23.5C"
	r4.Timestamp = time.Now()
	msg, ok := r4.DecodedMessage()
	if !ok {
		t.Fatalf("expected r4 to be recognized as decoded")
	}
	if msg != "Sensor reading: temp=23.5C" {
		t.Fatalf("expected decoded message 'Sensor reading: temp=23.5C', got %q", msg)
	}

	// Marker record
	r5 := NewMarkerRecord("milestone", time.Now())
	if _, ok := r5.DecodedMessage(); ok {
		t.Fatalf("expected marker record to not be decoded")
	}
}
