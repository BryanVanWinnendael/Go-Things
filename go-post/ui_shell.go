package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	sidebarDefaultWidth = float32(254)
	sidebarMinWidth     = float32(190)
)

// Tree rows glide to their new place after a drop instead of jumping. A spring
// with no bounce carries momentum through a second drag started mid-reflow;
// mygo settles it at once when the desktop asks for less motion. The header's
// drop highlight eases in only on the colours, never on layout.
var (
	treeRowTransition = ui.ElementTransition{
		Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 220 * time.Millisecond}},
		Position:  true,
	}
	treeHeaderTransition = ui.ElementTransition{
		Duration: 130 * time.Millisecond,
		Colors:   true,
	}
)

func (a *app) view(c *ui.Context) {
	a.viewToken = 1
	if a.palette.ID == "" {
		a.systemDark = c.Theme().Dark
	}
	p := resolvePalette(a.data.Settings, a.systemDark, a.data.CustomThemes)
	vibrancy := c.Vibrancy()
	if vibrancy {
		p = vibrancyPalette(p)
	}
	a.palette = p
	// The context starts each frame with the desktop's theme, so the app's
	// theme is set again before anything is built from it.
	a.useTheme(c, p)
	a.themeKey = fmt.Sprintf("%s|%s|%v|%.1f", p.ID, a.data.Settings.Accent, a.data.Settings.Compact, a.data.Settings.FontSize)
	a.paintChrome(c, p, vibrancy)

	c.OnShortcut(ui.Cmd, ui.KeySpace, func() {
		a.closePopovers()
		a.modal = "palette"
		a.search = ""
		a.searchThemeMode = false
		a.modalError = ""
	})
	c.OnShortcut(ui.Cmd, ui.KeyN, a.newRequest)
	c.OnShortcut(ui.Cmd, ui.KeyT, func() {
		a.closePopovers()
		a.openSettings("appearance")
	})
	c.OnShortcut(ui.Cmd, ui.KeyEnter, a.sendActive)
	c.OnShortcut(ui.Cmd, ui.KeyS, func() {
		a.toggleSidebar()
	})

	ui.Column(c).Fill().Children(func() {
		// The title bar spans the window, exactly like go-email: the collapse
		// toggle owns the top left corner and the window controls the top right.
		a.titleBar(c, p)
		// While the collapse animation runs it owns the width. Once it rests,
		// the split divider owns it: the drag writes straight into sidebarW
		// after the frame is built, so nothing here may reset it.
		if a.sidebarAnimating || a.sidebarCollapsed {
			a.sidebarW = a.sidebarAnimW
		}
		if !a.sidebarAnimating && !a.sidebarCollapsed && a.sidebarW < sidebarMinWidth {
			a.sidebarW = sidebarMinWidth
		}
		if a.sidebarW > 1 {
			split := a.splitWithHiddenDivider(c, p, vibrancy, &a.sidebarW, func() {
				a.sidebar(c)
			}, func() {
				a.mainPane(c, p)
			})
			split.Grow(1)
			if !vibrancy && !p.HasGradient {
				split.Background(p.Bg)
				hideSplitDivider(split, &a.sidebarW, p.Bg, c.Theme().Space(10))
			} else if p.HasGradient {
				split.Background(ui.Transparent)
			}
			// Remember a manual resize so expanding comes back to it instead
			// of snapping to the default width.
			if !a.sidebarAnimating && !a.sidebarCollapsed {
				a.sidebarExpandedW = a.sidebarW
			}
		} else {
			// Fully collapsed: the workspace takes the whole window, with no
			// divider or split to leave a sliver behind.
			a.sidebarW = 0
			a.sidebarAnimW = 0
			a.mainPane(c, p)
		}
	})

	a.modals(c)
	// Context menus come last so they also sit above modal inputs.
	a.contextMenu(c)
	a.autosave(c)
}

// toggleSidebar starts the collapse animation from either the title-bar button
// or Ctrl+S. The title bar syncs the animation to the width on screen on the
// next frame, so a drag is never undone by a toggle.
func (a *app) toggleSidebar() {
	a.sidebarCollapsed = !a.sidebarCollapsed
	a.sidebarAnimating = true
	a.sidebarTogglePending = true
}

