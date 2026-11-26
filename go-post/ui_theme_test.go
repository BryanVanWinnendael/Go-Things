package main

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestThemePresetsAreDistinctAndReadable(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range themePresets {
		if p.ID == "" || p.Name == "" {
			t.Fatalf("theme %+v has no id or name", p)
		}
		if seen[p.ID] {
			t.Fatalf("duplicate theme id %q", p.ID)
		}
		seen[p.ID] = true
		if got := colorContrast(p.Text, p.Bg); got < 7 {
			t.Errorf("%s text contrast is %.2f, want at least 7", p.Name, got)
		}
		if got := colorContrast(p.Muted, p.Bg); got < 4.5 {
			t.Errorf("%s muted contrast is %.2f, want at least 4.5", p.Name, got)
		}
	}
}

func TestThemeCatalogIsCurated(t *testing.T) {
	want := []string{"nocturne", "daylight", "slate", "cocoa", "linen", "dusk", "tide", "bloom"}
	if len(themePresets) != len(want) {
		t.Fatalf("found %d themes, want %d", len(themePresets), len(want))
	}
	for i, id := range want {
		if themePresets[i].ID != id {
			t.Errorf("theme %d is %q, want %q", i, themePresets[i].ID, id)
		}
	}
	gradients := 0
	for _, p := range themePresets {
		if p.HasGradient {
			gradients++
		}
	}
	if gradients != 3 {
		t.Fatalf("found %d gradient themes, want 3", gradients)
	}
}

// A theme belongs to the workspace it was chosen in. Switching workspace must
// swap the whole look, and each choice must survive a save and reload.
func TestThemeAndMaterialAreSavedPerWorkspace(t *testing.T) {
	a := testApp(t)
	personal := a.data.Workspaces[0].ID
	acme := a.data.Workspaces[1].ID

	a.switchWorkspace(personal)
	a.setTheme("daylight")
	a.setMaterial("acrylic")
	if ws := a.data.workspace(personal); ws.Theme != "daylight" || ws.Material != "acrylic" {
		t.Fatalf("personal workspace is %q/%q, want daylight/acrylic", ws.Theme, ws.Material)
	}

	a.switchWorkspace(acme)
	if a.data.Settings.Theme == "daylight" || a.data.Settings.Material == "acrylic" {
		t.Fatal("switching workspace carried the previous workspace's look over")
	}
	if ws := a.data.workspace(acme); a.data.Settings.Theme != ws.Theme {
		t.Fatalf("settings theme is %q, want the active workspace's %q", a.data.Settings.Theme, ws.Theme)
	}
	a.setTheme("slate")
	a.setMaterial("mica")

	a.switchWorkspace(personal)
	if a.data.Settings.Theme != "daylight" || a.data.Settings.Material != "acrylic" {
		t.Fatalf("returning to personal restored %q/%q, want daylight/acrylic", a.data.Settings.Theme, a.data.Settings.Material)
	}

	if err := a.store.Save(a.data); err != nil {
		t.Fatal(err)
	}
	loaded, err := a.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string][2]string{
		personal: {"daylight", "acrylic"},
		acme:     {"slate", "mica"},
	} {
		ws := loaded.workspace(id)
		if ws.Theme != want[0] || ws.Material != want[1] {
			t.Fatalf("%s reloaded as %q/%q, want %q/%q", ws.Name, ws.Theme, ws.Material, want[0], want[1])
		}
	}
}

// The custom theme library is shared, so deleting a theme has to clear it from
// every workspace that picked it, not only the active one.
func TestDeletingThemeClearsItFromEveryWorkspace(t *testing.T) {
	a := testApp(t)
	a.data.CustomThemes = append(a.data.CustomThemes, CustomTheme{
		ID: "theme_x", Name: "X", Base: "nocturne", Background: "#101010",
	})
	for i := range a.data.Workspaces {
		a.data.Workspaces[i].Theme = "theme_x"
	}
	a.data.Settings.Theme = "theme_x"

	a.deleteCustomTheme("theme_x")
	for _, ws := range a.data.Workspaces {
		if ws.Theme != "system" {
			t.Fatalf("%s kept deleted theme %q", ws.Name, ws.Theme)
		}
	}
	if a.data.Settings.Theme != "system" {
		t.Fatalf("active theme is %q after deleting, want system", a.data.Settings.Theme)
	}
}

