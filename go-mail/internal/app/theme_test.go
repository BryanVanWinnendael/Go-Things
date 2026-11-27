package app

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAllThemesRender builds the mailbox with every theme and accent. A bad
// colour or a missing preset would panic or fall back to an unreadable scheme.
func TestAllThemesRender(t *testing.T) {
	a := testApp()
	for _, p := range themePresets {
		a.themeSettings.Theme = p.ID
		a.themeSettings.Accent = ""
		ui.NewTester(a.view, 980, 620)
	}
	for _, acc := range accentPresets {
		a.themeSettings.Theme = "nocturne"
		a.themeSettings.Accent = acc.ID
		ui.NewTester(a.view, 980, 620)
	}
	a.themeSettings.Accent = "#ff00aa"
	ui.NewTester(a.view, 980, 620)
}

// TestDefaultThemes checks the shipped dark and light palettes: the dark one
// uses the near-black window background and both use the same cool accent,
// including when System follows the desktop.
func TestDefaultThemes(t *testing.T) {
	dark := resolvePalette(ThemeSettings{Theme: "nocturne"}, true, nil)
	if got := hexColor(dark.Bg); got != "#0b0b0b" {
		t.Fatalf("dark background = %s, want #0b0b0b", got)
	}
	for _, id := range []string{"nocturne", "daylight"} {
		if got := hexColor(resolvePalette(ThemeSettings{Theme: id}, true, nil).Accent); got != "#2b3b79" {
			t.Fatalf("%s accent = %s, want #2b3b79", id, got)
		}
	}
	if got := hexColor(resolvePalette(ThemeSettings{Theme: "system"}, true, nil).Accent); got != "#2b3b79" {
		t.Fatalf("system dark accent = %s, want #2b3b79", got)
	}
}

// TestThemeFallbacks covers unknown ids from old config files.
func TestThemeFallbacks(t *testing.T) {
	if themeByID("does-not-exist").ID != themePresets[0].ID {
		t.Fatal("unknown theme should fall back to the first preset")
	}
	if themeExists("does-not-exist") {
		t.Fatal("unknown theme reported as existing")
	}
	if accentHex("not-an-accent") != "" {
		t.Fatal("unknown accent should resolve to empty")
	}
	if accentHex("#123456") != "#123456" {
		t.Fatal("hex accents should pass through")
	}
}

// TestCopyTheme duplicates built-in and custom themes with fresh ids and
// selects the copy.
func TestCopyTheme(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)

	a.copyTheme("nocturne")
	if len(a.customThemes) != 1 {
		t.Fatalf("got %d custom themes after copy, want 1", len(a.customThemes))
	}
	cp := a.customThemes[0]
	if cp.Base != "nocturne" || cp.Name != "Dark copy" {
		t.Fatalf("unexpected copy: %+v", cp)
	}
	if a.themeSettings.Theme != cp.ID {
		t.Fatal("copy was not selected")
	}

	a.copyTheme(cp.ID)
	if len(a.customThemes) != 2 {
		t.Fatalf("got %d custom themes after second copy, want 2", len(a.customThemes))
	}
	if a.customThemes[1].ID == cp.ID {
		t.Fatal("copy kept the same id")
	}
	if a.customThemes[1].Name != "Dark copy 2" {
		t.Fatalf("got name %q", a.customThemes[1].Name)
	}
}

