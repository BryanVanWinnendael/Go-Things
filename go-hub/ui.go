package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// storePage is the storefront: one featured app with its real screen, then
// the whole suite as wide capsules.
func (a *App) storePage(c *ui.Context) {
	ui.Column(c).FillWidth().Gap(16).Children(func() {
		a.storeNotice(c)
		if len(a.apps) == 0 {
			ui.Column(c).FillWidth().Center().Gap(8).Children(func() {
				ui.Text(c, "No apps in the catalog").FontSize(15).FontWeight(650)
				ui.Text(c, "Publish a catalog.json in the project's latest GitHub release.").FontSize(12).TextColor(a.palette.Muted)
			})
			return
		}
		for _, app := range featuredApps(a.apps) {
			a.hero(c, app)
		}

		meta := fmt.Sprintf("%d apps in the suite", len(a.apps))
		if a.catalogUpdated != "" {
			meta += " · updated " + a.catalogUpdated
		}
		a.sectionLabel(c, "ALL APPS", meta)

		ui.Column(c).FillWidth().Gap(10).Children(func() {
			for _, app := range a.apps {
				a.storeRow(c, app)
			}
		})
	})
}

// storeNotice is honest about a failed catalog check. The built-in list stays
// usable, and one click tries GitHub again.
func (a *App) storeNotice(c *ui.Context) {
	if a.catalogErr == "" {
		return
	}
	p := a.palette
	row := ui.Row(c).FillWidth().Height(36).Shrink(0).Padding(0, 10).Radius(9).Gap(9).AlignItems(ui.Center)
	row.Background(p.Bad.Alpha(0.09))
	row.Border(1, p.Bad.Alpha(0.30))
	row.Children(func() {
		ui.Icon(c, iconAlert).Size(14, 14).TextColor(readableInk(p.Bad, p)).Shrink(0)
		ui.Text(c, "Showing the built-in apps: "+a.catalogErr).FontSize(11).TextColor(readableInk(p.Bad, p)).Grow(1).SingleLine()
		if a.quietButton(c, "Retry", iconRefresh, false).Clicked() {
			a.loadCatalog()
		}
	})
}

func (a *App) hero(c *ui.Context, app suiteApp) {
	p := a.palette
	card := ui.ButtonBase(c).FillWidth().Height(258).Radius(16).Border(1, p.Border).Clip().AlignItems(ui.Stretch).Shrink(0).FocusRing(true).Label("View " + app.Name + " details")
	card.LinearGradient(ui.LinearGradient{From: p.Field, To: p.Panel, Angle: 150})
	if card.Hovered() {
		card.Border(1, p.Border.Mix(p.Text, 0.10))
	}
	if card.Clicked() {
		a.goDetail(app.ID)
	}
	card.Children(func() {
		ui.Column(c).Grow(1).FillHeight().AlignItems(ui.Start).Padding(20, 22).Gap(7).Children(func() {
			ui.Row(c).Gap(10).Shrink(0).AlignItems(ui.Center).Children(func() {
				a.appTile(c, app, 34)
				ui.Column(c).Gap(0).Children(func() {
					ui.Text(c, "FEATURED APP").FontSize(9).FontWeight(700).LetterSpacing(1.3).TextColor(p.Muted)
					ui.Text(c, app.Short).FontSize(10.5).TextColor(p.Muted)
				})
			})

			ui.Text(c, app.Name).FontSize(27).FontWeight(720).LetterSpacing(-0.6).Margin(2, 0, 0, 0)

			ui.Text(c, app.Tagline).FontSize(12.5).TextColor(p.Muted).MaxWidth(430).MaxLines(2)

			ui.Row(c).Gap(6).Shrink(0).Margin(2, 0, 0, 0).Children(func() {
				for _, fact := range app.Facts {
					a.chip(c, fact)
				}
			})

			ui.Box(c).Grow(1)

			ui.Text(c, a.metaLine(app, true)).FontSize(10.5).TextColor(p.Muted).SingleLine()

			// The hero is a column; keep its actions on one line so the
			// uninstall control stays visible beside the primary action.
			ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Children(func() {
				a.actions(c, app, false)
			})

			a.errorLine(c, app)
		})

		ui.Row(c).Width(1).FillHeight().Shrink(0).Background(p.Border)

		ui.Box(c).Width(412).FillHeight().Shrink(0).Children(func() {
			a.heroShot(c, app)
		})
	})
}

