package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) modals(c *ui.Context) {
	// Drive the dialog's open animation from the window root every frame so it
	// plays even though MyGo lays overlays out after its transition pass.
	target := float32(0)
	if a.modal != "" {
		target = 1
	}
	a.dialogAnim = c.Root().Animate("dialog", target, 140*time.Millisecond)
	switch a.modal {
	case "new_workspace":
		a.nameModal(c, "New workspace", "Workspace name", "Create workspace", func(name string) {
			a.newWorkspace(name)
		})
	case "rename_workspace":
		a.renameWorkspaceModal(c)
	case "collection":
		a.nameModal(c, "New collection", "Collection name", "Create collection", func(name string) {
			a.newCollection(name)
		})
	case "folder":
		title := "New folder"
		if w := a.ws(); w != nil {
			if parent := w.collection(a.modalParent); parent != nil {
				title = "New folder in " + parent.Name
			}
		}
		a.nameModal(c, title, "Folder name", "Create folder", func(name string) {
			a.newCollectionIn(name, a.modalParent)
		})
	case "rename_collection":
		a.renameCollectionModal(c)
	case "collection_auth":
		a.collectionAuthModal(c)
	case "rename_request":
		a.renameRequestModal(c)
	case "env":
		a.nameModal(c, "New environment", "Environment name", "Create environment", func(name string) {
			a.newEnvironment(name)
		})
	case "cert_import":
		a.certImportModal(c)
	case "cert_test":
		a.certTestModal(c)
	case "confirm":
		a.confirmModal(c)
	case "palette":
		a.paletteModal(c)
	}
}

func (a *app) dialog(c *ui.Context, title string, width float32, fn func()) {
	p := a.palette
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Overlay).Opacity(a.dialogAnim)
		panel.Width(width).MaxWidthPercent(94).Background(p.Panel).Radius(16).Border(1, p.Border).
			Padding(20).Gap(14).Shadow(0, 22, 60, 0, p.Shadow).
			Opacity(a.dialogAnim).Translate(0, (1-a.dialogAnim)*-8)
		panel.Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(16).FontWeight(700).Grow(1)
				if a.iconButton(c, "x", "Close", 28).Clicked() {
					a.modal = ""
					a.modalError = ""
					a.modalInfo = ""
				}
			})
			if a.modalError != "" {
				ui.Row(c).FillWidth().Padding(10, 12).Gap(8).Radius(9).Background(p.Bad.Alpha(0.10)).
					Border(1, p.Bad.Alpha(0.18)).Children(func() {
					ui.Icon(c, icon("alert", 14)).Size(14, 14).TextColor(p.Bad)
					ui.Text(c, a.modalError).FontSize(10.8).TextColor(p.Text).Grow(1).Wrap()
				})
			}
			fn()
		})
	})
	if !open {
		a.modal = ""
		a.modalError = ""
		a.modalInfo = ""
	}
}

func (a *app) modalInput(c *ui.Context, label, placeholder string, ptr *string, password bool) ui.Element {
	p := a.palette
	ui.Column(c).FillWidth().Gap(5).Children(func() {
		ui.Text(c, label).FontSize(10.5).FontWeight(600).TextColor(p.Muted)
		var in ui.Element
		if password {
			in = a.secretInput(c, ptr)
		} else {
			in = a.textInput(c, ptr)
		}
		in.Placeholder(placeholder).Height(34).FillWidth().FontSize(12).Label(label)
	})
	return ui.Element{}
}

func (a *app) nameModal(c *ui.Context, title, label, action string, done func(string)) {
	a.dialog(c, title, 420, func() {
		a.modalInput(c, label, "Give it a name", &a.input1, false)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
			}
			if a.accentButton(c, action, "plus").Clicked() || c.Shortcut(0, ui.KeyEnter) {
				if strings.TrimSpace(a.input1) == "" {
					a.modalError = "Enter a name first."
					return
				}
				done(a.input1)
			}
		})
	})
}

func firstLetter(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "W"
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0]))
}

func countRequests(w *Workspace) int {
	n := 0
	for _, c := range w.Collections {
		n += len(c.Requests)
	}
	return n
}

