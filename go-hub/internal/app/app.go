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

	installs map[string]install.Info
	busy     map[string]string
	errors   map[string]string
	confirm  string

	refreshing bool
	assetsGen  int

	previews map[string]*ui.Bitmap
	logos    map[string]*ui.Bitmap
	shots    map[string][]*ui.Bitmap
	sizes    map[string]string
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
	a.page = p
	if p != pageDetail {
		a.detailID = ""
	}
}

func (a *App) goDetail(id string) {
	a.confirm = ""
	a.detailID = id
	a.page = pageDetail
}

func (a *App) setTheme(id string) {
	if a.themeSetting == id {
		return
	}
	a.themeSetting = id
	settings.Save(a.themeSetting)
}

const heroSuffix = ":hero"
