package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func testApp() *App {
	a := &App{
		listState: ui.ListState{},
		dark:      true, systemDark: true, settingsTab: "appearance", settingsW: settingsRailDefaultWidth,
		themeSettings: ThemeSettings{Theme: "nocturne", FontSize: 14, Radius: 9},
		connected:     true, acct: Account{Email: "a@b.com", Name: "Alex"},
		folders: []Folder{{Name: "INBOX", Label: "Inbox", Kind: "inbox"}},
	}
	a.msgs = []Message{
		{UID: 1, FromName: "Maya", FromAddr: "maya@example.com", Subject: "Hello", Date: time.Now()},
	}
	a.sel = 0
	return a
}

func TestSettingsViewRenders(t *testing.T) {
	a := testApp()
	ui.NewTester(a.settingsWindowView, 920, 760)
}

// TestSettingsOpensAndCloses drives the real frame loop: the click runs while
// the view holds the app mutex, so a handler that locked it again would
// deadlock here instead of in front of the user.
func TestSettingsOpensAndCloses(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	tester := ui.NewTester(a.view, 1320, 860)

	if err := tester.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !a.pendingSettings {
		t.Fatal("clicking Settings did not request the settings window")
	}

	settings := ui.NewTester(a.settingsWindowView, 920, 760)
	if !settings.HasText("Appearance") {
		t.Fatal("settings screen is not showing")
	}
	if err := settings.Click("Close"); err != nil {
		t.Fatal(err)
	}
	settings.Frame()
}

// TestThemeEditorRenders builds the standalone theme editor window.
func TestThemeEditorRenders(t *testing.T) {
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
	a.openThemePicker(nil, "card")
	a.pendingThemePicker = false
	ui.NewTester(a.themePickerWindowView, 430, 820)
	ui.NewTester(a.themePickerWindowView, 390, 600)
}

// TestAccountAppearanceFollowsSwitch checks that switching accounts loads that
// account's saved theme and window material, not just its mail data. Without
// this the material only caught up once Settings was opened.
func TestAccountAppearanceFollowsSwitch(t *testing.T) {
	a := testApp()
	a.accounts = []Account{
		{Email: "one@example.com", Theme: ThemeSettings{Theme: "nocturne", Material: "none"}},
		{Email: "two@example.com", Theme: ThemeSettings{Theme: "dusk", Material: "mica"}},
	}
	a.themeSettings = a.accounts[0].Theme

	a.applyAccountAppearance(a.accounts[1])
	if a.themeSettings.Theme != "dusk" {
		t.Fatalf("theme is %q, want dusk", a.themeSettings.Theme)
	}
	if a.themeSettings.Material != "mica" {
		t.Fatalf("material is %q, want mica", a.themeSettings.Material)
	}
	if !a.palette.Dark || !a.palette.HasGradient {
		t.Fatal("the Dusk palette was not applied")
	}

	a.applyAccountAppearance(a.accounts[0])
	if a.themeSettings.Material != "none" {
		t.Fatalf("material is %q, want none after switching back", a.themeSettings.Material)
	}
}

// TestWindowMaterialChangesAreDeferred checks that background callers queue a
// material change for the next frame instead of calling the window from the
// goroutine. Calling SetVibrancy off the main thread while holding the app
// mutex is what froze the app when switching accounts.
func TestWindowMaterialChangesAreDeferred(t *testing.T) {
	a := testApp()
	a.requestWindowMaterials()
	if !a.pendingMaterials.Load() {
		t.Fatal("requestWindowMaterials did not queue a change")
	}
	a.flushPending()
	if a.pendingMaterials.Load() {
		t.Fatal("flushPending did not clear the queued change")
	}
}

func TestMaterialDropdownSelectsFromSettings(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := testApp()
	a.palette = resolvePalette(a.themeSettings, true, nil)
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
	if a.themeSettings.Material != "acrylic" {
		t.Fatalf("material is %q, want acrylic", a.themeSettings.Material)
	}
	if a.materialOpen {
		t.Fatal("material menu stayed open after choosing")
	}
	if !tt.HasText("Blurs what is behind the window.") {
		t.Fatal("the row did not show the chosen material's description")
	}
}