func (a *app) deleteWorkspace(id string) {
	for i := range a.data.Workspaces {
		if a.data.Workspaces[i].ID == id {
			a.data.Workspaces = append(a.data.Workspaces[:i], a.data.Workspaces[i+1:]...)
			if a.data.ActiveWorkspace == id && len(a.data.Workspaces) > 0 {
				a.data.ActiveWorkspace = a.data.Workspaces[0].ID
				// The deleted workspace may have been the one whose look was
				// on screen; adopt the workspace that takes its place.
				a.applyWorkspaceAppearance()
			}
			a.save()
			return
		}
	}
}

func (a *app) renameWorkspaceModal(c *ui.Context) {
	w := a.data.workspace(a.modalTarget)
	if w == nil {
		a.modal = ""
		return
	}
	a.dialog(c, "Rename workspace", 420, func() {
		a.modalInput(c, "Workspace name", "Workspace name", &a.input1, false)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
			}
			if a.accentButton(c, "Save", "check").Clicked() || c.Shortcut(0, ui.KeyEnter) {
				if strings.TrimSpace(a.input1) == "" {
					a.modalError = "The name cannot be empty."
					return
				}
				w.Name = strings.TrimSpace(a.input1)
				a.modal = ""
				a.save()
			}
		})
	})
}

func (a *app) renameCollectionModal(c *ui.Context) {
	w := a.ws()
	col := w.collection(a.modalTarget)
	if col == nil {
		a.modal = ""
		return
	}
	title, label := "Rename collection", "Collection name"
	if col.ParentID != "" {
		title, label = "Rename folder", "Folder name"
	}
	a.dialog(c, title, 420, func() {
		a.modalInput(c, label, label, &a.input1, false)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
			}
			if a.accentButton(c, "Save", "check").Clicked() || c.Shortcut(0, ui.KeyEnter) {
				if strings.TrimSpace(a.input1) == "" {
					a.modalError = "The name cannot be empty."
					return
				}
				col.Name = strings.TrimSpace(a.input1)
				a.modal = ""
				a.save()
			}
		})
	})
}

// collectionAuthModal edits the authorization a folder or collection passes
// down to the requests below it.
func (a *app) collectionAuthModal(c *ui.Context) {
	p := a.palette
	w := a.ws()
	if w == nil {
		a.modal = ""
		return
	}
	col := w.collection(a.modalTarget)
	if col == nil {
		a.modal = ""
		return
	}
	kind, title := "collection", "Collection authorization"
	if col.ParentID != "" {
		kind, title = "folder", "Folder authorization"
	}
	a.dialog(c, title, 480, func() {
		ui.Text(c, "Requests in this "+kind+" use this authorization unless they set their own. Subfolders pass it on.").
			FontSize(11).TextColor(p.Muted).Wrap()
		hint := func(text string) {
			ui.Row(c).FillWidth().Padding(10, 11).Gap(8).Radius(9).
				Background(p.Field).Border(1, p.Border).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icon("shield", 14)).Size(14, 14).TextColor(p.Muted)
				ui.Text(c, text).FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
			})
		}

		opts := make([]string, 0, len(authOptions))
		keys := make([]string, 0, len(authOptions))
		for _, opt := range authOptions {
			// A top-level collection has nothing to inherit from.
			if opt.key == "inherit" && col.ParentID == "" {
				continue
			}
			opts = append(opts, opt.label)
			keys = append(keys, opt.key)
		}
		shown := a.modalAuth
		if shown == "inherit" && col.ParentID == "" {
			shown = "none"
		}
		choice := authOptions[authOptionIndex(shown)].label
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Type").FontSize(10.5).FontWeight(600).TextColor(p.Muted).Width(54)
			sel := ui.Select(c, &choice, opts).Grow(1).Height(32).Label("Authorization type")
			if sel.Changed() {
				for i, opt := range opts {
					if opt == choice {
						a.modalAuth = keys[i]
					}
				}
			}
		})

		// A top-level collection stores "" for inheritance, but the picker shows
		// "No auth" because there is no parent. Everything below uses the type
		// the picker shows so the hint and the saved value agree.
		current := a.modalAuth
		if current == "inherit" && col.ParentID == "" {
			current = "none"
		}
		switch current {
		case "bearer":
			a.modalInput(c, "Token", "{{token}}", &a.input1, false)
		case "basic":
			a.modalInput(c, "Username", "name@example.com", &a.input2, false)
			a.modalInput(c, "Password", "Password", &a.input3, true)
		case "inherit":
			parent := "the parent folder"
			if par := w.collection(col.ParentID); par != nil {
				parent = "“" + par.Name + "”"
			}
			hint("Requests follow the authorization of " + parent + ". If that folder inherits too, the nearest folder with a type wins.")
		default:
			hint("Requests in this " + kind + " send no authorization header.")
		}

		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
			}
			if a.accentButton(c, "Save", "check").Clicked() || c.Shortcut(0, ui.KeyEnter) {
				col.AuthType = current
				if col.AuthType == "inherit" {
					col.AuthType = ""
				}
				col.Bearer = a.input1
				col.Username = a.input2
				col.Password = a.input3
				a.modal = ""
				a.save()
			}
		})
	})
}

