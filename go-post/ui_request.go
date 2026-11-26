package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

var httpMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// requestActions is the request page's toolbar in the title bar: history, copy
// URL, and the request menu.
func (a *app) requestActions(c *ui.Context) {
	w := a.ws()
	var r *Request
	var col *Collection
	if w != nil {
		r, col = a.activeRequest()
	}
	hist := a.iconButton(c, "history", "History", 30)
	if hist.Clicked() {
		a.page = "history"
	}
	if r != nil {
		cp := a.iconButton(c, "copy", "Copy URL", 30)
		if cp.Clicked() {
			c.WriteClipboard(r.URL)
			a.toast(c, "URL copied", r.URL, "success")
		}
		more := a.iconButton(c, "more", "Request actions", 30).Role(ui.RoleMenuButton)
		if more.Clicked() {
			a.openContextMenuBelow(more, a.requestMenu(c, r, col))
		}
	}
}

func (a *app) environmentButton(c *ui.Context) {
	p := a.palette
	w := a.ws()
	b := ui.ButtonBase(c).Key("environment-switcher").Height(28).Padding(0, 9).Radius(7).Gap(6).Label("Environment")
	b.Background(p.Field)
	b.Border(1, p.Border)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		a.dot(c, p.Good, 6)
		ui.Text(c, a.envName()).FontSize(10.5).FontWeight(550).TextColor(p.Muted)
		chev := ui.Icon(c, icon("chevron-down", 10)).Size(10, 10).TextColor(p.Muted)
		target := float32(0)
		if a.envOpen {
			target = 180
		}
		chev.Rotate(chev.Animate("environment-chevron", target, 160*time.Millisecond))
	})
	if b.Clicked() {
		a.wsOpen = false
		a.envOpen = !a.envOpen
	}
	// The panel is posed from a value the switcher animates: MyGo lays
	// overlays out after its transition pass, so an ElementTransition on the
	// panel would never run.
	menuTarget := float32(0)
	if a.envOpen {
		menuTarget = 1
	}
	menuAnim := b.Animate("environment-menu", menuTarget, 140*time.Millisecond)
	ui.PopoverBase(c, b, &a.envOpen, func(panel ui.Element) {
		a.menuPanel(panel, "environment-menu", 286)
		panel.AttachTo(b, ui.AnchorBottomEnd, ui.AnchorTopEnd)
		panel.Opacity(menuAnim).Translate(0, (1-menuAnim)*-6)
		panel.Children(func() {
			count := "0"
			if w != nil {
				count = fmt.Sprintf("%d", len(w.Environments))
			}
			a.menuHeading(c, "Environment", count)
			ui.Scroll(c).MaxHeight(268).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					if w != nil {
						active := w.ActiveEnv == ""
						none := ui.ButtonBase(c).Key("env-menu:none").FillWidth().Height(40).Padding(0, 8).Radius(8).Gap(8).Label("No environment")
						if active {
							none.Background(p.Accent.Alpha(0.10))
						} else if none.Hovered() {
							none.Background(p.Hover)
						}
						none.Children(func() {
							ui.Icon(c, icon("circle", 12)).Size(12, 12).TextColor(p.Muted)
							ui.Column(c).Gap(0).Grow(1).MinWidth(0).Children(func() {
								ui.Text(c, "No environment").FontSize(11.3).FontWeight(540).SingleLine()
								ui.Text(c, "No variables applied").FontSize(9.3).TextColor(p.Muted).SingleLine()
							})
							if active {
								ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Accent)
							}
						})
						if none.Clicked() {
							w.ActiveEnv = ""
							a.envOpen = false
							a.save()
						}
						for i := range w.Environments {
							e := &w.Environments[i]
							on := w.ActiveEnv == e.ID
							item := ui.ButtonBase(c).Key("env-menu:"+e.ID).FillWidth().Height(40).Padding(0, 8).Radius(8).Gap(8).Label(e.Name)
							if on {
								item.Background(p.Accent.Alpha(0.10))
							} else if item.Hovered() {
								item.Background(p.Hover)
							}
							item.Children(func() {
								a.dot(c, p.Good, 6)
								ui.Column(c).Gap(0).Grow(1).MinWidth(0).Children(func() {
									ui.Text(c, e.Name).FontSize(11.3).FontWeight(func() int {
										if on {
											return 650
										}
										return 540
									}()).SingleLine()
									ui.Text(c, fmt.Sprintf("%d variables", len(e.Vars))).FontSize(9.3).TextColor(p.Muted).SingleLine()
								})
								if on {
									ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Accent)
								}
							})
							if item.Clicked() {
								w.ActiveEnv = e.ID
								a.envOpen = false
								a.save()
							}
						}
						if len(w.Environments) == 0 {
							ui.Text(c, "No environments yet").FontSize(10.3).TextColor(p.Muted).Padding(8, 6)
						}
					}
				})
			})
			ui.Box(c).FillWidth().Height(1).Background(p.Border)
			a.menuFooter(c, "Manage environments", "environment", func() {
				a.setPage("environments")
			})
		})
	})
}

