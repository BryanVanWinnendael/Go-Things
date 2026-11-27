package app

import (
	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
)

func (a *App) aboutPage(c *ui.Context) {
	p := a.palette
	ui.Column(c).FillWidth().MaxWidth(620).Gap(16).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "The home of the Go Things suite").FontSize(21).FontWeight(720).LetterSpacing(-0.4)
			ui.Text(c, "Go Hub reads the app list from this project's latest GitHub release, downloads only the setup you choose, opens it when you want it, and removes it again. No account or store service is involved.").
				FontSize(12.5).TextColor(p.Muted).LineHeight(1.6)
		})
		ui.Column(c).FillWidth().Gap(8).Margin(2, 0, 0, 0).Children(func() {
			a.fact(c, "Version", catalog.Version)
			a.fact(c, "Platform", "Windows 10 and 11, 64-bit")
			a.fact(c, "Installs to", `%LOCALAPPDATA%\Programs, for your user only`)
			a.fact(c, "Catalog", "catalog.json in the latest GitHub release")
			a.fact(c, "Network", "Reads the catalog and downloads only the setup you install.")
		})
		ui.Text(c, "Built with Go and MyGo. The previews are screenshots of the built apps.").FontSize(10.5).TextColor(p.Muted).Margin(2, 0, 0, 0)
	})
}
func (a *App) fact(c *ui.Context, label, value string) {
	p := a.palette
	ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Text(c, label).Width(104).Shrink(0).FontSize(11).FontWeight(600).TextColor(p.Muted).SingleLine()
		ui.Text(c, value).FontSize(12).Grow(1).MaxLines(2)
	})
}