func (a *app) renameRequestModal(c *ui.Context) {
	w := a.ws()
	r, _ := w.request(a.modalTarget)
	if r == nil {
		a.modal = ""
		return
	}
	a.dialog(c, "Rename request", 420, func() {
		a.modalInput(c, "Request name", "Request name", &a.input1, false)
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
			}
			if a.accentButton(c, "Save", "check").Clicked() || c.Shortcut(0, ui.KeyEnter) {
				if strings.TrimSpace(a.input1) == "" {
					a.modalError = "The name cannot be empty."
					return
				}
				r.Name = strings.TrimSpace(a.input1)
				a.modal = ""
				a.save()
			}
		})
	})
}

func (a *app) certImportModal(c *ui.Context) {
	p := a.palette
	a.dialog(c, "Import client certificate", 500, func() {
		ui.Row(c).FillWidth().Gap(8).Padding(9, 11).Radius(8).Background(p.Field).Children(func() {
			ui.Icon(c, icon("file", 14)).Size(14, 14).TextColor(p.Muted)
			ui.Text(c, filepath.Base(a.pendingCertPath)).FontSize(10.5).Font("monospace").TextColor(p.Muted).Grow(1).SingleLine().Ellipsis(a.pendingCertPath)
		})
		a.modalInput(c, "PFX password", "Password used to protect the file", &a.input1, true)
		a.modalInput(c, "Display name", "Optional name", &a.input3, false)
		a.modalInput(c, "Request host (optional)", "api.example.com", &a.input2, false)
		ui.Text(c, "Requests to this host use the certificate automatically. Leave it blank to use it as the fallback when no host-specific certificate matches.").
			FontSize(10).TextColor(p.Muted).Wrap()
		if a.certTestOK && a.pendingCertInfo != nil {
			info := a.pendingCertInfo
			ui.Column(c).FillWidth().Gap(3).Padding(10, 12).Radius(9).Background(p.Good.Alpha(0.08)).Border(1, p.Good.Alpha(0.18)).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Good)
					ui.Text(c, "Password is correct").FontSize(11).FontWeight(650).TextColor(p.Good)
				})
				ui.Text(c, info.Subject).FontSize(9.5).Font("monospace").TextColor(p.Muted).Wrap()
				ui.Text(c, "Expires "+prettyDate(info.NotAfter)+" · "+info.Issuer).FontSize(9.5).TextColor(p.Muted).Wrap()
			})
		}
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			if a.plainButton(c, "Test password", "key").Clicked() {
				a.modalError = ""
				if a.input1 == "" {
					a.modalError = "Enter the PFX password first."
				} else if info, err := a.store.TestPFX(a.pendingCertPath, a.input1); err != nil {
					a.modalError = err.Error()
					a.certTestOK = false
					a.pendingCertInfo = nil
				} else {
					a.certTestOK = true
					a.pendingCertInfo = &info
				}
			}
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
			}
			if a.accentButton(c, "Import", "download").Clicked() {
				if a.input1 == "" {
					a.modalError = "Enter the PFX password first."
					return
				}
				info, err := a.store.ImportPFX(a.pendingCertPath, a.input1, a.input2)
				if err != nil {
					a.modalError = err.Error()
					return
				}
				if strings.TrimSpace(a.input3) != "" {
					info.Name = strings.TrimSpace(a.input3)
				}
				a.data.Certificates = append(a.data.Certificates, info)
				a.modal = ""
				a.pendingCertPath = ""
				a.pendingCertInfo = nil
				a.certTestOK = false
				a.save()
				a.toast(c, "Certificate imported", info.Name, "success")
			}
		})
	})
}

