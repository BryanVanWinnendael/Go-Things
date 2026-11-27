package app

import (
	"fmt"
	"go-post/internal/core"
	"math"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	themePickerWidth  = float32(430)
	themePickerDot    = float32(10)
	themePickerSwatch = float32(26)
)

// openThemePicker starts the compact gradient picker. Every edit lands in the
// model immediately, so the real window behind the panel previews it; Done and
// the close button only dismiss the panel. anchor is the button the popover
// hangs from.
func (a *app) openThemePicker(ct *core.CustomTheme, anchor string) {
	a.themePickerOpen = true
	a.themePickerAnchor = anchor
	a.themePickerPoint = 0
	a.themePickerTarget = "gradient"
	a.modalError = ""

	if ct != nil {
		a.themePickerDraft = *ct
		a.ensurePickerDraft()
		a.themePickerIndex = a.customThemeIndex(ct.ID)
		if a.themePickerIndex < 0 {
			a.data.CustomThemes = append(a.data.CustomThemes, a.themePickerDraft)
			a.themePickerIndex = len(a.data.CustomThemes) - 1
		}
		a.data.Settings.Theme = a.themePickerDraft.ID
	} else {
		baseID := a.data.Settings.Theme
		if existing := customThemeByID(a.data.CustomThemes, baseID); existing != nil {
			baseID = existing.Base
		}
		if baseID == "" || baseID == "system" {
			baseID = a.palette.ID
			if baseID == "" {
				baseID = "nocturne"
			}
		}
		if !isPresetTheme(baseID) {
			baseID = "nocturne"
		}
		base := themeByID(baseID)
		// Start from the color the user actually sees, not a tint of the
		// accent. A solid theme stays solid until they choose a gradient.
		chrome := base.Bg
		if a.palette.Bg.A != 0 {
			chrome = a.palette.Bg
		}
		accent := a.palette.Accent
		if accent.A == 0 {
			if id, ok := presetAccents[baseID]; ok {
				accent = accentColor(id)
			}
		}
		if accent.A == 0 {
			accent = accentColor(a.data.Settings.Accent)
		}
		if accent.A == 0 {
			accent = ui.Hex("#10b981")
		}
		a.themePickerDraft = core.CustomTheme{
			ID:              core.NewID("theme"),
			Name:            a.nextThemeName(),
			Base:            baseID,
			Background:      hexColor(chrome),
			Accent:          hexColor(accent),
			Gradient:        true,
			GradientOpacity: defaultGradientAlpha,
			GradientMode:    "auto",
			TextColor:       "auto",
		}
		a.themePickerDraft.GradientPoints = a.defaultPickerPoints()
		a.data.CustomThemes = append(a.data.CustomThemes, a.themePickerDraft)
		a.themePickerIndex = len(a.data.CustomThemes) - 1
		a.data.Settings.Theme = a.themePickerDraft.ID
	}
	a.syncPickerLegacy()
	a.syncPickerInputs()
	a.pickerChanged()
	a.themePickerOriginal = a.themePickerDraft

	// Open as a separate draggable window instead of a popover.
	a.openThemePickerWindow()
}

// openThemePickerWindow shows the theme editor in its own draggable window.
func (a *app) openThemePickerWindow() {
	if a.themePickerWin != nil && !a.themePickerWin.IsDestroyed() {
		a.themePickerWin.Show()
		a.themePickerWin.Focus()
		a.themePickerWin.Invalidate()
		return
	}
	if a.win == nil {
		return
	}
	windowBg := "#0b0b0b"
	if a.palette.Bg.A != 0 {
		windowBg = hexColor(a.palette.Bg)
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:             "Theme editor",
		Width:             int(themePickerWidth),
		Height:            700,
		MinWidth:          390,
		MinHeight:         560,
		Parent:            a.win,
		Modal:             false,
		TitleBarStyle:     mygo.TitleBarHidden,
		HideWindowButtons: true,
		BackgroundColor:   windowBg,
		Vibrancy:          mygo.VibrancyAcrylic,
		StateKey:          "theme-picker",
		Content:           ui.View(a.themePickerWindowView),
	})
	a.themePickerWin = w
	a.applyWindowMaterials()
	w.OnClosed(func() {
		a.themePickerOpen = false
		a.closePopovers()
		a.save()
		a.themePickerWin = nil
		a.refreshWindows()
	})
	w.Show()
	w.Focus()
}

func (a *app) closeThemePicker() {
	a.themePickerOpen = false
	if a.themePickerWin != nil && !a.themePickerWin.IsDestroyed() {
		a.themePickerWin.Close()
		a.themePickerWin = nil
	}
	a.save()
	a.refreshWindows()
}

func (a *app) themePickerWindowView(c *ui.Context) {
	a.viewToken = 3
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
	a.paintChrome(c, p, vibrancy)

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Height(40).Shrink(0).Padding(0, 10).Gap(9).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(26, 26).Radius(8).Background(p.Accent).Center().Children(func() {
				ui.Icon(c, icon("palette", 14)).Size(14, 14).TextColor(contrastText(p.Accent))
			})
			ui.Column(c).Gap(0).Children(func() {
				ui.Text(c, "Theme editor").FontSize(12.5).FontWeight(700)
				ui.Text(c, "Changes apply live").FontSize(9).TextColor(p.Muted)
			})
			ui.Spacer(c).Grow(1)
			a.windowControls(c, a.themePickerWin)
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(18, 20, 26, 20).Gap(20).Children(func() {
				a.pickerName(c)
				a.pickerColors(c)
				a.pickerTextColorRow(c)
			})
		})
		a.pickerFooter(c)
	})
}

