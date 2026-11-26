package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	settingsRailDefaultWidth = float32(194)
	settingsRailMinWidth     = float32(170)
)

// openSettings shows preferences in their own modal child window, above the
// main window. Creating it again focuses the one already open.
func (a *app) openSettings(tab string) {
	a.closePopovers()
	if tab != "" {
		a.settingsTab = tab
	}
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.settingsWin.Invalidate()
		return
	}
	if a.win == nil {
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:             "Go Post Settings",
		Width:             920,
		Height:            760,
		MinWidth:          760,
		MinHeight:         600,
		Parent:            a.win,
		Modal:             true,
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
		BackgroundColor:   "#0a0b0f",
		// Created with a material so the workspace's setting can switch it on
		// and off without recreating the window, as the main window does.
		Vibrancy: mygo.VibrancyAcrylic,
		StateKey: "settings",
		Content:  ui.View(a.settingsWindowView),
	})
	a.settingsWin = w
	a.applyWindowMaterials()
	w.OnClosed(func() {
		a.closePopovers()
		a.save()
		a.settingsWin = nil
		a.refreshWindows()
	})
	w.Show()
	w.Focus()
}

func (a *app) settingsWindowView(c *ui.Context) {
	a.viewToken = 2
	if a.palette.ID == "" {
		a.systemDark = c.Theme().Dark
	}
	p := resolvePalette(a.data.Settings, a.systemDark, a.data.CustomThemes)
	vibrancy := c.Vibrancy()
	if vibrancy {
		p = vibrancyPalette(p)
	}
	a.palette = p
	a.useTheme(c, p)
	// Settings follows the workspace's material, like the main window: the rail
	// sits on the chrome and the rounded stage stays solid.
	a.paintChrome(c, p, vibrancy)

	ui.Column(c).Fill().Children(func() {
		// Window controls live in the chrome above the rounded stage, exactly
		// like the main window. The stage therefore sits under them instead of
		// wrapping them in its rounded corner.
		ui.Row(c).Height(40).Shrink(0).Padding(0, 12, 0, 16).Gap(9).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(26, 26).Radius(8).Background(p.Accent).Center().Children(func() {
				ui.Icon(c, icon("settings", 14)).Size(14, 14).TextColor(contrastText(p.Accent))
			})
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "Settings").FontSize(12.5).FontWeight(700)
				ui.Text(c, "Go Post").FontSize(9).TextColor(p.Muted)
			})
			ui.Spacer(c).Grow(1)
			a.windowControls(c, a.settingsWin)
		})
		split := a.splitWithHiddenDivider(c, p, vibrancy, &a.settingsW, func() {
			// Rail. It sits directly on the window chrome, with no panel of
			// its own, so the settings stage is the only raised surface.
			rail := ui.Column(c).HeightPercent(100).
				Padding(10, 8, 12, 10).Gap(3)
			rail.Children(func() {
				ui.Text(c, "PREFERENCES").FontSize(9).FontWeight(700).LetterSpacing(0.7).TextColor(p.Muted).
					Margin(12, 8, 6, 8)
				a.settingsRailItem(c, "appearance", "Appearance", "palette", "")
				a.settingsRailItem(c, "requests", "Requests", "send", "")
				a.settingsRailItem(c, "certificates", "Certificates", "certificate", fmt.Sprint(len(a.data.Certificates)))
				a.settingsRailItem(c, "data", "Data", "file", "")
				a.settingsRailItem(c, "about", "About", "info", "")
				ui.Spacer(c).Grow(1)
				ui.Text(c, "Go Post 1.0").FontSize(9).TextColor(p.Muted).Padding(0, 6)
			})
		}, func() {
			// The preferences face: the title and rounded stage both live inside
			// the panel, exactly like go-email's settings window.
			ui.Column(c).Grow(1).MinWidth(0).HeightPercent(100).Children(func() {
				stage := ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Margin(0, 4, 4, 4).
					Background(p.Panel).Radius(18).Border(1, p.Border).Clip()
				stage.Children(func() {
					ui.Row(c).Height(52).Shrink(0).Padding(0, 24).Gap(10).AlignItems(ui.Center).
						BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
						ui.Text(c, a.settingsTitle()).FontSize(15).FontWeight(700).SingleLine()
					})
					ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
						// A page fades in over the stage, matching the main
						// window, so the rail never cuts to a new page.
						ui.Column(c).Key("settings-page:"+a.settingsTab).Padding(22, 24, 28, 24).Gap(16).MaxWidth(790).
							Transition(ui.ElementTransition{
								Duration: 150 * time.Millisecond,
								Enter:    &ui.Motion{Y: 4, Opacity: 0},
							}).Children(func() {
							switch a.settingsTab {
							case "requests":
								a.settingsRequests(c)
							case "certificates":
								a.settingsCertificates(c)
							case "data":
								a.settingsData(c)
							case "about":
								a.settingsAbout(c)
							default:
								a.settingsAppearance(c)
							}
						})
					})
				})
			})
		}).Grow(1).FillWidth()
		if !vibrancy && !p.HasGradient {
			split.Background(p.Bg)
			hideSplitDivider(split, &a.settingsW, p.Bg, c.Theme().Space(10))
		} else {
			split.Background(ui.Transparent)
		}
	})
	if a.settingsW < settingsRailMinWidth {
		a.settingsW = settingsRailMinWidth
	}
	a.themePicker(c)
	a.modals(c)
	a.contextMenu(c)
	a.autosave(c)
}

