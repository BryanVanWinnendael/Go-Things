package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Both the settings rail and the About page spell out the same version.
func TestVersionShowsInSettingsRailAndAbout(t *testing.T) {
	a := testApp(t)
	a.settingsTab = "about"
	tt := ui.NewTester(a.settingsWindowView, 920, 1000)
	tt.Frame()
	tt.Frame()

	label := versionLabel()
	seen := 0
	for _, text := range tt.Texts() {
		if text == label {
			seen++
		}
	}
	if seen < 2 {
		t.Fatalf("%q appears %d times, want it in the rail and on the About page", label, seen)
	}
}