// pickerSectionHeader labels a group of controls. Groups stay visible so the
// editor reads as one page instead of a stack of folded panels.
func (a *app) pickerSectionHeader(c *ui.Context, title, subtitle string) {
	p := a.palette
	ui.Column(c).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(11.5).FontWeight(650)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(9.5).TextColor(p.Muted).Wrap()
		}
	})
}

// pickerColors edits one colour at a time. A segmented control chooses which
// part of the theme the field below is changing, so the window keeps a single
// colour picker instead of a stack of them.
func (a *app) pickerColors(c *ui.Context) {
	ui.Column(c).Gap(12).Children(func() {
		a.pickerSectionHeader(c, "Colors", "Choose a part of the theme, then pick its color.")
		a.pickerTargetRow(c)
		if a.themePickerTarget == "gradient" {
			a.pickerPresets(c)
			a.pickerCanvas(c)
			a.pickerColorRow(c)
		}
		a.pickerColorControls(c)
		switch a.themePickerTarget {
		case "gradient":
			a.pickerColorInput(c)
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					a.pickerPercentSlider(c, "Opacity", &a.themePickerDraft.GradientOpacity)
				})
				ui.Column(c).Grow(1).Children(func() {
					a.pickerPercentSlider(c, "Grain", &a.themePickerDraft.GradientGrain)
				})
			})
		case "accent":
			a.pickerAccentRow(c)
		case "surface":
			a.pickerSurfaceInput(c)
		}
	})
}

func (a *app) customThemeIndex(id string) int {
	for i := range a.data.CustomThemes {
		if a.data.CustomThemes[i].ID == id {
			return i
		}
	}
	return -1
}

func (a *app) nextThemeName() string {
	used := map[string]bool{}
	for _, ct := range a.data.CustomThemes {
		used[normalizedThemeName(ct.Name)] = true
	}
	for i := 1; ; i++ {
		name := "Custom theme"
		if i > 1 {
			name = fmt.Sprintf("Custom theme %d", i)
		}
		if !used[normalizedThemeName(name)] {
			return name
		}
	}
}

func normalizedThemeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// ensurePickerDraft converts an older theme to editable gradient points and
// makes sure the picker always has something to drag.
func (a *app) ensurePickerDraft() {
	points := customGradientPoints(a.themePickerDraft)
	converted := make([]core.CustomGradientPoint, 0, len(points))
	for _, p := range points {
		converted = append(converted, core.CustomGradientPoint{
			Color: hexColor(p.Color),
			X:     clampRange(p.X, 0, 1),
			Y:     clampRange(p.Y, 0, 1),
		})
	}
	if len(converted) == 0 {
		converted = a.defaultPickerPoints()[:1]
	}
	a.themePickerDraft.Gradient = true
	a.themePickerDraft.GradientPoints = converted
	if a.themePickerDraft.GradientOpacity <= 0 {
		a.themePickerDraft.GradientOpacity = defaultGradientAlpha
	}
	if a.themePickerDraft.GradientMode == "" {
		a.themePickerDraft.GradientMode = "auto"
	}
	if a.themePickerDraft.TextColor == "" {
		a.themePickerDraft.TextColor = "auto"
	}
	if a.themePickerPoint < 0 || a.themePickerPoint >= len(converted) {
		a.themePickerPoint = 0
	}
}

// defaultPickerPoints seeds a mesh from the colors already on screen. When the
// theme the user started from has a gradient, its mesh is kept. A solid theme
// gets a single point of its own chrome color: editing it must not silently
// replace that color with the accent.
func (a *app) defaultPickerPoints() []core.CustomGradientPoint {
	if len(a.palette.Gradient) > 0 {
		points := make([]core.CustomGradientPoint, 0, maxThemeGradientPoints)
		for i, point := range a.palette.Gradient {
			if i == maxThemeGradientPoints {
				break
			}
			points = append(points, core.CustomGradientPoint{
				Color: hexColor(point.Color),
				X:     point.X,
				Y:     point.Y,
			})
		}
		return points
	}
	chrome := a.palette.Bg
	if chrome.A == 0 {
		chrome = themeByID(a.themePickerDraft.Base).Bg
	}
	return []core.CustomGradientPoint{{Color: hexColor(chrome), X: 0.5, Y: 0.5}}
}

// pickerSeedColor is the colour new gradient points start from.
func (a *app) pickerSeedColor() ui.Color {
	if a.themePickerDraft.Accent != "" {
		return accentColor(a.themePickerDraft.Accent)
	}
	if a.palette.Accent.A != 0 {
		return a.palette.Accent
	}
	if a.data.Settings.Accent != "" {
		return accentColor(a.data.Settings.Accent)
	}
	return ui.Hex("#10b981")
}

