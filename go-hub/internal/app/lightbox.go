package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
)

// shotMatch ties a gallery thumbnail to the large copy in the lightbox. MyGo
// moves the enlarged image from the thumbnail's box and back, which is the
// continuity that makes clicking a screenshot feel like opening it rather
// than swapping views.
type shotMatch struct {
	App   string
	Index int
}

func shotMorph(m shotMatch) ui.ElementTransition {
	return ui.ElementTransition{
		Animation: &ui.Smooth,
		Match:     m,
		Enter:     &ui.Motion{Opacity: 0, Scale: 0.97},
	}
}

var shotScrim = ui.ElementTransition{
	Duration: 170 * time.Millisecond,
	Ease:     ui.EaseOut,
	Enter:    &ui.Motion{Opacity: 0},
	Exit:     &ui.Motion{Opacity: 0},
}

// stepShot moves through the gallery, wrapping at both ends.
func (a *App) stepShot(delta, count int) {
	if count <= 0 {
		return
	}
	a.shotIndex = (a.shotIndex + delta + count) % count
	if a.win != nil {
		a.win.Invalidate()
	}
}

// shotView shows one screenshot large over a dimmed window. DialogBase gives
// outside-click and Escape dismissal; the only authored motion is the image
// morphing from the thumbnail it came from, with the scrim fading under it.
func (a *App) shotView(c *ui.Context, app catalog.App, refs []string, shots []*ui.Bitmap) {
	if len(refs) == 0 {
		a.shotOpen = false
		return
	}
	if a.shotIndex < 0 || a.shotIndex >= len(refs) {
		a.shotIndex = 0
	}
	p := a.palette

	ui.DialogBase(c, &a.shotOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(4, 4, 6, 0.78)).Transition(shotScrim)
		panel.Key("shot-panel").WidthPercent(82).HeightPercent(84).MaxWidth(960).MaxHeight(660).
			Radius(16).Background(p.Panel).Border(1, p.Border).Clip().
			Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.42))

		c.OnShortcut(0, ui.KeyLeft, func() { a.stepShot(-1, len(refs)) })
		c.OnShortcut(0, ui.KeyRight, func() { a.stepShot(1, len(refs)) })

		panel.Children(func() {
			ui.Column(c).Fill().Children(func() {
				ui.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 12, 0, 16).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Text(c, app.Name).FontSize(12.5).FontWeight(650).Grow(1).SingleLine()
					ui.Text(c, fmt.Sprintf("%d / %d", a.shotIndex+1, len(refs))).FontSize(11).TextColor(p.Muted).SingleLine()
					if a.iconButton(c, "shot-close", iconX, "Close screenshot").Clicked() {
						a.shotOpen = false
					}
				})
				ui.Box(c).FillWidth().Height(1).Shrink(0).Background(p.Border)
				ui.Box(c).Grow(1).MinHeight(0).Padding(12, 16).Center().Children(func() {
					if a.shotIndex < len(shots) && shots[a.shotIndex] != nil {
						img := ui.Image(c, shots[a.shotIndex]).Fit(ui.Contain).FillWidth().FillHeight()
						img.Key(fmt.Sprintf("shot-image:%d", a.shotIndex)).
							Transition(shotMorph(shotMatch{App: app.ID, Index: a.shotIndex}))
					} else {
						ui.Icon(c, appGlyph(app.ID)).Size(28, 28).TextColor(p.Muted)
					}
				})
				ui.Box(c).FillWidth().Height(1).Shrink(0).Background(p.Border)
				ui.Row(c).FillWidth().Height(48).Shrink(0).Padding(0, 12, 0, 16).Gap(10).AlignItems(ui.Center).Children(func() {
					if a.iconButton(c, "shot-prev", iconBack, "Previous screenshot").Clicked() {
						a.stepShot(-1, len(refs))
					}
					if a.iconButton(c, "shot-next", iconNext, "Next screenshot").Clicked() {
						a.stepShot(1, len(refs))
					}
					ui.Spacer(c).Grow(1)
					ui.Text(c, "← → to browse · Esc to close").FontSize(10.5).TextColor(p.Muted).SingleLine()
				})
			})
		})
	})
}
