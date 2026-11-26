package main

import "github.com/egoist/mygo/ui"

// detailPage is the product page for one app: what it does, its real
// screens, and the actions its state allows.
func (a *App) detailPage(c *ui.Context) {
	app, ok := a.appByID(a.detailID)
	if !ok {
		ui.Column(c).FillWidth().Center().Gap(8).Children(func() {
			ui.Text(c, "App not found").FontSize(15).FontWeight(650)
			b := a.quietButton(c, "Back to store", iconBack, false)
			if b.Clicked() {
				a.goPage(pageStore)
			}
		})
		return
	}
	p := a.palette

	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Start).Children(func() {
		back := a.linkButton(c, "Back to store", iconBack)
		if back.Clicked() {
			a.goPage(pageStore)
		}
	})
	ui.Box(c).Height(10).Shrink(0)

	ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
		a.appTile(c, app, 56)
		ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, app.Name).FontSize(22).FontWeight(720).LetterSpacing(-0.4).SingleLine()
				a.statePill(c, app)
				ui.Box(c).Grow(1)
				ui.Row(c).Gap(8).Shrink(0).AlignItems(ui.Center).Children(func() {
					a.actions(c, app, false)
				})
			})
			ui.Text(c, app.Tagline).FontSize(12.5).TextColor(p.Muted).MaxLines(2)
			ui.Row(c).Gap(6).Shrink(0).Margin(2, 0, 0, 0).Children(func() {
				for _, fact := range app.Facts {
					a.chip(c, fact)
				}
			})
			ui.Text(c, a.metaLine(app, true)).FontSize(10.5).TextColor(p.Muted).SingleLine().Margin(4, 0, 0, 0)
			a.errorLine(c, app)
		})
	})

	ui.Box(c).Height(12).Shrink(0)

	a.sectionLabel(c, "SCREENS", "")
	if len(app.Screenshots) > 0 {
		shots := a.shots[app.ID]
		ui.ScrollHorizontal(c).FillWidth().Height(184).Children(func() {
			for i := range app.Screenshots {
				frame := ui.Box(c).Width(288).Height(172).Shrink(0).Radius(10).Clip().Background(p.Bg).Border(1, p.Border)
				frame.Shadow(0, 10, 24, 0, ui.RGBA(0, 0, 0, 0.28))
				frame.Children(func() {
					if i < len(shots) && shots[i] != nil {
						ui.Image(c, shots[i]).Fit(ui.Cover).FillWidth().FillHeight()
					} else {
						ui.Box(c).FillWidth().FillHeight().Center().Children(func() {
							ui.Icon(c, appGlyph(app.ID)).Size(22, 22).TextColor(p.Muted)
						})
					}
				})
				ui.Box(c).Width(8).Shrink(0)
			}
		})
	}

	ui.Box(c).Height(12).Shrink(0)

	a.sectionLabel(c, "FEATURES", "")
	ui.Column(c).FillWidth().Gap(5).Children(func() {
		for _, feature := range app.Features {
			ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Size(5, 5).Radius(3).Background(func() ui.Color {
					if app.Accent != "" {
						return ui.Hex(app.Accent)
					}
					return p.Accent
				}()).Shrink(0).Margin(6, 0, 0, 0)
				ui.Text(c, feature).FontSize(12).TextColor(p.Text).Grow(1).MaxLines(2)
			})
		}
	})

	ui.Box(c).Height(12).Shrink(0)

	a.sectionLabel(c, "DETAILS", "")
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		a.fact(c, "Version", app.Version)
		a.fact(c, "Identifier", app.Identifier)
		if info, ok := a.installs[app.ID]; ok {
			a.fact(c, "Installed", info.Version)
			if info.Location != "" {
				a.fact(c, "Location", info.Location)
			}
		}
		size := "Downloaded when you install"
		if s := a.sizes[app.ID]; s != "" {
			size = s
		}
		a.fact(c, "Setup size", size)
	})
}