// pickerChanged writes the live draft back into the model. Both windows are
// invalidated so the preview behind the panel follows every drag.
func (a *app) pickerChanged() {
	a.syncPickerLegacy()
	// Intensity is fully user-controlled; readability is handled by the
	// chosen text color (black/white) rather than clamping opacity.
	if a.themePickerIndex >= 0 && a.themePickerIndex < len(a.data.CustomThemes) {
		a.data.CustomThemes[a.themePickerIndex] = a.themePickerDraft
	} else if i := a.customThemeIndex(a.themePickerDraft.ID); i >= 0 {
		a.themePickerIndex = i
		a.data.CustomThemes[i] = a.themePickerDraft
	} else {
		a.data.CustomThemes = append(a.data.CustomThemes, a.themePickerDraft)
		a.themePickerIndex = len(a.data.CustomThemes) - 1
	}
	a.data.Settings.Theme = a.themePickerDraft.ID
	// The editor previews live, so the workspace follows the draft as it is
	// edited. Closing or reverting therefore commits the right choice, and a
	// reload does not undo it.
	if w := a.ws(); w != nil {
		w.Theme = a.themePickerDraft.ID
	}
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Invalidate()
	}
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Invalidate()
	}
	if a.themePickerWin != nil && !a.themePickerWin.IsDestroyed() {
		a.themePickerWin.Invalidate()
	}
}

// clampPickerReadability lowers the gradient opacity until chrome text keeps
// its 4.5:1 contrast. The colour picker can produce a bright colour at any
// moment, and silently backing the wash off is better than shipping an
// unreadable theme.
func (a *app) clampPickerReadability() {
	accent := a.palette.Accent
	if accent.A == 0 {
		accent = accentColor(a.data.Settings.Accent)
	}
	for i := 0; i < 20; i++ {
		if customThemeProblem(a.themePickerDraft, accent) == "" {
			return
		}
		next := a.themePickerDraft.GradientOpacity - 0.05
		if next < 0 {
			next = 0
		}
		if next == a.themePickerDraft.GradientOpacity {
			return
		}
		a.themePickerDraft.GradientOpacity = next
	}
}

func (a *app) syncPickerLegacy() {
	if len(a.themePickerDraft.GradientPoints) > 0 {
		a.themePickerDraft.From = a.themePickerDraft.GradientPoints[0].Color
	}
	if len(a.themePickerDraft.GradientPoints) > 1 {
		a.themePickerDraft.To = a.themePickerDraft.GradientPoints[1].Color
	}
}

// syncPickerInputs points the hex fields at the current draft. It runs when the
// picker opens or resets, not on every keystroke, so typing is never rewritten.
func (a *app) syncPickerInputs() {
	a.syncPickerPointHex()
	a.themePickerSurface = strings.TrimSpace(a.themePickerDraft.Surface)
	if accent := a.themePickerDraft.Accent; accent != "" && !strings.HasPrefix(accent, "#") {
		a.themePickerAccentHex = hexColor(accentColor(accent))
	} else {
		a.themePickerAccentHex = accent
	}
}

func (a *app) syncPickerPointHex() {
	if i := a.themePickerPoint; i >= 0 && i < len(a.themePickerDraft.GradientPoints) {
		a.themePickerHex = a.themePickerDraft.GradientPoints[i].Color
	}
}

func (a *app) pickerSurfaceColor() ui.Color {
	return colorOr(a.themePickerDraft.Surface, themeByID(a.themePickerDraft.Base).Panel)
}

// pickerPointIndex is the selected gradient point, clamped to the list.
func (a *app) pickerPointIndex() int {
	if i := a.themePickerPoint; i >= 0 && i < len(a.themePickerDraft.GradientPoints) {
		return i
	}
	if len(a.themePickerDraft.GradientPoints) > 0 {
		a.themePickerPoint = 0
		return 0
	}
	return -1
}

// pickerActiveColor is the colour the colour field edits.
func (a *app) pickerActiveColor() ui.Color {
	switch a.themePickerTarget {
	case "accent":
		return a.pickerAccentColor()
	case "surface":
		return a.pickerSurfaceColor()
	default:
		if i := a.pickerPointIndex(); i >= 0 {
			return colorOr(a.themePickerDraft.GradientPoints[i].Color, a.palette.Accent)
		}
		return a.palette.Accent
	}
}

// setPickerActiveColor writes the colour field back into the active part of
// the theme. Colours are opaque; opacity belongs to the gradient as a whole.
func (a *app) setPickerActiveColor(col ui.Color) {
	col = ui.RGB(col.R, col.G, col.B)
	hex := hexColor(col)
	switch a.themePickerTarget {
	case "accent":
		a.themePickerDraft.Accent = hex
		a.themePickerAccentHex = hex
	case "surface":
		a.themePickerDraft.Surface = hex
		a.themePickerSurface = hex
	default:
		if i := a.pickerPointIndex(); i >= 0 {
			a.themePickerDraft.GradientPoints[i].Color = hex
			a.themePickerHex = hex
		}
	}
	a.pickerChanged()
}

