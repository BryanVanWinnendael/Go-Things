package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	settingsRailDefaultWidth = float32(194)
	settingsRailMinWidth     = float32(170)
)

// openSettings asks for the preferences window on the next frame. Creating a
// window from inside a frame is avoided by setting a flag and flushing it at
// the top of the next frame.
func (a *App) openSettings() {
	a.settingsTab = "appearance"
	a.pendingSettings = true
	a.invalidate()
}

func (a *App) flushPending() {
	if a.pendingMaterials.Swap(false) {
		a.applyWindowMaterials()
	}
	if a.pendingSettings {
		if a.showSettingsWindow() {
			a.pendingSettings = false
		}
	}
	if a.pendingThemePicker {
		if a.showThemePickerWindow() {
			a.pendingThemePicker = false
		}
	}
}

// showSettingsWindow opens preferences in their own modal child window, above
// the main window. Opening it again focuses the one already open.
func (a *App) showSettingsWindow() bool {
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.settingsWin.Invalidate()
		return true
	}
	if a.mainWindow == nil {
		return false
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:             "Go Mail Settings",
		Width:             920,
		Height:            760,
		MinWidth:          760,
		MinHeight:         600,
		Parent:            a.mainWindow,
		Modal:             true,
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
		BackgroundColor:   "#0b0b0b",
		// On Windows only a window created with a material can show one, so
		// create with the current one and let applyWindowMaterials switch it.
		Vibrancy: mygo.VibrancyAcrylic,
		StateKey: "settings",
		Content:  ui.View(a.settingsWindowView),
	})
	a.settingsWin = w
	a.applyWindowMaterials()
	w.OnClosed(func() {
		a.saveConfig()
		a.settingsWin = nil
		a.invalidateAll()
	})
	w.Show()
	w.Focus()
	return true
}

func (a *App) closeSettings() {
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Close()
		a.settingsWin = nil
	}
	a.saveConfig()
	a.invalidateAll()
}

func (a *App) settingsWindowView(c *ui.Context) {
	a.flushPending()
	a.applyPalette(c)
	p := a.palette
	a.paintChrome(c)
	if a.settingsW < settingsRailMinWidth {
		a.settingsW = settingsRailDefaultWidth
	}

	// Drive the confirm dialog's motion from this window's root every frame,
	// like Go Post drives its dialogs. The channel must sit at 0 while closed
	// so opening animates instead of snapping to its target.
	confirmTarget := float32(0)
	if a.confirmFn != nil {
		confirmTarget = 1
	}
	a.confirmAnim = c.Root().Animate("confirm-dialog", confirmTarget, 140*time.Millisecond)

	bar := c.TitleBar()
	height := bar.Height
	if height < 40 {
		height = 40
	}
	ui.Column(c).Fill().Children(func() {
		// Full-width title bar, above the rail and stage, exactly like the
		// main window. The rounded stage therefore sits under the window
		// controls instead of wrapping them.
		ui.Row(c).Height(height).Shrink(0).Padding(0, bar.Right+12, 0, bar.Left+16).
			Gap(9).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(26, 26).Radius(8).Background(p.Accent).Center().Children(func() {
				ui.Icon(c, icon("settings", 14)).Size(14, 14).TextColor(contrastText(p.Accent))
			})
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "Settings").FontSize(12.5).FontWeight(700)
				ui.Text(c, "Go Mail").FontSize(9).TextColor(p.Muted)
			})
			ui.Spacer(c).Grow(1)
			a.windowControls(c, a.settingsWin)
		})
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			// Rail.
			rail := ui.Column(c).Width(a.settingsW).Shrink(0).FillHeight().
				Padding(10, 8, 12, 10).Gap(3)
			rail.Children(func() {
				ui.Text(c, "PREFERENCES").FontSize(9).FontWeight(700).LetterSpacing(0.7).TextColor(p.Muted).
					Margin(12, 8, 6, 8)
				a.settingsRailItem(c, "appearance", "Appearance", "palette", "")
				a.settingsRailItem(c, "mail", "Mail", "pen", "")
				a.settingsRailItem(c, "about", "About", "info", "")
				ui.Spacer(c).Grow(1)
				ui.Text(c, versionLabel()).FontSize(9).TextColor(p.Muted).Padding(0, 6)
			})

			// Rounded stage, like the main window's stage.
			stage := ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Margin(0, 4, 4, 4).
				Background(p.Panel).Radius(18).Border(1, p.Border).Clip()
			stage.Children(func() {
				// The page header stays pinned above the scroll, like Go Post.
				ui.Row(c).Height(52).Shrink(0).Padding(0, 24).Gap(10).AlignItems(ui.Center).
					BorderWidth(0, 0, 1, 0).BorderColor(p.Border).DragWindow().Children(func() {
					ui.Text(c, a.settingsTitle()).FontSize(15).FontWeight(700).SingleLine()
				})
				ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
					// Pages fade in over the stage, matching the main window.
					ui.Column(c).Key("settings-page:"+a.settingsTab).Padding(22, 24, 28, 24).Gap(16).MaxWidth(790).
						Transition(ui.ElementTransition{
							Duration: 150 * time.Millisecond,
							Enter:    &ui.Motion{Y: 4, Opacity: 0},
						}).Children(func() {
						if a.settingsTab == "about" {
							a.settingsAbout(c)
						} else if a.settingsTab == "mail" {
							a.settingsMail(c)
						} else {
							a.settingsAppearance(c)
						}
					})
				})
			})
		})
	})
	a.contextMenu(c)
	a.confirmDialog(c)
}

