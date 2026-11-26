package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// contextMenuItem is one row of the app's own context menu. The menu is drawn
// in the window so it follows the selected theme instead of the desktop's.
type contextMenuItem struct {
	Label     string
	Icon      string
	Shortcut  string
	Checked   bool
	Disabled  bool
	Danger    bool
	Separator bool
	Submenu   []contextMenuItem
	Action    func()
}

type contextMenuState struct {
	open    bool
	token   int
	x, y    float32
	items   []contextMenuItem
	submenu int
}

func menuAction(label, ic string, fn func()) contextMenuItem {
	return contextMenuItem{Label: label, Icon: ic, Action: fn}
}

func menuSeparator() contextMenuItem {
	return contextMenuItem{Separator: true}
}

func menuDanger(label, ic string, fn func()) contextMenuItem {
	return contextMenuItem{Label: label, Icon: ic, Danger: true, Action: fn}
}

func (a *app) openContextMenu(items []contextMenuItem, x, y float32) {
	a.openContextMenuToken(a.viewToken, items, x, y)
}

func (a *app) openContextMenuToken(token int, items []contextMenuItem, x, y float32) {
	// Only one floating surface is open at a time.
	a.envOpen = false
	a.wsOpen = false
	a.materialOpen = false
	if len(items) == 0 {
		a.closeContextMenu()
		return
	}
	a.ctxMenu = contextMenuState{open: true, token: token, x: x, y: y, items: items, submenu: -1}
}

func (a *app) closeContextMenu() {
	a.ctxMenu.open = false
	a.ctxMenu.submenu = -1
}

func (a *app) openContextMenuAt(el ui.Element, items []contextMenuItem) {
	a.openContextMenuAtToken(a.viewToken, el, items)
}

func (a *app) openContextMenuAtToken(token int, el ui.Element, items []contextMenuItem) {
	b := el.Bounds()
	x, y, ok := el.PointerPosition()
	if !ok {
		x, y = b.W/2, b.H/2
	}
	a.openContextMenuToken(token, items, b.X+x, b.Y+y)
}

func (a *app) openContextMenuBelow(el ui.Element, items []contextMenuItem) {
	b := el.Bounds()
	a.openContextMenu(items, b.X, b.Y+b.H+4)
}

func (a *app) methodMenu(r *Request) []contextMenuItem {
	items := make([]contextMenuItem, 0, len(httpMethods))
	for _, method := range httpMethods {
		m := method
		items = append(items, contextMenuItem{
			Label:   m,
			Checked: r.Method == m,
			Action: func() {
				r.Method = m
			},
		})
	}
	return items
}

func (a *app) contextMenu(c *ui.Context) {
	// Drive the open animation from the root (overlays are laid out after
	// MyGo's transition pass, so the panel itself cannot use one).
	target := float32(0)
	if a.ctxMenu.open {
		target = 1
	}
	a.ctxMenuAnim = c.Root().Animate("context-menu", target, 120*time.Millisecond)
	if !a.ctxMenu.open || len(a.ctxMenu.items) == 0 || a.ctxMenu.token != a.viewToken {
		return
	}
	p := a.palette
	c.OnShortcut(0, ui.KeyEscape, a.closeContextMenu)

	const menuWidth = float32(228)
	const submenuWidth = float32(244)
	winW, winH := c.Size()
	height := contextMenuHeight(a.ctxMenu.items)
	x := clampMenuPos(a.ctxMenu.x, 8, winW-menuWidth-8)
	y := clampMenuPos(a.ctxMenu.y, 8, winH-height-8)

	ui.Overlay(c, func() {
		backdrop := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Label("Close menu").Background(ui.RGBA(0, 0, 0, 0.001)).
			OnClick(a.closeContextMenu)
		if backdrop.Hovered() {
			a.ctxMenu.submenu = -1
		}
		if backdrop.RightClicked() {
			a.closeContextMenu()
			return
		}

		panel := ui.Column(c).Key("context-menu").Role(ui.RoleMenu).Label("Context menu").Width(menuWidth).
			Padding(6).Gap(1).Background(p.Panel).Radius(11).Border(1, p.Border).
			Shadow(0, 18, 48, 0, p.Shadow).OnClick(func() {}).
			Absolute().Left(x).Top(y).
			Opacity(a.ctxMenuAnim).TransformOrigin(0.5, 0.5).
			Scale(0.96+0.04*a.ctxMenuAnim, 0.96+0.04*a.ctxMenuAnim)

		submenu := a.ctxMenu.submenu
		var submenuTarget ui.Element
		var submenuItems []contextMenuItem
		submenuTitle := ""
		hasSubmenu := false

		panel.Children(func() {
			for i := range a.ctxMenu.items {
				it := &a.ctxMenu.items[i]
				if it.Separator {
					ui.Row(c).Height(1).FillWidth().Background(p.Border).Margin(4, 4, 4, 4)
					continue
				}
				row := a.contextMenuRow(c, it)
				if i == submenu && len(it.Submenu) > 0 {
					row.Background(p.Hover)
				}
				if row.Hovered() {
					if !it.Disabled && len(it.Submenu) > 0 {
						submenu = i
						submenuTarget = row
						submenuItems = it.Submenu
						submenuTitle = it.Label
						hasSubmenu = true
					} else {
						submenu = -1
					}
				} else if i == submenu {
					submenuTarget = row
					submenuItems = it.Submenu
					submenuTitle = it.Label
					hasSubmenu = len(it.Submenu) > 0
				}
				if row.Clicked() && !it.Disabled && len(it.Submenu) == 0 && it.Action != nil {
					a.closeContextMenu()
					it.Action()
				}
			}
		})

		if hasSubmenu && len(submenuItems) > 0 {
			a.contextSubmenu(c, submenuTarget, submenuTitle, submenuItems, submenuWidth)
		}
		a.ctxMenu.submenu = submenu
	})
}