// pickerColorControls edits the active colour: a saturation and brightness
// square with a hue bar under it. There is no separate brightness control, so
// a colour always keeps the exact shade the user picks.
func (a *app) pickerColorControls(c *ui.Context) {
	p := a.palette
	col := a.pickerActiveColor()
	h, s, v := rgbToHSV(col)
	ui.Column(c).Gap(9).Children(func() {
		square := ui.Box(c).FillWidth().Height(116).Radius(10).Clip().Border(1, p.Border).
			Cursor(ui.CursorCrosshair).Focusable().FocusRing(true).Label("Color square")
		square.Draw(func(pt *ui.Painter, r ui.Rect) {
			hue := hsvColor(h, 1, 1)
			pt.FillGradient(r, ui.LinearGradient{From: ui.RGB(255, 255, 255), To: hue, Angle: 90}, 0)
			pt.FillGradient(r, ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0), To: ui.RGB(0, 0, 0), Angle: 180}, 0)
			cx, cy := r.X+r.W*clamp01(s), r.Y+r.H*(1-clamp01(v))
			d := float32(9)
			pt.Stroke(ui.Rect{X: cx - d, Y: cy - d, W: d * 2, H: d * 2}, ui.RGBA(0, 0, 0, 0.35), d, 1)
			pt.Stroke(ui.Rect{X: cx - d + 1, Y: cy - d + 1, W: d*2 - 2, H: d*2 - 2}, ui.RGB(255, 255, 255), d, 2)
		})
		bounds := square.Bounds()
		px, py, over := square.PointerPosition()
		if over && square.Pressed() && bounds.W > 0 && bounds.H > 0 {
			a.setPickerActiveColor(hsvColor(h, clamp01(px/bounds.W), 1-clamp01(py/bounds.H)))
		}
		square.OnShortcut(0, ui.KeyLeft, func() { a.setPickerActiveColor(hsvColor(h, clamp01(s-0.02), v)) })
		square.OnShortcut(0, ui.KeyRight, func() { a.setPickerActiveColor(hsvColor(h, clamp01(s+0.02), v)) })
		square.OnShortcut(0, ui.KeyUp, func() { a.setPickerActiveColor(hsvColor(h, s, clamp01(v+0.02))) })
		square.OnShortcut(0, ui.KeyDown, func() { a.setPickerActiveColor(hsvColor(h, s, clamp01(v-0.02))) })

		hue := ui.Box(c).FillWidth().Height(12).Radius(6).Clip().Border(1, p.Border).
			Cursor(ui.CursorCrosshair).Focusable().FocusRing(true).Label("Hue")
		hue.Draw(func(pt *ui.Painter, r ui.Rect) {
			hues := []ui.Color{
				ui.RGB(255, 0, 0), ui.RGB(255, 255, 0), ui.RGB(0, 255, 0),
				ui.RGB(0, 255, 255), ui.RGB(0, 0, 255), ui.RGB(255, 0, 255), ui.RGB(255, 0, 0),
			}
			w := r.W / 6
			for i := 0; i < 6; i++ {
				pt.FillGradient(ui.Rect{X: r.X + w*float32(i), Y: r.Y, W: w + 0.5, H: r.H},
					ui.LinearGradient{From: hues[i], To: hues[i+1], Angle: 90}, 0)
			}
			kx := r.X + r.W*(h/360)
			pt.Fill(ui.Rect{X: kx - 2.5, Y: r.Y, W: 5, H: r.H}, ui.RGB(255, 255, 255), 2.5)
			pt.Stroke(ui.Rect{X: kx - 2.5, Y: r.Y, W: 5, H: r.H}, ui.RGBA(0, 0, 0, 0.28), 2.5, 1)
		})
		hb := hue.Bounds()
		hx, _, hover := hue.PointerPosition()
		if hover && hue.Pressed() && hb.W > 0 {
			a.setPickerActiveColor(hsvColor(clamp01(hx/hb.W)*360, s, v))
		}
		hue.OnShortcut(0, ui.KeyLeft, func() { a.setPickerActiveColor(hsvColor(normalizeAngle(h-4), s, v)) })
		hue.OnShortcut(0, ui.KeyRight, func() { a.setPickerActiveColor(hsvColor(normalizeAngle(h+4), s, v)) })
	})
}

// applyPickerSurface writes a main view color into the draft. Any color is
// allowed: text can be set to black or white separately, so readability is the
// user's choice rather than a restriction here.
func (a *app) applyPickerSurface(color string) {
	a.themePickerDraft.Surface = color
	a.pickerChanged()
}