func (a *app) requestPage(c *ui.Context) {
	w := a.ws()
	if w == nil {
		a.emptyState(c, "send", "No workspace", "Create a workspace to start sending requests.", "New request")
		return
	}

	r, col := a.activeRequest()
	a.openTabs(c)
	if r == nil {
		clicked := a.emptyState(c, "send", "Nothing open yet", "Pick a request from a collection, or start with a blank one.", "New request")
		if clicked {
			a.newRequest()
		}
		return
	}
	ui.SplitVerticalPercent(c, &a.split, func() {
		a.requestEditor(c, r, col)
	}, func() {
		a.responsePane(c, r)
	}).Grow(1).MinHeight(0)
}

func (a *app) openTabs(c *ui.Context) {
	p := a.palette
	w := a.ws()
	ui.Row(c).Height(38).Shrink(0).Padding(0, 8).Gap(5).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
		ui.ScrollHorizontal(c).Grow(1).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				for _, id := range w.OpenTabs {
					r, _ := w.request(id)
					if r == nil {
						continue
					}
					on := w.ActiveTab == id
					// Tabs slide open and closed by their own width, and their
					// active colors fade rather than snap.
					tab := ui.Row(c).Key("tab:"+id).Height(28).Shrink(0).Radius(7).Gap(1).Padding(0, 4, 0, 2).
						Transition(ui.ElementTransition{
							Duration: 150 * time.Millisecond,
							Colors:   true,
							Enter:    &ui.Motion{Collapse: true, Opacity: 0},
							Exit:     &ui.Motion{Collapse: true, Opacity: 0},
						})
					// The border is always drawn so activating a tab fades its
					// outline in instead of shifting the row by a pixel.
					tab.Border(1, ui.Transparent)
					if on {
						tab.Background(p.Field)
						tab.Border(1, p.Border)
					}
					tab.Children(func() {
						label := ui.ButtonBase(c).Height(26).Padding(0, 6).Gap(6).Label(r.Name)
						if !on && label.Hovered() {
							tab.Background(p.Hover)
						}
						label.Children(func() {
							a.dot(c, methodColor(p, r.Method), 5)
							ui.Text(c, r.Name).FontSize(10.8).FontWeight(func() int {
								if on {
									return 620
								}
								return 480
							}()).MaxWidth(126).SingleLine().Ellipsis(r.Name).TextColor(func() ui.Color {
								if on {
									return p.Text
								}
								return p.Muted
							}())
						})
						if label.Clicked() {
							w.ActiveTab = id
							a.page = "request"
						}
						x := a.iconButton(c, "x", "Close tab", 21)
						if x.Clicked() {
							w.closeTab(id)
							a.save()
						}
					})
				}
			})
		})
		plus := a.iconButton(c, "plus", "New request", 26)
		if plus.Clicked() {
			a.newRequest()
		}
	})
}

