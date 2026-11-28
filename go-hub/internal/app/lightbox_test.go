package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
	"go-hub/internal/install"
	"go-hub/internal/theme"
)

func TestShotLightbox(t *testing.T) {
	a := &App{
		apps:          catalog.Fallback,
		installs:      map[string]install.Info{},
		busy:          map[string]string{},
		errors:        map[string]string{},
		previews:      map[string]*ui.Bitmap{},
		previewsLight: map[string]*ui.Bitmap{},
		logos:         map[string]*ui.Bitmap{},
		shots:         map[string][]*ui.Bitmap{},
		shotsLight:    map[string][]*ui.Bitmap{},
		sizes:         map[string]string{},
		hub:           catalog.DefaultHub(),
		shotIndex:     -1,
	}
	a.themeSetting = "nocturne"
	a.palette = theme.Resolve("nocturne", true)
	a.page = pageDetail
	a.detailID = "gopost"

	tt := ui.NewTester(a.view, 1180, 760)
	if err := tt.Click("View screenshot 1 full size"); err != nil {
		t.Fatalf("thumbnail did not open the lightbox: %v", err)
	}
	if !a.shotOpen || a.shotIndex != 0 {
		t.Fatalf("lightbox state after click: open=%v index=%d", a.shotOpen, a.shotIndex)
	}
	if !tt.HasText("1 / 3") {
		t.Fatal("lightbox does not show its position in the gallery")
	}

	tt.Key(0, ui.KeyRight)
	if a.shotIndex != 1 {
		t.Fatalf("right arrow moved to %d, want 1", a.shotIndex)
	}
	tt.Key(0, ui.KeyEscape)
	if a.shotOpen {
		t.Fatal("Escape did not close the lightbox")
	}
}