// rgbToHSV is the inverse of hsvColor, used to place the picker's dot.
func rgbToHSV(c ui.Color) (h, s, v float32) {
	r := float32(c.R) / 255
	g := float32(c.G) / 255
	b := float32(c.B) / 255
	maxC := max(r, max(g, b))
	minC := min(r, min(g, b))
	v = maxC
	d := maxC - minC
	if d == 0 {
		return 0, 0, v
	}
	s = d / maxC
	switch maxC {
	case r:
		h = 60 * float32(math.Mod(float64((g-b)/d), 6))
	case g:
		h = 60 * ((b-r)/d + 2)
	default:
		h = 60 * ((r-g)/d + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, s, v
}

func (a *app) resetThemePicker() {
	a.themePickerDraft = a.themePickerOriginal
	a.themePickerPoint = 0
	a.syncPickerInputs()
	a.pickerChanged()
}

func (a *app) addPickerPoint() {
	if len(a.themePickerDraft.GradientPoints) >= maxThemeGradientPoints {
		return
	}
	x, y := a.freePickerSpot()
	a.themePickerDraft.GradientPoints = append(a.themePickerDraft.GradientPoints, core.CustomGradientPoint{
		Color: hexColor(a.pickerSeedColor()),
		X:     x,
		Y:     y,
	})
	a.themePickerPoint = len(a.themePickerDraft.GradientPoints) - 1
	a.syncPickerPointHex()
	a.pickerChanged()
}

// freePickerSpot picks the candidate farthest from the dots already placed.
func (a *app) freePickerSpot() (float32, float32) {
	candidates := [][2]float32{
		{0.50, 0.18}, {0.82, 0.50}, {0.50, 0.82}, {0.18, 0.50},
		{0.78, 0.22}, {0.22, 0.78}, {0.78, 0.78}, {0.22, 0.22},
	}
	best := candidates[0]
	bestGap := float32(-1)
	for _, cand := range candidates {
		near := float32(2)
		for _, point := range a.themePickerDraft.GradientPoints {
			dx := cand[0] - point.X
			dy := cand[1] - point.Y
			if d := dx*dx + dy*dy; d < near {
				near = d
			}
		}
		if near > bestGap {
			bestGap, best = near, cand
		}
	}
	return best[0], best[1]
}

func (a *app) removePickerPoint() {
	if len(a.themePickerDraft.GradientPoints) <= minThemeGradientPoints {
		return
	}
	i := a.themePickerPoint
	if i < 0 || i >= len(a.themePickerDraft.GradientPoints) {
		i = len(a.themePickerDraft.GradientPoints) - 1
	}
	points := a.themePickerDraft.GradientPoints
	a.themePickerDraft.GradientPoints = append(points[:i], points[i+1:]...)
	if a.themePickerPoint >= len(a.themePickerDraft.GradientPoints) {
		a.themePickerPoint = len(a.themePickerDraft.GradientPoints) - 1
	}
	a.syncPickerPointHex()
	a.pickerChanged()
}

func (a *app) nudgePickerPoint(dx, dy float32) {
	i := a.pickerPointIndex()
	if i < 0 {
		return
	}
	p := &a.themePickerDraft.GradientPoints[i]
	p.X = clamp01(p.X + dx)
	p.Y = clamp01(p.Y + dy)
	a.pickerChanged()
}

func (a *app) nearestPickerPoint(px, py, w, h float32) int {
	closest, distance := -1, float32(28*28)
	for i, point := range a.themePickerDraft.GradientPoints {
		dx := px - point.X*w
		dy := py - point.Y*h
		if d := dx*dx + dy*dy; d <= distance {
			closest, distance = i, d
		}
	}
	return closest
}

func (a *app) applyGradientPreset(gp gradientPreset) {
	a.themePickerDraft.Gradient = true
	a.themePickerDraft.GradientPoints = presetThemePoints(gp)
	a.themePickerDraft.GradientOpacity = defaultGradientAlpha
	a.themePickerDraft.GradientGrain = 0
	a.themePickerPoint = 0
	a.syncPickerPointHex()
	a.pickerChanged()
}

func (a *app) gradientPresetPalette(gp gradientPreset) palette {
	p := a.palette
	p.HasGradient = true
	p.GradientAlpha = 1
	p.GradientGrain = 0
	p.Gradient = presetGradientPoints(gp)
	p.Bg = customChromeColor(themeByID(a.themePickerDraft.Base), colorOr(a.themePickerDraft.Background, p.Bg))
	return p
}

func presetGradientPoints(gp gradientPreset) []gradientPoint {
	points := make([]gradientPoint, 0, maxThemeGradientPoints)
	for i, hex := range gp.Colors {
		if i == maxThemeGradientPoints {
			break
		}
		points = append(points, gradientPoint{
			Color: ui.Hex(hex),
			X:     themeGradientPositions[i][0],
			Y:     themeGradientPositions[i][1],
		})
	}
	return points
}

func presetThemePoints(gp gradientPreset) []core.CustomGradientPoint {
	points := make([]core.CustomGradientPoint, 0, maxThemeGradientPoints)
	for i, hex := range gp.Colors {
		if i == maxThemeGradientPoints {
			break
		}
		points = append(points, core.CustomGradientPoint{
			Color: hex,
			X:     themeGradientPositions[i][0],
			Y:     themeGradientPositions[i][1],
		})
	}
	return points
}

var themeGradientPositions = [maxThemeGradientPoints][2]float32{
	{0.18, 0.26},
	{0.52, 0.58},
	{0.82, 0.76},
}

// themePicker is now a no-op; the picker lives in its own window.
func (a *app) themePicker(c *ui.Context) {
	return
}

func (a *app) pickerName(c *ui.Context) {
	p := a.palette
	ui.Column(c).Gap(5).Children(func() {
		ui.Text(c, "Theme name").FontSize(10).FontWeight(600).TextColor(p.Muted)
		name := ui.TextInput(c, &a.themePickerDraft.Name)
		name.Placeholder("Name this theme").Height(34).FillWidth().FontSize(12).Label("Theme name")
		if name.Changed() {
			a.pickerChanged()
		}
	})
}

// pickerTargetRow switches which part of the theme the colour field edits.
func (a *app) pickerTargetRow(c *ui.Context) {
	p := a.palette
	targets := []struct{ id, label string }{
		{"gradient", "Background"},
		{"accent", "Accent"},
		{"surface", "Main view"},
	}
	ui.Row(c).FillWidth().Gap(4).Padding(3).Radius(10).Background(p.Field).
		Border(1, p.Border).Children(func() {
		for _, t := range targets {
			t := t
			selected := a.themePickerTarget == t.id
			b := ui.ButtonBase(c).Grow(1).Height(28).Radius(7).Center().FocusRing(true).Label(t.label)
			if selected {
				b.Background(p.Panel)
				b.Border(1, p.Border)
			} else if b.Hovered() {
				b.Background(p.Hover)
			}
			b.Children(func() {
				col := p.Muted
				if selected {
					col = p.Text
				}
				ui.Text(c, t.label).FontSize(10.5).FontWeight(func() int {
					if selected {
						return 650
					}
					return 500
				}()).TextColor(col)
			})
			if b.Clicked() {
				a.themePickerTarget = t.id
			}
		}
	})
}

func (a *app) pickerPresets(c *ui.Context) {
	p := a.palette
	ui.Column(c).Gap(7).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Start from").FontSize(10).FontWeight(600).TextColor(p.Muted).Grow(1)
			ui.Text(c, "Pick a look, then tune it").FontSize(9).TextColor(p.Muted)
		})
		ui.Row(c).FillWidth().Gap(7).Children(func() {
			for _, gp := range gradientPresets {
				gp := gp
				preset := a.gradientPresetPalette(gp)
				selected := a.pickerPresetSelected(gp)
				b := ui.ButtonBase(c).Size(themePickerSwatch, themePickerSwatch).
					Radius(themePickerSwatch / 2).Tooltip(gp.Name).Label(gp.Name).FocusRing(true)
				b.Draw(func(pt *ui.Painter, r ui.Rect) {
					pt.Clip(r, r.W/2, func() {
						pt.Fill(r, preset.Bg, 0)
						paintChromeGradient(pt, r, preset, 1)
					})
					border := p.Border
					if b.Hovered() || selected {
						border = p.Text.Alpha(0.55)
					}
					pt.Stroke(r, border, r.W/2, 1)
					if selected {
						pt.Stroke(ui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4}, p.Accent, r.W/2+2, 2)
					}
				})
				if b.Clicked() {
					a.applyGradientPreset(gp)
				}
			}
		})
	})
}