// settingsTitle is the header of the open preferences page, kept pinned above
// the scroll like Go Post's settings window.
func (a *App) settingsTitle() string {
	switch a.settingsTab {
	case "mail":
		return "Mail"
	case "about":
		return "About"
	default:
		return "Appearance"
	}
}

func (a *App) settingsRailItem(c *ui.Context, tab, label, ic, count string) ui.Element {
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
		a.settingsTab = tab
		a.closeContextMenu()
	}
	return b
}

func (a *App) settingsAppearance(c *ui.Context) {
	p := a.palette
	// Theme and material belong to the signed-in account. Say so where they
	// are chosen, otherwise switching accounts looks like the setting was lost.
	name := ""
	if a.acct.Email != "" {
		name = a.acct.Email
	}
	hint := "Theme and window background belong to this account. Every account can look different."
	if name != "" {
		hint = "Theme and window background belong to " + name + ". Every account can look different."
	}
	ui.Row(c).FillWidth().Padding(10, 12).Gap(9).Radius(10).AlignItems(ui.Center).
		Background(p.Field).Border(1, p.Border).Children(func() {
		ui.Icon(c, iconMail).Size(14, 14).TextColor(p.Muted)
		ui.Text(c, hint).FontSize(10.5).TextColor(p.Muted).Grow(1).Wrap()
	})

	a.settingsSection(c, "Theme", "Dark and Light are the defaults; System follows your desktop's light or dark mode.")
	ui.Grid(c).Columns(4).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1)).Gap(10).Children(func() {
		// The two defaults come first, named for what they are: the palettes
		// System switches between.
		a.themeCard(c, "nocturne", "Dark", themeByID("nocturne"))
		a.themeCard(c, "daylight", "Light", themeByID("daylight"))
		sys := resolvePalette(ThemeSettings{Theme: "system"}, a.systemDark, a.customThemes)
		a.themeCard(c, "system", "System", sys)
		for _, tp := range themePresets {
			if tp.ID == "nocturne" || tp.ID == "daylight" {
				continue
			}
			a.themeCard(c, tp.ID, tp.Name, tp)
		}
	})

	a.settingsSection(c, "Custom themes", "Start from a built-in palette, then tint the window chrome, pick the rounded stage color, and paint a one-to-three-color background. Hex and rgb values work in the editor.")
	ui.Grid(c).Columns(4).ColumnTracks(ui.Fr(1), ui.Fr(1), ui.Fr(1), ui.Fr(1)).Gap(10).Children(func() {
		for i := range a.customThemes {
			ct := &a.customThemes[i]
			a.themeCard(c, ct.ID, ct.Name, customPalette(*ct))
		}
		a.newThemeCard(c)
	})

	a.settingsSection(c, "Window background", "Use the desktop's material behind the main window's sidebar and title bar. The rounded stage stays solid.")
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
func (a *App) materialPicker(c *ui.Context) {
	p := a.palette
	current := materialByID(a.themeSettings.Material)

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

func (a *App) materialDropdown(c *ui.Context) {
	p := a.palette
	current := materialByID(a.themeSettings.Material)
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
			a.menuHeading(c, "Window material")
			ui.Column(c).Gap(2).Children(func() {
				for _, m := range materialOptions {
					m := m
					active := a.themeSettings.Material == m.ID
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

func (a *App) settingsMail(c *ui.Context) {
	p := a.palette
	a.settingsSection(c, "Signature", "A rich signature for new messages, replies and forwards. Leave it empty to send without one.")
	if a.acct.Email == "" {
		ui.Text(c, "Connect an account to add a signature.").FontSize(11).TextColor(p.Muted)
	} else {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Avatar(c, a.displayName(), nil).Size(24, 24)
			ui.Text(c, a.acct.Email).FontSize(11.5).FontWeight(600).SingleLine()
		})
		a.signatureEditorView(c)
		ui.Text(c, "PNG, JPEG, GIF or WebP images, up to 500 KB each. They are sent inline with the message.").
			FontSize(9.5).TextColor(p.Muted).Wrap()
	}

	a.settingsSection(c, "Shared mailbox", "Aggregate mail from every logged-in account into one mailbox.")
	ui.Column(c).Gap(6).Children(func() {
		shared := a.sharedMailbox
		if ui.Checkbox(c, &shared, "Enable shared mailbox").Changed() {
			a.sharedMailbox = shared
			a.saveConfig()
			a.invalidateAll()
		}
		ui.Text(c, "When enabled, a “Shared Mailbox” entry appears in the sidebar.").FontSize(9.5).TextColor(p.Muted).Wrap()
	})

	a.settingsSection(c, "Debug", "Developer tools for diagnosing rendering issues.")
	ui.Column(c).Gap(6).Children(func() {
		dbg := a.themeSettings.Debug
		if ui.Checkbox(c, &dbg, "Show 'View as text' button in the mail viewer").Changed() {
			a.themeSettings.Debug = dbg
			a.saveConfig()
			a.refreshWindows()
		}
		ui.Text(c, "When enabled, a button appears next to Delete to toggle between the rendered HTML and the plain-text body.").FontSize(9.5).TextColor(p.Muted).Wrap()
	})
}

func (a *App) settingsAbout(c *ui.Context) {
	p := a.palette
	a.settingsSection(c, "About", "A native desktop mail client built with MyGo.")
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		if bmp := a.appIconBitmap(); bmp != nil {
			ui.Image(c, bmp).Size(42, 42).Radius(12)
		} else {
			ui.Box(c).Size(42, 42).Radius(12).Background(p.Accent).Center().Children(func() {
				ui.Icon(c, iconMail).Size(20, 20).TextColor(contrastText(p.Accent))
			})
		}
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, versionLabel()).FontSize(12.5).FontWeight(650)
			ui.Text(c, "IMAP + SMTP, no sync service, credentials stay on this device.").FontSize(10).TextColor(p.Muted)
		})
	})
	a.settingsSection(c, "Local storage", "Go Mail keeps its state on this device only.")
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, "Config file").FontSize(11).FontWeight(600)
			ui.Text(c, configPath()).FontSize(9.5).Font("monospace").TextColor(p.Muted).SingleLine()
		})
		if a.plainButton(c, "Open folder", "external").Clicked() {
			_ = mygo.Shell.OpenPath(configPath())
		}
	})
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, "Mail cache").FontSize(11).FontWeight(600)
			ui.Text(c, formatBytes(a.cacheSize())+"  ·  "+a.cachePath()).FontSize(9.5).Font("monospace").TextColor(p.Muted).SingleLine()
		})
		if a.plainButton(c, "Delete cache", "trash").Clicked() {
			a.confirmClearCache = true
		}
	})
	if a.confirmClearCache {
		ui.Row(c).FillWidth().Padding(10, 12).Gap(10).Radius(9).AlignItems(ui.Center).
			Background(p.Bad.Alpha(0.09)).Border(1, p.Bad.Alpha(0.18)).Children(func() {
			ui.Icon(c, icon("alert", 14)).Size(14, 14).TextColor(p.Bad)
			ui.Text(c, "Clear cached mail from this device? Messages stay on the server and the cache rebuilds as mail loads.").
				FontSize(10.5).TextColor(p.Text).Grow(1).Wrap()
			if a.ghostButton(c, "Cancel", "").Clicked() {
				a.confirmClearCache = false
			}
			if a.plainButton(c, "Delete", "trash").Clicked() {
				a.confirmClearCache = false
				a.clearDiskCache()
			}
		})
	}

	a.settingsSection(c, "Account", "")
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Avatar(c, a.displayName(), nil).Size(34, 34)
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, a.acct.Email).FontSize(12).FontWeight(600).SingleLine()
			ui.Text(c, a.acct.IMAPHost+"  ·  "+a.acct.SMTPHost).FontSize(9.5).Font("monospace").TextColor(p.Muted).SingleLine()
		})
	})
}