func (a *app) settingsTitle() string {
	switch a.settingsTab {
	case "requests":
		return "Requests"
	case "certificates":
		return "Certificates"
	case "data":
		return "Data"
	case "about":
		return "About"
	default:
		return "Appearance"
	}
}

func (a *app) settingsRailItem(c *ui.Context, tab, label, ic, count string) ui.Element {
	p := a.palette
	on := a.settingsTab == tab
	b := ui.ButtonBase(c).Key("settings:" + tab).FillWidth().Height(36).PaddingX(10).Radius(8).Gap(9).Label(label).
		Transition(ui.ElementTransition{Duration: 140 * time.Millisecond, Colors: true})
	if on {
		b.Background(p.Accent.Alpha(0.12))
	} else if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		iconCol := p.Muted
		if on {
			iconCol = p.Accent
		}
		ui.Icon(c, icon(ic, 15)).Size(15, 15).TextColor(iconCol)
		ui.Text(c, label).FontSize(12).FontWeight(func() int {
			if on {
				return 650
			}
			return 500
		}()).Grow(1).SingleLine()
		if count != "" {
			ui.Text(c, count).FontSize(9.5).TextColor(p.Muted).Padding(2, 6).Radius(99).Background(p.Field)
		}
	})
	if b.Clicked() {
		a.materialOpen = false
		a.settingsTab = tab
	}
	return b
}

func (a *app) settingsAppearance(c *ui.Context) {
	p := a.palette
	// Theme and material are workspace-scoped. Say so where they are chosen,
	// otherwise switching workspace looks like the setting was lost.
	name := ""
	if w := a.ws(); w != nil {
		name = w.Name
	}
	hint := "Theme and window background belong to this workspace. Every workspace can look different."
	if name != "" {
		hint = "Theme and window background belong to the " + name + " workspace. Every workspace can look different."
	}
	ui.Row(c).FillWidth().Padding(10, 12).Gap(9).Radius(10).AlignItems(ui.Center).
		Background(p.Field).Border(1, p.Border).Children(func() {
		ui.Icon(c, icon("workspace", 14)).Size(14, 14).TextColor(p.Muted)
		ui.Text(c, hint).FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
	})

	a.settingsSection(c, "Theme", "Dark and Light are the defaults; System follows your desktop's light or dark mode.")
	ui.Grid(c).Columns(3).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1)).Gap(10).Children(func() {
		// The two defaults come first, named for what they are: the palettes
		// System switches between.
		a.themeCard(c, "nocturne", "Dark", themeByID("nocturne"))
		a.themeCard(c, "daylight", "Light", themeByID("daylight"))
		sys := resolvePalette(Settings{Theme: "system"}, a.systemDark, a.data.CustomThemes)
		a.themeCard(c, "system", "System", sys)
		for _, tp := range themePresets {
			if tp.ID == "nocturne" || tp.ID == "daylight" {
				continue
			}
			a.themeCard(c, tp.ID, tp.Name, tp)
		}
	})

	a.settingsSection(c, "Custom themes", "Start from a built-in palette, then tint the window chrome, pick the rounded workspace color, and paint a one-to-three-color background. Hex and rgb values work in the editor.")
	ui.Grid(c).Columns(4).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1)).Gap(10).Children(func() {
		for i := range a.data.CustomThemes {
			ct := &a.data.CustomThemes[i]
			a.themeCard(c, ct.ID, ct.Name, customPalette(*ct))
		}
		a.newThemeCard(c)
	})
	a.settingsSection(c, "Window background", "Use the desktop's material behind the main window's sidebar and title bar. The rounded workspace stays solid.")
	a.materialPicker(c)
}