func (a *app) pickerPresetSelected(gp gradientPreset) bool {
	want := presetThemePoints(gp)
	got := a.themePickerDraft.GradientPoints
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if got[i].Color != want[i].Color ||
			math.Abs(float64(got[i].X-want[i].X)) > 0.001 ||
			math.Abs(float64(got[i].Y-want[i].Y)) > 0.001 {
			return false
		}
	}
	return true
}

// pickerCanvas is the mesh map: dots show where each colour lands in the
// window. Dragging moves a dot; the colour itself is chosen below.
func (a *app) pickerCanvas(c *ui.Context) {
	p := a.palette
	canvas := ui.Box(c).FillWidth().Height(118).Radius(10).Clip().Border(1, p.Border).
		Cursor(ui.CursorGrab).Focusable().FocusRing(true).Label("Background color canvas")
	canvas.Draw(func(pt *ui.Painter, r ui.Rect) {
		a.drawPickerCanvas(pt, r)
	})
	canvas.OnShortcut(0, ui.KeyLeft, func() { a.nudgePickerPoint(-0.02, 0) })
	canvas.OnShortcut(0, ui.KeyRight, func() { a.nudgePickerPoint(0.02, 0) })
	canvas.OnShortcut(0, ui.KeyUp, func() { a.nudgePickerPoint(0, -0.02) })
	canvas.OnShortcut(0, ui.KeyDown, func() { a.nudgePickerPoint(0, 0.02) })

	bounds := canvas.Bounds()
	px, py, over := canvas.PointerPosition()
	drag := ui.Local(canvas, "theme-picker-drag", func() int { return -1 })
	if over && canvas.Pressed() && bounds.W > 0 && bounds.H > 0 {
		if *drag < 0 {
			if nearest := a.nearestPickerPoint(px, py, bounds.W, bounds.H); nearest >= 0 {
				*drag = nearest
				a.themePickerTarget = "gradient"
				a.themePickerPoint = nearest
				a.syncPickerPointHex()
			}
		}
		if *drag >= 0 && *drag < len(a.themePickerDraft.GradientPoints) {
			point := &a.themePickerDraft.GradientPoints[*drag]
			point.X = clamp01(px / bounds.W)
			point.Y = clamp01(py / bounds.H)
			canvas.Cursor(ui.CursorGrabbing)
			a.pickerChanged()
		}
	} else if !canvas.Pressed() {
		*drag = -1
	}
}

