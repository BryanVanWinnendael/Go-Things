package theme

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// Palette is the small set of colors the hub draws with. It follows the
// Nocturne palette of Go Post and Go Mail so the suite reads as one family.
// Its own accent is the theme's extreme: white on Nocturne, black on Daylight.
type Palette struct {
	ID     string
	Name   string
	Dark   bool
	Bg     ui.Color
	Panel  ui.Color
	Field  ui.Color
	Hover  ui.Color
	Border ui.Color
	Text   ui.Color
	Muted  ui.Color
	Accent ui.Color
	Good   ui.Color
	Warn   ui.Color
	Bad    ui.Color
}

var nocturne = Palette{
	ID: "nocturne", Name: "Nocturne", Dark: true,
	Bg:     ui.Hex("#0b0b0b"),
	Panel:  ui.Hex("#161616"),
	Field:  ui.Hex("#1d1d1d"),
	Hover:  ui.Hex("#242424"),
	Border: ui.RGBA(255, 255, 255, 0.075),
	Text:   ui.Hex("#f4f4f6"),
	Muted:  ui.Hex("#8a8f9f"),
	Accent: ui.Hex("#ffffff"),
	Good:   ui.Hex("#59d39b"),
	Warn:   ui.Hex("#f2b35d"),
	Bad:    ui.Hex("#ff6b70"),
}

var daylight = Palette{
	ID: "daylight", Name: "Daylight", Dark: false,
	Bg:     ui.Hex("#e9ebef"),
	Panel:  ui.Hex("#ffffff"),
	Field:  ui.Hex("#f2f3f6"),
	Hover:  ui.Hex("#e9ebef"),
	Border: ui.RGBA(20, 24, 34, 0.12),
	Text:   ui.Hex("#1a1c22"),
	Muted:  ui.Hex("#5d6472"),
	Accent: ui.Hex("#000000"),
	Good:   ui.Hex("#159664"),
	Warn:   ui.Hex("#c47b16"),
	Bad:    ui.Hex("#c73f45"),
}

// AccentState moves the flat accent away from its extreme so a pure white
// or black face still reacts to hover and press.
func (p Palette) AccentState(amount float32) ui.Color {
	toward := ui.Hex("#ffffff")
	if p.Dark {
		toward = ui.Hex("#000000")
	}
	return p.Accent.Mix(toward, amount)
}

// Resolve picks the app's palette. "system" follows the desktop.
func Resolve(id string, systemDark bool) Palette {
	switch id {
	case "nocturne":
		return nocturne
	case "daylight":
		return daylight
	}
	if systemDark {
		return nocturne
	}
	return daylight
}

func Apply(p Palette) *ui.Theme {
	t := ui.DarkTheme()
	if !p.Dark {
		t = ui.LightTheme()
	}
	t.Dark = p.Dark
	t.Background = p.Bg
	t.Surface = p.Field
	t.SurfaceHover = p.Hover
	t.SurfacePressed = p.Hover.Mix(p.Text, 0.06)
	t.Border = p.Border
	t.Text = p.Text
	t.TextMuted = p.Muted
	t.Accent = p.Accent
	t.AccentHover = p.AccentState(0.10)
	t.AccentPressed = p.AccentState(0.20)
	t.AccentText = ContrastText(p.Accent)
	t.Danger = p.Bad
	t.Warning = p.Warn
	t.Success = p.Good
	t.Selection = p.Accent.Alpha(0.28)
	t.Focus = p.Accent.Alpha(0.7)
	t.Inverse = p.Text
	t.InverseText = p.Bg
	t.Scrollbar = p.Text.Alpha(0.18)
	t.ScrollbarWidth = 8
	t.Radius = 8
	t.Spacing = 4
	t.Font = "Segoe UI"
	t.FontSize = 13
	return t
}

// ContrastText is the ink that stays readable on c.
func ContrastText(c ui.Color) ui.Color {
	l := relativeLuminance(c)
	dark := ui.Hex("#16171b")
	ld := relativeLuminance(dark)
	whiteContrast := 1.05 / (l + 0.05)
	darkContrast := (l + 0.05) / (ld + 0.05)
	if darkContrast >= whiteContrast {
		return dark
	}
	return ui.Hex("#ffffff")
}

func relativeLuminance(c ui.Color) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

// ReadableInk keeps a state color legible as small text: on light grounds
// it is darkened toward the text color.
func ReadableInk(c ui.Color, p Palette) ui.Color {
	if p.Dark {
		return c
	}
	return c.Mix(p.Text, 0.38)
}