// heroShot is a full-bleed capture of the featured app: the storefront's
// window into what the software actually looks like.
func (a *App) heroShot(c *ui.Context, app suiteApp) {
	p := a.palette
	if bmp := a.previews[app.ID+heroSuffix]; bmp != nil {
		ui.Image(c, bmp).Fit(ui.Cover).FillWidth().FillHeight()
		return
	}
	if bmp := a.previews[app.ID]; bmp != nil {
		ui.Image(c, bmp).Fit(ui.Cover).FillWidth().FillHeight()
		return
	}
	ui.Box(c).FillWidth().FillHeight().Background(p.Bg).Center().Children(func() {
		ui.Icon(c, appGlyph(app.ID)).Size(26, 26).TextColor(p.Muted)
	})
}

// storeRow is one capsule in the suite index: a real screen, what the app
// does, and the single action its state calls for.
func (a *App) storeRow(c *ui.Context, app suiteApp) {
	p := a.palette
	row := ui.ButtonBase(c).FillWidth().Height(126).Radius(14).Background(p.Field).Border(1, p.Border).Padding(12).Gap(14).AlignItems(ui.Center).Shrink(0).FocusRing(true).Label("View " + app.Name + " details")
	if row.Hovered() {
		row.Background(p.Hover)
	}
	if row.Clicked() {
		a.goDetail(app.ID)
	}
	row.Children(func() {
		a.preview(c, app, 163, 102, 10)

		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, app.Name).FontSize(14.5).FontWeight(680).LetterSpacing(-0.15).SingleLine()
				a.statePill(c, app)
			})
			ui.Text(c, app.Tagline).FontSize(11.5).TextColor(p.Muted).SingleLine()
			if msg := a.errors[app.ID]; msg != "" {
				ui.Text(c, msg).FontSize(10.5).TextColor(readableInk(p.Bad, p)).SingleLine()
			} else {
				ui.Text(c, a.metaLine(app, false)).FontSize(10.5).TextColor(p.Muted).SingleLine()
			}
		})

		a.actions(c, app, true)
	})
}

// preview frames a real capture of the app. Nothing here is a mockup: the
// bitmaps are screenshots of the built programs.
func (a *App) preview(c *ui.Context, app suiteApp, w, h, radius float32) {
	p := a.palette
	frame := ui.Box(c).Width(w).Height(h).Shrink(0).Radius(radius).Clip().Background(p.Bg).Border(1, p.Border)
	frame.Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, 0.28))
	frame.Children(func() {
		if bmp := a.previews[app.ID]; bmp != nil {
			ui.Image(c, bmp).Fit(ui.Cover).FillWidth().FillHeight()
		} else {
			ui.Box(c).FillWidth().FillHeight().Center().Children(func() {
				ui.Icon(c, appGlyph(app.ID)).Size(22, 22).TextColor(p.Muted)
			})
		}
	})
}

// appTile is the app's mark read from its own setup, with the line glyph as
// a fallback when Windows cannot read an icon.
func (a *App) appTile(c *ui.Context, app suiteApp, size float32) {
	p := a.palette
	tile := ui.Box(c).Size(size, size).Shrink(0).Radius(size * 0.26).Center()
	if bmp := a.logos[app.ID]; bmp != nil {
		tile.Background(ui.Transparent)
		tile.Children(func() {
			ui.Image(c, bmp).Fit(ui.Contain).Size(size, size)
		})
		return
	}
	accent := p.Accent
	if app.Accent != "" {
		accent = ui.Hex(app.Accent)
	}
	tile.Background(accent)
	tile.Children(func() {
		ui.Icon(c, appGlyph(app.ID)).Size(size*0.5, size*0.5).TextColor(contrastText(accent))
	})
}

// statePill names the install state in words, with color as a second cue.
func (a *App) statePill(c *ui.Context, app suiteApp) {
	p := a.palette
	info, installed := a.installs[app.ID]
	if !installed {
		return
	}
	label, color := "Installed", p.Good
	if info.Version != app.Version {
		label, color = "Update ready", p.Warn
	}
	ink := readableInk(color, p)
	e := ui.Row(c).Height(20).Padding(0, 7).Radius(6).Gap(5).Shrink(0).AlignItems(ui.Center)
	e.Background(color.Alpha(0.14))
	e.Children(func() {
		ui.Box(c).Size(5, 5).Radius(3).Background(ink).Shrink(0)
		ui.Text(c, label).FontSize(9.5).FontWeight(650).TextColor(ink).SingleLine()
	})
}

// metaLine is the honest small print: which version ships, how large the
// setup is, and what is installed right now.
func (a *App) metaLine(app suiteApp, full bool) string {
	info, installed := a.installs[app.ID]
	var parts []string
	if installed {
		parts = append(parts, "Installed "+info.Version)
		if info.Version != app.Version {
			parts = append(parts, "version "+app.Version+" available")
		}
	} else {
		parts = append(parts, "Version "+app.Version)
		if size := a.sizes[app.ID]; size != "" {
			parts = append(parts, size+" setup")
		}
	}
	if full && !installed {
		parts = append(parts, "installs for your user")
	}
	return strings.Join(parts, " · ")
}