func (a *app) requestEditor(c *ui.Context, r *Request, col *Collection) {
	p := a.palette
	t := c.Theme()
	pane := ui.Column(c).MinHeight(0).Clip().Children(func() {
		// URL bar.
		ui.Row(c).Height(58).Shrink(0).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
			mc := methodColor(p, r.Method)
			method := ui.ButtonBase(c).Key("method").Role(ui.RoleMenuButton).Width(104).Height(34).Padding(0, 10).Radius(8).Gap(6).Label("Method")
			method.Border(1, p.Border).Background(mc.Alpha(0.12))
			if method.Hovered() {
				method.Background(mc.Alpha(0.20))
			}
			method.Children(func() {
				ui.Text(c, r.Method).FontSize(11).FontWeight(650).TextColor(mc).Grow(1)
				ui.Icon(c, icon("chevron-down", 11)).Size(11, 11).TextColor(p.Muted)
			})
			if method.Clicked() {
				a.openContextMenuBelow(method, a.methodMenu(r))
			}
			url := a.textInput(c, &r.URL).Placeholder("https://api.example.com/endpoint").
				Height(34).Grow(1).FontSize(12.2).Font("monospace").Label("Request URL")
			url = a.variableField(c, url, &r.URL, "monospace", 12.2, "url:"+r.ID, nil, true)
			// Typing a query into the URL fills the Params tab, and typing in
			// the tab rewrites the URL: the two are one value.
			if url.Changed() {
				syncQueryFromURL(r)
			}
			if a.sending[r.ID] {
				btn := ui.ButtonBase(c).Height(34).Padding(0, 14).Radius(8).Gap(7).Disabled(true).Label("Sending")
				btn.Background(p.Accent.Alpha(0.55))
				btn.Children(func() {
					ui.Spinner(c).Size(13, 13)
					ui.Text(c, "Sending…").FontSize(12).FontWeight(650).TextColor(contrastText(p.Accent))
				})
			} else {
				send := a.accentButton(c, "Send", "send").Height(34)
				if send.Clicked() {
					a.sendActive()
				}
			}
			save := a.iconButton(c, "save", "Save request", 34)
			if save.Clicked() {
				a.save()
				a.toast(c, "Request saved", r.Name, "success")
			}
		})

		// Headers, body, auth and options.
		ui.Row(c).Height(38).Shrink(0).Padding(0, 12).Gap(2).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
			headers := 0
			for _, h := range r.Headers {
				if h.Enabled {
					headers++
				}
			}
			params := 0
			for _, q := range r.Query {
				if q.Enabled {
					params++
				}
			}
			tabs := []struct {
				label string
				count string
			}{
				{"Params", fmt.Sprint(params)},
				{"Headers", fmt.Sprint(headers)},
				{"Body", func() string {
					if r.BodyType == "none" {
						return ""
					}
					return "•"
				}()},
				{"Auth", func() string {
					if a.ws().resolveAuth(r, col).Type == "none" {
						return ""
					}
					return "•"
				}()},
				{"Options", ""},
			}
			for i, tab := range tabs {
				tb := a.tabButton(c, tab.label, a.reqTab == i, tab.count)
				if tb.Clicked() {
					a.reqTab = i
				}
			}
			ui.Spacer(c).Grow(1)
			ui.Text(c, fmt.Sprintf("%d params · %d headers", params, headers)).FontSize(10).TextColor(p.Muted)
		})

		// The active editor tab fades in over the cleared pane, so switching
		// reads as a change of view rather than a jump cut.
		ui.Column(c).Key(fmt.Sprintf("req-tab:%d", a.reqTab)).Grow(1).MinHeight(0).Clip().
			Transition(ui.ElementTransition{
				Duration: 130 * time.Millisecond,
				Enter:    &ui.Motion{Y: 4, Opacity: 0},
			}).Children(func() {
			switch a.reqTab {
			case 0:
				a.paramsTab(c, r)
			case 1:
				a.headersTab(c, r)
			case 2:
				a.bodyTab(c, r)
			case 3:
				a.authTab(c, r, col)
			default:
				a.optionsTab(c, r)
			}
		})
		_ = t
	})
	_ = pane
}

func (a *app) paramsTab(c *ui.Context, r *Request) {
	p := a.palette
	ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(12, 16, 18, 16).Gap(4).Children(func() {
			a.paramHeader(c)
			a.paramRows(c, r)
			ui.Row(c).Padding(4, 2).Gap(4).Children(func() {
				if a.addRow(c, "Add parameter") {
					r.Query = append(r.Query, KV{ID: newID("q"), Key: "", Value: "", Enabled: true})
					a.save()
				}
			})
			ui.Row(c).Margin(12, 0, 0, 0).Gap(8).Children(func() {
				ui.Icon(c, icon("braces", 13)).Size(13, 13).TextColor(p.Accent)
				a.keyValueHint(c, "Parameters stay in sync with the URL. Enabled rows are sent with the request, using {{variables}}.")
			})
		})
	})
}

func (a *app) headersTab(c *ui.Context, r *Request) {
	p := a.palette
	ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(12, 16, 18, 16).Gap(4).Children(func() {
			a.kvHeader(c)
			a.kvRows(c, &r.Headers, "req-h", func() {})
			ui.Row(c).Padding(4, 2).Gap(4).Children(func() {
				if a.addRow(c, "Add header") {
					r.Headers = append(r.Headers, KV{ID: newID("h"), Key: "", Value: "", Enabled: true})
				}
			})
			ui.Row(c).Margin(12, 0, 0, 0).Gap(8).Children(func() {
				ui.Icon(c, icon("braces", 13)).Size(13, 13).TextColor(p.Accent)
				a.keyValueHint(c, "Use {{variables}} from the active environment in keys and values. Header values are sent exactly as written.")
			})
		})
	})
}

