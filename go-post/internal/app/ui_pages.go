package app

import (
	"fmt"
	"go-post/internal/core"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *app) collectionsPage(c *ui.Context) {
	p := a.palette
	w := a.ws()
	if w == nil {
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(22, 24, 28, 24).Gap(14).MaxWidth(1060).Children(func() {
			if len(w.Collections) == 0 {
				if a.emptyState(c, "folder", "No collections yet", "A collection keeps related requests together. Start with one for your API.", "New collection") {
					a.modal = "collection"
					a.input1 = ""
				}
				return
			}
			for ci := range w.Collections {
				col := &w.Collections[ci]
				card := a.card(c).FillWidth().Key("collection:" + col.ID).
					Transition(ui.ElementTransition{
						Duration: 160 * time.Millisecond,
						Colors:   true,
						Enter:    &ui.Motion{Collapse: true, Opacity: 0},
						Exit:     &ui.Motion{Collapse: true, Opacity: 0},
					})
				if card.RightClicked() {
					a.openContextMenuAt(card, a.collectionMenu(col))
				}
				card.Children(func() {
					top := ui.Row(c).Height(50).Shrink(0).Padding(0, 14).Gap(9).AlignItems(ui.Center)
					if top.RightClicked() {
						a.openContextMenuAt(top, a.collectionMenu(col))
					}
					top.Children(func() {
						folder := ui.Box(c).Size(28, 28).Radius(8).Background(p.Accent.Alpha(0.10)).Center()
						folder.Drag(collectionDrag{ID: col.ID})
						folder.Children(func() {
							ui.Icon(c, icon("folder", 15)).Size(15, 15).TextColor(p.Accent)
						})
						info := ui.Column(c).Gap(0).Grow(1)
						info.Drag(collectionDrag{ID: col.ID})
						info.Children(func() {
							ui.Text(c, col.Name).FontSize(12.5).FontWeight(650).SingleLine()
							subtitle := fmt.Sprintf("%d requests", w.CollectionRequestCount(col.ID))
							if parent := w.Collection(col.ParentID); parent != nil {
								subtitle = "in " + parent.Name + " · " + subtitle
							}
							ui.Text(c, subtitle).FontSize(10).TextColor(p.Muted)
						})
						add := a.iconButton(c, "plus", "Add request", 28)
						if col.AuthType == "bearer" || col.AuthType == "basic" {
							a.badge(c, col.AuthType, p.Muted)
						}
						if add.Clicked() {
							a.addRequestTo(col)
						}
						more := a.iconButton(c, "more", "Collection actions", 28).Role(ui.RoleMenuButton)
						if more.Clicked() {
							a.openContextMenuBelow(more, a.collectionMenu(col))
						}
					})
					if len(col.Requests) == 0 && len(w.CollectionChildren(col.ID)) == 0 {
						ui.Row(c).FillWidth().Height(42).Padding(0, 14).AlignItems(ui.Center).
							BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Children(func() {
							if a.addRow(c, "Add the first request") {
								a.addRequestTo(col)
							}
						})
					}
					for ri := range col.Requests {
						r := &col.Requests[ri]
						row := ui.ButtonBase(c).Key("colreq:"+r.ID).FillWidth().Height(43).Padding(0, 14).Gap(10).
							BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Label(r.Name)
						if row.RightClicked() {
							a.openContextMenuAt(row, a.requestMenu(c, r, col))
						}
						row.Drag(requestDrag{CollectionID: col.ID, RequestID: r.ID})
						if row.Hovered() {
							row.Background(p.Field.Alpha(0.65))
						}
						row.Children(func() {
							a.methodChip(c, r.Method, 56)
							ui.Column(c).Gap(0).Grow(1).MinWidth(0).Children(func() {
								ui.Text(c, r.Name).FontSize(11.5).FontWeight(560).SingleLine().Ellipsis(r.Name)
								ui.Text(c, r.URL).FontSize(9.8).Font("monospace").TextColor(p.Muted).SingleLine().Ellipsis(r.URL)
							})
							if auth := w.ResolveAuth(r, col); auth.Type != "none" {
								badgeColor := p.Warn
								if r.AuthType == "inherit" || r.AuthType == "" {
									badgeColor = p.Muted
								}
								a.badge(c, auth.Type, badgeColor)
							}
							ui.Icon(c, icon("chevron-right", 12)).Size(12, 12).TextColor(p.Muted)
						})
						if row.Clicked() {
							a.openRequest(r.ID)
						}
					}
				})
				if v, over := ui.DragOver[any](card); over && a.canDropOnCollection(v, col.ID) {
					card.BorderColor(p.Accent)
				}
				if v, ok := ui.Drop[any](card); ok {
					a.dropOnCollection(v, col.ID)
				}
			}
		})
	})
}