// ------------------------------------------------------------- theme ops ---

func (a *App) themeLabel() string {
	id := a.themeSettings.Theme
	if id == "" || id == "system" {
		return "System"
	}
	if ct := customThemeByID(a.customThemes, id); ct != nil {
		if name := strings.TrimSpace(ct.Name); name != "" {
			return name
		}
	}
	return themeByID(id).Name
}

func (a *App) setTheme(id string) {
	a.themeSettings.Theme = id
	a.saveConfig()
	a.refreshWindows()
}

func (a *App) setAccent(id string) {
	a.themeSettings.Accent = id
	a.saveConfig()
	a.refreshWindows()
}

func (a *App) setMaterial(material string) {
	a.themeSettings.Material = material
	a.saveConfig()
	a.applyWindowMaterials()
	a.refreshWindows()
}

func (a *App) copyTheme(id string) {
	if ct := customThemeByID(a.customThemes, id); ct != nil {
		cp := *ct
		cp.ID = newID("theme")
		cp.Name = a.copyThemeName(ct.Name)
		cp.GradientPoints = append([]CustomGradientPoint(nil), ct.GradientPoints...)
		a.customThemes = append(a.customThemes, cp)
		a.setTheme(cp.ID)
		return
	}
	if !isPresetTheme(id) {
		return
	}
	base := themeByID(id)
	cp := CustomTheme{
		ID:   newID("theme"),
		Name: a.copyThemeName(base.Name),
		Base: base.ID,
	}
	if actual := resolvePalette(ThemeSettings{Theme: id}, a.systemDark, a.customThemes); actual.Accent.A != 0 {
		cp.Accent = hexColor(actual.Accent)
	}
	a.customThemes = append(a.customThemes, cp)
	a.setTheme(cp.ID)
}