func (a *app) bodyTab(c *ui.Context, r *Request) {
	p := a.palette
	ui.Column(c).Grow(1).MinHeight(0).Padding(12, 16, 16, 16).Gap(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Body type").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Width(66)
			types := []string{"None", "JSON", "Text", "XML"}
			idx := map[string]int{"none": 0, "json": 1, "text": 2, "xml": 3}[r.BodyType]
			if seg := ui.Segmented(c, &idx, types...).Height(30).Label("Body type"); seg.Changed() {
				r.BodyType = []string{"none", "json", "text", "xml"}[idx]
			}
			ui.Spacer(c).Grow(1)
			if r.BodyType != "none" {
				ui.Text(c, fmt.Sprintf("%d bytes", len(r.Body))).FontSize(10).TextColor(p.Muted)
			}
			if r.BodyType == "json" && strings.TrimSpace(r.Body) != "" {
				if a.smallButton(c, "Pretty", "sparkles", "Format the JSON body").Clicked() {
					if pretty, err := prettyJSONBody(r.Body); err != nil {
						a.toast(c, "Cannot format the body", "This is not valid JSON yet. Fix the syntax and try Pretty again.", "error")
					} else {
						r.Body = pretty
						a.save()
					}
				}
			}
		})
		if r.BodyType == "none" {
			ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
				ui.Icon(c, icon("file", 27)).Size(28, 28).TextColor(p.Muted.Alpha(0.6))
				ui.Text(c, "This request sends no body").FontSize(12).TextColor(p.Muted)
			})
		} else {
			ta := a.textArea(c, &r.Body).Placeholder("Write the request body…").Font("monospace").FontSize(12).
				FillWidth().Grow(1).Lines(5, 40).Label("Request body")
			if r.BodyType == "json" {
				ta = ta.TextRanges(a.bodyRanges(r.ID, r.Body, syntaxColors(p))...)
			}
			ta = a.variableField(c, ta, &r.Body, "monospace", 12, "body:"+r.ID, nil, false)
			_ = ta
		}
		ui.Row(c).Gap(7).Children(func() {
			ui.Icon(c, icon("sparkles", 12)).Size(12, 12).TextColor(p.Accent)
			a.keyValueHint(c, "Environment variables are substituted before sending. JSON is colored as you type, here and in responses; Pretty indents the body.")
		})
	})
}

func (a *app) authTab(c *ui.Context, r *Request, col *Collection) {
	p := a.palette
	w := a.ws()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(16, 18, 20, 18).Gap(14).MaxWidth(620).Children(func() {
			ui.Text(c, "Authentication").FontSize(13).FontWeight(650)
			ui.Text(c, "Requests use the nearest folder's authorization unless they set their own. Headers from here are added after the headers tab, so they win when both set Authorization.").
				FontSize(11).TextColor(p.Muted).Wrap()

			opts := make([]string, 0, len(authOptions))
			keys := make([]string, 0, len(authOptions))
			for _, opt := range authOptions {
				opts = append(opts, opt.label)
				keys = append(keys, opt.key)
			}
			cur := 0
			if r.AuthType != "" {
				cur = authOptionIndex(r.AuthType)
			}
			choice := opts[cur]
			if sel := ui.Select(c, &choice, opts).Width(240).Height(32).Label("Auth type"); sel.Changed() {
				for i, opt := range opts {
					if opt == choice {
						r.AuthType = keys[i]
					}
				}
			}

			switch r.AuthType {
			case "inherit", "":
				res := w.resolveAuth(r, col)
				tint := p.Muted
				if res.Type != "none" {
					tint = p.Accent
				}
				ui.Row(c).FillWidth().Padding(11, 12).Gap(9).Radius(9).
					Background(p.Field).Border(1, p.Border).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("shield", 15)).Size(15, 15).TextColor(tint)
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						if res.Type == "none" {
							ui.Text(c, "No authorization is inherited").FontSize(11.5).FontWeight(600)
							ui.Text(c, "Set it on a folder, or choose a type above.").FontSize(10.5).TextColor(p.Muted)
						} else {
							source := ""
							if res.Source != nil {
								source = " from " + w.collectionPath(res.Source.ID)
							}
							ui.Text(c, authLabel(res.Type)+source).FontSize(11.5).FontWeight(600)
							ui.Text(c, "The folder's value is used whenever this request is sent.").FontSize(10.5).TextColor(p.Muted)
						}
					})
				})
				target := col
				if res.Source != nil {
					target = res.Source
				}
				if target != nil {
					label := "Folder authorization"
					if target.ParentID == "" {
						label = "Collection authorization"
					}
					ui.Row(c).Gap(8).Children(func() {
						if a.plainButton(c, label, "shield").Clicked() {
							a.openCollectionAuth(target)
						}
						ui.Spacer(c).Grow(1)
					})
				}
			case "bearer":
				ui.Column(c).Gap(5).Children(func() {
					ui.Text(c, "Token").FontSize(10.5).FontWeight(600).TextColor(p.Muted)
					bearer := a.textInput(c, &r.Bearer).Placeholder("{{token}}").Height(34).FillWidth().Font("monospace").FontSize(12).Label("Bearer token")
					bearer = a.variableField(c, bearer, &r.Bearer, "monospace", 12, "bearer:"+r.ID, nil, true)
					_ = bearer
				})
			case "basic":
				ui.Row(c).Gap(10).Children(func() {
					ui.Column(c).Gap(5).Grow(1).Children(func() {
						ui.Text(c, "Username").FontSize(10.5).FontWeight(600).TextColor(p.Muted)
						a.textInput(c, &r.Username).Height(34).FillWidth().Label("Username")
					})
					ui.Column(c).Gap(5).Grow(1).Children(func() {
						ui.Text(c, "Password").FontSize(10.5).FontWeight(600).TextColor(p.Muted)
						a.secretInput(c, &r.Password).Height(34).FillWidth().Label("Password")
					})
				})
			default:
				ui.Text(c, "This request has no authorization header.").FontSize(11).TextColor(p.Muted)
			}
		})
	})
}