func (a *app) deleteCollection(id string) {
	w := a.ws()
	if w == nil {
		return
	}
	remove := map[string]bool{}
	var mark func(string)
	mark = func(cid string) {
		if remove[cid] {
			return
		}
		remove[cid] = true
		for _, child := range w.CollectionChildren(cid) {
			mark(child.ID)
		}
	}
	mark(id)

	kept := w.Collections[:0]
	for _, col := range w.Collections {
		if !remove[col.ID] {
			kept = append(kept, col)
			continue
		}
		for _, r := range col.Requests {
			w.CloseTab(r.ID)
			delete(a.responses, r.ID)
		}
		delete(a.collapsed, col.ID)
	}
	w.Collections = kept
	a.save()
}

func (a *app) environmentsPage(c *ui.Context) {
	p := a.palette
	w := a.ws()
	if w == nil {
		return
	}
	ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
		// Environment list.
		ui.Column(c).Width(262).Shrink(0).FillHeight().Padding(14, 12).Gap(6).
			BorderWidth(0, 1, 0, 0).BorderColor(p.Border).Children(func() {
			ui.Row(c).Height(24).PaddingX(4).Children(func() {
				a.sectionLabel(c, "Variable sets").Grow(1)
				ui.Text(c, fmt.Sprintf("%d", len(w.Environments))).FontSize(10).TextColor(p.Muted)
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					for i := range w.Environments {
						e := &w.Environments[i]
						on := w.ActiveEnv == e.ID
						b := ui.ButtonBase(c).Key("env:"+e.ID).FillWidth().Height(46).Padding(0, 10).Radius(9).Gap(9).Label(e.Name).
							Transition(ui.ElementTransition{
								Duration: 150 * time.Millisecond,
								Colors:   true,
								Enter:    &ui.Motion{Collapse: true, Opacity: 0},
								Exit:     &ui.Motion{Collapse: true, Opacity: 0},
							})
						if on {
							b.Background(p.Accent.Alpha(0.10))
							b.Border(1, p.Accent.Alpha(0.22))
						} else if b.Hovered() {
							b.Background(p.Field)
						}
						b.Children(func() {
							a.dot(c, func() ui.Color {
								if on {
									return p.Accent
								}
								return p.Muted.Alpha(0.35)
							}(), 7)
							ui.Column(c).Gap(0).Grow(1).Children(func() {
								ui.Text(c, e.Name).FontSize(11.8).FontWeight(func() int {
									if on {
										return 620
									}
									return 500
								}()).SingleLine()
								ui.Text(c, fmt.Sprintf("%d variables", len(e.Vars))).FontSize(9.5).TextColor(p.Muted)
							})
							if on {
								ui.Text(c, "ACTIVE").FontSize(8).FontWeight(700).TextColor(p.Accent)
							}
						})
						if b.Clicked() {
							w.ActiveEnv = e.ID
							a.save()
						}
					}
					if len(w.Environments) == 0 {
						ui.Column(c).Padding(10).Gap(8).Children(func() {
							ui.Text(c, "No environments").FontSize(11).TextColor(p.Muted)
							if a.addRow(c, "Create one") {
								a.modal = "env"
								a.input1 = ""
							}
						})
					}
				})
			})
		})

		// Variables of the active environment.
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			e := w.Environment(w.ActiveEnv)
			if e == nil {
				if a.emptyState(c, "environment", "No environment selected", "Create an environment and add variables such as baseUrl and token.", "New environment") {
					a.modal = "env"
					a.input1 = ""
				}
				return
			}
			ui.Row(c).Height(54).Shrink(0).Padding(0, 18).Gap(10).AlignItems(ui.Center).
				BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
				a.textInput(c, &e.Name).Height(32).Width(220).FontSize(12.5).FontWeight(600).Label("Environment name")
				if w.ActiveEnv == e.ID {
					a.badge(c, "active", p.Good)
				}
				ui.Spacer(c).Grow(1)
				del := a.iconButton(c, "trash", "Delete environment", 30)
				if del.Clicked() {
					id, name := e.ID, e.Name
					a.openConfirm("Delete environment?", "“"+name+"” and its variables will be removed.", func() {
						for i := range w.Environments {
							if w.Environments[i].ID == id {
								w.Environments = append(w.Environments[:i], w.Environments[i+1:]...)
								if w.ActiveEnv == id {
									w.ActiveEnv = ""
									if len(w.Environments) > 0 {
										w.ActiveEnv = w.Environments[0].ID
									}
								}
								a.save()
								return
							}
						}
					})
				}
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(18, 20, 24, 20).Gap(3).Children(func() {
					a.envVarHeader(c)
					for i := range e.Vars {
						v := &e.Vars[i]
						idx := i
						reveal := a.envReveal != "" && v.Key == a.envReveal && !a.envRevealDone
						row := ui.Row(c).FillWidth().Gap(7).Padding(3, 2).Children(func() {
							ui.Checkbox(c, &v.Enabled, "").Width(22).Label("Enabled")
							name := a.textInput(c, &v.Key).Placeholder("Key").Height(32).Width(220).Font("monospace").FontSize(11).Label("Variable name")
							if reveal {
								// A variable followed here from a request gets the focus,
								// which also brings its row into view.
								a.inputHandle(name.ID()).Focus(c)
								a.envRevealDone = true
							}
							var val ui.Element
							if v.Secret && !a.reveal[v.ID] {
								val = a.secretInput(c, &v.Value)
							} else {
								val = a.textInput(c, &v.Value)
							}
							val = val.Placeholder("Value").Height(32).Grow(1).Font("monospace").FontSize(11).Label("Variable value")
							secret := v.Secret
							sb := a.iconButton(c, "lock", "Mark as secret", 28)
							if secret {
								sb.Background(p.Accent.Alpha(0.10))
							}
							if sb.Clicked() {
								v.Secret = !v.Secret
								a.save()
							}
							if v.Secret {
								eyeName := "eye"
								if a.reveal[v.ID] {
									eyeName = "eye-off"
								}
								if a.iconButton(c, eyeName, "Show or hide value", 28).Clicked() {
									a.reveal[v.ID] = !a.reveal[v.ID]
								}
							} else {
								ui.Box(c).Width(28)
							}
							if a.iconButton(c, "x", "Remove variable", 28).Clicked() {
								e.Vars = append(e.Vars[:idx], e.Vars[idx+1:]...)
								a.save()
							}
						}).Key(fmt.Sprintf("envvar:%s:%d", e.ID, idx))
						if reveal {
							row.Background(p.Accent.Alpha(0.10)).Radius(8)
						}
					}
					ui.Row(c).Margin(4, 0, 0, 0).Children(func() {
						if a.addRow(c, "Add variable") {
							e.Vars = append(e.Vars, core.KV{ID: core.NewID("v"), Enabled: true})
						}
					})
					ui.Row(c).Margin(18, 0, 0, 0).Gap(7).Children(func() {
						ui.Icon(c, icon("braces", 12)).Size(12, 12).TextColor(p.Accent)
						a.keyValueHint(c, "Reference variables anywhere with {{name}}. Secret values are hidden on screen but still sent in requests.")
					})
				})
			})
		})
	})
}