// Older files kept one global theme. Loading one must hand that look to every
// workspace before the per-workspace choice takes over.
func TestLoadGivesEveryWorkspaceTheOldGlobalTheme(t *testing.T) {
	dir := t.TempDir()
	st := &Store{dir: dir, file: filepath.Join(dir, "gopost.json"), fallback: map[string]string{}}
	d := defaultData()
	d.Settings.Theme = "slate"
	d.Settings.Material = "mica"
	for i := range d.Workspaces {
		d.Workspaces[i].Theme = ""
		d.Workspaces[i].Material = ""
	}
	if err := st.Save(d); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, ws := range loaded.Workspaces {
		if ws.Theme != "slate" || ws.Material != "mica" {
			t.Fatalf("%s loaded as %q/%q, want slate/mica", ws.Name, ws.Theme, ws.Material)
		}
	}
	active := loaded.workspace(loaded.ActiveWorkspace)
	if loaded.Settings.Theme != active.Theme || loaded.Settings.Material != active.Material {
		t.Fatal("settings do not mirror the active workspace after migration")
	}
}

func TestNewWorkspaceInheritsCurrentLook(t *testing.T) {
	a := testApp(t)
	a.setTheme("tide")
	a.setMaterial("acrylic")
	a.newWorkspace("Docs")
	if ws := a.ws(); ws.Name != "Docs" || ws.Theme != "tide" || ws.Material != "acrylic" {
		t.Fatalf("new workspace is %q/%q, want the current tide/acrylic", ws.Theme, ws.Material)
	}
}

// Deleting the active workspace must put the replacement's look on screen,
// not leave the deleted workspace's theme behind.
func TestDeletingActiveWorkspaceAdoptsTheNextLook(t *testing.T) {
	a := testApp(t)
	first := a.data.Workspaces[0].ID
	second := a.data.Workspaces[1].ID
	a.switchWorkspace(first)
	a.setTheme("daylight")
	a.switchWorkspace(second)
	a.setTheme("slate")

	a.deleteWorkspace(second)
	if a.data.ActiveWorkspace != first {
		t.Fatalf("active workspace is %q, want %q", a.data.ActiveWorkspace, first)
	}
	if a.data.Settings.Theme != "daylight" {
		t.Fatalf("theme after delete is %q, want the replacement's daylight", a.data.Settings.Theme)
	}
}

func TestCopyingGradientThemeKeepsItsColors(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	base := themeByID("tide")
	if !base.HasGradient {
		t.Fatal("tide preset has no gradient")
	}

	a.copyTheme("tide")
	if len(a.data.CustomThemes) != 1 {
		t.Fatalf("copied %d themes, want 1", len(a.data.CustomThemes))
	}
	cp := a.data.CustomThemes[0]
	// Copying a copy must keep the same mesh too.
	a.copyTheme(cp.ID)
	if len(a.data.CustomThemes) != 2 {
		t.Fatalf("copied %d themes, want 2", len(a.data.CustomThemes))
	}
	got := customPalette(a.data.CustomThemes[1])
	if !got.HasGradient {
		t.Fatal("copied theme lost its gradient")
	}
	if len(got.Gradient) != len(base.Gradient) {
		t.Fatalf("copy has %d gradient points, want %d", len(got.Gradient), len(base.Gradient))
	}
	for i, want := range base.Gradient {
		if got.Gradient[i].Color != want.Color {
			t.Errorf("point %d color is %v, want %v", i, got.Gradient[i].Color, want.Color)
		}
		if math.Abs(float64(got.Gradient[i].X-want.X)) > 0.001 || math.Abs(float64(got.Gradient[i].Y-want.Y)) > 0.001 {
			t.Errorf("point %d is at %.2f, %.2f, want %.2f, %.2f", i,
				got.Gradient[i].X, got.Gradient[i].Y, want.X, want.Y)
		}
	}
	if got.GradientAlpha != base.GradientAlpha {
		t.Errorf("copy alpha is %.2f, want %.2f", got.GradientAlpha, base.GradientAlpha)
	}

	// Opening the editor must not swap in a different mesh.
	a.openThemePicker(&a.data.CustomThemes[1], "edit")
	pt := ui.NewTester(a.themePickerWindowView, 430, 900)
	pt.Frame()
	pt.Frame()
	if len(a.themePickerDraft.GradientPoints) != len(base.Gradient) {
		t.Fatalf("editor has %d points, want %d", len(a.themePickerDraft.GradientPoints), len(base.Gradient))
	}
	for i, want := range base.Gradient {
		if got := a.themePickerDraft.GradientPoints[i].Color; got != hexColor(want.Color) {
			t.Errorf("editor point %d is %s, want %s", i, got, hexColor(want.Color))
		}
	}
}