func (a *app) optionsTab(c *ui.Context, r *Request) {
	p := a.palette
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(16, 18, 20, 18).Gap(16).MaxWidth(650).Children(func() {
			ui.Row(c).FillWidth().Padding(10, 12).Gap(8).Radius(9).Background(p.Field).Border(1, p.Border).Children(func() {
				ui.Icon(c, icon("shield", 14)).Size(14, 14).TextColor(p.Accent)
				ui.Text(c, "Client certificates are matched to the request host automatically. Manage them in Settings.").
					FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
			})
			ui.Column(c).Gap(5).Children(func() {
				ui.Text(c, "Description").FontSize(11).FontWeight(600).TextColor(p.Muted)
				a.textInput(c, &r.Description).Placeholder("What does this request do?").Height(34).FillWidth().Label("Request description")
			})
			ui.Row(c).Gap(10).Children(func() {
				follow := a.data.Settings.Follow
				if ui.Checkbox(c, &follow, "Follow redirects").Changed() {
					a.data.Settings.Follow = follow
				}
				skip := a.data.Settings.SkipVerify
				if ui.Checkbox(c, &skip, "Skip TLS verification").Changed() {
					a.data.Settings.SkipVerify = skip
				}
				if skip {
					a.badge(c, "insecure", p.Bad)
				}
			})
			ui.Text(c, "Redirect and TLS options apply to every request. The request timeout is in Settings.").FontSize(10.5).TextColor(p.Muted)
		})
	})
}

