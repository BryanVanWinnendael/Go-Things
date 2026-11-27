package app

import (
	"go-post/internal/core"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func testApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	st := core.NewStore(dir, filepath.Join(dir, "gopost.json"))
	return newApp(st, core.DefaultData())
}

// A page you navigate away to names itself and keeps its way back where it can
// be seen: a labelled chip in the page, not a lone glyph in the window chrome.
func TestSecondaryPageHasLabelledBack(t *testing.T) {
	for _, page := range []string{"collections", "environments", "workspaces", "history"} {
		page := page
		t.Run(page, func(t *testing.T) {
			a := testApp(t)
			a.page = page
			tt := ui.NewTester(a.view, 1280, 820)
			tt.Frame()
			tt.Frame()

			title, _ := pageMeta(page)
			if !tt.HasText(title) {
				t.Fatalf("%s page header does not name the page", page)
			}
			back, ok := tt.Find("Back")
			if !ok {
				t.Fatalf("%s page has no back control", page)
			}
			if back.W < 48 || back.H < 26 {
				t.Fatalf("back control is %.0fx%.0f, too small to notice", back.W, back.H)
			}
			if err := tt.Click("Back"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if a.page != "request" {
				t.Fatalf("page is %q after Back, want request", a.page)
			}
		})
	}
}

func TestCollectionCollapsesAndExpands(t *testing.T) {
	a := testApp(t)
	id := a.ws().Collections[0].ID
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Starter kit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	tt.Frame()
	if !a.collapsed[id] {
		t.Fatal("collection did not collapse")
	}
	if _, ok := tt.Find("HTTPBin / Post"); ok {
		t.Fatal("collapsed collection still shows its requests")
	}

	if err := tt.Click("Starter kit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	tt.Frame()
	if a.collapsed[id] {
		t.Fatal("collection did not expand")
	}
	if _, ok := tt.Find("HTTPBin / Post"); !ok {
		t.Fatal("expanded collection hides its requests")
	}
}

func TestAcrylicTintsChromeButKeepsStageSolid(t *testing.T) {
	a := testApp(t)
	a.data.Settings.Acrylic = true
	scrimAlpha := float32(chromeScrimAlpha)
	scrim := uint8(scrimAlpha * 255)

	main := ui.NewTester(a.view, 1280, 820)
	main.Frame()
	main.SetVibrancy(true)
	main.Frame()
	img := main.Image()
	if got := img.RGBAAt(2, 300).A; got != scrim {
		t.Fatalf("acrylic sidebar alpha is %d, want the %d scrim", got, scrim)
	}
	if got := img.RGBAAt(int(a.sidebarW), 300).A; got > scrim {
		t.Fatalf("acrylic divider alpha is %d, want at most the %d scrim", got, scrim)
	}
	if got := img.RGBAAt(400, 300).A; got != 255 {
		t.Fatalf("acrylic workspace alpha is %d, want 255", got)
	}

	// Settings follows the material too: its rail shows the acrylic, while the
	// rounded stage stays solid.
	settings := ui.NewTester(a.settingsWindowView, 920, 760)
	settings.Frame()
	settings.SetVibrancy(true)
	simg := settings.Image()
	if got := simg.RGBAAt(2, 300).A; got != scrim {
		t.Fatalf("settings rail alpha is %d, want the %d scrim", got, scrim)
	}
	if got := simg.RGBAAt(400, 300).A; got != 255 {
		t.Fatalf("settings stage alpha is %d, want 255", got)
	}
}

func TestCustomGradientTintsAcrylicButKeepsStageSolid(t *testing.T) {
	a := testApp(t)
	a.data.CustomThemes = []core.CustomTheme{{
		ID: "theme_night_swim", Name: "Night swim", Base: "nocturne",
		Background: "#1a2a44", Gradient: true,
		GradientPoints: []core.CustomGradientPoint{
			{Color: "#6d5dfc", X: 0.18, Y: 0.24},
			{Color: "#39c6a4", X: 0.56, Y: 0.54},
			{Color: "#5ca8ff", X: 0.84, Y: 0.78},
		},
		GradientOpacity: 0.55,
	}}
	a.data.Settings.Theme = "theme_night_swim"
	a.data.Settings.Acrylic = true

	main := ui.NewTester(a.view, 1280, 820)
	main.Frame()
	main.SetVibrancy(true)
	main.Frame()
	img := main.Image()
	got := img.RGBAAt(2, 300).A
	scrimAlpha := float32(chromeScrimAlpha)
	scrim := uint8(scrimAlpha * 255)
	if got >= 255 {
		t.Fatalf("acrylic gradient chrome alpha is %d, want the material visible", got)
	}
	if got < scrim {
		t.Fatalf("acrylic gradient chrome alpha is %d, want at least the %d scrim", got, scrim)
	}
	if got := img.RGBAAt(400, 300).A; got != 255 {
		t.Fatalf("acrylic gradient workspace alpha is %d, want 255", got)
	}

	settings := ui.NewTester(a.settingsWindowView, 920, 760)
	settings.Frame()
	settings.SetVibrancy(true)
	simg := settings.Image()
	got = simg.RGBAAt(2, 300).A
	if got >= 255 {
		t.Fatalf("settings gradient rail alpha is %d, want the material visible", got)
	}
	if got < scrim {
		t.Fatalf("settings gradient rail alpha is %d, want at least the %d scrim", got, scrim)
	}
	if got := simg.RGBAAt(400, 300).A; got != 255 {
		t.Fatalf("settings gradient stage alpha is %d, want 255", got)
	}
}

func TestSolidBackgroundKeepsChromeOpaque(t *testing.T) {
	a := testApp(t)
	main := ui.NewTester(a.view, 1280, 820)
	main.Frame()
	img := main.Image()
	if got := img.RGBAAt(2, 300).A; got != 255 {
		t.Fatalf("solid sidebar alpha is %d, want 255", got)
	}
	if got := img.RGBAAt(int(a.sidebarW), 300).A; got != 255 {
		t.Fatalf("solid divider alpha is %d, want 255", got)
	}
}

func TestMaterialDropdownSelectsFromSettings(t *testing.T) {
	a := testApp(t)
	a.systemDark = true
	tt := ui.NewTester(a.settingsWindowView, 920, 1200)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Window material: None"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !a.materialOpen {
		t.Fatal("material menu did not open")
	}
	if err := tt.Click("Acrylic"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if a.data.Settings.Material != "acrylic" {
		t.Fatalf("material is %q, want acrylic", a.data.Settings.Material)
	}
	if a.materialOpen {
		t.Fatal("material menu stayed open after choosing")
	}
	if !tt.HasText("Blurs what is behind the window.") {
		t.Fatal("the row did not show the chosen material's description")
	}
}

func TestMaterialSettingToggles(t *testing.T) {
	a := testApp(t)
	if a.data.Settings.Material != "none" {
		t.Fatal("material starts on")
	}
	a.setMaterial("acrylic")
	if a.data.Settings.Material != "acrylic" {
		t.Fatal("setMaterial did not persist the setting")
	}
	loaded, err := a.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Settings.Material != "acrylic" {
		t.Fatal("material setting did not survive a reload")
	}
}

func TestSidebarDragSticksAndSurvivesCollapse(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	div, ok := tt.Find("Divider")
	if !ok {
		t.Fatal("splitter not found")
	}
	y := div.Y + div.H/2
	tt.Press(div.X+div.W/2, y)
	tt.Frame()
	tt.Move(200, y)
	tt.Frame()
	tt.Release(200, y)
	tt.Frame()
	tt.Frame()

	dragged := a.sidebarW
	if dragged >= sidebarDefaultWidth-20 || dragged < sidebarMinWidth {
		t.Fatalf("drag left sidebar at %v, want a smaller width of at least %v", dragged, sidebarMinWidth)
	}
	if a.sidebarExpandedW != dragged {
		t.Fatalf("expanded width is %v, want the dragged %v", a.sidebarExpandedW, dragged)
	}

	// Collapsing and expanding must return to the dragged width, not the
	// default. Advancing the clock lets the animation finish.
	for _, collapse := range []bool{true, false} {
		if err := tt.Click("Toggle sidebar"); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 12; i++ {
			tt.Advance(40 * time.Millisecond)
			tt.Frame()
		}
		if a.sidebarAnimating {
			t.Fatalf("sidebar still animating after collapse=%v", collapse)
		}
		if collapse {
			if a.sidebarW != 0 {
				t.Fatalf("collapsed sidebar is %v, want 0", a.sidebarW)
			}
		} else if a.sidebarW != dragged {
			t.Fatalf("expanded sidebar is %v, want dragged %v", a.sidebarW, dragged)
		}
	}
}

func TestSidebarsKeepTheirMinimumWidth(t *testing.T) {
	a := testApp(t)

	dragTo := func(tt *ui.Tester, x float32) {
		t.Helper()
		r, ok := tt.Find("Divider")
		if !ok {
			t.Fatal("splitter not found")
		}
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Move(x, r.Y+r.H/2)
		tt.Release(x, r.Y+r.H/2)
		tt.Frame()
	}

	main := ui.NewTester(a.view, 1280, 820)
	main.Frame()
	main.Frame()
	dragTo(main, 60)
	if a.sidebarW < sidebarMinWidth {
		t.Fatalf("sidebar width is %v, want at least %v", a.sidebarW, sidebarMinWidth)
	}

	settings := ui.NewTester(a.settingsWindowView, 920, 760)
	settings.Frame()
	settings.Frame()
	dragTo(settings, 40)
	if a.settingsW < settingsRailMinWidth {
		t.Fatalf("settings rail width is %v, want at least %v", a.settingsW, settingsRailMinWidth)
	}
}