func (a *App) copyThemeName(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Theme"
	}
	// Copied copies read better as "Name copy 2" than "Name copy copy".
	if idx := strings.LastIndex(strings.ToLower(base), " copy"); idx > 0 {
		tail := strings.TrimSpace(base[idx+len(" copy"):])
		if tail == "" || digitsOnly(tail) {
			base = strings.TrimSpace(base[:idx])
		}
	}
	used := make(map[string]bool, len(a.customThemes))
	for _, ct := range a.customThemes {
		used[normalizedThemeName(ct.Name)] = true
	}
	name := base + " copy"
	if !used[normalizedThemeName(name)] {
		return name
	}
	for i := 2; ; i++ {
		name = fmt.Sprintf("%s copy %d", base, i)
		if !used[normalizedThemeName(name)] {
			return name
		}
	}
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (a *App) deleteCustomTheme(id string) {
	for i := range a.customThemes {
		if a.customThemes[i].ID == id {
			a.customThemes = append(a.customThemes[:i], a.customThemes[i+1:]...)
			break
		}
	}
	if a.themeSettings.Theme == id {
		a.themeSettings.Theme = "system"
	}
	a.saveConfig()
	a.refreshWindows()
}

// requestWindowMaterials asks the next frame to apply the window material.
// Background goroutines must not call window methods directly: a vibrancy
// change waits for the main thread, and if the calling goroutine holds the
// app mutex while the main thread's frame is waiting for it, both hang.
func (a *App) requestWindowMaterials() {
	a.pendingMaterials.Store(true)
}

