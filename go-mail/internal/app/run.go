package app

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func Main() {
	mygo.App.SetName("Go Mail")
	defer func() {
		if r := recover(); r != nil {
			logPanic("main", r)
		}
	}()
	a := &App{
		sel: -1, split: 420, sidebarW: 250, loadedLimit: 100, sidebarSel: "INBOX", folder: "INBOX",
		cache:           map[string]*folderCache{},
		folderListState: ui.ListState{},
		systemDark:      true, settingsTab: "appearance", settingsW: settingsRailDefaultWidth,
		themeSettings: ThemeSettings{Theme: "nocturne", FontSize: 14, Radius: 9},
	}
	a.loadConfig()
	if a.form.IMAPPort == "" {
		a.form.IMAPPort = "993"
	}
	if a.form.SMTPPort == "" {
		a.form.SMTPPort = "587"
	}

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
		}))
		a.mainWindow = mygo.NewWindow(mygo.WindowOptions{
			Title:             "Go Mail",
			Width:             1320,
			Height:            860,
			MinWidth:          980,
			MinHeight:         620,
			TitleBarStyle:     mygo.TitleBarHidden,
			HideWindowButtons: true,
			BackgroundColor:   "#0b0b0b",
			Vibrancy:          mygo.VibrancyAcrylic,
			StateKey:          "main",
			Content:           ui.View(a.view),
		})
		a.applyWindowMaterials()
		a.mainWindow.Show()
		if v, err := a.mainWindow.NewWebView(mygo.WebViewOptions{BackgroundColor: "#15161c"}); err == nil {
			a.readerView = v
		} else {
			log.Printf("NewWebView failed: %v", err)
		}
		if len(a.accounts) > 0 && a.active >= 0 && a.active < len(a.accounts) && a.accounts[a.active].Email != "" && a.accounts[a.active].Password != "" {
			a.connecting = true
			a.busy = true
			go func() {
				time.Sleep(250 * time.Millisecond)
				a.connect()
			}()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