func (a *app) envVarHeader(c *ui.Context) {
	p := a.palette
	ui.Row(c).FillWidth().Height(24).Padding(4, 2).Children(func() {
		ui.Box(c).Width(26)
		ui.Text(c, "Variable").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Width(220)
		ui.Text(c, "Value").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Grow(1)
		ui.Box(c).Width(96)
	})
}

func (a *app) pickPFX(c *ui.Context) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      a.settingsWin,
		Title:       "Import a client certificate",
		ButtonLabel: "Choose PFX",
		Filters:     []mygo.FileFilter{{Name: "PKCS#12 certificates", Extensions: []string{"pfx", "p12"}}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	a.pendingCertPath = paths[0]
	a.pendingCertInfo = nil
	a.certTestOK = false
	a.modal = "cert_import"
	a.input1 = ""
	a.input2 = ""
	a.input3 = strings.TrimSuffix(filepath.Base(paths[0]), filepath.Ext(paths[0]))
	a.modalError = ""
	a.modalInfo = paths[0]
}

func expiryState(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	if time.Now().After(t) {
		return "expired"
	}
	if time.Until(t) < 30*24*time.Hour {
		return "soon"
	}
	return "ok"
}

func prettyDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Format("2 Jan 2006")
}

func (a *app) workspacesPage(c *ui.Context) {
	p := a.palette
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(22, 24, 28, 24).Gap(12).MaxWidth(1000).Children(func() {
			ui.Text(c, "Each workspace keeps its own collections, environments, open tabs and history.").
				FontSize(10.8).TextColor(p.Muted).Wrap()
			for i := range a.data.Workspaces {
				ws := &a.data.Workspaces[i]
				active := ws.ID == a.data.ActiveWorkspace
				card := a.card(c).FillWidth().Key("workspace:" + ws.ID).
					Transition(ui.ElementTransition{
						Duration: 160 * time.Millisecond,
						Colors:   true,
						Enter:    &ui.Motion{Collapse: true, Opacity: 0},
						Exit:     &ui.Motion{Collapse: true, Opacity: 0},
					})
				if active {
					card.BorderColor(p.Accent.Alpha(0.28))
				}
				card.Children(func() {
					ui.Row(c).FillWidth().Height(74).Padding(0, 16).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(38, 38).Radius(10).Background(p.Accent.Alpha(0.14)).Center().Children(func() {
							ui.Text(c, firstLetter(ws.Name)).FontSize(15).FontWeight(750).TextColor(p.Accent)
						})
						ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, ws.Name).FontSize(13).FontWeight(650).SingleLine()
								if active {
									a.badge(c, "active", p.Good)
								}
							})
							ui.Text(c, fmt.Sprintf("%d collections · %d requests · %d environments",
								len(ws.Collections), countRequests(ws), len(ws.Environments))).
								FontSize(9.8).TextColor(p.Muted).SingleLine()
						})
						if !active {
							if a.plainButton(c, "Open", "").Clicked() {
								a.switchWorkspace(ws.ID)
							}
						}
						if a.iconButton(c, "edit", "Rename workspace", 30).Clicked() {
							a.modal = "rename_workspace"
							a.modalTarget = ws.ID
							a.input1 = ws.Name
							a.modalError = ""
						}
						if len(a.data.Workspaces) > 1 {
							if a.iconButton(c, "trash", "Delete workspace", 30).Clicked() {
								id, name := ws.ID, ws.Name
								a.openConfirm("Delete workspace?", "“"+name+"” and everything inside it will be removed.", func() {
									a.deleteWorkspace(id)
								})
							}
						}
					})
				})
			}
		})
	})
}