func (a *app) drawPickerCanvas(pt *ui.Painter, r ui.Rect) {
	display := customPalette(a.themePickerDraft)
	if display.Accent.A == 0 {
		display.Accent = a.palette.Accent
	}
	pt.Fill(r, display.Bg, 0)
	if display.HasGradient {
		paintChromeGradient(pt, r, display, 1)
	}
	for i, point := range a.themePickerDraft.GradientPoints {
		col := colorOr(point.Color, a.palette.Accent)
		cx := r.X + clamp01(point.X)*r.W
		cy := r.Y + clamp01(point.Y)*r.H
		radius := themePickerDot
		if i == a.themePickerPoint {
			radius += 2
		}
		drawPickerDot(pt, cx, cy, radius, col, i == a.themePickerPoint)
	}
}

func drawPickerDot(pt *ui.Painter, cx, cy, radius float32, col ui.Color, selected bool) {
	dot := ui.Rect{X: cx - radius, Y: cy - radius, W: radius * 2, H: radius * 2}
	pt.Stroke(ui.Rect{X: dot.X - 2, Y: dot.Y - 2, W: dot.W + 4, H: dot.H + 4}, ui.RGBA(0, 0, 0, 0.30), radius+2, 1)
	pt.Fill(dot, col, radius)
	pt.Stroke(dot, ui.RGBA(255, 255, 255, 0.92), radius, 2)
	if selected {
		pt.Stroke(ui.Rect{X: dot.X - 4, Y: dot.Y - 4, W: dot.W + 8, H: dot.H + 8}, ui.RGBA(255, 255, 255, 0.40), radius+4, 1)
	}
}

func (a *app) pickerColorRow(c *ui.Context) {
	p := a.palette
	canAdd := len(a.themePickerDraft.GradientPoints) < maxThemeGradientPoints
	canRemove := len(a.themePickerDraft.GradientPoints) > minThemeGradientPoints
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Colors").FontSize(9.5).TextColor(p.Muted)
		for i := range a.themePickerDraft.GradientPoints {
			i := i
			point := a.themePickerDraft.GradientPoints[i]
			selected := i == a.themePickerPoint
			chip := ui.ButtonBase(c).Size(24, 24).Radius(12).Center().FocusRing(true).
				Tooltip(fmt.Sprintf("Color %d", i+1)).Label(fmt.Sprintf("Color %d", i+1))
			chip.Background(colorOr(point.Color, p.Bg))
			if selected {
				chip.Border(2, p.Text)
			} else {
				chip.Border(1, p.Border)
			}
			if chip.Clicked() {
				a.themePickerTarget = "gradient"
				a.themePickerPoint = i
				a.syncPickerPointHex()
			}
		}
		ui.Spacer(c).Grow(1)
		add := ui.ButtonBase(c).Size(24, 24).Radius(6).Center().FocusRing(true).
			Tooltip("Add color").Label("Add color").Disabled(!canAdd)
		add.Background(p.Field)
		add.Border(1, p.Border)
		add.Children(func() {
			col := p.Text
			if !canAdd {
				col = p.Muted.Alpha(0.45)
			}
			ui.Icon(c, icon("plus", 14)).Size(14, 14).TextColor(col)
		})
		if add.Clicked() && canAdd {
			a.addPickerPoint()
		}
		remove := ui.ButtonBase(c).Size(24, 24).Radius(6).Center().FocusRing(true).
			Tooltip("Remove color").Label("Remove color").Disabled(!canRemove)
		remove.Background(p.Field)
		remove.Border(1, p.Border)
		remove.Children(func() {
			col := p.Text
			if !canRemove {
				col = p.Muted.Alpha(0.45)
			}
			ui.Icon(c, icon("minus", 14)).Size(14, 14).TextColor(col)
		})
		if remove.Clicked() && canRemove {
			a.removePickerPoint()
		}
	})
}

func (a *app) pickerColorInput(c *ui.Context) {
	p := a.palette
	points := a.themePickerDraft.GradientPoints
	if len(points) == 0 {
		return
	}
	i := a.themePickerPoint
	if i < 0 || i >= len(points) {
		i = 0
	}
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, fmt.Sprintf("Color %d", i+1)).FontSize(9.5).TextColor(p.Muted).Width(46)
		input := a.textInput(c, &a.themePickerHex)
		input.Placeholder("#6d5dfc or rgb(109, 93, 252)").Height(28).FillWidth().
			FontSize(10.5).Font("monospace").Label(fmt.Sprintf("Color %d value", i+1))
		if input.Changed() {
			if col, ok := parseThemeColor(a.themePickerHex); ok {
				a.themePickerDraft.GradientPoints[i].Color = hexColor(col)
				a.pickerChanged()
			}
		}
	})
}

