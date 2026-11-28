package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
	"go-hub/internal/install"
	"go-hub/internal/settings"
	"go-hub/internal/theme"
)

type page int

const (
	pageStore page = iota
	pageLibrary
	pageAbout
	pageDetail
)

type App struct {
	win          *mygo.Window
	page         page
	detailID     string
	themeSetting string
	systemDark   bool
	palette      theme.Palette

	apps           []catalog.App
	catalogErr     string
	catalogUpdated string
	catalogLoading bool
	lastCatalog    time.Time
	hub            catalog.HubUpdate

	installs map[string]install.Info
	busy     map[string]string
	errors   map[string]string
	confirm  string

	refreshing bool
	assetsGen  int

	previews      map[string]*ui.Bitmap
	previewsLight map[string]*ui.Bitmap
	logos         map[string]*ui.Bitmap
	shots         map[string][]*ui.Bitmap
	shotsLight    map[string][]*ui.Bitmap
	sizes         map[string]string
	hubIcon       *ui.Bitmap

	shotOpen  bool
	shotIndex int
}

// view builds the window frame: the sidebar, then the stage the page sits in.
func (a *App) view(c *ui.Context) {
	if a.themeSetting == "system" {
		a.systemDark = c.Theme().Dark
	}
	p := theme.Resolve(a.themeSetting, a.systemDark)
	a.palette = p
	c.SetTheme(theme.Apply(p))
	c.Root().Background(p.Bg)

	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.goPage(pageStore) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.goPage(pageLibrary) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.goPage(pageAbout) })
	c.OnShortcut(ui.Cmd, ui.KeyR, a.refreshAll)

	ui.Row(c).FillWidth().FillHeight().Children(func() {
		a.sidebar(c)
		a.stage(c)
	})
}

func (a *App) goPage(p page) {
	a.confirm = ""
	a.shotOpen = false
	a.page = p
	if p != pageDetail {
		a.detailID = ""
	}
}

func (a *App) goDetail(id string) {
	a.confirm = ""
	a.shotOpen = false
	a.detailID = id
	a.page = pageDetail
}

func (a *App) setTheme(id string) {
	if a.themeSetting == id {
		return
	}
	a.themeSetting = id
	settings.Save(a.themeSetting)
	if a.win != nil {
		a.win.Invalidate()
	}
}

const heroSuffix = ":hero"

// previewBitmap returns the card preview for the current theme. A catalog
// without a light variant falls back to its single image.
func (a *App) previewBitmap(app catalog.App) *ui.Bitmap {
	if !a.palette.Dark {
		if bmp := a.previewsLight[app.ID]; bmp != nil {
			return bmp
		}
	}
	return a.previews[app.ID]
}

// heroBitmap returns the large storefront image for the current theme.
func (a *App) heroBitmap(app catalog.App) *ui.Bitmap {
	if !a.palette.Dark {
		if bmp := a.previewsLight[app.ID+heroSuffix]; bmp != nil {
			return bmp
		}
		if bmp := a.previewsLight[app.ID]; bmp != nil {
			return bmp
		}
	}
	if bmp := a.previews[app.ID+heroSuffix]; bmp != nil {
		return bmp
	}
	return a.previews[app.ID]
}

// shotBitmaps returns the gallery for the current theme, falling back to the
// dark gallery when no light screenshots were published.
func (a *App) shotBitmaps(app catalog.App) []*ui.Bitmap {
	if !a.palette.Dark {
		if shots := a.shotsLight[app.ID]; len(shots) > 0 {
			return shots
		}
	}
	return a.shots[app.ID]
}