func (a *app) historyPage(c *ui.Context) {
	p := a.palette
	w := a.ws()
	if w == nil {
		return
	}
	ui.Row(c).Height(50).Shrink(0).Padding(0, 18).AlignItems(ui.Center).Gap(10).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
		sf := ui.Row(c).Width(300).Height(32).Padding(0, 10).Gap(7).Radius(8).
			Background(p.Field).Border(1, p.Border).AlignItems(ui.Center)
		sf.Children(func() {
			ui.Icon(c, icon("search", 13)).Size(13, 13).TextColor(p.Muted)
			a.textInput(c, &a.search).Placeholder("Filter history").Height(30).Grow(1).FontSize(12).Label("Filter history")
			if a.search != "" {
				if a.iconButton(c, "x", "Clear filter", 22).Clicked() {
					a.search = ""
				}
			}
		})
		ui.Text(c, fmt.Sprintf("%d calls", len(w.History))).FontSize(10.5).TextColor(p.Muted)
	})
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(14, 16, 22, 16).Gap(2).MaxWidth(1100).Children(func() {
			n := 0
			for i := range w.History {
				h := &w.History[i]
				q := strings.ToLower(strings.TrimSpace(a.search))
				if q != "" && !strings.Contains(strings.ToLower(h.URL+" "+h.Method), q) {
					continue
				}
				n++
				row := ui.ButtonBase(c).Key("hist:"+h.ID).FillWidth().Height(46).Padding(0, 12).Gap(10).Radius(8).Label(h.URL)
				if row.Hovered() {
					row.Background(p.Field)
				}
				row.Children(func() {
					a.methodChip(c, h.Method, 54)
					ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, h.URL).FontSize(11).Font("monospace").TextColor(p.Text).SingleLine().Ellipsis(h.URL)
						ui.Text(c, prettyHistoryTime(h.At)).FontSize(9.5).TextColor(p.Muted)
					})
					if h.Status > 0 {
						a.statusPill(c, h.Status)
					}
					ui.Text(c, core.ShortDuration(h.Duration)).FontSize(9.5).TextColor(p.Muted).Width(54).TextAlign(ui.End)
					ui.Text(c, core.ShortSize(h.Size)).FontSize(9.5).TextColor(p.Muted).Width(52).TextAlign(ui.End)
				})
				if row.Clicked() {
					a.replayHistory(*h)
				}
			}
			if n == 0 {
				a.emptyState(c, "history", "No matching calls", "Send a request, or clear the filter to see older calls.", "")
			}
		})
	})
}

func prettyHistoryTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	if time.Since(t) < 24*time.Hour {
		return t.Format("15:04:05")
	}
	return t.Format("2 Jan 2006 · 15:04")
}

func (a *app) replayHistory(h core.History) {
	w := a.ws()
	if r, _ := w.Request(h.RequestID); r != nil {
		a.openRequest(r.ID)
		return
	}
	r := h.Request
	r.ID = core.NewID("req")
	if r.Name == "" {
		r.Name = "History replay"
	}
	col := w.Collection("col_starter")
	if col == nil {
		for i := range w.Collections {
			if w.Collections[i].Name == "Scratch" && w.Collections[i].ParentID == "" {
				col = &w.Collections[i]
			}
		}
		if col == nil {
			w.Collections = append(w.Collections, core.Collection{ID: core.NewID("col"), Name: "Scratch"})
			col = &w.Collections[len(w.Collections)-1]
		}
	}
	for i := range r.Headers {
		r.Headers[i].ID = core.NewID("h")
	}
	col.Requests = append(col.Requests, r)
	w.OpenTab(r.ID)
	a.page = "request"
	a.save()
}

func (a *app) settingsSection(c *ui.Context, title, subtitle string) {
	p := a.palette
	ui.Column(c).Gap(3).Margin(10, 0, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(680)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(10.5).TextColor(p.Muted).Wrap()
		}
	})
}