func (a *app) responsePane(c *ui.Context, r *Request) {
	p := a.palette
	res, has := a.responses[r.ID]
	sending := a.sending[r.ID]
	col := ui.Column(c).Fill().MinHeight(0).Children(func() {
		ui.Row(c).Height(40).Shrink(0).Padding(0, 14).Gap(8).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(p.Border).Children(func() {
			ui.Text(c, "Response").FontSize(11.5).FontWeight(650).PaddingEnd(8).TextColor(p.Muted)
			if sending {
				ui.Spinner(c).Size(12, 12)
				ui.Text(c, "Waiting for server…").FontSize(10.5).TextColor(p.Muted)
			} else if has {
				if res.Error != "" {
					a.badge(c, "error", p.Bad)
				} else {
					a.statusPill(c, res.StatusCode)
					ui.Text(c, res.Status).FontSize(10).TextColor(p.Muted)
					ui.Text(c, "·").FontSize(10).TextColor(p.Muted)
					ui.Text(c, shortDuration(res.Duration)).FontSize(10).TextColor(p.Muted)
					ui.Text(c, "·").FontSize(10).TextColor(p.Muted)
					ui.Text(c, shortSize(res.Size)).FontSize(10).TextColor(p.Muted)
				}
			}
			ui.Spacer(c).Grow(1)
			if has && res.Error == "" {
				if a.tabButton(c, "Body", a.respTab == 0, "").Clicked() {
					a.respTab = 0
				}
				if a.tabButton(c, "Headers", a.respTab == 1, fmt.Sprint(len(res.Headers))).Clicked() {
					a.respTab = 1
				}
				cp := a.iconButton(c, "copy", "Copy response", 28)
				if cp.Clicked() {
					if a.respTab == 1 {
						c.WriteClipboard(headersText(res.Headers))
					} else {
						c.WriteClipboard(res.Body)
					}
					a.toast(c, "Copied to clipboard", "The response is on your clipboard.", "success")
				}
			}
		})
		// One state of the pane at a time, each fading in over the cleared
		// pane, so a send goes from empty to waiting to the result smoothly.
		state := "body"
		switch {
		case sending:
			state = "sending"
		case !has:
			state = "empty"
		case res.Error != "":
			state = "error"
		case a.respTab == 1:
			state = "headers"
		}
		ui.Column(c).Key("response:" + r.ID + ":" + state).Grow(1).MinHeight(0).Clip().
			Transition(ui.ElementTransition{
				Duration: 140 * time.Millisecond,
				Enter:    &ui.Motion{Y: 3, Opacity: 0},
			}).Children(func() {
			switch state {
			case "sending":
				ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
					ui.Spinner(c).Size(26, 26)
					ui.Text(c, "Sending request").FontSize(13).FontWeight(600)
					ui.Text(c, "The response will appear here.").FontSize(11).TextColor(p.Muted)
				})
			case "empty":
				ui.Column(c).Grow(1).Center().Gap(7).Children(func() {
					ui.Icon(c, icon("send", 28)).Size(28, 28).TextColor(p.Muted.Alpha(0.45))
					ui.Text(c, "Send a request to see the response").FontSize(12).TextColor(p.Muted)
					ui.Text(c, sendShortcut()+" sends from anywhere").FontSize(10).TextColor(p.Muted.Alpha(0.7))
				})
			case "error":
				ui.Column(c).Grow(1).Center().Padding(24).Children(func() {
					ui.Column(c).MaxWidth(520).Gap(8).Padding(16).Radius(12).Background(p.Bad.Alpha(0.08)).Border(1, p.Bad.Alpha(0.18)).Children(func() {
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icon("alert", 16)).Size(16, 16).TextColor(p.Bad)
							ui.Text(c, "Request failed").FontSize(12).FontWeight(650).TextColor(p.Bad)
						})
						_ = a.selectableMenu(c, ui.Text(c, res.Error).FontSize(11).TextColor(p.Text).Wrap().Selectable(), res.Error)
					})
				})
			case "headers":
				ui.Scroll(c).Grow(1).Padding(12, 16, 16, 16).Children(func() {
					ui.Column(c).Gap(4).Children(func() {
						for _, h := range res.Headers {
							ui.Row(c).FillWidth().Gap(12).Padding(3, 2).Children(func() {
								ui.Text(c, h.Key).Font("monospace").FontSize(10.5).FontWeight(650).Width(190).TextColor(p.Text)
								_ = a.selectableMenu(c, ui.Text(c, h.Value).Font("monospace").FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap().Selectable(), h.Value)
							})
						}
					})
				})
			default:
				ui.ScrollBoth(c).Grow(1).Padding(14, 16, 16, 16).Children(func() {
					body := res.Body
					if strings.TrimSpace(body) == "" {
						el := ui.Text(c, "(empty body)").Font("monospace").FontSize(11.2).LineHeight(1.55).
							NoWrap().Selectable().TextColor(p.Text)
						_ = a.selectableMenu(c, el, "(empty body)")
						return
					}
					if res.JSON {
						if spans := a.responseSpans(r.ID, body, syntaxColors(p)); len(spans) > 0 {
							el := ui.RichText(c, spans...).Font("monospace").FontSize(11.2).LineHeight(1.55).
								NoWrap().Selectable().TextColor(p.Text)
							_ = a.selectableMenu(c, el, body)
							return
						}
					}
					el := ui.Text(c, body).Font("monospace").FontSize(11.2).LineHeight(1.55).
						NoWrap().Selectable().TextColor(p.Text).Label("Response body")
					_ = a.selectableMenu(c, el, body)
				})
			}
		})
	})
	_ = col
}