func TestEditingSolidThemeKeepsItsSingleColor(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	base := themeByID("nocturne")
	want := hexColor(base.Bg)

	a.copyTheme("nocturne")
	a.openThemePicker(&a.data.CustomThemes[0], "edit")
	pt := ui.NewTester(a.themePickerWindowView, 430, 900)
	pt.Frame()
	pt.Frame()

	if len(a.themePickerDraft.GradientPoints) != 1 {
		t.Fatalf("editor has %d points, want 1", len(a.themePickerDraft.GradientPoints))
	}
	if got := a.themePickerDraft.GradientPoints[0].Color; got != want {
		t.Fatalf("single gradient color is %s, want the theme's %s", got, want)
	}
	if got := a.themePickerDraft.Background; got != want {
		t.Fatalf("background is %s, want the theme's %s", got, want)
	}
}

func TestVibrancyPalettePushesMutedTextAwayFromTheBackdrop(t *testing.T) {
	for _, p := range themePresets {
		v := vibrancyPalette(p)
		before, after := relativeLuminance(p.Muted), relativeLuminance(v.Muted)
		if p.Dark && after <= before {
			t.Errorf("%s muted color got darker under vibrancy", p.Name)
		}
		if !p.Dark && after >= before {
			t.Errorf("%s muted color got lighter under vibrancy", p.Name)
		}
	}
}

func TestCustomGradientKeepsChromeReadableAndStageSolid(t *testing.T) {
	ct := CustomTheme{
		ID: "theme_night_swim", Name: "Night swim", Base: "nocturne",
		Background: "#1a2a44", Gradient: true,
		GradientPoints: []CustomGradientPoint{
			{Color: "#6d5dfc", X: 0.18, Y: 0.24},
			{Color: "#39c6a4", X: 0.56, Y: 0.54},
			{Color: "#5ca8ff", X: 0.84, Y: 0.78},
		},
		GradientOpacity: 0.55,
	}
	p := customPalette(ct)
	p.Accent = accentColor("sky")
	p.Muted = readableMuted(p)
	if !p.HasGradient {
		t.Fatal("custom theme did not keep its gradient")
	}
	if len(p.Gradient) != 3 {
		t.Fatalf("custom theme has %d gradient points, want 3", len(p.Gradient))
	}
	if p.Panel != themeByID("nocturne").Panel {
		t.Fatal("custom theme changed the solid workspace panel")
	}
	for _, bg := range gradientBackdrops(p) {
		if got := colorContrast(p.Text, bg); got < 4.5 {
			t.Errorf("chrome text contrast is %.2f, want at least 4.5", got)
		}
		if got := colorContrast(p.Muted, bg); got < 4.5 {
			t.Errorf("chrome muted contrast is %.2f, want at least 4.5", got)
		}
	}
}

func TestLegacyTwoColorCustomThemeStillLoads(t *testing.T) {
	p := customPalette(CustomTheme{
		ID: "theme_old", Name: "Old", Base: "nocturne", Gradient: true,
		From: "#6d5dfc", To: "#39c6a4", Angle: 135,
	})
	if !p.HasGradient {
		t.Fatal("legacy gradient did not load")
	}
	if len(p.Gradient) != 2 {
		t.Fatalf("legacy gradient has %d points, want 2", len(p.Gradient))
	}
	if p.Gradient[0].Color != ui.Hex("#6d5dfc") || p.Gradient[1].Color != ui.Hex("#39c6a4") {
		t.Fatal("legacy gradient colors changed")
	}
}

func TestCustomThemeCanTurnOffABaseGradient(t *testing.T) {
	p := customPalette(CustomTheme{ID: "theme_solid_tide", Name: "Solid tide", Base: "tide"})
	if p.HasGradient {
		t.Fatal("solid custom theme inherited the base palette's gradient")
	}
	if len(p.Gradient) != 0 {
		t.Fatal("solid custom theme kept the base palette's gradient points")
	}
}

