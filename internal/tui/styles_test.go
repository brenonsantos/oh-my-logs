package tui

import (
	"testing"

	"github.com/brenoniehues/oh-my-logs/internal/record"
	"github.com/brenoniehues/oh-my-logs/internal/timing"
)

func TestTheme_CuratedPalettes(t *testing.T) {
	themes := AvailableThemes()
	expected := []string{
		"Dark Slate",
		"Monokai",
		"Nord",
		"Gruvbox",
		"Tokyo Night",
		"High Contrast",
	}

	if len(themes) != len(expected) {
		t.Fatalf("expected %d themes, got %d: %v", len(expected), len(themes), themes)
	}

	for _, name := range expected {
		p, ok := FindPalette(name)
		if !ok {
			t.Errorf("expected palette %q to be found", name)
		}
		if p.Name != name {
			t.Errorf("expected palette name %q, got %q", name, p.Name)
		}
		if p.Bg == "" || p.Fg == "" || p.Accent == "" {
			t.Errorf("palette %q has empty essential colors", name)
		}

		// Verify BuildTheme produces valid styles
		th := BuildTheme(p)
		if th.Name != name {
			t.Errorf("expected theme name %q, got %q", name, th.Name)
		}
		if th.Primary.GetForeground() == nil {
			t.Errorf("theme %q has nil primary foreground", name)
		}
	}
}

func TestTheme_SetCurrentTheme(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")

	// 1. Exact name
	matched := SetCurrentTheme("Monokai")
	if matched != "Monokai" {
		t.Errorf("expected matched name 'Monokai', got %q", matched)
	}
	if CurrentThemeName() != "Monokai" {
		t.Errorf("expected CurrentThemeName 'Monokai', got %q", CurrentThemeName())
	}
	if theme.Name != "Monokai" {
		t.Errorf("expected active theme 'Monokai', got %q", theme.Name)
	}

	// 2. Slug / lowercase
	matched = SetCurrentTheme("tokyo-night")
	if matched != "Tokyo Night" {
		t.Errorf("expected 'Tokyo Night' for slug 'tokyo-night', got %q", matched)
	}
	if CurrentThemeName() != "Tokyo Night" {
		t.Errorf("expected 'Tokyo Night', got %q", CurrentThemeName())
	}

	// 3. Fallback on unknown
	matched = SetCurrentTheme("NonExistentThemeXYZ")
	if matched != "Dark Slate" {
		t.Errorf("expected fallback to 'Dark Slate', got %q", matched)
	}
	if CurrentThemeName() != "Dark Slate" {
		t.Errorf("expected CurrentThemeName 'Dark Slate', got %q", CurrentThemeName())
	}
}

func TestTheme_StylesAndLevels(t *testing.T) {
	defer SetCurrentTheme("Dark Slate")
	SetCurrentTheme("Gruvbox")

	th := theme

	// LevelStyle
	errStyle := th.LevelStyle("ERROR")
	if errStyle.GetForeground() == nil {
		t.Errorf("expected ERROR style to have foreground")
	}

	warnStyle := th.LevelStyle("WARN")
	if warnStyle.GetForeground() == nil {
		t.Errorf("expected WARN style to have foreground")
	}

	infoStyle := th.LevelStyle("INFO")
	if infoStyle.GetForeground() == nil {
		t.Errorf("expected INFO style to have foreground")
	}

	txStyle := th.LevelStyle("TX")
	if txStyle.GetForeground() == nil {
		t.Errorf("expected TX style to have foreground")
	}

	// DeltaStyle
	deltaBurst := th.DeltaStyle(timing.LevelBurst)
	if deltaBurst.GetForeground() == nil {
		t.Errorf("expected burst delta style to have foreground")
	}

	deltaAlert := th.DeltaStyle(timing.LevelAlert)
	if deltaAlert.GetForeground() == nil {
		t.Errorf("expected alert delta style to have foreground")
	}

	// ResolveCellStyle
	col := record.Column{Field: "level", Style: "level"}
	cellStyle := th.ResolveCellStyle(col, "ERROR")
	if cellStyle.GetForeground() == nil {
		t.Errorf("expected cellStyle for level to have foreground")
	}

	colCustom := record.Column{
		Field:  "status",
		Colors: map[string]string{"OK": "green", "FAIL": "red"},
	}
	cellOK := th.ResolveCellStyle(colCustom, "OK")
	if cellOK.GetForeground() == nil {
		t.Errorf("expected cellOK to have foreground")
	}
}