func (a *app) newThemeCard(c *ui.Context) {
	p := a.palette
	b := ui.ButtonBase(c).Key("theme:new").Column().Height(94).Padding(10).Radius(11).Gap(7).Label("New custom theme").
		Bind(&a.themePickerCardRef)
	b.Background(p.Field)
	b.Border(1, p.Border)
	if b.Hovered() {
		b.Border(1, p.Text.Alpha(0.16))
	}
	b.Children(func() {
		ui.Box(c).FillWidth().Height(44).Radius(7).Center().Background(p.Bg.Alpha(0.35)).
			Border(1, p.Border).Children(func() {
			ui.Icon(c, icon("plus", 18)).Size(18, 18).TextColor(p.Accent)
		})
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "New custom theme").FontSize(10.8).FontWeight(600).Grow(1).SingleLine().Ellipsis("New custom theme")
			ui.Text(c, "gradient").FontSize(8.5).TextColor(p.Muted)
		})
	})
	if b.Clicked() {
		a.openThemePicker(nil, "card")
	}
}

func (a *app) themeCard(c *ui.Context, id, name string, display palette) {
	p := a.palette
	if display.Accent.A == 0 {
		if accent, ok := presetAccents[id]; ok {
			display.Accent = accentColor(accent)
		} else {
			display.Accent = p.Accent
		}
	}
	selected := a.data.Settings.Theme == id || (a.data.Settings.Theme == "" && id == "system")
	b := ui.ButtonBase(c).Key("theme:" + id).Column().Height(94).Padding(10).Radius(11).Gap(7).Label(name)
	b.Background(p.Field)
	b.Border(1, func() ui.Color {
		if selected {
			return p.Accent
		}
		return p.Border
	}())
	if b.Hovered() && !selected {
		b.Border(1, p.Text.Alpha(0.16))
	}
	b.Children(func() {
		ui.Row(c).FillWidth().Gap(5).Children(func() {
			art := ui.Box(c).Grow(1).Height(44).Radius(7).Clip().Border(1, p.Border).Background(display.Bg)
			if display.HasGradient {
				art.Draw(func(pt *ui.Painter, r ui.Rect) {
					paintChromeGradient(pt, r, display, 1)
				})
			}
			art.Children(func() {
				ui.Box(c).Margin(5).WidthPercent(42).HeightPercent(70).Radius(5).Background(display.Panel)
			})
			ui.Column(c).Width(44).Gap(4).Children(func() {
				ui.Box(c).FillWidth().Height(18).Radius(4).Background(display.Field)
				ui.Box(c).FillWidth().Height(18).Radius(4).Background(display.Accent)
			})
		})
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, name).FontSize(10.8).FontWeight(600).Grow(1).SingleLine().Ellipsis(name)
			if selected {
				ui.Icon(c, icon("check", 12)).Size(12, 12).TextColor(p.Accent)
			} else {
				note := map[bool]string{true: "dark", false: "light"}[display.Dark]
				if display.HasGradient {
					note = "gradient"
				} else if strings.HasPrefix(id, "theme_") {
					note = "custom"
				}
				ui.Text(c, note).FontSize(8.5).TextColor(p.Muted)
			}
		})
	})
	if b.RightClicked() && id != "system" {
		custom := customThemeByID(a.data.CustomThemes, id)
		items := []contextMenuItem{}
		if custom != nil {
			ct := custom
			items = append(items, menuAction("Edit", "palette", func() {
				a.openThemePicker(ct, "edit")
			}), menuSeparator())
		}
		items = append(items, menuAction("Duplicate", "copy", func() {
			a.copyTheme(id)
		}))
		if custom != nil {
			name := custom.Name
			items = append(items, menuSeparator(), menuDanger("Delete", "trash", func() {
				a.openConfirm("Delete theme?", "“"+name+"” will be removed. Your workspaces and requests stay untouched.", func() {
					a.deleteCustomTheme(id)
				})
			}))
		}
		a.openContextMenuAtToken(2, b, items)
	}
	if b.Clicked() {
		a.setTheme(id)
	}
}