func TestBuiltInGradientThemesKeepChromeReadable(t *testing.T) {
	gradients := 0
	for _, p := range themePresets {
		if !p.HasGradient {
			continue
		}
		gradients++
		if len(p.Gradient) != maxThemeGradientPoints {
			t.Errorf("%s gradient has %d points, want %d", p.Name, len(p.Gradient), maxThemeGradientPoints)
		}
		if p.GradientAlpha <= 0 {
			t.Errorf("%s gradient has no opacity", p.Name)
		}
		p.Accent = accentColor("sky")
		p.Muted = readableMuted(p)
		for _, bg := range gradientBackdrops(p) {
			if got := colorContrast(p.Text, bg); got < 4.5 {
				t.Errorf("%s chrome text contrast is %.2f, want at least 4.5", p.Name, got)
			}
			if got := colorContrast(p.Muted, bg); got < 4.5 {
				t.Errorf("%s chrome muted contrast is %.2f, want at least 4.5", p.Name, got)
			}
		}
	}
	if gradients < 3 {
		t.Fatalf("found %d built-in gradient themes, want at least 3", gradients)
	}
}

func TestCustomThemeRejectsUnreadableTint(t *testing.T) {
	// A white gradient point over a dark base leaves the chrome's white text
	// unreadable at full strength.
	ct := CustomTheme{
		Name: "Washed out", Base: "nocturne", Background: "#1a2a44",
		Gradient: true,
		GradientPoints: []CustomGradientPoint{
			{Color: "#ffffff", X: 0.50, Y: 0.50},
		},
		GradientOpacity: 1,
	}
	if problem := customThemeProblem(ct, accentColor("sky")); problem == "" {
		t.Fatal("unreadable gradient was accepted")
	}
	a := testApp(t)
	a.systemDark = true
	a.themePickerDraft = ct
	a.clampPickerReadability()
	if problem := customThemeProblem(a.themePickerDraft, accentColor("sky")); problem != "" {
		t.Fatalf("picker did not make the theme readable: %s", problem)
	}
	if a.themePickerDraft.GradientOpacity >= 1 {
		t.Fatal("picker did not lower the gradient opacity")
	}
}

// Tooltips and toasts use the theme's Inverse colours. They must stay inside
// the palette: using p.Text there is a near-white popup in every dark theme.
func TestInverseSurfacesFollowTheme(t *testing.T) {
	for _, id := range []string{"nocturne", "daylight", "tide"} {
		id := id
		t.Run(id, func(t *testing.T) {
			p := themeByID(id)
			var th *ui.Theme
			ui.NewTester(func(c *ui.Context) {
				th = applyTheme(c, p, Settings{FontSize: 14})
			}, 80, 80).Frame()
			if th.Inverse == p.Text || th.InverseText == p.Bg {
				t.Fatal("inverse surfaces still use the raw text/background colors")
			}
			if ratio := colorContrast(th.Inverse, th.InverseText); ratio < 4.5 {
				t.Fatalf("tooltip contrast is %.2f, want at least 4.5", ratio)
			}
		})
	}
}

func TestThemePickerCreatesAndSelectsTheme(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 360, 1100)
	pt.Frame()
	pt.Frame()
	if !a.themePickerOpen {
		t.Fatal("picker did not open")
	}
	if a.modal != "" {
		t.Fatalf("modal is %q, want empty", a.modal)
	}
	if len(a.data.CustomThemes) != 1 {
		t.Fatalf("created %d custom themes, want 1", len(a.data.CustomThemes))
	}
	id := a.data.CustomThemes[0].ID
	if a.data.Settings.Theme != id {
		t.Fatalf("selected theme is %q, want %q", a.data.Settings.Theme, id)
	}
	// A theme chosen in the editor belongs to the workspace, so it must be
	// recorded there and not only in the mirrored Settings.
	if ws := a.ws(); ws.Theme != id {
		t.Fatalf("workspace theme is %q, want %q", ws.Theme, id)
	}
	if got := len(a.data.CustomThemes[0].GradientPoints); got < minThemeGradientPoints || got > maxThemeGradientPoints {
		t.Fatalf("new theme has %d points, want %d to %d", got, minThemeGradientPoints, maxThemeGradientPoints)
	}
	if err := pt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.themePickerOpen {
		t.Fatal("picker stayed open after Done")
	}
	if len(a.data.CustomThemes) != 1 {
		t.Fatalf("picker changed the theme count to %d", len(a.data.CustomThemes))
	}
	if err := a.store.Save(a.data); err != nil {
		t.Fatal(err)
	}
	loaded, err := a.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if ws := loaded.workspace(loaded.ActiveWorkspace); ws.Theme != id {
		t.Fatalf("workspace theme reloaded as %q, want %q", ws.Theme, id)
	}
	if loaded.Settings.Theme != id {
		t.Fatalf("settings theme reloaded as %q, want %q", loaded.Settings.Theme, id)
	}
}

