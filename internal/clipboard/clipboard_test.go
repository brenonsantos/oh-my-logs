package clipboard

import (
	"testing"
)

func TestClipboardCopy(t *testing.T) {
	// Should not panic or error
	err := Copy("hello from oml test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