func (a *app) contextMenuRow(c *ui.Context, it *contextMenuItem) ui.Element {
	p := a.palette
	row := ui.ButtonBase(c).FillWidth().Height(30).PaddingX(9).Radius(6).Gap(8).
		AlignItems(ui.Center).Label(it.Label).Role(ui.RoleMenuItem)
	if it.Disabled {
		row.Disabled(true)
	} else if row.Hovered() {
		if it.Danger {
			row.Background(p.Bad.Alpha(0.10))
		} else {
			row.Background(p.Hover)
		}
	}
	textColor := p.Text
	iconColor := p.Muted
	switch {
	case it.Disabled:
		textColor = p.Muted.Alpha(0.5)
		iconColor = p.Muted.Alpha(0.5)
	case it.Danger:
		textColor = p.Bad
		iconColor = p.Bad
	}
	row.Children(func() {
		if it.Checked {
			ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Accent)
		} else if it.Icon != "" {
			ui.Icon(c, icon(it.Icon, 13)).Size(13, 13).TextColor(iconColor)
		} else {
			ui.Box(c).Size(13, 13)
		}
		ui.Text(c, it.Label).FontSize(11.2).TextColor(textColor).Grow(1).SingleLine().Ellipsis(it.Label)
		if it.Shortcut != "" {
			ui.Text(c, it.Shortcut).FontSize(9.5).TextColor(p.Muted)
		}
		if len(it.Submenu) > 0 {
			ui.Icon(c, icon("chevron-right", 11)).Size(11, 11).TextColor(p.Muted)
		}
	})
	return row
}

func (a *app) contextSubmenu(c *ui.Context, target ui.Element, title string, items []contextMenuItem, width float32) {
	p := a.palette
	panel := ui.Column(c).Key("context-submenu").Role(ui.RoleMenu).Label(title+" menu").Width(width).
		Padding(6).Gap(1).Background(p.Panel).Radius(11).Border(1, p.Border).
		Shadow(0, 18, 48, 0, p.Shadow).OnClick(func() {})
	panel.AttachTo(target, ui.AnchorRight, ui.AnchorLeft)
	panel.Children(func() {
		a.menuHeading(c, title, "")
		ui.Scroll(c).MaxHeight(300).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				for i := range items {
					it := &items[i]
					row := a.contextMenuRow(c, it)
					if row.Clicked() && !it.Disabled && it.Action != nil {
						a.closeContextMenu()
						it.Action()
					}
				}
			})
		})
	})
}

func contextMenuHeight(items []contextMenuItem) float32 {
	h := float32(12) // panel padding
	if len(items) > 1 {
		h += float32(len(items) - 1) // gaps between rows
	}
	for _, it := range items {
		if it.Separator {
			h += 9
		} else {
			h += 30
		}
	}
	return h
}

func clampMenuPos(v, lo, hi float32) float32 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