func TestThemePickerCanvasDragsColorPoints(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()

	pt := ui.NewTester(a.themePickerWindowView, 360, 1100)
	pt.Frame()
	pt.Frame()

	canvas, ok := pt.Find("Background color canvas")
	if !ok {
		t.Fatal("gradient canvas not found")
	}
	start := a.themePickerDraft.GradientPoints[0]
	fromX := canvas.X + start.X*canvas.W
	fromY := canvas.Y + start.Y*canvas.H
	toX := canvas.X + canvas.W*0.70
	toY := canvas.Y + canvas.H*0.72
	pt.Press(fromX, fromY)
	pt.Move(toX, toY)
	pt.Frame()
	pt.Release(toX, toY)
	pt.Frame()
	pt.Frame()

	got := a.themePickerDraft.GradientPoints[0]
	if math.Abs(float64(got.X-0.70)) > 0.04 || math.Abs(float64(got.Y-0.72)) > 0.04 {
		t.Fatalf("dragged point is at %.2f, %.2f, want near 0.70, 0.72", got.X, got.Y)
	}
	if a.themePickerPoint != 0 {
		t.Fatalf("active point is %d, want 0", a.themePickerPoint)
	}
	if got.Color == "" {
		t.Fatal("dragged point lost its derived color")
	}
}

func TestThemePickerMainViewColor(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 360, 1100)
	pt.Frame()
	pt.Frame()

	a.themePickerSurface = ""
	a.applyPickerSurface("")
	pt.Frame()
	pt.Frame()
	if a.themePickerDraft.Surface != "" {
		t.Fatalf("empty surface was rewritten to %q", a.themePickerDraft.Surface)
	}
}

func TestThemePickerSurfaceAcceptsAnyColor(t *testing.T) {
	a := testApp(t)
	a.themePickerTarget = "surface"
	a.themePickerDraft = CustomTheme{Base: "nocturne"}
	a.setPickerActiveColor(ui.Hex("#a78bfa"))
	if got := a.themePickerDraft.Surface; got != "#a78bfa" {
		t.Fatalf("surface is %q, want #a78bfa", got)
	}
	// Even a color that leaves light text hard to read is allowed: the user
	// can switch the text color to black in the same editor.
	a.setPickerActiveColor(ui.Hex("#ffffff"))
	if got := a.themePickerDraft.Surface; got != "#ffffff" {
		t.Fatalf("surface is %q, want #ffffff", got)
	}
}

func TestThemePickerKeepsOneToThreeColors(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 360, 1100)
	pt.Frame()
	pt.Frame()
	start := len(a.themePickerDraft.GradientPoints)
	if start < minThemeGradientPoints || start > maxThemeGradientPoints {
		t.Fatalf("new theme has %d colors, want %d to %d", start, minThemeGradientPoints, maxThemeGradientPoints)
	}
	// Add colors up to max.
	for i := start; i < maxThemeGradientPoints; i++ {
		if err := pt.Click("Add color"); err != nil {
			t.Fatal(err)
		}
		pt.Frame()
		pt.Frame()
	}
	if got := len(a.themePickerDraft.GradientPoints); got != maxThemeGradientPoints {
		t.Fatalf("after adding colors the theme has %d, want %d", got, maxThemeGradientPoints)
	}
	_ = pt.Click("Add color")
	pt.Frame()
	if got := len(a.themePickerDraft.GradientPoints); got != maxThemeGradientPoints {
		t.Fatalf("added a fourth color: %d", got)
	}
	// Remove colors down to min.
	for i := maxThemeGradientPoints; i > minThemeGradientPoints; i-- {
		if err := pt.Click("Remove color"); err != nil {
			t.Fatal(err)
		}
		pt.Frame()
		pt.Frame()
	}
	if got := len(a.themePickerDraft.GradientPoints); got != minThemeGradientPoints {
		t.Fatalf("after removing colors the theme has %d, want %d", got, minThemeGradientPoints)
	}
	// The remove button is disabled at one color, so a further click is a no-op.
	_ = pt.Click("Remove color")
	pt.Frame()
	if got := len(a.themePickerDraft.GradientPoints); got != minThemeGradientPoints {
		t.Fatalf("removed the last color: %d", got)
	}
}