// paintChrome fills the window behind the solid workspace. A chrome gradient
// lives here, not in the stage; over acrylic it is translucent so the material
// still shows through.
func (a *app) paintChrome(c *ui.Context, p palette, vibrancy bool) {
	if p.HasGradient {
		// The base stays solid (or lightly tinted over acrylic); the gradient
		// is painted behind every child so the stage and cards remain opaque.
		if vibrancy {
			c.Root().Background(p.Bg.Alpha(chromeScrimAlpha))
			c.Root().Draw(func(pt *ui.Painter, r ui.Rect) {
				paintChromeGradient(pt, r, p, chromeAcrylicGradientScale)
			})
		} else {
			c.Root().Background(p.Bg)
			c.Root().Draw(func(pt *ui.Painter, r ui.Rect) {
				paintChromeGradient(pt, r, p, 2)
			})
		}
		return
	}
	c.Root().Draw(nil)
	if vibrancy {
		// A tinted scrim keeps the material visible while muted text and icons
		// stay readable over whatever is behind the window.
		c.Root().Background(p.Bg.Alpha(chromeScrimAlpha))
		return
	}
	c.Root().Background(p.Bg)
}

// splitWithHiddenDivider builds a Split whose one-pixel line has no border
// over a material or a chrome gradient, so it blends into the chrome instead of
// drawing a solid line. The panes get the app theme back before they build;
// the line still turns accent on hover and keeps its keyboard focus ring.
func (a *app) splitWithHiddenDivider(c *ui.Context, p palette, vibrancy bool, size *float32, first, second func()) ui.Element {
	if !vibrancy && !p.HasGradient {
		return ui.Split(c, size, first, second)
	}
	appTheme := c.Theme()
	dividerTheme := *appTheme
	dividerTheme.Border = ui.Transparent
	c.SetTheme(&dividerTheme)
	restore := func() {
		c.SetTheme(appTheme)
		a.paintChrome(c, p, vibrancy)
	}
	split := ui.Split(c, size, func() {
		restore()
		first()
	}, func() {
		restore()
		second()
	})
	restore()
	return split
}

// hideSplitDivider paints over the one-pixel line a Split draws between its
// panes. The line stays focusable and draggable; only its ink goes away.
func hideSplitDivider(e ui.Element, size *float32, bg ui.Color, minPane float32) {
	e.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 0 {
			return
		}
		x := *size
		upper := r.W - 1 - minPane
		if upper < minPane {
			upper = minPane
		}
		x = min(max(x, minPane), upper)
		if x < 0 || x >= r.W {
			return
		}
		p.Fill(ui.Rect{X: r.X + x, Y: r.Y, W: 1, H: r.H}, bg, 0)
	})
}

// mainPane is the workspace: the page header over the elevated, rounded stage.
func (a *app) mainPane(c *ui.Context, p palette) {
	main := ui.Column(c).MinWidth(0).HeightPercent(100)
	main.Children(func() {
		t := c.Theme()
		// The rounded workspace starts level with the sidebar's first card, so
		// both panes sit on the same line under the title bar.
		stage := ui.Column(c).Grow(1).MinHeight(0).Margin(8, 4, 4, 4).
			Background(p.Panel).Radius(18).Border(1, t.Border).Clip()
		stage.Children(func() {
			// A new page fades and rises into place instead of teleporting.
			// The page it replaces leaves at once: two dense pages dissolving
			// into each other reads as a muddle, not as movement. Switching
			// workspaces is a change of every page at once, so it fades too.
			ui.Column(c).Key("page:" + a.page + ":" + a.data.ActiveWorkspace).Grow(1).MinHeight(0).Clip().
				Transition(ui.ElementTransition{
					Duration: 170 * time.Millisecond,
					Enter:    &ui.Motion{Y: 5, Opacity: 0},
				}).Children(func() {
				switch a.page {
				case "request":
					a.requestStage(c, func() { a.requestPage(c) })
				case "collections":
					a.pageShell(c, "collections", func() { a.collectionsPage(c) })
				case "environments":
					a.pageShell(c, "environments", func() {
						a.stageBezel(c, func() { a.environmentsPage(c) })
					})
				case "workspaces":
					a.pageShell(c, "workspaces", func() { a.workspacesPage(c) })
				case "history":
					a.pageShell(c, "history", func() {
						a.stageBezel(c, func() { a.historyPage(c) })
					})
				default:
					a.requestStage(c, func() { a.requestPage(c) })
				}
			})
		})
	})
}

