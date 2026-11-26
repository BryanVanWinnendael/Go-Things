package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestColorPickerStaysInside checks that the signature editor's color picker
// opens to the left of the swatch instead of poking out of the right edge.
func TestColorPickerStaysInside(t *testing.T) {
	a := &App{dark: true, systemDark: true, settingsTab: "mail"}
	a.palette = resolvePalette(ThemeSettings{Theme: "nocturne", FontSize: 14, Radius: 9}, true, nil)
	a.acct = Account{Name: "Alex Doe", Email: "alex@example.com"}
	a.accounts = []Account{a.acct}
	a.active = 0

	const width = float32(920)
	tt := ui.NewTester(a.settingsWindowView, int(width), 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Text color"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	panel, ok := tt.Find("Color picker")
	if !ok {
		t.Fatalf("color picker did not open; texts %q", tt.Texts())
	}
	well, ok := tt.Find("Text color")
	if !ok {
		t.Fatal("no color swatch")
	}
	// The panel's right edge should line up with the swatch's, so the picker
	// expands leftward inside the toolbar instead of past its right edge.
	if right := panel.X + panel.W; right > well.X+well.W+8 {
		t.Fatalf("color picker sticks out to the right of the swatch: panel right %v, swatch right %v", right, well.X+well.W)
	}
	if panel.X < 0 {
		t.Fatalf("color picker sticks out to the left: %v", panel)
	}
}