func (a *app) certTestModal(c *ui.Context) {
	p := a.palette
	var cert *Certificate
	for i := range a.data.Certificates {
		if a.data.Certificates[i].ID == a.modalTarget {
			cert = &a.data.Certificates[i]
		}
	}
	if cert == nil {
		a.modal = ""
		return
	}
	a.dialog(c, "Test certificate password", 460, func() {
		ui.Row(c).FillWidth().Gap(9).Padding(10, 12).Radius(9).Background(p.Field).Children(func() {
			ui.Icon(c, icon("certificate", 15)).Size(15, 15).TextColor(p.Accent)
			ui.Column(c).Gap(0).Grow(1).Children(func() {
				ui.Text(c, cert.Name).FontSize(11.5).FontWeight(620)
				ui.Text(c, cert.Subject).FontSize(9.5).Font("monospace").TextColor(p.Muted).SingleLine().Ellipsis(cert.Subject)
			})
		})
		a.modalInput(c, "Password", "Enter the PFX password", &a.input1, true)
		if a.certTestOK {
			ui.Row(c).FillWidth().Gap(7).Padding(9, 11).Radius(8).Background(p.Good.Alpha(0.08)).Children(func() {
				ui.Icon(c, icon("check", 13)).Size(13, 13).TextColor(p.Good)
				ui.Text(c, "The password opens this certificate.").FontSize(10.5).TextColor(p.Good).Grow(1)
			})
		}
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			if a.plainButton(c, "Test", "key").Clicked() {
				a.modalError = ""
				a.certTestOK = false
				if a.input1 == "" {
					a.modalError = "Enter the certificate password."
				} else if err := a.store.TestStoredCert(*cert, a.input1); err != nil {
					a.modalError = err.Error()
				} else {
					a.certTestOK = true
					a.modalInfo = "Password is correct"
				}
			}
			if a.certTestOK {
				if a.plainButton(c, "Update saved password", "save").Clicked() {
					if err := a.store.SavePassword(cert.ID, a.input1); err != nil {
						a.modalError = err.Error()
					} else {
						a.toast(c, "Password updated", "The certificate password was saved.", "success")
						a.modal = ""
					}
				}
			}
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Close", "").Clicked() {
				a.modal = ""
			}
		})
	})
}

func (a *app) confirmModal(c *ui.Context) {
	p := a.palette
	title := a.modalTitle
	if title == "" {
		title = "Are you sure?"
	}
	a.dialog(c, title, 430, func() {
		ui.Text(c, a.modalInfo).FontSize(11.2).TextColor(p.Muted).Wrap()
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c).Grow(1)
			if a.plainButton(c, "Cancel", "").Clicked() {
				a.modal = ""
				a.confirm = nil
			}
			b := a.dangerButton(c, "Delete", "trash")
			if b.Clicked() {
				a.runConfirm()
			}
		})
	})
}

type command struct {
	label string
	hint  string
	ic    string
	run   func()
	id    string
}

