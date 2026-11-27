package app

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
	"go-hub/internal/install"
	"go-hub/internal/settings"
)

func Main() {
	a := &App{
		apps:     catalog.Fallback,
		installs: map[string]install.Info{},
		busy:     map[string]string{},
		errors:   map[string]string{},
		previews: map[string]*ui.Bitmap{},
		logos:    map[string]*ui.Bitmap{},
		shots:    map[string][]*ui.Bitmap{},
		sizes:    map[string]string{},
	}
	a.themeSetting = settings.Load().Theme

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
