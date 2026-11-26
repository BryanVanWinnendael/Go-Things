package main

import "github.com/egoist/mygo/ui"

// accentButton is the suite's primary action: a white face on Nocturne and
// a black face on Daylight, each with the ink that reads on it.
func (a *App) accentButton(c *ui.Context, label string, ic *ui.SVG) ui.Element {
	p := a.palette
	fg := contrastText(p.Accent)
	b := ui.ButtonBase(c).Height(34).Padding(0, 14).Radius(8).Gap(7).Shrink(0).FocusRing(true).Label(label)
	b.Background(p.Accent)
	b.TextColor(fg)
	if b.Hovered() {
		b.Background(p.accentState(0.10))
	}
	if b.Pressed() {
		b.Background(p.accentState(0.20))
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(13, 13).TextColor(fg)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(650).TextColor(fg).SingleLine()
	})
	return b
}

// quietButton is the second action: a bordered face that never competes
// with the primary one. With danger, it is the quieter half of an uninstall.
func (a *App) quietButton(c *ui.Context, label string, ic *ui.SVG, danger bool) ui.Element {
	p := a.palette
	fg := p.Text
	b := ui.ButtonBase(c).Height(34).Padding(0, 12).Radius(8).Gap(7).Shrink(0).FocusRing(true).Label(label)
	b.Background(ui.Transparent)
	b.Border(1, p.Border)
	if danger {
		fg = p.Bad
		b.Border(1, p.Bad.Alpha(0.38))
		if b.Hovered() {
			b.Background(p.Bad.Alpha(0.10))
		}
	} else if b.Hovered() {
		b.Background(p.Hover)
	}
	if b.Pressed() {
		b.Background(p.Hover.Mix(p.Text, 0.05))
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(13, 13).TextColor(fg)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(600).TextColor(fg).SingleLine()
	})
	return b
}

// dangerButton is the filled removal action shown after a confirmation.
func (a *App) dangerButton(c *ui.Context, label string, ic *ui.SVG) ui.Element {
	p := a.palette
	fg := contrastText(p.Bad)
	b := ui.ButtonBase(c).Height(34).Padding(0, 13).Radius(8).Gap(7).Shrink(0).FocusRing(true).Label(label)
	b.Background(p.Bad)
	b.TextColor(fg)
	if b.Hovered() {
		b.Background(p.Bad.Mix(p.Text, 0.10))
	}
	if b.Pressed() {
		b.Background(p.Bad.Mix(ui.Hex("#000000"), 0.16))
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(13, 13).TextColor(fg)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(650).TextColor(fg).SingleLine()
	})
	return b
}

// iconButton is a square, quiet control for refresh and folder actions.
func (a *App) iconButton(c *ui.Context, key string, ic *ui.SVG, tip string) ui.Element {
	p := a.palette
	b := ui.ButtonBase(c).Key(key).Size(30, 30).Radius(7).Center().Shrink(0).Label(tip).Tooltip(tip).FocusRing(true)
	fg := p.Muted
	if b.Hovered() {
		b.Background(p.Hover)
		fg = p.Text
	}
	if b.Pressed() {
		b.Background(p.Hover.Mix(p.Text, 0.08))
		fg = p.Text
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(15, 15).TextColor(fg)
	})
	return b
}

// linkButton is a ghost navigation control: a chevron plus a label, no box.
// It carries navigation, not action, so it stays quiet at rest and lifts
// only on hover. It never stretches to its parent's width.
func (a *App) linkButton(c *ui.Context, label string, ic *ui.SVG) ui.Element {
	p := a.palette
	fg := p.Muted
	b := ui.ButtonBase(c).Key("link:"+label).Height(26).Padding(0, 8).Radius(7).Gap(5).Shrink(0).Center().FocusRing(true).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(p.Hover)
		fg = p.Text
	}
	if b.Pressed() {
		b.Background(p.Hover.Mix(p.Text, 0.08))
		fg = p.Text
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(14, 14).TextColor(fg)
		}
		ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(fg).SingleLine()
	})
	return b
}

// chip states one fact about an app in a compact pill.
func (a *App) chip(c *ui.Context, text string) ui.Element {
	p := a.palette
	e := ui.Row(c).Height(24).Padding(0, 9).Radius(7).Gap(5).Shrink(0).AlignItems(ui.Center)
	e.Background(p.Panel)
	e.Border(1, p.Border)
	e.Children(func() {
		ui.Text(c, text).FontSize(10.5).FontWeight(550).TextColor(p.Muted).SingleLine()
	})
	return e
}

// sectionLabel heads a group of cards or rows.
func (a *App) sectionLabel(c *ui.Context, label, meta string) {
	p := a.palette
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(10).FontWeight(700).LetterSpacing(1.2).TextColor(p.Muted)
		ui.Box(c).Grow(1)
		if meta != "" {
			ui.Text(c, meta).FontSize(11).TextColor(p.Muted)
		}
	})
}

// windowControls draws minimize, maximize and close on the frameless window.
func (a *App) windowControls(c *ui.Context) {
	win := a.win
	maxIcon := iconMaximize
	maxName := "maximize"
	maxLabel := "Maximize"
	if win != nil && win.IsMaximized() {
		maxIcon = iconRestore
		maxName = "restore"
		maxLabel = "Restore"
	}
	ui.Row(c).Gap(2).Shrink(0).Children(func() {
		min := a.windowControl(c, "window:minimize", iconMinimize, "Minimize", false)
		if min.Clicked() && win != nil {
			win.Minimize()
		}
		max := a.windowControl(c, "window:"+maxName, maxIcon, maxLabel, false)
		if max.Clicked() && win != nil {
			win.ToggleMaximize()
		}
		close := a.windowControl(c, "window:close", iconX, "Close", true)
		if close.Clicked() && win != nil {
			win.Close()
		}
	})
}

func (a *App) windowControl(c *ui.Context, key string, ic *ui.SVG, label string, danger bool) ui.Element {
	p := a.palette
	b := ui.ButtonBase(c).Key(key).Size(36, 30).Radius(7).Center().Shrink(0).Label(label).Tooltip(label).FocusRing(false)
	fg := p.Muted
	if b.Hovered() {
		fg = p.Text
		if danger {
			b.Background(p.Bad)
			fg = contrastText(p.Bad)
		} else {
			b.Background(p.Hover)
		}
	}
	if b.Pressed() {
		if danger {
			b.Background(p.Bad.Mix(ui.Hex("#000000"), 0.12))
			fg = contrastText(p.Bad)
		} else {
			b.Background(p.Border)
		}
	}
	b.Children(func() {
		ui.Icon(c, ic).Size(14, 14).TextColor(fg)
	})
	return b
}