func (a *app) sidebar(c *ui.Context) {
	p := a.palette
	w := a.ws()
	side := ui.Column(c).HeightPercent(100).Padding(8, 8, 12, 10).Gap(6)
	if p.HasGradient {
		side.Background(ui.Transparent)
	} else if !c.Vibrancy() {
		side.Background(p.Bg)
	}
	side.Children(func() {
		a.workspaceButton(c)

		nb := a.accentButton(c, "New request", "plus").FillWidth().Height(36)
		if nb.Clicked() {
			a.newRequest()
		}

		// Collections are the workspace: no separate navigation gets in the
		// way of them.
		section := ui.Row(c).FillWidth().Padding(4, 4).Radius(6).AlignItems(ui.Center)
		section.Children(func() {
			overview := ui.ButtonBase(c).Height(22).PaddingX(4).Radius(5).Label("Browse collections")
			if overview.Hovered() {
				overview.Background(p.Hover)
			}
			overview.Children(func() {
				a.sectionLabel(c, "Collections")
			})
			if overview.Clicked() {
				a.page = "collections"
			}
			ui.Spacer(c).Grow(1)
			if w != nil && len(w.Collections) > 0 {
				allCollapsed := true
				for i := range w.Collections {
					if !a.collapsed[w.Collections[i].ID] {
						allCollapsed = false
						break
					}
				}
				if allCollapsed {
					if a.iconButton(c, "chevrons-down-up", "Expand all", 24).Clicked() {
						a.collapsed = map[string]bool{}
					}
				} else if a.iconButton(c, "chevrons-up-down", "Collapse all", 24).Clicked() {
					for i := range w.Collections {
						a.collapsed[w.Collections[i].ID] = true
					}
				}
			}
			plus := a.iconButton(c, "plus", "New collection", 24)
			if plus.Clicked() {
				a.modal = "collection"
				a.input1 = ""
				a.modalError = ""
			}
		})
		if v, over := ui.DragOver[collectionDrag](section); over && a.canMoveCollection(v.ID, "") {
			section.Background(p.Accent.Alpha(0.10))
		}
		if v, ok := ui.Drop[collectionDrag](section); ok {
			a.moveCollection(v.ID, "")
		}

		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				if w != nil {
					a.collectionTree(c, "", 0, map[string]bool{})
					if len(w.collectionChildren("")) == 0 {
						ui.Text(c, "No collections yet").FontSize(11).TextColor(p.Muted).Padding(8)
					}
				}
			})
		})

		ui.Column(c).Gap(6).Shrink(0).Children(func() {
			a.searchJumpButton(c)
			settingsBtn := ui.ButtonBase(c).Key("settings").Padding(6, 8).Radius(8).Grow(1).FillWidth().Label("Settings")
			if settingsBtn.Hovered() {
				settingsBtn.Background(p.Hover)
			}
			settingsBtn.Children(func() {
				ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("settings", 14)).Size(14, 14).TextColor(p.Muted)
					ui.Text(c, "Settings").FontSize(12).TextColor(p.Muted).Grow(1).SingleLine()
					ui.Text(c, "Ctrl+T").FontSize(9.5).TextColor(p.Muted)
				})
			})
			if settingsBtn.Clicked() {
				a.openSettings("appearance")
			}
		})
	})
}