func (a *app) pickerSurfaceInput(c *ui.Context) {
	p := a.palette
	base := themeByID(a.themePickerDraft.Base)
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Hex").FontSize(9.5).TextColor(p.Muted).Width(24)
		swatch := ui.ButtonBase(c).Size(20, 20).Radius(5).Center().
			Tooltip("Main view color").Label("Main view color")
		swatch.Background(a.pickerSurfaceColor())
		swatch.Border(1, p.Border)
		input := a.textInput(c, &a.themePickerSurface)
		input.Placeholder(hexColor(base.Panel)).Height(28).FillWidth().
			FontSize(10.5).Font("monospace").Label("Main view value")
		if input.Changed() {
			raw := strings.TrimSpace(a.themePickerSurface)
			if raw == "" {
				a.applyPickerSurface("")
			} else if col, ok := parseThemeColor(raw); ok {
				a.applyPickerSurface(hexColor(col))
			}
		}
	})
}

func (a *app) pickerAccentRow(c *ui.Context) {
	p := a.palette
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(5).Wrap().Children(func() {
			for _, accent := range accentPresets {
				col := accentColor(accent.ID)
				if !p.Dark && accent.ID == "lime" {
					col = ui.Hex("#6d9e2f")
				}
				selected := a.themePickerDraft.Accent == accent.ID ||
					strings.EqualFold(a.themePickerDraft.Accent, hexColor(col))
				b := ui.ButtonBase(c).Size(themePickerSwatch, themePickerSwatch).
					Radius(themePickerSwatch / 2).Center().Tooltip(accent.Name).Label(accent.Name).FocusRing(true)
				b.Background(col)
				if selected {
					b.Border(2, p.Text)
				}
				b.Children(func() {
					if selected {
						ui.Icon(c, icon("check", 12)).Size(12, 12).TextColor(contrastText(col))
					}
				})
				if b.Clicked() {
					a.themePickerDraft.Accent = accent.ID
					a.themePickerAccentHex = hexColor(col)
					a.pickerChanged()
				}
			}
		})
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Hex").FontSize(9.5).TextColor(p.Muted).Width(24)
			input := a.textInput(c, &a.themePickerAccentHex)
			input.Placeholder("#ff7058").Height(30).FillWidth().FontSize(10.5).Font("monospace").Label("Accent color")
			if input.Changed() {
				raw := strings.TrimSpace(a.themePickerAccentHex)
				if raw == "" {
					a.themePickerDraft.Accent = ""
					a.pickerChanged()
				} else if col, ok := parseThemeColor(raw); ok {
					a.themePickerDraft.Accent = hexColor(col)
					a.pickerChanged()
				}
			}
		})
	})
}
func (a *app) pickerTextColorRow(c *ui.Context) {
	p := a.palette
	ui.Column(c).Gap(8).Children(func() {
		a.pickerSectionHeader(c, "Text", "Let the palette pick the text color, or force black or white.")
		mode := a.themePickerDraft.TextColor
		if mode == "" {
			mode = "auto"
		}
		modes := []struct{ id, label string }{
			{"auto", "Auto"},
			{"black", "Black"},
			{"white", "White"},
		}
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			for _, m := range modes {
				m := m
				selected := mode == m.id
				b := ui.ButtonBase(c).Grow(1).Height(30).Radius(7).Center().FocusRing(true).Label(m.label)
				if selected {
					b.Background(p.Panel)
					b.Border(1, p.Accent)
				} else if b.Hovered() {
					b.Background(p.Hover)
				}
				b.Children(func() {
					col := p.Muted
					if selected {
						col = p.Text
					}
					ui.Text(c, m.label).FontSize(10.5).FontWeight(func() int {
						if selected {
							return 650
						}
						return 500
					}()).TextColor(col)
				})
				if b.Clicked() {
					a.themePickerDraft.TextColor = m.id
					a.pickerChanged()
				}
			}
		})
	})
}

func (a *app) pickerPercentSlider(c *ui.Context, label string, value *float32) {
	p := a.palette
	pct := float64(clamp01(*value) * 100)
	ui.Column(c).Gap(3).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(10).FontWeight(600).TextColor(p.Muted).Grow(1)
			ui.Text(c, fmt.Sprintf("%.0f%%", pct)).FontSize(9.5).Font("monospace").TextColor(p.Muted)
		})
		if ui.Slider(c, &pct, 0, 100).FillWidth().Label(label).Changed() {
			if newVal := clamp01(float32(pct / 100)); newVal != *value {
				*value = newVal
				a.pickerChanged()
			}
		}
	})
}

func (a *app) pickerFooter(c *ui.Context) {
	p := a.palette
	ui.Row(c).FillWidth().Shrink(0).Padding(12, 18).Gap(8).AlignItems(ui.Center).
		Background(p.Bg).BorderWidth(1, 0, 0, 0).BorderColor(p.Border).Children(func() {
		if a.plainButton(c, "Revert", "refresh").Clicked() {
			a.resetThemePicker()
		}
		ui.Spacer(c).Grow(1)
		if a.accentButton(c, "Done", "check").Clicked() {
			a.closeThemePicker()
		}
	})
}

// pickerAccentColor is the accent the editor is tuning, falling back to the
// base theme's accent before the user has chosen one.
func (a *app) pickerAccentColor() ui.Color {
	if a.themePickerDraft.Accent == "" {
		return a.palette.Accent
	}
	return accentColor(a.themePickerDraft.Accent)
}