func (a *app) paletteModal(c *ui.Context) {
	p := a.palette
	title := "Commands"
	if a.searchThemeMode {
		title = "Choose theme"
	}
	a.dialog(c, title, 560, func() {
		in := a.textInput(c, &a.search).Placeholder("Type a command…").Height(36).FillWidth().FontSize(12.5).AutoFocus().Label("Search commands")
		_ = in
		q := strings.ToLower(strings.TrimSpace(a.search))
		shown := 0
		ui.Scroll(c).MaxHeight(400).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(2).Children(func() {
				if a.searchThemeMode {
					// Back to the command list.
					if q == "" || strings.Contains("back", q) {
						shown++
						b := ui.ButtonBase(c).FillWidth().Height(38).Padding(0, 9).Radius(8).Gap(9).Label("Back")
						if b.Hovered() {
							b.Background(p.Field)
						}
						b.Children(func() {
							ui.Icon(c, icon("arrow-left", 15)).Size(15, 15).TextColor(p.Muted)
							ui.Text(c, "Back").FontSize(11.5).FontWeight(560).Grow(1)
							ui.Text(c, "Return to commands").FontSize(9.5).TextColor(p.Muted).MaxWidth(240).SingleLine()
						})
						if b.Clicked() {
							if a.themePreviewLast != "" {
								a.previewTheme(a.themePreviewBase)
								a.themePreviewLast = ""
							}
							a.searchThemeMode = false
							a.search = ""
						}
					}
					themes := []command{{"System", "Follow desktop light/dark", "palette", func() { a.setTheme("system") }, "system"}}
					for _, tp := range themePresets {
						themes = append(themes, command{tp.Name, "Built-in theme", "palette", func(id string) func() { return func() { a.setTheme(id) } }(tp.ID), tp.ID})
					}
					for i := range a.data.CustomThemes {
						ct := &a.data.CustomThemes[i]
						name := ct.Name
						if name == "" {
							name = "Custom theme"
						}
						themes = append(themes, command{name, "Custom theme", "palette", func(id string) func() { return func() { a.setTheme(id) } }(ct.ID), ct.ID})
					}
					for _, cm := range themes {
						if q != "" && !strings.Contains(strings.ToLower(cm.label), q) {
							continue
						}
						shown++
						b := ui.ButtonBase(c).FillWidth().Height(38).Padding(0, 9).Radius(8).Gap(9).Label(cm.label)
						if b.Hovered() || b.Focused() {
							b.Background(p.Field)
							if cm.id != "" && a.themePreviewLast != cm.id {
								a.themePreviewLast = cm.id
								a.previewTheme(cm.id)
							}
						}
						b.Children(func() {
							ui.Icon(c, icon(cm.ic, 15)).Size(15, 15).TextColor(p.Muted)
							ui.Text(c, cm.label).FontSize(11.5).FontWeight(560).Grow(1)
							ui.Text(c, cm.hint).FontSize(9.5).TextColor(p.Muted).MaxWidth(240).SingleLine()
						})
						if b.Clicked() {
							if cm.id != "" {
								a.setTheme(cm.id)
							}
							a.themePreviewLast = ""
							a.themePreviewBase = ""
							a.searchThemeMode = false
							a.modal = ""
							a.search = ""
							if cm.run != nil {
								cm.run()
							}
						}
					}
				} else {
					commands := []command{
						{"New request", "Create a blank request", "plus", func() { a.newRequest() }, ""},
						{"Request", "Open the active request", "send", func() { a.setPage("request") }, ""},
						{"Collections", "Browse collections", "folder", func() { a.setPage("collections") }, ""},
						{"Environments", "Choose or manage variables", "environment", func() { a.setPage("environments") }, ""},
						{"Workspaces", "Switch or manage workspaces", "workspace", func() { a.setPage("workspaces") }, ""},
						{"Certificates", "PFX client certificates", "certificate", func() { a.openSettings("certificates") }, ""},
						{"History", "Past responses", "history", func() { a.setPage("history") }, ""},
						{"Settings", "Appearance and defaults", "settings", func() { a.openSettings("appearance") }, ""},
						{"Switch theme", "Choose from available themes", "palette", func() {
							a.searchThemeMode = true
							a.search = ""
							a.themePreviewBase = a.data.Settings.Theme
							a.themePreviewLast = ""
						}, ""},
						{"Toggle light / dark", "Switch between dark and light", "palette", func() {
							if a.palette.Dark {
								a.setTheme("daylight")
							} else {
								a.setTheme("nocturne")
							}
						}, ""},
						{"Import PFX certificate", "Client certificate for mTLS", "upload", func() { a.pickPFX(c) }, ""},
					}
					for _, cm := range commands {
						if q != "" && !strings.Contains(strings.ToLower(cm.label+" "+cm.hint), q) {
							continue
						}
						shown++
						b := ui.ButtonBase(c).FillWidth().Height(38).Padding(0, 9).Radius(8).Gap(9).Label(cm.label)
						if b.Hovered() {
							b.Background(p.Field)
						}
						b.Children(func() {
							ui.Icon(c, icon(cm.ic, 15)).Size(15, 15).TextColor(p.Muted)
							ui.Text(c, cm.label).FontSize(11.5).FontWeight(560).Grow(1)
							ui.Text(c, cm.hint).FontSize(9.5).TextColor(p.Muted).MaxWidth(240).SingleLine()
						})
						if b.Clicked() {
							cm.run()
							if !a.searchThemeMode {
								a.modal = ""
								a.search = ""
							}
						}
					}
				}
				if shown == 0 {
					ui.Text(c, "No commands match.").FontSize(11).TextColor(p.Muted).Padding(10)
				}
			})
		})
	})
	// Closing the palette without choosing a theme restores the previewed one.
	if a.modal != "palette" {
		if a.themePreviewLast != "" {
			a.previewTheme(a.themePreviewBase)
			a.themePreviewLast = ""
		}
		a.searchThemeMode = false
		a.search = ""
	}
}

// previewTheme applies a theme without saving, so the command palette can show
// each option live while it is hovered.
func (a *app) previewTheme(id string) {
	a.data.Settings.Theme = id
	a.themeKey = ""
	a.refreshWindows()
}