func (a *app) workspaceButton(c *ui.Context) {
	p := a.palette
	w := a.ws()
	name := "Workspace"
	if w != nil {
		name = w.Name
	}
	b := ui.ButtonBase(c).Key("workspace-switcher").FillWidth().Height(48).Padding(0, 9).Radius(10).Gap(9).Label("Switch workspace")
	b.Border(1, p.Border)
	b.Background(p.Field)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		ui.Box(c).Size(30, 30).Radius(9).Background(p.Accent.Alpha(0.16)).Center().Children(func() {
			ui.Text(c, firstLetter(name)).FontSize(13).FontWeight(750).TextColor(p.Accent)
		})
		ui.Column(c).Gap(0).Grow(1).Children(func() {
			ui.Text(c, "WORKSPACE").FontSize(8).FontWeight(650).LetterSpacing(0.6).TextColor(p.Muted)
			ui.Text(c, name).FontSize(12).FontWeight(650).SingleLine()
		})
		chev := ui.Icon(c, icon("chevron-down", 13)).Size(13, 13).TextColor(p.Muted)
		target := float32(0)
		if a.wsOpen {
			target = 180
		}
		chev.Rotate(chev.Animate("workspace-chevron", target, 160*time.Millisecond))
	})
	if b.Clicked() {
		a.envOpen = false
		a.wsOpen = !a.wsOpen
	}
	// MyGo lays overlays out after its transition pass, so a popover panel
	// cannot use an ElementTransition: the switcher animates a value and the
	// panel is posed from it, as go-email's account menu does.
	menuTarget := float32(0)
	if a.wsOpen {
		menuTarget = 1
	}
	menuAnim := b.Animate("workspace-menu", menuTarget, 140*time.Millisecond)
	ui.PopoverBase(c, b, &a.wsOpen, func(panel ui.Element) {
		a.menuPanel(panel, "workspace-menu", 306)
		panel.Opacity(menuAnim).Translate(0, (1-menuAnim)*-6)
		panel.Children(func() {
			a.menuHeading(c, "Workspaces", fmt.Sprintf("%d", len(a.data.Workspaces)))
			ui.Scroll(c).MaxHeight(268).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for i := range a.data.Workspaces {
						ws := &a.data.Workspaces[i]
						active := ws.ID == a.data.ActiveWorkspace
						item := ui.ButtonBase(c).Key("ws-menu:"+ws.ID).FillWidth().Height(44).Padding(0, 8).Radius(8).Gap(8).Label(ws.Name)
						if active {
							item.Background(p.Accent.Alpha(0.10))
						} else if item.Hovered() {
							item.Background(p.Hover)
						}
						item.Children(func() {
							ui.Box(c).Size(28, 28).Radius(8).Background(p.Accent.Alpha(0.14)).Center().Children(func() {
								ui.Text(c, firstLetter(ws.Name)).FontSize(11).FontWeight(750).TextColor(p.Accent)
							})
							ui.Column(c).Gap(0).Grow(1).MinWidth(0).Children(func() {
								ui.Text(c, ws.Name).FontSize(11.5).FontWeight(func() int {
									if active {
										return 650
									}
									return 540
								}()).SingleLine()
								ui.Text(c, fmt.Sprintf("%d collections · %d requests", len(ws.Collections), countRequests(ws))).FontSize(9.3).TextColor(p.Muted).SingleLine()
							})
							if active {
								ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Accent)
							}
						})
						if item.Clicked() {
							if active {
								a.wsOpen = false
							} else {
								a.switchWorkspace(ws.ID)
							}
						}
					}
				})
			})
			a.menuFooter(c, "Manage workspaces", "workspace", func() {
				a.setPage("workspaces")
			})
		})
	})
}

func (a *app) searchJumpButton(c *ui.Context) {
	p := a.palette
	b := ui.ButtonBase(c).Key("search").FillWidth().Padding(7, 9).Radius(8)
	b.Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon("search", 15)).Size(15, 15).TextColor(p.Muted)
			ui.Text(c, "Command").FontSize(12.5).TextColor(p.Muted).Grow(1)
			ui.Text(c, searchShortcut()).FontSize(9.5).TextColor(p.Muted)
		})
	})
	if b.Hovered() {
		b.Background(p.Hover)
	}
	if b.Clicked() {
		a.closePopovers()
		a.modal = "palette"
		a.search = ""
		a.searchThemeMode = false
		a.modalError = ""
	}
}

func (a *app) menuPanel(panel ui.Element, key string, width float32) {
	p := a.palette
	panel.Key(key).Width(width).Padding(8).Gap(4).Background(p.Panel).Radius(13).
		Border(1, p.Border).Shadow(0, 18, 48, 0, p.Shadow).Margin(6, 0, 0, 0)
}

func (a *app) menuHeading(c *ui.Context, label, count string) {
	p := a.palette
	ui.Row(c).FillWidth().Height(24).PaddingX(4).AlignItems(ui.Center).Children(func() {
		a.sectionLabel(c, label).Grow(1)
		ui.Text(c, count).FontSize(9.5).TextColor(p.Muted)
	})
}

func (a *app) menuFooter(c *ui.Context, label, ic string, fn func()) {
	p := a.palette
	b := ui.ButtonBase(c).FillWidth().Height(32).Padding(0, 8).Radius(7).Gap(7).Label(label)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		ui.Icon(c, icon(ic, 13)).Size(13, 13).TextColor(p.Muted)
		ui.Text(c, label).FontSize(10.8).TextColor(p.Muted).Grow(1)
		ui.Icon(c, icon("chevron-right", 11)).Size(11, 11).TextColor(p.Muted)
	})
	if b.Clicked() {
		fn()
	}
}

