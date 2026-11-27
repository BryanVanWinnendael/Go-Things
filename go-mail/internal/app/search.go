package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// searchPalette is the Ctrl+Space command palette: run actions (compose,
// settings, theme, sidebar, refresh, ...). Mail search lives in the folder
// search bar above the message list.
func (a *App) searchPalette(c *ui.Context) {
	p := a.palette
	open := a.searchOpen
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.62)).Opacity(a.dialogAnim)
		panel.Width(560).MaxWidthPercent(94).Background(p.Panel).Radius(16).Border(1, p.Border).
			Padding(20).Gap(14).Shadow(0, 22, 60, 0, ui.RGBA(0, 0, 0, 0.34)).
			Opacity(a.dialogAnim).Translate(0, (1-a.dialogAnim)*-8)
		panel.Children(func() {
			title := "Commands"
			if a.searchThemeMode {
				title = "Choose theme"
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(16).FontWeight(700).Grow(1)
				if a.iconButton(c, "x", "Close", 28).Clicked() {
					a.searchOpen = false
				}
			})
			in := ui.TextInput(c, &a.searchQuery).Placeholder("Type a command…").Height(36).FillWidth().FontSize(12.5).AutoFocus().Label("Search")
			_ = in
			q := strings.ToLower(strings.TrimSpace(a.searchQuery))

			type cmd struct {
				label string
				hint  string
				ic    string
				id    string
				run   func()
			}

			shown := 0
			ui.Scroll(c).MaxHeight(400).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					if a.searchThemeMode {
						// Back command.
						if q == "" || strings.Contains("back", q) {
							shown++
							b := ui.ButtonBase(c).FillWidth().Height(38).Padding(0, 9).Radius(8).Gap(9).Label("Back")
							if b.Hovered() {
								b.Background(p.Field)
							}
							b.Children(func() {
								ui.Icon(c, iconChevron).Rotate(180).Size(15, 15).TextColor(p.Muted)
								ui.Text(c, "Back").FontSize(11.5).FontWeight(560).Grow(1)
								ui.Text(c, "Return to commands").FontSize(9.5).TextColor(p.Muted).MaxWidth(240).SingleLine()
							})
							if b.Clicked() {
								if a.themePreviewLast != "" {
									a.themeSettings.Theme = a.themePreviewBase
									a.themePreviewLast = ""
									a.invalidateAll()
								}
								a.searchThemeMode = false
								a.searchQuery = ""
							}
						}
						themes := []cmd{{"System", "Follow desktop light/dark", "palette", "system", func() { a.setTheme("system") }}}
						for _, tp := range themePresets {
							themes = append(themes, cmd{tp.Name, "Built-in theme", "palette", tp.ID, func(id string) func() { return func() { a.setTheme(id) } }(tp.ID)})
						}
						for i := range a.customThemes {
							ct := &a.customThemes[i]
							name := ct.Name
							if name == "" {
								name = "Custom theme"
							}
							themes = append(themes, cmd{name, "Custom theme", "palette", ct.ID, func(id string) func() { return func() { a.setTheme(id) } }(ct.ID)})
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
									a.themeSettings.Theme = cm.id
									a.invalidateAll()
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
								a.searchOpen = false
								a.searchQuery = ""
								if cm.run != nil {
									cm.run()
								}
							}
						}
					} else {
						commands := []cmd{
							{"Compose new mail", "Open the compose window", "pen", "", func() { a.startCompose() }},
							{"Open settings", "Appearance and accounts", "settings", "", func() { a.openSettings() }},
							{"Switch theme", "Choose from available themes", "palette", "", func() {
								a.searchThemeMode = true
								a.searchQuery = ""
								a.themePreviewBase = a.themeSettings.Theme
								a.themePreviewLast = ""
							}},
							{"Toggle light / dark", "Switch between dark and light", "palette", "", func() { a.toggleLightDark() }},
							{"Toggle sidebar", "Show or hide the sidebar", "folder", "", func() { a.sidebarCollapsed = !a.sidebarCollapsed }},
							{"Refresh", "Reload the current folder", "refresh", "", func() { a.refresh() }},
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
									a.searchOpen = false
									a.searchQuery = ""
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
	})
	if !open {
		if a.themePreviewLast != "" {
			a.themeSettings.Theme = a.themePreviewBase
			a.themePreviewLast = ""
			a.invalidateAll()
		}
		a.searchOpen = false
		a.searchQuery = ""
		a.searchThemeMode = false
	}
}