// actions renders the one primary action the app's state calls for, plus
// uninstall when it is installed. Every label names its result: the hero
// spells out the app, the compact form in a card keeps the name in the
// accessible label.
func (a *App) actions(c *ui.Context, app suiteApp, compact bool) {
	p := a.palette
	if working := a.busy[app.ID]; working != "" {
		label := "Installing " + app.Name + "…"
		switch working {
		case "download":
			label = "Downloading " + app.Name + "…"
		case "uninstall":
			label = "Removing " + app.Name + "…"
		}
		ui.Row(c).Height(34).Gap(9).Shrink(0).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Size(14, 14)
			ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(p.Text).SingleLine()
		})
		return
	}

	if a.confirm == app.ID {
		ui.Row(c).Height(34).Gap(8).Shrink(0).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Remove "+app.Name+"?").FontSize(11.5).TextColor(p.Muted).SingleLine()
			remove := a.dangerButton(c, "Remove", iconTrash)
			remove.Label("Remove " + app.Name + " and its shortcuts")
			if remove.Clicked() {
				a.startUninstall(app)
			}
			if a.quietButton(c, "Keep", nil, false).Clicked() {
				a.confirm = ""
			}
		})
		return
	}

	info, installed := a.installs[app.ID]
	if !installed {
		label := "Install"
		if !compact {
			label = "Install " + app.Name
		}
		b := a.accentButton(c, label, iconDownload)
		b.Label("Install " + app.Name)
		if b.Clicked() {
			a.startInstall(app)
		}
		return
	}

	if info.Version != app.Version {
		label := "Update"
		if !compact {
			label = "Update " + app.Name
		}
		b := a.accentButton(c, label, iconRefresh)
		b.Label("Update " + app.Name)
		if b.Clicked() {
			a.startInstall(app)
		}
	} else {
		label := "Open"
		if !compact {
			label = "Open " + app.Name
		}
		b := a.accentButton(c, label, iconPlay)
		b.Label("Open " + app.Name)
		if b.Clicked() {
			a.launch(app)
		}
	}
	u := a.quietButton(c, "Uninstall", iconTrash, true)
	u.Label("Uninstall " + app.Name)
	if u.Clicked() {
		a.confirm = app.ID
	}
}

func (a *App) errorLine(c *ui.Context, app suiteApp) {
	msg := a.errors[app.ID]
	if msg == "" {
		return
	}
	p := a.palette
	ui.Row(c).FillWidth().Shrink(0).Gap(7).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, iconAlert).Size(13, 13).TextColor(readableInk(p.Bad, p))
		ui.Text(c, msg).FontSize(11).TextColor(readableInk(p.Bad, p)).Grow(1).MaxLines(2)
	})
}

// libraryPage lists what is installed on this PC, with the actions that
// matter once an app is there.
func (a *App) libraryPage(c *ui.Context) {
	var installed []suiteApp
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

func (a *App) libraryRow(c *ui.Context, app suiteApp) {
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

func (a *App) aboutPage(c *ui.Context) {
	p := a.palette
	ui.Column(c).FillWidth().MaxWidth(620).Gap(16).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "The home of the Go Things suite").FontSize(21).FontWeight(720).LetterSpacing(-0.4)
			ui.Text(c, "Go Hub reads the app list from this project's latest GitHub release, downloads only the setup you choose, opens it when you want it, and removes it again. No account or store service is involved.").
				FontSize(12.5).TextColor(p.Muted).LineHeight(1.6)
		})

		ui.Column(c).FillWidth().Gap(8).Margin(2, 0, 0, 0).Children(func() {
			a.fact(c, "Version", hubVersion)
			a.fact(c, "Platform", "Windows 10 and 11, 64-bit")
			a.fact(c, "Installs to", `%LOCALAPPDATA%\Programs, for your user only`)
			a.fact(c, "Catalog", "catalog.json in the latest GitHub release")
			a.fact(c, "Network", "Reads the catalog and downloads only the setup you install.")
		})

		ui.Text(c, "Built with Go and MyGo. The previews are screenshots of the built apps.").FontSize(10.5).TextColor(p.Muted).Margin(2, 0, 0, 0)
	})
}

func (a *App) fact(c *ui.Context, label, value string) {
	p := a.palette
	ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Text(c, label).Width(104).Shrink(0).FontSize(11).FontWeight(600).TextColor(p.Muted).SingleLine()
		ui.Text(c, value).FontSize(12).Grow(1).MaxLines(2)
	})
}
