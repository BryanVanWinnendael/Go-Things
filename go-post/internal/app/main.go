package app

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go-post/internal/core"
)

// Main sets up the menus and main window and runs the MyGo application.
func Main() {
	mygo.App.SetName("Go Post")
	store, err := core.OpenStore()
	if err != nil {
		log.Fatal(err)
	}
	data, err := store.Load()
	if err != nil {
		log.Printf("starting with default data: %v", err)
	}
	a := newApp(store, data)
	a.save()

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Label: "File", Submenu: []*mygo.MenuItem{
				{Label: "New Request", Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(a.newRequest)
					}
				}},
				{Label: "New Workspace", Accelerator: "CmdOrCtrl+Shift+N", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(func() {
							a.closePopovers()
							a.modal = "new_workspace"
							a.input1 = ""
							a.modalError = ""
						})
					}
				}},
				mygo.Separator(),
				{Label: "Send Request", Accelerator: "CmdOrCtrl+Enter", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(a.sendActive)
					}
				}},
				mygo.Separator(),
				{Role: mygo.RoleClose},
			}},
			{Label: "View", Submenu: []*mygo.MenuItem{
				{Label: "Request", Accelerator: "CmdOrCtrl+1", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(func() { a.setPage("request") })
					}
				}},
				{Label: "Collections", Accelerator: "CmdOrCtrl+2", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(func() { a.setPage("collections") })
					}
				}},
				{Label: "Environments", Accelerator: "CmdOrCtrl+3", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(func() { a.setPage("environments") })
					}
				}},
				{Label: "Workspaces", Accelerator: "CmdOrCtrl+6", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(func() { a.setPage("workspaces") })
					}
				}},
				{Label: "Certificates", Accelerator: "CmdOrCtrl+4", Click: func(*mygo.MenuItem, *mygo.Window) {
					a.openSettings("certificates")
				}},
				{Label: "History", Accelerator: "CmdOrCtrl+5", Click: func(*mygo.MenuItem, *mygo.Window) {
					if a.win != nil {
						a.win.Update(func() { a.setPage("history") })
					}
				}},
				mygo.Separator(),
				{Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) {
					a.openSettings("appearance")
				}},
			}},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleWindowMenu},
			{Label: "Help", Submenu: []*mygo.MenuItem{
				{Label: "Go Post on MyGo", Click: func(*mygo.MenuItem, *mygo.Window) {
					_ = mygo.Shell.OpenExternal("https://mygo.egoist.dev")
				}},
			}},
		}))

		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:             "Go Post",
			Width:             1280,
			Height:            820,
			MinWidth:          960,
			MinHeight:         640,
			TitleBarStyle:     mygo.TitleBarHidden,
			HideWindowButtons: true,
			BackgroundColor:   "#0b0b0b",
			// Created with a material so the acrylic setting can switch it on
			// and off without recreating the window. It starts off.
			Vibrancy: mygo.VibrancyAcrylic,
			StateKey: "main",
			Content:  ui.View(a.view),
		})
		a.applyWindowMaterials()
		a.win.Show()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
