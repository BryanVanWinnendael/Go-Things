package main

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
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
	palette      palette

	apps           []suiteApp
	catalogErr     string
	catalogUpdated string
	catalogLoading bool
	lastCatalog    time.Time

	installs map[string]installedInfo
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

func main() {
	a := &App{
		apps:     fallbackCatalog,
		installs: map[string]installedInfo{},
		busy:     map[string]string{},
		errors:   map[string]string{},
		previews: map[string]*ui.Bitmap{},
		logos:    map[string]*ui.Bitmap{},
		shots:    map[string][]*ui.Bitmap{},
		sizes:    map[string]string{},
	}
	a.themeSetting = loadSettings().Theme

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
		}))
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:             "Go Hub",
			Width:             1180,
			Height:            760,
			MinWidth:          1020,
			MinHeight:         680,
			TitleBarStyle:     mygo.TitleBarHidden,
			HideWindowButtons: true,
			BackgroundColor:   "#0b0b0b",
			StateKey:          "main",
			Content:           ui.View(a.view),
		})
		a.win.Show()
		a.loadAssets()
		a.refresh()
		a.loadCatalog()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// view builds the window frame: the sidebar, then the stage the page sits in.
func (a *App) view(c *ui.Context) {
	if a.themeSetting == "system" {
		a.systemDark = c.Theme().Dark
	}
	p := resolvePalette(a.themeSetting, a.systemDark)
	a.palette = p
	c.SetTheme(applyTheme(p))
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
	a.saveSettings()
}

const heroSuffix = ":hero"