type collectionDrag struct {
	ID string
}

type requestDrag struct {
	CollectionID string
	RequestID    string
}

func (a *app) collectionTree(c *ui.Context, parentID string, depth int, seen map[string]bool) {
	w := a.ws()
	if w == nil {
		return
	}
	for _, col := range w.collectionChildren(parentID) {
		if seen[col.ID] {
			continue
		}
		seen[col.ID] = true
		a.collectionPreview(c, col, depth, seen)
	}
}

func (a *app) collectionPreview(c *ui.Context, col *Collection, depth int, seen map[string]bool) {
	p := a.palette
	open := !a.collapsed[col.ID]
	parts := ui.CollapsibleBase(c.Key("col:"+col.ID), &open)
	indent := float32(depth) * 14

	head := parts.Trigger.FillWidth().Height(28).PaddingStart(6 + indent).PaddingEnd(6).
		Radius(6).Gap(7).AlignItems(ui.Center).Label(col.Name).Transition(treeHeaderTransition)
	if head.RightClicked() {
		a.openContextMenuAt(head, a.collectionMenu(col))
	}
	head.Drag(collectionDrag{ID: col.ID})
	if head.Hovered() {
		head.Background(p.Hover)
	}
	head.Children(func() {
		ui.Icon(c, icon("chevron-down", 11)).Size(11, 11).TextColor(p.Muted).
			Rotate(90*parts.Progress()-90).TransformOrigin(0.5, 0.5)
		ui.Icon(c, icon("folder", 13)).Size(13, 13).TextColor(p.Muted)
		ui.Text(c, col.Name).FontSize(11.5).FontWeight(560).Grow(1).SingleLine()
		ui.Text(c, fmt.Sprintf("%d", a.ws().collectionRequestCount(col.ID))).FontSize(9.5).TextColor(p.Muted)
	})

	panel := parts.Panel(func() {
		a.collectionTree(c, col.ID, depth+1, seen)
		// A drop changes col.Requests, so apply it after the loop instead of
		// mutating the slice while the rows are still being built from it.
		var pending requestDrag
		pendingAt, hasDrop := 0, false
		for ri := range col.Requests {
			r := &col.Requests[ri]
			active := a.ws().ActiveTab == r.ID
			textCol := p.Muted
			if active {
				textCol = p.Text
			}
			btn := ui.ButtonBase(c).Key("req:" + r.ID).FillWidth().Height(27).PaddingStart(30 + indent).
				PaddingEnd(6).Radius(6).Gap(7).Label(r.Name).Transition(treeRowTransition)
			if btn.RightClicked() {
				a.openContextMenuAt(btn, a.requestMenu(c, r, col))
			}
			btn.Drag(requestDrag{CollectionID: col.ID, RequestID: r.ID})

			// The row is its own drop target. Which half of it the pointer is
			// over decides whether the request lands above or below it, so a
			// drag can reorder inside a folder or move into another one.
			insert := -1
			if v, ok := ui.DragOver[requestDrag](btn); ok && v.RequestID != r.ID {
				bounds := btn.Bounds()
				_, py, over := btn.PointerPosition()
				if over && bounds.H > 0 && py >= bounds.H/2 {
					insert = ri + 1
				} else {
					insert = ri
				}
			}
			after := ui.Local(btn, "treeDropAfter", func() bool { return false })
			if insert >= 0 {
				*after = insert == ri+1
			}
			lineTarget := float32(0)
			if insert >= 0 {
				lineTarget = 1
			}
			line := btn.Animate("treeDropLine", lineTarget, 140*time.Millisecond)
			btn.DrawOver(func(pt *ui.Painter, box ui.Rect) {
				if line <= 0.01 {
					return
				}
				y := box.Y
				if *after {
					y = box.Y + box.H - 2
				}
				pt.Fill(ui.Rect{X: box.X + 4, Y: y, W: box.W - 8, H: 2}, p.Accent.Alpha(line), 1)
			})
			if v, ok := ui.Drop[requestDrag](btn); ok && !hasDrop {
				pending, pendingAt, hasDrop = v, ri, true
				if _, py, over := btn.PointerPosition(); over && btn.Bounds().H > 0 && py >= btn.Bounds().H/2 {
					pendingAt = ri + 1
				}
			}

			if active {
				btn.Background(p.Field)
				btn.Border(1, p.Border)
			} else if btn.Hovered() {
				btn.Background(p.Hover)
			}
			btn.Children(func() {
				a.dot(c, methodColor(p, r.Method), 4)
				ui.Text(c, r.Name).FontSize(11).FontWeight(func() int {
					if active {
						return 620
					}
					return 480
				}()).TextColor(textCol).Grow(1).SingleLine()
			})
			if btn.Clicked() {
				a.openRequest(r.ID)
			}
		}
		if hasDrop {
			a.reorderRequest(pending.CollectionID, pending.RequestID, col.ID, pendingAt)
		}
	})
	panel.Gap(1)

	dropActive := false
	if v, over := ui.DragOver[any](head); over && a.canDropOnCollection(v, col.ID) {
		dropActive = true
	}
	if v, over := ui.DragOver[any](panel); over && a.canDropOnCollection(v, col.ID) {
		dropActive = true
	}
	if dropActive {
		head.Background(p.Accent.Alpha(0.14))
	}
	if v, ok := ui.Drop[any](head); ok {
		a.dropOnCollection(v, col.ID)
	}
	if v, ok := ui.Drop[any](panel); ok {
		a.dropOnCollection(v, col.ID)
	}

	if head.Changed() {
		a.collapsed[col.ID] = !open
	}
}

