package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
)

// libraryPage lists what is installed on this PC, with the actions that
// matter once an app is there.
func (a *App) libraryPage(c *ui.Context) {
	var installed []catalog.App
	for _, app := range a.apps {
		if _, ok := a.installs[app.ID]; ok {
			installed = append(installed, app)
		}
	}
	if len(installed) == 0 {
		a.libraryEmpty(c)
		return
	}
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		a.sectionLabel(c, "INSTALLED", fmt.Sprintf("%d of %d apps", len(installed), len(a.apps)))
		for _, app := range installed {
			a.libraryRow(c, app)
		}
		for _, app := range installed {
			if msg := a.errors[app.ID]; msg != "" {
				a.errorLine(c, app)
			}
		}
	})
}
func (a *App) libraryRow(c *ui.Context, app catalog.App) {
	p := a.palette
	info := a.installs[app.ID]
	row := ui.Row(c).FillWidth().Height(76).Radius(14).Background(p.Field).Border(1, p.Border).Padding(12, 14).Gap(13).AlignItems(ui.Center)
	row.Children(func() {
		a.appTile(c, app, 40)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, app.Name).FontSize(13.5).FontWeight(650).SingleLine()
			ui.Text(c, "Version "+info.Version+" · "+app.Short).FontSize(10.5).TextColor(p.Muted).SingleLine()
		})
		if a.confirm == app.ID {
			ui.Row(c).Gap(8).Shrink(0).Children(func() {
				if a.dangerButton(c, "Remove "+app.Name, iconTrash).Clicked() {
					a.startUninstall(app)
				}
				if a.quietButton(c, "Keep", nil, false).Clicked() {
					a.confirm = ""
				}
			})
			return
		}
		if a.busy[app.ID] != "" {
			ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Size(14, 14)
				ui.Text(c, "Working…").FontSize(11.5).TextColor(p.Muted)
			})
			return
		}
		ui.Row(c).Gap(6).Shrink(0).AlignItems(ui.Center).Children(func() {
			if info.Version != app.Version {
				b := a.accentButton(c, "Update", iconRefresh)
				b.Label("Update " + app.Name)
				if b.Clicked() {
					a.startInstall(app)
				}
			} else {
				b := a.accentButton(c, "Open", iconPlay)
				b.Label("Open " + app.Name)
				if b.Clicked() {
					a.launch(app)
				}
			}
			un := a.quietButton(c, "Uninstall", iconTrash, true)
			un.Label("Uninstall " + app.Name)
			if un.Clicked() {
				a.confirm = app.ID
			}
			if b := a.iconButton(c, "folder:"+app.ID, iconFolder, "Open install folder"); b.Clicked() {
				a.openFolder(app)
			}
		})
	})
}
func (a *App) libraryEmpty(c *ui.Context) {
	p := a.palette
	ui.Column(c).FillWidth().MinHeight(430).Center().Gap(9).Children(func() {
		ui.Box(c).Size(46, 46).Radius(13).Background(p.Field).Border(1, p.Border).Center().Shrink(0).Children(func() {
			ui.Icon(c, iconLibrary).Size(21, 21).TextColor(p.Muted)
		})
		ui.Text(c, "Nothing installed yet").FontSize(15).FontWeight(650).Margin(6, 0, 0, 0)
		ui.Text(c, "Install an app from the store and it will show up here.").FontSize(12).TextColor(p.Muted)
		ui.Box(c).Height(4)
		b := a.accentButton(c, "Browse the store", iconStore)
		if b.Clicked() {
			a.goPage(pageStore)
		}
	})
}
