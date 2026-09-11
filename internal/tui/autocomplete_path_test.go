package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oml-autocomplete-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory tree
	_ = os.MkdirAll(filepath.Join(tempDir, "documents", "reports"), 0o755)
	_ = os.MkdirAll(filepath.Join(tempDir, "documents", "receipts"), 0o755)
	_ = os.MkdirAll(filepath.Join(tempDir, "downloads"), 0o755)
	_ = os.WriteFile(filepath.Join(tempDir, "notes.txt"), []byte("hi"), 0o644)

	// 1. Single match autocomplete
	res := CompletePath(filepath.Join(tempDir, "down"), "")
	if !res.IsExactSingle {
		t.Fatalf("expected single exact match for 'down', got matches: %v", res.Matches)
	}
	expected := filepath.Join(tempDir, "downloads") + string(filepath.Separator)
	if res.Completed != expected {
		t.Fatalf("expected completed %q, got %q", expected, res.Completed)
	}

	// 2. Ambiguous match with common prefix
	res = CompletePath(filepath.Join(tempDir, "d"), "")
	if len(res.Matches) != 2 {
		t.Fatalf("expected 2 matches for 'd', got %v", res.Matches)
	}
	if !strings.HasPrefix(res.CommonPrefix, "do") {
		t.Fatalf("expected common prefix starting with 'do', got %q", res.CommonPrefix)
	}

	// 3. Ambiguous match without common prefix extension
	res = CompletePath(filepath.Join(tempDir, "doc"), "")
	if !res.IsExactSingle {
		t.Fatalf("expected single exact match for 'doc' ('documents/'), got %v", res.Matches)
	}
	expected = filepath.Join(tempDir, "documents") + string(filepath.Separator)
	if res.Completed != expected {
		t.Fatalf("expected %q, got %q", expected, res.Completed)
	}

	// 4. Subdirectory completion inside documents
	res = CompletePath(filepath.Join(tempDir, "documents", "rep"), "")
	if !res.IsExactSingle {
		t.Fatalf("expected single match for 'rep', got %v", res.Matches)
	}
	expected = filepath.Join(tempDir, "documents", "reports") + string(filepath.Separator)
	if res.Completed != expected {
		t.Fatalf("expected %q, got %q", expected, res.Completed)
	}

	// 5. Tilde expansion
	res = CompletePath("~", "")
	if res.Completed != "~/" {
		t.Fatalf("expected '~/' for '~', got %q", res.Completed)
	}
}