// applyWindowMaterials puts the selected material behind the main and settings
// windows when the setting is on.
func (a *App) applyWindowMaterials() {
	material := mygo.VibrancyNone
	switch a.themeSettings.Material {
	case "acrylic":
		material = mygo.VibrancyAcrylic
	case "mica":
		material = mygo.VibrancyMica
	}
	if a.mainWindow != nil && !a.mainWindow.IsDestroyed() {
		a.mainWindow.SetVibrancy(material)
	}
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.SetVibrancy(material)
	}
	if a.themePickerWin != nil && !a.themePickerWin.IsDestroyed() {
		a.themePickerWin.SetVibrancy(material)
	}
}

func (a *App) refreshWindows() {
	if c := a.ctx.Load(); c != nil {
		c.Invalidate()
	}
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Invalidate()
	}
	if a.themePickerWin != nil && !a.themePickerWin.IsDestroyed() {
		a.themePickerWin.Invalidate()
	}
}

func (a *App) save() { a.saveConfig() }

// toggleLightDark jumps between the default dark and light themes.
func (a *App) toggleLightDark() {
	if a.palette.Dark {
		a.themeSettings.Theme = "daylight"
	} else {
		a.themeSettings.Theme = "nocturne"
	}
	a.saveConfig()
	a.invalidateAll()
}

func (a *App) closePopovers() {
	a.materialOpen = false
	a.closeContextMenu()
}

// openConfirm opens the shared confirmation popup. The action runs only when
// the user confirms, so a destructive menu item cannot fire on a stray click.
func (a *App) openConfirm(title, message string, fn func()) {
	a.confirmTitle = title
	a.confirmInfo = message
	a.confirmFn = fn
}

func (a *App) closeConfirm() {
	a.confirmTitle = ""
	a.confirmInfo = ""
	a.confirmFn = nil
}

func (a *App) runConfirm() {
	if a.confirmFn == nil {
		return
	}
	fn := a.confirmFn
	a.closeConfirm()
	fn()
}

// confirmDialog draws the confirmation popup over the settings window.
func (a *App) confirmDialog(c *ui.Context) {
	if a.confirmFn == nil {
		return
	}
	p := a.palette
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.62)).Opacity(a.confirmAnim)
		panel.Width(430).MaxWidthPercent(94).Background(p.Panel).Radius(16).Border(1, p.Border).
			Padding(20).Gap(14).Shadow(0, 22, 60, 0, ui.RGBA(0, 0, 0, 0.34)).
			Opacity(a.confirmAnim).Translate(0, (1-a.confirmAnim)*-8)
		panel.Children(func() {
			title := a.confirmTitle
			if title == "" {
				title = "Are you sure?"
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(16).FontWeight(700).Grow(1)
				if a.iconButton(c, "x", "Close", 28).Clicked() {
					a.closeConfirm()
				}
			})
			ui.Text(c, a.confirmInfo).FontSize(11.2).TextColor(p.Muted).Wrap()
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c).Grow(1)
				if a.plainButton(c, "Cancel", "").Clicked() {
					a.closeConfirm()
				}
				if a.dangerButton(c, "Delete", "trash").Clicked() {
					a.runConfirm()
				}
			})
		})
	})
	if !open {
		a.closeConfirm()
	}
}

