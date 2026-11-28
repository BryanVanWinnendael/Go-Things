package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"go-hub"
	"go-hub/internal/catalog"
)

const sidebarWidth = float32(246)

// sidebar holds the suite's identity, the three sections, and the live
// install state of each app. It sits on the ground, not in a panel, as in
// Go Post and Go Mail.
func (a *App) sidebar(c *ui.Context) {
	p := a.palette
	ui.Column(c).Width(sidebarWidth).Shrink(0).FillHeight().Padding(8, 8, 12, 10).Gap(4).Children(func() {
		ui.Row(c).Height(38).Shrink(0).Padding(0, 6).Gap(9).AlignItems(ui.Center).DragWindow().Children(func() {
			a.brandTile(c)
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "Go Hub").FontSize(13).FontWeight(750).LetterSpacing(-0.15)
				ui.Text(c, "Go Things suite").FontSize(9).TextColor(p.Muted).LetterSpacing(0.2)
			})
		})
		ui.Column(c).Gap(2).Margin(8, 2, 0, 2).Children(func() {
			a.navItem(c, pageStore, iconStore, "Store", "")
			a.navItem(c, pageLibrary, iconLibrary, "Library", a.installedBadge())
			a.navItem(c, pageAbout, iconInfo, "About", "")
		})
		ui.Column(c).Gap(1).Margin(16, 2, 0, 2).Children(func() {
			ui.Row(c).Height(20).Padding(0, 6).Shrink(0).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "SUITE").FontSize(9.5).FontWeight(700).LetterSpacing(1.2).TextColor(p.Muted)
			})
			for _, app := range a.apps {
				a.suiteRow(c, app)
			}
		})
		ui.Box(c).Grow(1)
		a.themeToggle(c)
	})
}

// brandTile shows Go Hub's own logo from resources/icon.png. The mark is
// black, so it sits on a white face in both themes.
func (a *App) brandTile(c *ui.Context) {
	if a.hubIcon == nil {
		if bmp, err := ui.DecodeBitmap(manifest.IconPNG()); err == nil {
			a.hubIcon = bmp
		}
	}
	tile := ui.Box(c).Size(26, 26).Radius(8).Background(ui.Hex("#ffffff")).Center().Shrink(0)
	if a.hubIcon == nil {
		tile.Children(func() {
			ui.Icon(c, iconTiles).Size(15, 15).TextColor(ui.Hex("#111111"))
		})
		return
	}
	tile.Children(func() {
		ui.Image(c, a.hubIcon).Fit(ui.Contain).Size(18, 18)
	})
}

func (a *App) navItem(c *ui.Context, target page, ic *ui.SVG, label, meta string) {
	p := a.palette
	on := a.page == target
	if target == pageStore && a.page == pageDetail {
		on = true
	}
	fg := p.Muted
	b := ui.ButtonBase(c).Key(fmt.Sprintf("nav:%s", label)).FillWidth().Height(34).Padding(0, 10).Radius(9).Gap(9).Shrink(0).FocusRing(true).Label(label)
	b.Background(ui.Transparent)
	if on {
		b.Background(p.Field)
		fg = p.Text
	} else if b.Hovered() {
		b.Background(p.Field.Alpha(0.55))
		fg = p.Text
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(15, 15).TextColor(fg)
		ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(fg).Grow(1).SingleLine()
		if meta != "" {
			ui.Text(c, meta).FontSize(10.5).FontWeight(600).TextColor(p.Muted)
		}
	})
	if b.Clicked() {
		a.goPage(target)
	}
}

// suiteRow shows, at a glance, which apps are on this PC and at what
// version. Clicking it goes to the store page where the actions live.
func (a *App) suiteRow(c *ui.Context, app catalog.App) {
	p := a.palette
	info, installed := a.installs[app.ID]
	dot := p.Muted.Alpha(0.5)
	version := ""
	if installed {
		dot = p.Good
		version = info.Version
	}
	if a.busy[app.ID] != "" {
		dot = p.Warn
		version = ""
	}
	b := ui.ButtonBase(c).Key("suite:"+app.ID).FillWidth().Height(28).Padding(0, 6).Gap(8).Radius(7).Shrink(0).Label(app.Name).FocusRing(true).Tooltip(func() string {
		if installed && a.busy[app.ID] == "" {
			return "Open " + app.Name
		}
		return "View " + app.Name + " in the store"
	}())
	if b.Hovered() {
		b.Background(p.Field.Alpha(0.55))
	}
	b.Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(dot).Shrink(0)
		ui.Text(c, app.Name).FontSize(12).FontWeight(550).TextColor(p.Text).Grow(1).SingleLine()
		if version != "" {
			ui.Text(c, version).FontSize(10).TextColor(p.Muted)
		}
		if installed && a.busy[app.ID] == "" {
			ui.Icon(c, iconPlay).Size(11, 11).TextColor(p.Muted).Shrink(0)
		}
	})
	if b.Clicked() {
		if installed && a.busy[app.ID] == "" {
			a.launch(app)
		} else {
			a.goPage(pageStore)
		}
	}
}
func (a *App) installedBadge() string {
	if len(a.installs) == 0 {
		return ""
	}
	return fmt.Sprintf("%d", len(a.installs))
}

// themeToggle is a three-way choice between the desktop's theme, Nocturne
// and Daylight, kept in the sidebar so it is one click from anywhere.
func (a *App) themeToggle(c *ui.Context) {
	p := a.palette
	row := ui.Row(c).FillWidth().Height(30).Shrink(0).Padding(3).Radius(9).Gap(2).AlignItems(ui.Center)
	row.Background(p.Field)
	row.Border(1, p.Border)
	row.Children(func() {
		a.themeSegment(c, "system", "Auto", iconMonitor)
		a.themeSegment(c, "nocturne", "Night", iconMoon)
		a.themeSegment(c, "daylight", "Day", iconSun)
	})
}
func (a *App) themeSegment(c *ui.Context, id, label string, ic *ui.SVG) {
	p := a.palette
	on := a.themeSetting == id
	fg := p.Muted
	b := ui.ButtonBase(c).Key("theme:" + id).Grow(1).Height(22).Radius(7).Gap(5).Center().Shrink(0).FocusRing(true).Label(label)
	if on {
		b.Background(p.Field.Mix(p.Text, 0.10))
		fg = p.Text
	} else if b.Hovered() {
		fg = p.Text
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(11, 11).TextColor(fg)
		ui.Text(c, label).FontSize(10).FontWeight(600).TextColor(fg).SingleLine()
	})
	if b.Clicked() {
		a.setTheme(id)
	}
}

// stage is the rounded working panel beside the sidebar.
func (a *App) stage(c *ui.Context) {
	p := a.palette
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		a.stageHeader(c)
		panel := ui.Column(c).Grow(1).MinHeight(0).Margin(0, 4, 4, 4).Radius(18).Background(p.Panel).Border(1, p.Border).Clip()
		panel.Children(func() {
			ui.Scroll(c).Grow(1).FillWidth().Padding(22, 24, 28, 24).Children(func() {
				switch a.page {
				case pageStore:
					a.storePage(c)
				case pageLibrary:
					a.libraryPage(c)
				case pageAbout:
					a.aboutPage(c)
				case pageDetail:
					a.detailPage(c)
				}
			})
		})
	})
}
func (a *App) stageHeader(c *ui.Context) {
	row := ui.Row(c).Height(52).Shrink(0).Padding(0, 8, 0, 20).AlignItems(ui.Center).DragWindow()
	row.Children(func() {
		ui.Box(c).Grow(1)
		a.windowControls(c)
	})
}