type materialOption struct {
	ID, Label, Desc, Icon string
}

var materialOptions = []materialOption{
	{"none", "None", "Solid background, no desktop material.", "dot"},
	{"acrylic", "Acrylic", "Blurs what is behind the window.", "layers"},
	{"mica", "Mica", "Tints the window with the desktop wallpaper.", "sparkles"},
}

func materialByID(id string) materialOption {
	for _, m := range materialOptions {
		if m.ID == id {
			return m
		}
	}
	return materialOptions[0]
}

// materialPicker is one setting row with a dropdown. The rows the chooser used
// to stack are now a menu: the current choice stays visible in the trigger and
// its description fades in below the label when the choice changes.
func (a *app) materialPicker(c *ui.Context) {
	p := a.palette
	current := materialByID(a.data.Settings.Material)

	ui.Column(c).Gap(7).Children(func() {
		ui.Row(c).FillWidth().Padding(12, 12).Radius(12).Gap(12).AlignItems(ui.Center).
			Background(p.Field).Border(1, p.Border).Children(func() {
			ui.Box(c).Size(34, 34).Radius(10).Background(p.Accent.Alpha(0.10)).Center().Children(func() {
				ui.Icon(c, icon(current.Icon, 16)).Size(16, 16).TextColor(p.Accent)
			})
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, "Material").FontSize(11.5).FontWeight(600)
				ui.Text(c, current.Desc).Key("material-desc:" + current.ID).
					FontSize(9.5).TextColor(p.Muted).SingleLine().Ellipsis(current.Desc).
					Transition(ui.ElementTransition{
						Duration: 140 * time.Millisecond,
						Enter:    &ui.Motion{Y: 3, Opacity: 0},
					})
			})
			a.materialDropdown(c)
		})
		ui.Text(c, "Available on Windows 11 and macOS. Other systems keep the solid background.").
			FontSize(9.5).TextColor(p.Muted)
	})
}

func (a *app) materialDropdown(c *ui.Context) {
	p := a.palette
	current := materialByID(a.data.Settings.Material)
	b := ui.ButtonBase(c).Key("material-select").Height(32).Padding(0, 10).Radius(8).Gap(8).
		MinWidth(172).Label("Window material: " + current.Label).
		Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	b.Background(p.Panel)
	b.Border(1, p.Border)
	if b.Hovered() {
		b.Background(p.Hover)
	}
	b.Children(func() {
		ui.Icon(c, icon(current.Icon, 14)).Size(14, 14).TextColor(p.Muted)
		ui.Text(c, current.Label).FontSize(10.5).FontWeight(600).Grow(1)
		chev := ui.Icon(c, icon("chevron-down", 12)).Size(12, 12).TextColor(p.Muted)
		target := float32(0)
		if a.materialOpen {
			target = 180
		}
		chev.Rotate(chev.Animate("material-chevron", target, 160*time.Millisecond))
	})
	if b.Clicked() {
		a.envOpen = false
		a.wsOpen = false
		a.closeContextMenu()
		a.materialOpen = !a.materialOpen
	}

	menuTarget := float32(0)
	if a.materialOpen {
		menuTarget = 1
	}
	menuAnim := b.Animate("material-menu", menuTarget, 140*time.Millisecond)
	ui.PopoverBase(c, b, &a.materialOpen, func(panel ui.Element) {
		a.menuPanel(panel, "material-menu", 300)
		panel.AttachTo(b, ui.AnchorBottomEnd, ui.AnchorTopEnd)
		panel.Opacity(menuAnim).Translate(0, (1-menuAnim)*-6)
		panel.Children(func() {
			a.menuHeading(c, "Window material", "")
			ui.Column(c).Gap(2).Children(func() {
				for _, m := range materialOptions {
					m := m
					active := a.data.Settings.Material == m.ID
					item := ui.ButtonBase(c).Key("material:"+m.ID).FillWidth().Height(44).Padding(0, 8).
						Radius(8).Gap(9).Label(m.Label)
					if active {
						item.Background(p.Accent.Alpha(0.10))
					} else if item.Hovered() {
						item.Background(p.Hover)
					}
					item.Children(func() {
						ui.Box(c).Size(28, 28).Radius(8).Background(func() ui.Color {
							if active {
								return p.Accent.Alpha(0.14)
							}
							return p.Field
						}()).Center().Children(func() {
							ui.Icon(c, icon(m.Icon, 14)).Size(14, 14).TextColor(func() ui.Color {
								if active {
									return p.Accent
								}
								return p.Muted
							}())
						})
						ui.Column(c).Gap(0).Grow(1).MinWidth(0).Children(func() {
							ui.Text(c, m.Label).FontSize(11.3).FontWeight(func() int {
								if active {
									return 650
								}
								return 540
							}()).SingleLine()
							ui.Text(c, m.Desc).FontSize(9.3).TextColor(p.Muted).SingleLine().Ellipsis(m.Desc)
						})
						if active {
							ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Accent)
						}
					})
					if item.Clicked() {
						a.materialOpen = false
						a.setMaterial(m.ID)
					}
				}
			})
		})
	})
}