func TestThemePickerEditsHexAndRGBValues(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 360, 1100)
	pt.Frame()
	pt.Frame()

	if err := pt.Click("Color 1 value"); err != nil {
		t.Fatal(err)
	}
	pt.Command("selectAll")
	pt.Type("#ff0000")
	pt.Frame()
	pt.Frame()
	if got := a.themePickerDraft.GradientPoints[0].Color; got != "#ff0000" {
		t.Fatalf("color point is %q, want #ff0000", got)
	}

	if err := pt.Click("Main view"); err != nil {
		t.Fatal(err)
	}
	pt.Frame()
	pt.Frame()
	if err := pt.Click("Main view value"); err != nil {
		t.Fatal(err)
	}
	pt.Command("selectAll")
	pt.Type("rgb(20, 30, 40)")
	pt.Frame()
	pt.Frame()
	if got := a.themePickerDraft.Surface; got != "#141e28" {
		t.Fatalf("main view color is %q, want #141e28", got)
	}
	if got := a.palette.Panel; got != ui.Hex("#141e28") {
		t.Fatalf("palette panel is %v, want #141e28", got)
	}
}

func TestParseThemeColor(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"#6d5dfc", "#6d5dfc", true},
		{"6d5dfc", "#6d5dfc", true},
		{"#ABC", "#aabbcc", true},
		{"rgb(109, 93, 252)", "#6d5dfc", true},
		{"RGB(109,93,252)", "#6d5dfc", true},
		{"rgb(20 30 40)", "#141e28", true},
		{"rgb(256, 0, 0)", "", false},
		{"#12345", "", false},
		{"not a color", "", false},
	}
	for _, tc := range tests {
		got, ok := parseThemeColor(tc.in)
		if ok != tc.ok {
			t.Fatalf("parseThemeColor(%q) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if ok && hexColor(got) != tc.want {
			t.Fatalf("parseThemeColor(%q) = %s, want %s", tc.in, hexColor(got), tc.want)
		}
	}
}

func TestCustomThemeSurfaceColorsTheWorkspace(t *testing.T) {
	base := themeByID("nocturne")
	ct := CustomTheme{ID: "theme_surface", Name: "Surface", Base: "nocturne", Surface: "#202a3a"}
	p := customPalette(ct)
	if p.Panel != ui.Hex("#202a3a") {
		t.Fatalf("workspace panel is %v, want #202a3a", p.Panel)
	}
	if p.Field == base.Field {
		t.Fatal("workspace fields did not follow the chosen panel")
	}

	plain := customPalette(CustomTheme{ID: "theme_plain", Name: "Plain", Base: "nocturne"})
	if plain.Panel != base.Panel {
		t.Fatal("a theme without a surface changed the base panel")
	}

	light := CustomTheme{ID: "theme_light", Name: "Light", Base: "nocturne", Surface: "#ffffff"}
	if got := customPalette(light).Panel; got != ui.Hex("#ffffff") {
		t.Fatalf("workspace panel is %v, want white", got)
	}
}

func TestThemePickerEditsExistingTheme(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	ct := CustomTheme{
		ID: "theme_edit", Name: "Editable", Base: "nocturne", Gradient: true,
		GradientPoints: []CustomGradientPoint{
			{Color: "#6d5dfc", X: 0.30, Y: 0.30},
			{Color: "#f2a65a", X: 0.70, Y: 0.70},
		},
		GradientOpacity: 0.55,
		GradientMode:    "auto",
	}
	a.data.CustomThemes = []CustomTheme{ct}
	a.setTheme(ct.ID)
	tt := ui.NewTester(a.settingsWindowView, 920, 1100)
	tt.Frame()
	tt.Frame()
	if tt.HasText("Selected:") {
		t.Fatal("settings still shows the selected-theme row")
	}
	if err := tt.RightClick("Editable"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Edit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 360, 1100)
	pt.Frame()
	pt.Frame()
	if !a.themePickerOpen {
		t.Fatal("picker did not open from Edit")
	}
	if a.themePickerAnchor != "edit" {
		t.Fatalf("anchor is %q, want edit", a.themePickerAnchor)
	}
	if a.themePickerDraft.ID != ct.ID {
		t.Fatalf("draft is %q, want %q", a.themePickerDraft.ID, ct.ID)
	}
	if got := len(a.themePickerDraft.GradientPoints); got != 2 {
		t.Fatalf("draft has %d points, want 2", got)
	}
	a.themePickerDraft.Name = "Renamed"
	a.pickerChanged()
	if err := pt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.themePickerOpen {
		t.Fatal("picker stayed open after Done")
	}
	if got := a.data.CustomThemes[0].Name; got != "Renamed" {
		t.Fatalf("saved name is %q, want Renamed", got)
	}
	if len(a.data.CustomThemes) != 1 {
		t.Fatalf("editing duplicated the theme: %d", len(a.data.CustomThemes))
	}
}