// TestThemePickerPresets checks that choosing a gradient preset fills the draft
// and that the selected ring follows the draft once it is tuned.
func TestThemePickerPresets(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	a.openThemePicker(nil, "card")
	a.pendingThemePicker = false

	gp := gradientPresets[1]
	a.applyGradientPreset(gp)
	if !a.pickerPresetSelected(gp) {
		t.Fatal("preset was not selected after applying it")
	}
	want := presetThemePoints(gp)
	got := a.themePickerDraft.GradientPoints
	if len(got) != len(want) {
		t.Fatalf("got %d points, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Color != want[i].Color || got[i].X != want[i].X || got[i].Y != want[i].Y {
			t.Fatalf("point %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	a.nudgePickerPoint(0.05, 0.05)
	if a.pickerPresetSelected(gp) {
		t.Fatal("tuned draft should no longer read as the untouched preset")
	}
}

// Tooltips and toasts use the theme's Inverse colours. They must stay inside
// the palette: using p.Text there is a near-white popup in every dark theme.
func TestInverseSurfacesFollowTheme(t *testing.T) {
	for _, id := range []string{"nocturne", "daylight", "dusk"} {
		id := id
		t.Run(id, func(t *testing.T) {
			p := themeByID(id)
			var th *ui.Theme
			ui.NewTester(func(c *ui.Context) {
				th = applyTheme(c, p, ThemeSettings{FontSize: 14})
			}, 80, 80).Frame()
			if th.Inverse == p.Text || th.InverseText == p.Bg {
				t.Fatal("inverse surfaces still use the raw text/background colors")
			}
			if ratio := contrastRatio(th.Inverse, th.InverseText); ratio < 4.5 {
				t.Fatalf("tooltip contrast is %.2f, want at least 4.5", ratio)
			}
		})
	}
}

// TestPickerColorRoundTripsThroughHSV checks that the color square edits the
// active color without losing its exact shade.
func TestPickerColorRoundTripsThroughHSV(t *testing.T) {
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	a.themePickerTarget = "gradient"
	a.themePickerDraft = CustomTheme{
		Base:           "nocturne",
		Gradient:       true,
		GradientPoints: []CustomGradientPoint{{Color: "#6d5dfc", X: 0.4, Y: 0.5}},
	}
	a.themePickerPoint = 0
	h, s, v := rgbToHSV(a.pickerActiveColor())
	back := hsvColor(h, s, v)
	if got := hexColor(back); got != "#6d5dfc" {
		t.Fatalf("hsv round trip gave %s, want #6d5dfc", got)
	}
	a.setPickerActiveColor(hsvColor(0, 1, 1))
	if got := a.themePickerDraft.GradientPoints[0].Color; got != "#ff0000" {
		t.Fatalf("red square corner wrote %s, want #ff0000", got)
	}
}

// TestPickerColorSquareSelectsColor drives the shared color field: clicking the
// top-right corner picks a bright color for the active theme part.
func TestPickerColorSquareSelectsColor(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	tt := ui.NewTester(a.settingsWindowView, 920, 1100)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	a.pendingThemePicker = false

	pt := ui.NewTester(a.themePickerWindowView, 430, 900)
	pt.Frame()
	pt.Frame()
	square, ok := pt.Find("Color square")
	if !ok {
		t.Fatal("color square not found")
	}
	before := a.themePickerDraft.GradientPoints[0].Color
	x := square.X + square.W*0.92
	y := square.Y + square.H*0.08
	pt.Press(x, y)
	pt.Frame()
	pt.Frame()
	pt.Release(x, y)
	got := a.themePickerDraft.GradientPoints[0].Color
	if got == before {
		t.Fatalf("color square did not change the color: %s", got)
	}
	if col, ok := parseThemeColor(got); !ok || max(col.R, max(col.G, col.B)) < 200 {
		t.Fatalf("top-right of the square should be a bright color, got %s", got)
	}
}

// TestPickerSegmentedTargetsAndMeshPositions checks the one-picker layout:
// Background, Accent and Main view each edit their own part of the theme, and
// dragging a dot on the mesh only moves it instead of recoloring it.
func TestPickerSegmentedTargetsAndMeshPositions(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	tt := ui.NewTester(a.settingsWindowView, 920, 1100)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	a.pendingThemePicker = false

	a.themePickerDraft.GradientPoints = []CustomGradientPoint{{Color: "#6d5dfc", X: 0.5, Y: 0.5}}
	a.themePickerPoint = 0
	a.syncPickerInputs()

	pt := ui.NewTester(a.themePickerWindowView, 430, 900)
	pt.Frame()
	pt.Frame()

	// Accent: the shared color field writes the accent, not the mesh point.
	if err := pt.Click("Accent"); err != nil {
		t.Fatal(err)
	}
	pt.Frame()
	if a.themePickerTarget != "accent" {
		t.Fatalf("target is %q, want accent", a.themePickerTarget)
	}
	a.setPickerActiveColor(ui.Hex("#ff0000"))
	if a.themePickerDraft.Accent != "#ff0000" {
		t.Fatalf("accent is %q, want #ff0000", a.themePickerDraft.Accent)
	}
	if a.themePickerDraft.GradientPoints[0].Color != "#6d5dfc" {
		t.Fatal("editing the accent changed the background point")
	}

	// Main view: any color is accepted, and the mesh is left alone.
	if err := pt.Click("Main view"); err != nil {
		t.Fatal(err)
	}
	pt.Frame()
	if a.themePickerTarget != "surface" {
		t.Fatalf("target is %q, want surface", a.themePickerTarget)
	}
	a.applyPickerSurface("#101010")
	if a.themePickerDraft.Surface != "#101010" {
		t.Fatalf("surface is %q, want #101010", a.themePickerDraft.Surface)
	}

	// Background: dragging a dot moves it and keeps its color.
	if err := pt.Click("Background"); err != nil {
		t.Fatal(err)
	}
	pt.Frame()
	pt.Frame()
	canvas, ok := pt.Find("Background color canvas")
	if !ok {
		t.Fatal("background canvas not found")
	}
	fromX := canvas.X + 0.5*canvas.W
	fromY := canvas.Y + 0.5*canvas.H
	toX := canvas.X + canvas.W*0.72
	toY := canvas.Y + canvas.H*0.68
	pt.Press(fromX, fromY)
	pt.Move(toX, toY)
	pt.Frame()
	pt.Release(toX, toY)
	pt.Frame()
	pt.Frame()

	got := a.themePickerDraft.GradientPoints[0]
	if math.Abs(float64(got.X-0.72)) > 0.04 || math.Abs(float64(got.Y-0.68)) > 0.04 {
		t.Fatalf("dragged point is at %.2f, %.2f, want near 0.72, 0.68", got.X, got.Y)
	}
	if got.Color != "#6d5dfc" {
		t.Fatalf("dragging recolored the point to %s, want #6d5dfc", got.Color)
	}
}

// TestCustomThemeRoundTrip creates a custom theme through the editor's own
// state, edits it, and checks the palette resolver picks it up.
func TestCustomThemeRoundTrip(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	a.openThemePicker(nil, "card")
	a.pendingThemePicker = false
	ui.NewTester(a.themePickerWindowView, 430, 820)
	if len(a.customThemes) != 1 {
		t.Fatalf("got %d custom themes, want 1", len(a.customThemes))
	}
	ct := a.customThemes[0]
	if a.themeSettings.Theme != ct.ID {
		t.Fatal("new theme was not selected")
	}
	a.themePickerDraft.GradientPoints[0].Color = "#ff0000"
	a.pickerChanged()
	if a.customThemes[0].GradientPoints[0].Color != "#ff0000" {
		t.Fatal("edit did not reach the stored theme")
	}
	a.addPickerPoint()
	a.addPickerPoint()
	a.addPickerPoint()
	if got := len(a.themePickerDraft.GradientPoints); got != maxThemeGradientPoints {
		t.Fatalf("got %d points, want %d", got, maxThemeGradientPoints)
	}
	a.removePickerPoint()
	if got := len(a.themePickerDraft.GradientPoints); got != maxThemeGradientPoints-1 {
		t.Fatalf("got %d points after remove", got)
	}
	p := resolvePalette(a.themeSettings, true, a.customThemes)
	if p.ID != ct.ID {
		t.Fatalf("resolved %q, want custom %q", p.ID, ct.ID)
	}
	a.deleteCustomTheme(ct.ID)
	if customThemeByID(a.customThemes, ct.ID) != nil {
		t.Fatal("delete left the theme behind")
	}
	if a.themeSettings.Theme != "system" {
		t.Fatal("deleting the active theme should fall back to System")
	}
}

// TestThemeContextMenuEditsAndConfirmsDelete checks the appearance page's
// right-click menu: Edit opens the picker for the theme, and Delete asks for
// confirmation before removing it.
func TestThemeContextMenuEditsAndConfirmsDelete(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	a.customThemes = []CustomTheme{{ID: "theme_del", Name: "Deletable", Base: "nocturne"}}
	a.themeSettings.Theme = "theme_del"
	tt := ui.NewTester(a.settingsWindowView, 920, 1100)
	tt.Frame()
	tt.Frame()
	if tt.HasText("Selected:") {
		t.Fatal("settings still shows the selected-theme row")
	}

	if err := tt.RightClick("Deletable"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Edit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.themePickerOpen || a.themePickerAnchor != "edit" {
		t.Fatal("Edit did not open the theme picker")
	}
	if a.themePickerDraft.ID != "theme_del" {
		t.Fatalf("draft is %q, want theme_del", a.themePickerDraft.ID)
	}
	a.themePickerOpen = false
	a.pendingThemePicker = false

	if err := tt.RightClick("Deletable"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.confirmFn == nil {
		t.Fatal("delete did not ask for confirmation")
	}
	if len(a.customThemes) != 1 {
		t.Fatal("theme was deleted before confirming")
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if len(a.customThemes) != 0 {
		t.Fatalf("theme was not deleted: %d left", len(a.customThemes))
	}
	if a.confirmFn != nil {
		t.Fatal("confirm dialog stayed open")
	}
}