func (a *app) settingsRequests(c *ui.Context) {
	p := a.palette
	s := &a.data.Settings
	a.settingsSection(c, "Timeouts", "How long Go Post waits before it gives up on a request.")
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(1).Width(210).Children(func() {
			ui.Text(c, "Request timeout").FontSize(11).FontWeight(600)
			ui.Text(c, "Seconds, between 1 and 300").FontSize(9.5).TextColor(p.Muted)
		})
		secs := float64(s.TimeoutMS) / 1000
		if n := ui.NumberInput(c, &secs, 1, 300, 1).Width(120).Height(32).Label("Timeout in seconds"); n.Changed() {
			s.TimeoutMS = int(secs * 1000)
		}
	})

	a.settingsSection(c, "Behaviour", "Safe defaults, with the escape hatches clearly marked.")
	ui.Column(c).Gap(10).Children(func() {
		follow := s.Follow
		if ui.Checkbox(c, &follow, "Follow redirects").Changed() {
			s.Follow = follow
		}
		persist := s.PersistBody
		if ui.Checkbox(c, &persist, "Store response bodies in history").Changed() {
			s.PersistBody = persist
		}
		skip := s.SkipVerify
		if ui.Checkbox(c, &skip, "Skip TLS verification (insecure)").Changed() {
			s.SkipVerify = skip
		}
	})
	if s.SkipVerify {
		ui.Row(c).FillWidth().Padding(10, 12).Gap(8).Radius(9).Background(p.Warn.Alpha(0.09)).
			Border(1, p.Warn.Alpha(0.18)).Children(func() {
			ui.Icon(c, icon("alert", 14)).Size(14, 14).TextColor(p.Warn)
			ui.Text(c, "TLS verification is off. Only use this while testing against a host you trust.").
				FontSize(10.5).TextColor(p.Text).Grow(1).Wrap()
		})
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon("certificate", 13)).Size(13, 13).TextColor(p.Accent)
		ui.Text(c, "Client certificates are matched to each request's host automatically.").FontSize(10.5).TextColor(p.Muted)
	})
}