func TestGradientChromeIsSmooth(t *testing.T) {
	p := palette{
		Bg: ui.Hex("#0a0b0f"), HasGradient: true, GradientAlpha: 0.8,
		Gradient: []gradientPoint{
			{Color: ui.Hex("#ef4444"), X: 0.15, Y: 0.22},
			{Color: ui.Hex("#22c55e"), X: 0.50, Y: 0.50},
			{Color: ui.Hex("#3b82f6"), X: 0.85, Y: 0.78},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(r, p.Bg, 0)
			paintChromeGradient(pt, r, p, 1)
		})
	}, 240, 150)
	tt.Frame()
	img := tt.Image()
	maxDelta := 0
	for x := 1; x < 240; x++ {
		a, b := img.RGBAAt(x-1, 75), img.RGBAAt(x, 75)
		delta := 0
		for _, d := range []int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
			if d < 0 {
				d = -d
			}
			delta += d
		}
		if delta > maxDelta {
			maxDelta = delta
		}
	}
	if maxDelta > 30 {
		t.Fatalf("gradient has a hard edge: adjacent colors differ by %d", maxDelta)
	}
	if img.RGBAAt(20, 75) == img.RGBAAt(220, 75) {
		t.Fatal("gradient is a flat fill")
	}
}

func colorContrast(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func TestAccentAndThemeInteraction(t *testing.T) {
	dir := t.TempDir()
	st := &Store{dir: dir, file: filepath.Join(dir, "gopost.json"), fallback: map[string]string{}}
	a := newApp(st, defaultData())
	a.systemDark = true

	stt := ui.NewTester(a.settingsWindowView, 920, 760)
	stt.SetDark(true)
	stt.Frame()
	stt.Frame()
	// Accent is now per-theme; verify the settings view renders cleanly.
	if stt.FramePending() {
		t.Fatal("settings view keeps invalidating after load")
	}

	mt := ui.NewTester(a.view, 1280, 820)
	mt.SetDark(true)
	mt.Frame()
	mt.Frame()
	if mt.FramePending() {
		t.Fatal("main view keeps invalidating after an accent change")
	}

	if err := stt.Click("Light"); err != nil {
		t.Fatal(err)
	}
	stt.Frame()
	stt.Frame()
	if a.data.Settings.Theme != "daylight" {
		t.Fatalf("theme is %q, want daylight", a.data.Settings.Theme)
	}
	mt.Frame()
	mt.Frame()
	// Colors fade into the new theme, so let the transition finish before
	// checking that nothing keeps invalidating.
	mt.Advance(200 * time.Millisecond)
	mt.Frame()
	if mt.FramePending() {
		t.Fatal("main view keeps invalidating after a theme change")
	}
	// A light theme must not borrow the dark theme's hover fill.
	mt.Move(22, 124)
	mt.Frame()
	hc := mt.Image().At(22, 124)
	hr, hg, hb, _ := hc.RGBA()
	if hg < 0x3000 || hb < 0x4000 {
		t.Fatalf("hovered primary button in a light theme is %04x %04x %04x", hr, hg, hb)
	}
}

func TestCustomThemePersistsAndUpdates(t *testing.T) {
	a := testApp(t)
	ct := CustomTheme{
		ID: "theme_night_swim", Name: "Night swim", Base: "nocturne", Background: "#1a2a44",
		Gradient: true,
		GradientPoints: []CustomGradientPoint{
			{Color: "#6d5dfc", X: 0.18, Y: 0.24},
			{Color: "#39c6a4", X: 0.56, Y: 0.54},
			{Color: "#5ca8ff", X: 0.84, Y: 0.78},
		},
		GradientOpacity: 0.55,
	}
	a.data.CustomThemes = append(a.data.CustomThemes, ct)
	a.setTheme(ct.ID)
	a.save()
	if len(a.data.CustomThemes) != 1 {
		t.Fatalf("saved %d custom themes, want 1", len(a.data.CustomThemes))
	}
	loaded, err := a.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.CustomThemes) != 1 {
		t.Fatalf("loaded %d custom themes, want 1", len(loaded.CustomThemes))
	}
	if loaded.CustomThemes[0].ID != ct.ID || loaded.Settings.Theme != ct.ID {
		t.Fatal("custom theme or selection did not persist")
	}

	loaded.CustomThemes[0].Name = "Night swim 2"
	a.data = loaded
	a.save()
	again, err := a.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.CustomThemes) != 1 {
		t.Fatalf("editing created a second theme: %d", len(again.CustomThemes))
	}
	if again.CustomThemes[0].Name != "Night swim 2" || again.CustomThemes[0].ID != ct.ID {
		t.Fatal("custom theme was not updated in place")
	}
}

