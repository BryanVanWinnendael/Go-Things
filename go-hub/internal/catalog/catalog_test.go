package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The catalog checked here is the exact file that gets attached to a release.
func TestPublishedCatalogIsValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "catalog.json"))
	if err != nil {
		t.Fatalf("read catalog.json: %v", err)
	}
	var doc Doc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("decode catalog.json: %v", err)
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("catalog.json is not publishable: %v", err)
	}
	if len(doc.Apps) == 0 {
		t.Fatal("catalog.json contains no apps")
	}
	featured := Featured(doc.Apps)
	if len(featured) == 0 {
		t.Fatal("catalog.json should mark at least one app as featured")
	}
	if featured[0].ID != "gopost" {
		t.Fatalf("expected Go Post to be featured, got %q", featured[0].ID)
	}
	for _, app := range doc.Apps {
		if app.PreviewLight == "" || app.HeroLight == "" || len(app.ScreenshotsLight) == 0 {
			t.Errorf("app %q has no light-mode pictures", app.ID)
			continue
		}
		if got := app.PreviewRef(false); got != app.PreviewLight {
			t.Errorf("app %q light preview = %q, want %q", app.ID, got, app.PreviewLight)
		}
		if got := app.PreviewRef(true); got != app.Preview {
			t.Errorf("app %q dark preview = %q, want %q", app.ID, got, app.Preview)
		}
		if got := app.ScreenshotRefs(false); len(got) != len(app.ScreenshotsLight) {
			t.Errorf("app %q light screenshots = %d, want %d", app.ID, len(got), len(app.ScreenshotsLight))
		}
	}
	if doc.Hub.Version == "" || !IsHTTPS(doc.Hub.Setup) {
		t.Fatal("catalog.json needs a hub version and an https setup URL")
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.1.0", "1.0.0", true},
		{"1.0.1", "1.0.0", true},
		{"1.10.0", "1.9.0", true},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.1.0", false},
		{"", "1.0.0", false},
	}
	for _, tc := range cases {
		if got := IsNewer(tc.latest, tc.current); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

// An older catalog without light fields keeps its single picture in both modes.
func TestThemeRefsFallBackToDark(t *testing.T) {
	app := App{ID: "demo", Preview: "dark.png", Hero: "hero.png", Screenshots: []string{"one.png"}}
	if app.PreviewRef(false) != "dark.png" || app.HeroRef(false) != "hero.png" {
		t.Fatal("light refs should fall back to the dark pictures")
	}
	if got := app.ScreenshotRefs(false); len(got) != 1 || got[0] != "one.png" {
		t.Fatalf("light screenshots should fall back to dark: %v", got)
	}
}