// --------------------------------------------------------------- widgets ---

func (a *App) settingsSection(c *ui.Context, title, subtitle string) {
	p := a.palette
	ui.Column(c).Gap(3).Margin(10, 0, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(680)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(10.5).TextColor(p.Muted).Wrap()
		}
	})
}

func (a *App) themeCard(c *ui.Context, id, name string, display palette) {
	p := a.palette
	if display.Accent.A == 0 {
		if accent, ok := presetAccents[id]; ok {
			display.Accent = accentColor(accent)
		} else {
			display.Accent = p.Accent
		}
	}
	selected := a.themeSettings.Theme == id || (a.themeSettings.Theme == "" && id == "system")
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
			ui.Text(c, name).FontSize(10.8).FontWeight(600).Grow(1).SingleLine()
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
		a.openThemeMenu(b, id)
	}
	if b.Clicked() {
		a.setTheme(id)
	}
}

func (a *App) newThemeCard(c *ui.Context) {
	p := a.palette
	b := ui.ButtonBase(c).Key("theme:new").Column().Height(94).Padding(10).Radius(11).Gap(7).Label("New custom theme")
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
			ui.Text(c, "New custom theme").FontSize(10.8).FontWeight(600).Grow(1).SingleLine()
			ui.Text(c, "gradient").FontSize(8.5).TextColor(p.Muted)
		})
	})
	if b.Clicked() {
		a.openThemePicker(nil, "card")
	}
}

func (a *App) card(c *ui.Context) ui.Element {
	p := a.palette
	return ui.Column(c).Background(p.Panel).Radius(14).Border(1, p.Border)
}

func (a *App) iconButton(c *ui.Context, name, tip string, size float32) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(size, size).Radius(8).Center().Tooltip(tip).Label(tip)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, icon(name, size)).Size(size*0.46, size*0.46).TextColor(t.TextMuted)
	})
	return b
}

func (a *App) plainButton(c *ui.Context, label, ic string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(34).Padding(0, 12).Radius(8).Gap(7).FocusRing(true).Label(label)
	b.Background(t.Surface)
	b.Border(1, t.Border)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 14)).Size(14, 14).TextColor(t.TextMuted)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(550)
	})
	return b
}

func (a *App) accentButton(c *ui.Context, label, ic string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(34).Padding(0, 14).Radius(8).Gap(7).FocusRing(true).Label(label)
	b.Background(t.Accent)
	b.TextColor(t.AccentText)
	if b.Hovered() {
		b.Background(t.AccentHover)
	}
	if b.Pressed() {
		b.Background(t.AccentPressed)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 15)).Size(14, 14).TextColor(t.AccentText)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(650).TextColor(t.AccentText)
	})
	return b
}

func (a *App) dangerButton(c *ui.Context, label, ic string) ui.Element {
	p := a.palette
	fg := contrastText(p.Bad)
	b := ui.ButtonBase(c).Height(34).Padding(0, 14).Radius(8).Gap(7).FocusRing(true).Label(label)
	b.Background(p.Bad)
	b.TextColor(fg)
	if b.Hovered() {
		b.Background(p.Bad.Mix(p.Text, 0.06))
	}
	if b.Pressed() {
		b.Background(p.Bad.Mix(ui.Hex("#000000"), 0.12))
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 15)).Size(14, 14).TextColor(fg)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(650).TextColor(fg)
	})
	return b
}

func (a *App) ghostButton(c *ui.Context, label, ic string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(32).Padding(0, 9).Radius(7).Gap(6).Label(label)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		if ic != "" {
			ui.Icon(c, icon(ic, 14)).Size(14, 14).TextColor(t.TextMuted)
		}
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
	})
	return b
}

func (a *App) badge(c *ui.Context, text string, color ui.Color) ui.Element {
	return ui.Text(c, text).FontSize(9).FontWeight(650).TextColor(contrastText(color)).
		Padding(2, 7).Radius(99).Background(color)
}