func TestDeletingSelectedCustomThemeFallsBackToSystem(t *testing.T) {
	a := testApp(t)
	a.data.CustomThemes = []CustomTheme{{ID: "theme_x", Name: "X", Base: "nocturne"}}
	a.setTheme("theme_x")
	a.deleteCustomTheme("theme_x")
	if len(a.data.CustomThemes) != 0 {
		t.Fatal("custom theme was not deleted")
	}
	if a.data.Settings.Theme != "system" {
		t.Fatalf("theme is %q, want system", a.data.Settings.Theme)
	}
}

func TestThemeContextMenuConfirmDelete(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	a.data.CustomThemes = []CustomTheme{{ID: "theme_del", Name: "Deletable", Base: "nocturne"}}
	a.setTheme("theme_del")
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
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
	if a.modal != "confirm" {
		t.Fatalf("modal is %q, want confirm", a.modal)
	}
	if len(a.data.CustomThemes) != 1 {
		t.Fatal("theme was deleted before confirming")
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if len(a.data.CustomThemes) != 0 {
		t.Fatalf("theme was not deleted: %d left", len(a.data.CustomThemes))
	}
	if a.modal != "" {
		t.Fatal("confirm modal stayed open")
	}
}

func TestSettingsNewThemeCardOpensThemePicker(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("New theme"); ok {
		t.Fatal("settings header still has a New theme button")
	}
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 430, 1100)
	pt.Frame()
	pt.Frame()
	if !a.themePickerOpen {
		t.Fatal("picker did not open from the theme card")
	}
	if a.modal != "" {
		t.Fatalf("modal is %q, want empty", a.modal)
	}
}

func TestThemePickerRevertRestoresOpeningState(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 760)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("New custom theme"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pt := ui.NewTester(a.themePickerWindowView, 430, 1100)
	pt.Frame()
	pt.Frame()
	original := a.themePickerOriginal
	a.themePickerDraft.Name = "Changed"
	a.themePickerDraft.GradientGrain = 0.8
	a.pickerChanged()
	if err := pt.Click("Revert"); err != nil {
		t.Fatal(err)
	}
	pt.Frame()
	pt.Frame()
	if a.themePickerDraft.Name != original.Name {
		t.Fatalf("revert kept name %q, want %q", a.themePickerDraft.Name, original.Name)
	}
	if a.themePickerDraft.GradientGrain != original.GradientGrain {
		t.Fatalf("revert kept grain %.2f, want %.2f", a.themePickerDraft.GradientGrain, original.GradientGrain)
	}
	if a.themePickerDraft.GradientOpacity != original.GradientOpacity {
		t.Fatalf("revert kept opacity %.2f, want %.2f", a.themePickerDraft.GradientOpacity, original.GradientOpacity)
	}
}

func TestGradientGrainAndSinglePointPersist(t *testing.T) {
	ct := CustomTheme{
		ID: "theme_grain", Name: "Grain", Base: "nocturne", Gradient: true,
		GradientPoints:  []CustomGradientPoint{{Color: "#6d5dfc", X: 0.40, Y: 0.50}},
		GradientOpacity: 0.50,
		GradientGrain:   0.35,
		GradientMode:    "light",
	}
	p := customPalette(ct)
	if !p.HasGradient {
		t.Fatal("single-point gradient did not load")
	}
	if len(p.Gradient) != 1 {
		t.Fatalf("single-point gradient has %d points, want 1", len(p.Gradient))
	}
	if p.GradientGrain != 0.35 {
		t.Fatalf("grain is %.2f, want 0.35", p.GradientGrain)
	}
	if p.GradientMode != "light" {
		t.Fatalf("mode is %q, want light", p.GradientMode)
	}
	if p.GradientAlpha != 0.50 {
		t.Fatalf("opacity is %.2f, want 0.50", p.GradientAlpha)
	}
}

func TestPickerColorRoundTripsThroughHSV(t *testing.T) {
	a := testApp(t)
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

func TestPickerColorSquareSelectsColor(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	if err := func() error {
		tt := ui.NewTester(a.settingsWindowView, 920, 760)
		tt.Frame()
		tt.Frame()
		return tt.Click("New custom theme")
	}(); err != nil {
		t.Fatal(err)
	}

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