func (a *app) settingsCertificates(c *ui.Context) {
	p := a.palette
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, "Client certificates").FontSize(13).FontWeight(680)
			ui.Text(c, "Import a PFX/P12 file and set its request host. Matching requests use it automatically.").FontSize(10.5).TextColor(p.Muted).Wrap()
		})
		if a.accentButton(c, "Import PFX", "upload").Clicked() {
			a.pickPFX(c)
		}
	})
	ui.Row(c).FillWidth().Padding(12, 14).Gap(9).Radius(10).Background(p.Field).Border(1, p.Border).Children(func() {
		ui.Icon(c, icon("shield", 15)).Size(15, 15).TextColor(p.Accent)
		ui.Text(c, "PFX files are copied into the app's private data folder. Passwords go to the OS credential store, with a user-only fallback file where that is unavailable.").
			FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
	})
	if len(a.data.Certificates) == 0 {
		a.emptyState(c, "certificate", "No certificates yet", "Use Import PFX above to add a client certificate. Matching requests use it automatically by host.", "")
		return
	}
	ui.Grid(c).Columns(2).Gap(12).Children(func() {
		for i := range a.data.Certificates {
			cert := &a.data.Certificates[i]
			card := a.card(c).Padding(14).Gap(9).MinHeight(196)
			card.Children(func() {
				ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(32, 32).Radius(9).Background(p.Accent.Alpha(0.10)).Center().Children(func() {
						ui.Icon(c, icon("certificate", 17)).Size(17, 17).TextColor(p.Accent)
					})
					ui.Column(c).Gap(0).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, cert.Name).FontSize(12.5).FontWeight(650).SingleLine().Ellipsis(cert.Name)
						if cert.Host != "" {
							ui.Text(c, cert.Host).FontSize(9.5).Font("monospace").TextColor(p.Muted).SingleLine()
						} else {
							ui.Text(c, "Any host (fallback)").FontSize(9.5).TextColor(p.Muted)
						}
					})
					if expiryState(cert.NotAfter) == "expired" {
						a.badge(c, "expired", p.Bad)
					} else if expiryState(cert.NotAfter) == "soon" {
						a.badge(c, "expires soon", p.Warn)
					}
				})
				ui.Text(c, cert.Subject).FontSize(9.8).Font("monospace").TextColor(p.Muted).MaxLines(2).Wrap()
				ui.Row(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "Issuer").FontSize(9.5).TextColor(p.Muted).Width(42)
					ui.Text(c, cert.Issuer).FontSize(9.8).TextColor(p.Text).Grow(1).SingleLine().Ellipsis(cert.Issuer)
				})
				ui.Row(c).FillWidth().Gap(6).Children(func() {
					ui.Text(c, "Valid to").FontSize(9.5).TextColor(p.Muted).Width(42)
					ui.Text(c, prettyDate(cert.NotAfter)).FontSize(9.8).TextColor(p.Text).Grow(1)
				})
				ui.Spacer(c).Grow(1)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if a.plainButton(c, "Test password", "key").Clicked() {
						a.modal = "cert_test"
						a.modalTarget = cert.ID
						a.input1 = ""
						a.modalError = ""
						a.modalInfo = ""
						a.certTestOK = false
					}
					ui.Spacer(c).Grow(1)
					if a.iconButton(c, "trash", "Delete certificate", 30).Clicked() {
						id, name := cert.ID, cert.Name
						a.openConfirm("Delete certificate?", "â€œ"+name+"â€ will be removed and no longer used automatically.", func() {
							a.deleteCertificate(id)
							a.refreshWindows()
						})
					}
				})
			})
		}
	})
}

func (a *app) settingsAbout(c *ui.Context) {
	p := a.palette
	a.settingsSection(c, "About", "A native HTTP client built with MyGo.")
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(42, 42).Radius(12).Background(p.Accent).Center().Children(func() {
			ui.Icon(c, icon("send", 20)).Size(20, 20).TextColor(contrastText(p.Accent))
		})
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, "Go Post 1.0").FontSize(12.5).FontWeight(650)
			ui.Text(c, "Requests, collections and environments, stored on this device.").FontSize(10).TextColor(p.Muted)
		})
	})

	a.settingsSection(c, "Local storage", "Go Post keeps its state on this device only. There is no sync service.")
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, "Data folder").FontSize(11).FontWeight(600)
			ui.Text(c, a.store.Dir()).FontSize(9.5).Font("monospace").TextColor(p.Muted).SingleLine().Ellipsis(a.store.Dir())
		})
		if a.plainButton(c, "Open folder", "external").Clicked() {
			_ = mygo.Shell.OpenPath(a.store.Dir())
		}
	})
	ui.Row(c).Gap(8).Wrap().Children(func() {
		if a.plainButton(c, "Clear all history", "trash").Clicked() {
			a.openConfirm("Clear history everywhere?", "Every workspace loses its saved response history. Collections and certificates stay.", func() {
				for i := range a.data.Workspaces {
					a.data.Workspaces[i].History = nil
				}
				a.save()
				a.refreshWindows()
			})
		}
		if a.plainButton(c, "Reset app data", "refresh").Clicked() {
			a.openConfirm("Reset everything?", "Workspaces, collections, environments, certificates and settings will be replaced with the starter data. This cannot be undone.", func() {
				for _, cert := range a.data.Certificates {
					_ = removeFile(filepath.Join(a.store.CertDir(), cert.FileName))
					a.store.DeletePassword(cert.ID)
				}
				a.data = defaultData()
				a.themeKey = ""
				a.responses = map[string]SendResult{}
				a.applyWindowMaterials()
				a.save()
				a.refreshWindows()
			})
		}
	})
}