func (a *app) collectionMenu(col *Collection) []contextMenuItem {
	items := []contextMenuItem{
		menuAction("New request", "plus", func() {
			a.addRequestTo(col)
		}),
		menuAction("New folder", "folder", func() {
			a.modal = "folder"
			a.modalParent = col.ID
			a.input1 = ""
			a.modalError = ""
		}),
		menuAction("Authorization…", "shield", func() {
			a.openCollectionAuth(col)
		}),
		menuAction("Rename", "edit", func() {
			a.modal = "rename_collection"
			a.input1 = col.Name
			a.modalTarget = col.ID
			a.modalError = ""
		}),
	}
	if col.ParentID != "" {
		items = append(items, menuAction("Move to top level", "arrow-left", func() {
			a.moveCollection(col.ID, "")
		}))
	}
	kind := "collection"
	if col.ParentID != "" {
		kind = "folder"
	}
	id, name := col.ID, col.Name
	items = append(items, menuSeparator(), menuDanger("Delete", "trash", func() {
		a.openConfirm("Delete "+kind+"?", "“"+name+"” and everything inside it will be removed.", func() {
			a.deleteCollection(id)
		})
	}))
	return items
}

func (a *app) requestMenu(c *ui.Context, r *Request, col *Collection) []contextMenuItem {
	items := []contextMenuItem{
		menuAction("Open request", "arrow-up-right", func() {
			a.openRequest(r.ID)
		}),
		menuAction("Rename request", "edit", func() {
			a.modal = "rename_request"
			a.input1 = r.Name
			a.modalTarget = r.ID
			a.modalError = ""
		}),
		menuAction("Duplicate", "copy", func() {
			a.duplicateRequest(r.ID)
		}),
		{Label: "Move to", Icon: "folder", Submenu: a.moveRequestMenu(r, col)},
		menuAction("Save now", "save", func() {
			a.save()
			a.toast(c, "Request saved", r.Name, "success")
		}),
		menuSeparator(),
	}
	id, name, colName := r.ID, r.Name, col.Name
	items = append(items, menuDanger("Delete request", "trash", func() {
		a.openConfirm("Delete request?", "“"+name+"” will be removed from "+colName+".", func() {
			a.deleteRequest(id)
		})
	}))
	return items
}

func (a *app) moveRequestMenu(r *Request, col *Collection) []contextMenuItem {
	w := a.ws()
	if w == nil {
		return nil
	}
	var items []contextMenuItem
	for i := range w.Collections {
		target := &w.Collections[i]
		if target.ID == col.ID {
			continue
		}
		label := w.collectionPath(target.ID)
		items = append(items, menuAction(label, "folder", func() {
			a.moveRequest(col.ID, r.ID, target.ID)
		}))
	}
	if len(items) == 0 {
		items = append(items, contextMenuItem{Label: "No other collections", Icon: "folder", Disabled: true})
	}
	return items
}

func (a *app) canDropOnCollection(v any, targetID string) bool {
	switch v := v.(type) {
	case requestDrag:
		return v.CollectionID != targetID
	case collectionDrag:
		return a.canMoveCollection(v.ID, targetID)
	}
	return false
}

func (a *app) dropOnCollection(v any, targetID string) {
	switch v := v.(type) {
	case requestDrag:
		a.moveRequest(v.CollectionID, v.RequestID, targetID)
	case collectionDrag:
		a.moveCollection(v.ID, targetID)
	}
}

// titleBar is the window chrome above the panes: the sidebar toggle in the top
// left corner, the page's actions and the window controls in the top right,
// matching go-email. Keeping the toggle here, outside the sidebar, means it
// survives the collapse animation and can always bring the sidebar back.
func (a *app) titleBar(c *ui.Context, p palette) {
	ui.Row(c).Height(40).Shrink(0).Padding(0, 12, 0, 10).Gap(9).
		AlignItems(ui.Center).DragWindow().Children(func() {
		toggle := ui.ButtonBase(c).Key("toggleSidebar").Size(30, 30).Radius(7).Center().
			Label("Toggle sidebar").Tooltip("Toggle sidebar").FocusRing(false)
		if toggle.Hovered() {
			toggle.Background(p.Hover)
		}
		// Animate only while the toggle is actually opening or closing the
		// sidebar. Calling Animate on every frame would keep pulling a width
		// the user just dragged back to its target.
		if a.sidebarAnimating {
			if a.sidebarTogglePending {
				// Begin from the width currently on screen, which may have
				// been resized by dragging the divider.
				a.sidebarAnimW = toggle.Jump("sidebar-width", a.sidebarW)
				a.sidebarTogglePending = false
			}
			widthTarget := a.sidebarExpandedW
			if a.sidebarCollapsed {
				widthTarget = 0
			}
			a.sidebarAnimW = toggle.Animate("sidebar-width", widthTarget, 200*time.Millisecond)
			if math.Abs(float64(a.sidebarAnimW-widthTarget)) <= 0.5 {
				a.sidebarAnimW = widthTarget
				a.sidebarAnimating = false
				if !a.sidebarCollapsed {
					a.sidebarW = widthTarget
				}
			}
		}
		toggle.Children(func() {
			ui.Icon(c, icon("sidebar", 14)).Size(14, 14).TextColor(p.Muted)
		})
		if toggle.Clicked() {
			a.toggleSidebar()
		}
		ui.Spacer(c).Grow(1)
		a.pageActions(c)
		a.windowControls(c, a.win)
	})
}

// pageActions is the right side of the title bar: the environment switcher and
// whatever the current page can do, next to the window controls.
func (a *app) pageActions(c *ui.Context) {
	a.environmentButton(c)
	switch a.page {
	case "collections":
		if a.accentButton(c, "New collection", "plus").Clicked() {
			a.modal = "collection"
			a.input1 = ""
			a.modalError = ""
		}
	case "environments":
		if a.accentButton(c, "New environment", "plus").Clicked() {
			a.modal = "env"
			a.input1 = ""
			a.modalError = ""
		}
	case "workspaces":
		if a.accentButton(c, "New workspace", "plus").Clicked() {
			a.modal = "new_workspace"
			a.input1 = ""
			a.modalError = ""
		}
	case "history":
		w := a.ws()
		if w != nil && len(w.History) > 0 {
			if a.plainButton(c, "Clear", "trash").Clicked() {
				a.openConfirm("Clear history?", "All saved responses in this workspace will be removed. Collections and requests stay.", func() {
					w.History = nil
					a.save()
				})
			}
		}
	default:
		a.requestActions(c)
	}
}

// windowControls draws the minimize, maximize and close buttons. The window is
// frameless, so these are ours to draw and they stay legible in every theme.
func (a *app) windowControls(c *ui.Context, win *mygo.Window) {
	maxIcon := "maximize"
	maxLabel := "Maximize"
	if win != nil && win.IsMaximized() {
		maxIcon = "restore"
		maxLabel = "Restore"
	}
	ui.Row(c).Gap(2).Shrink(0).Children(func() {
		min := a.windowControl(c, "window:minimize", "minimize", "Minimize", false)
		if min.Clicked() && win != nil {
			win.Minimize()
		}
		max := a.windowControl(c, "window:"+maxIcon, maxIcon, maxLabel, false)
		if max.Clicked() && win != nil {
			win.ToggleMaximize()
		}
		close := a.windowControl(c, "window:close", "x", "Close", true)
		if close.Clicked() && win != nil {
			a.closePopovers()
			win.Close()
		}
	})
}

func (a *app) windowControl(c *ui.Context, key, name, label string, danger bool) ui.Element {
	p := a.palette
	b := ui.ButtonBase(c).Key(key).Size(36, 30).Radius(7).Center().Label(label).Tooltip(label).FocusRing(false)
	iconColor := p.Muted
	if b.Hovered() {
		iconColor = p.Text
		if danger {
			b.Background(p.Bad)
			iconColor = contrastText(p.Bad)
		} else {
			b.Background(p.Hover)
		}
	}
	if b.Pressed() {
		if danger {
			b.Background(p.Bad.Mix(ui.Hex("#000000"), 0.12))
			iconColor = contrastText(p.Bad)
		} else {
			b.Background(p.Border)
		}
	}
	b.Children(func() {
		ui.Icon(c, icon(name, 14)).Size(14, 14).TextColor(iconColor)
	})
	return b
}

func (a *app) envName() string {
	w := a.ws()
	if w == nil {
		return ""
	}
	if e := w.environment(w.ActiveEnv); e != nil {
		return e.Name
	}
	return "No environment"
}

func (a *app) scrollPage(c *ui.Context, fn func()) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(22, 24, 28, 24).Gap(16).MaxWidth(1080).Children(fn)
	})
}

// stageBezel gives tool-style pages the same inset between the rounded
// stage border and their content as the scrolling pages and the settings
// windows, so the stage reads identically in every app.
func (a *app) stageBezel(c *ui.Context, fn func()) {
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(22, 24, 28, 24).Children(fn)
}

// requestStage keeps the request page's tab strip against the stage's top
// border. The tabs are the first row of the page, so the usual top bezel reads
// as an empty band above them.
func (a *app) requestStage(c *ui.Context, fn func()) {
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Padding(0, 24, 28, 24).Children(fn)
}

// pageMeta names a secondary page and gives it the icon used for it elsewhere
// in the app, so its header matches the menus and empty states.
func pageMeta(page string) (title, ic string) {
	switch page {
	case "collections":
		return "Collections", "folder"
	case "environments":
		return "Environments", "environment"
	case "workspaces":
		return "Workspaces", "workspace"
	case "history":
		return "History", "history"
	}
	return "", ""
}

// pageShell puts a secondary page under its own header. The request page is
// the home of the app, so only pages you navigate away to get one: it names
// where you are and gives the way back a fixed, labelled place of its own
// instead of a glyph lost among the window chrome.
func (a *app) pageShell(c *ui.Context, page string, body func()) {
	title, ic := pageMeta(page)
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		a.pageHeader(c, title, ic)
		body()
	})
}

func (a *app) pageHeader(c *ui.Context, title, ic string) {
	p := a.palette
	ui.Row(c).FillWidth().Height(48).Shrink(0).Padding(0, 16).Gap(11).
		AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(p.Border).
		Children(func() {
			// A bordered chip, not a bare glyph: the arrow alone was too easy
			// to mistake for the sidebar toggle beside it and too easy to miss.
			back := ui.ButtonBase(c).Key("page-back").Height(30).Padding(0, 10).Radius(8).Gap(6).
				Label("Back").Tooltip("Back to requests").FocusRing(true).
				Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
			back.Background(p.Field)
			back.Border(1, p.Border)
			if back.Hovered() {
				back.Background(p.Hover)
				back.Border(1, p.Muted.Alpha(0.45))
			}
			back.Children(func() {
				ui.Icon(c, icon("arrow-left", 14)).Size(14, 14).TextColor(p.Text)
				ui.Text(c, "Back").FontSize(12).FontWeight(620).TextColor(p.Text)
			})
			if back.Clicked() {
				a.setPage("request")
			}
			ui.Box(c).Width(1).Height(18).Shrink(0).Background(p.Border)
			ui.Icon(c, icon(ic, 15)).Size(15, 15).TextColor(p.Muted)
			ui.Text(c, title).FontSize(13.5).FontWeight(650).TextColor(p.Text)
		})
}
