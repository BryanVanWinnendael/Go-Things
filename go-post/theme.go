package main

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

type palette struct {
	ID          string
	Name        string
	Dark        bool
	Bg          ui.Color
	Side        ui.Color
	Panel       ui.Color
	Field       ui.Color
	Hover       ui.Color
	Border      ui.Color
	Text        ui.Color
	Muted       ui.Color
	Accent      ui.Color
	Good        ui.Color
	Warn        ui.Color
	Bad         ui.Color
	Shadow      ui.Color
	Overlay     ui.Color
	HasGradient bool
	// Gradient is a small mesh: one to three colored points in normalized
	// window coordinates. Rendering it as a smooth field, rather than a
	// stack of rings, keeps the chrome free of visible circles.
	Gradient      []gradientPoint
	GradientAlpha float32
	GradientGrain float32
	GradientMode  string
}

type gradientPoint struct {
	Color ui.Color
	X, Y  float32
}

type gradientPreset struct {
	Name   string
	Colors []string
}

// Gradients are tints rather than full-strength wallpaper: the transparency
// and the chrome scrim keep the text contrast predictable.
var gradientPresets = []gradientPreset{
	{"Dusk", []string{"#6d5dfc", "#a855f7", "#c45f93"}},
	{"Tide", []string{"#2f9fd8", "#1fb3a8", "#5fb56d"}},
	{"Bloom", []string{"#b84f90", "#e2697e", "#d98b4f"}},
}

const customTintMix = float32(0.45)

type accentPreset struct {
	ID   string
	Name string
	Hex  string
}

var presetAccents = map[string]string{
	"nocturne": "emerald",
	"daylight": "emerald",
	"slate":    "sky",
	"cocoa":    "ember",
	"linen":    "coral",
	"dusk":     "violet",
	"tide":     "mint",
	"bloom":    "rose",
}

var accentPresets = []accentPreset{
	{"coral", "Coral", "#ff7058"},
	{"ember", "Ember", "#f59e0b"},
	{"mint", "Mint", "#54d6a0"},
	{"emerald", "Emerald", "#10b981"},
	{"sky", "Sky", "#5ca8ff"},
	{"violet", "Violet", "#a78bfa"},
	{"rose", "Rose", "#f472b6"},
	{"lime", "Lime", "#a8e063"},
	{"slate", "Slate", "#a9afba"},
}

var themePresets = []palette{
	{
		ID: "nocturne", Name: "Dark", Dark: true,
		Bg: ui.Hex("#0b0b0b"), Side: ui.Hex("#111111"), Panel: ui.Hex("#161616"),
		Field: ui.Hex("#1d1d1d"), Hover: ui.Hex("#242424"), Border: ui.RGBA(255, 255, 255, 0.075),
		Text: ui.Hex("#f4f4f6"), Muted: ui.Hex("#8a8f9f"),
		Good: ui.Hex("#59d39b"), Warn: ui.Hex("#f2b35d"), Bad: ui.Hex("#ff6b70"),
		Shadow: ui.RGBA(0, 0, 0, 0.34), Overlay: ui.RGBA(4, 5, 8, 0.62),
	},
	{
		ID: "daylight", Name: "Light", Dark: false,
		Bg: ui.Hex("#e9ebef"), Side: ui.Hex("#f6f7f9"), Panel: ui.Hex("#ffffff"),
		Field: ui.Hex("#f2f3f6"), Hover: ui.Hex("#e9ebef"), Border: ui.RGBA(20, 24, 34, 0.12),
		Text: ui.Hex("#1a1c22"), Muted: ui.Hex("#5d6472"),
		Good: ui.Hex("#159664"), Warn: ui.Hex("#c47b16"), Bad: ui.Hex("#d84a4f"),
		Shadow: ui.RGBA(30, 34, 44, 0.16), Overlay: ui.RGBA(24, 28, 36, 0.35),
	},
	{
		ID: "slate", Name: "Slate", Dark: true,
		Bg: ui.Hex("#0d1117"), Side: ui.Hex("#131922"), Panel: ui.Hex("#19212b"),
		Field: ui.Hex("#222b36"), Hover: ui.Hex("#2b3541"), Border: ui.RGBA(160, 190, 220, 0.10),
		Text: ui.Hex("#e8eef6"), Muted: ui.Hex("#93a3b5"),
		Good: ui.Hex("#5fcf9c"), Warn: ui.Hex("#e9bd6d"), Bad: ui.Hex("#f27b85"),
		Shadow: ui.RGBA(0, 0, 0, 0.38), Overlay: ui.RGBA(4, 8, 14, 0.66),
	},
	{
		ID: "cocoa", Name: "Cocoa", Dark: true,
		Bg: ui.Hex("#110e0c"), Side: ui.Hex("#191512"), Panel: ui.Hex("#221c18"),
		Field: ui.Hex("#2b241f"), Hover: ui.Hex("#352d26"), Border: ui.RGBA(235, 205, 170, 0.09),
		Text: ui.Hex("#f6eee5"), Muted: ui.Hex("#b3a08d"),
		Good: ui.Hex("#a7d18a"), Warn: ui.Hex("#e9bc72"), Bad: ui.Hex("#f08379"),
		Shadow: ui.RGBA(0, 0, 0, 0.4), Overlay: ui.RGBA(10, 6, 4, 0.66),
	},
	{
		ID: "linen", Name: "Linen", Dark: false,
		Bg: ui.Hex("#ebe7df"), Side: ui.Hex("#f6f2ea"), Panel: ui.Hex("#fffdf8"),
		Field: ui.Hex("#f2eee6"), Hover: ui.Hex("#e9e4da"), Border: ui.RGBA(70, 58, 42, 0.13),
		Text: ui.Hex("#26221c"), Muted: ui.Hex("#6d6456"),
		Good: ui.Hex("#2f7d58"), Warn: ui.Hex("#a86d1e"), Bad: ui.Hex("#c04a45"),
		Shadow: ui.RGBA(60, 50, 36, 0.15), Overlay: ui.RGBA(42, 34, 24, 0.34),
	},
	{
		ID: "dusk", Name: "Dusk", Dark: true,
		Bg: ui.Hex("#0b0a16"), Side: ui.Hex("#121029"), Panel: ui.Hex("#171430"),
		Field: ui.Hex("#1f1a3b"), Hover: ui.Hex("#272146"), Border: ui.RGBA(185, 175, 255, 0.10),
		Text: ui.Hex("#f1edff"), Muted: ui.Hex("#a29cc4"),
		Good: ui.Hex("#70d6b6"), Warn: ui.Hex("#f0c477"), Bad: ui.Hex("#ff8296"),
		Shadow: ui.RGBA(0, 0, 0, 0.42), Overlay: ui.RGBA(5, 4, 14, 0.68),
		HasGradient:   true,
		GradientAlpha: builtInGradientAlpha,
		Gradient: []gradientPoint{
			{ui.Hex("#6d5dfc"), 0.12, 0.20},
			{ui.Hex("#a855f7"), 0.50, 0.54},
			{ui.Hex("#c45f93"), 0.86, 0.82},
		},
	},
	{
		ID: "tide", Name: "Tide", Dark: true,
		Bg: ui.Hex("#06151b"), Side: ui.Hex("#0a1f27"), Panel: ui.Hex("#0e2932"),
		Field: ui.Hex("#14343f"), Hover: ui.Hex("#1b414d"), Border: ui.RGBA(150, 225, 235, 0.10),
		Text: ui.Hex("#edf9fb"), Muted: ui.Hex("#94b6c0"),
		Good: ui.Hex("#6bcf9a"), Warn: ui.Hex("#e8c16c"), Bad: ui.Hex("#ff7f86"),
		Shadow: ui.RGBA(0, 0, 0, 0.42), Overlay: ui.RGBA(3, 10, 14, 0.67),
		HasGradient:   true,
		GradientAlpha: builtInGradientAlpha,
		Gradient: []gradientPoint{
			{ui.Hex("#2f9fd8"), 0.10, 0.18},
			{ui.Hex("#1fb3a8"), 0.50, 0.56},
			{ui.Hex("#5fb56d"), 0.88, 0.84},
		},
	},
	{
		ID: "bloom", Name: "Bloom", Dark: true,
		Bg: ui.Hex("#150c14"), Side: ui.Hex("#1d121c"), Panel: ui.Hex("#251826"),
		Field: ui.Hex("#2f2031"), Hover: ui.Hex("#3a293c"), Border: ui.RGBA(255, 190, 220, 0.10),
		Text: ui.Hex("#fdf0f7"), Muted: ui.Hex("#b99eaf"),
		Good: ui.Hex("#90d4a3"), Warn: ui.Hex("#f1bd73"), Bad: ui.Hex("#ff7d8d"),
		Shadow: ui.RGBA(0, 0, 0, 0.42), Overlay: ui.RGBA(12, 5, 11, 0.68),
		HasGradient:   true,
		GradientAlpha: builtInGradientAlpha,
		Gradient: []gradientPoint{
			{ui.Hex("#b84f90"), 0.12, 0.20},
			{ui.Hex("#e2697e"), 0.50, 0.55},
			{ui.Hex("#d98b4f"), 0.88, 0.82},
		},
	},
}

// chromeScrimAlpha keeps the material visible while giving the chrome enough
// tint for muted text and icons to stay readable over a bright desktop.
const chromeScrimAlpha = float32(0.46)

// A chrome gradient is a mesh of one to three colored points blended across
// the window. It has no drawn rings or blooms: the renderer interpolates a
// small bitmap, so only smooth color fields reach the screen.
const (
	defaultGradientAlpha = float32(0.55)
	chromeTextScrim      = float32(0.30)
	// Built-in gradients are tints on the chrome, not wallpapers: a lower
	// alpha keeps even the warm meshes calm behind small labels.
	builtInGradientAlpha = float32(0.46)
	// Over acrylic, a slightly lighter gradient leaves more of the desktop
	// material visible without washing out the chrome's text.
	chromeAcrylicGradientScale = float32(0.75)
	maxThemeGradientPoints     = 3
	minThemeGradientPoints     = 1
	gradientBitmapSize         = 320
	gradientBitmapCacheLimit   = 32
)

// vibrancyPalette pushes secondary text away from the middle of the scale.
// Chrome over a material has no known backdrop color, so more contrast in the
// grays is the difference between readable labels and a washed-out header.
func vibrancyPalette(p palette) palette {
	if p.Dark {
		p.Muted = p.Muted.Mix(ui.Hex("#ffffff"), 0.42)
		p.Border = p.Text.Alpha(0.14)
	} else {
		p.Muted = p.Muted.Mix(ui.Hex("#000000"), 0.32)
		p.Border = p.Text.Alpha(0.13)
	}
	return p
}

func accentColor(id string) ui.Color {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "#") {
		if col, ok := parseThemeColor(id); ok {
			return col
		}
	}
	for _, a := range accentPresets {
		if a.ID == id {
			return ui.Hex(a.Hex)
		}
	}
	return ui.Hex("#ff7058")
}

func accentLabel(id string) string {
	for _, a := range accentPresets {
		if a.ID == id {
			return a.Name
		}
	}
	return "Coral"
}

func themeByID(id string) palette {
	for _, p := range themePresets {
		if p.ID == id {
			return p
		}
	}
	return themePresets[0]
}

func customThemeByID(themes []CustomTheme, id string) *CustomTheme {
	for i := range themes {
		if themes[i].ID == id {
			return &themes[i]
		}
	}
	return nil
}

// customPalette applies the custom tint to a base palette. Tinting, rather than
// replacing the background, is what lets a custom gradient stay as readable as
// the palette it started from.
func customPalette(ct CustomTheme) palette {
	base := themeByID(ct.Base)
	p := base
	p.ID = ct.ID
	p.Name = strings.TrimSpace(ct.Name)
	if p.Name == "" {
		p.Name = "Custom theme"
	}
	p.Bg = customChromeColor(base, colorOr(ct.Background, base.Bg))
	p.Side = p.Bg
	surface := colorOr(ct.Surface, ui.Transparent)
	if surface.A != 0 {
		// The rounded workspace gets the chosen color, and its chips and
		// hairlines follow it instead of keeping the old panel's shades.
		p.Panel = surface
		p.Field = p.Panel.Mix(p.Text, 0.06)
		p.Hover = p.Panel.Mix(p.Text, 0.10)
		p.Border = p.Text.Alpha(0.10)
	}
	p.HasGradient = false
	p.Gradient = nil
	p.GradientAlpha = 0
	p.GradientGrain = clamp01(ct.GradientGrain)
	p.GradientMode = strings.TrimSpace(ct.GradientMode)
	if ct.Gradient {
		if points := customGradientPoints(ct); len(points) >= minThemeGradientPoints {
			p.HasGradient = true
			p.Gradient = points
			p.GradientAlpha = customGradientAlpha(ct)
		}
	}
	bgs := gradientBackdrops(p)
	if surface.A != 0 {
		bgs = append(bgs, p.Panel)
	}
	p.Muted = readableMutedAgainst(p, bgs)
	// Apply explicit text color choice.
	switch strings.TrimSpace(ct.TextColor) {
	case "black":
		p.Text = ui.Hex("#000000")
	case "white":
		p.Text = ui.Hex("#ffffff")
	}
	// Apply explicit accent choice.
	if ct.Accent != "" {
		p.Accent = accentColor(ct.Accent)
		if !p.Dark && ct.Accent == "lime" {
			p.Accent = ui.Hex("#6d9e2f")
		}
	}
	return p
}

// customGradientPoints reads the saved mesh points and keeps old two-color
// themes working. The colors are opaque; transparency belongs to the gradient
// as a whole so that the slider stays predictable.
func customGradientPoints(ct CustomTheme) []gradientPoint {
	points := make([]gradientPoint, 0, maxThemeGradientPoints)
	for _, cp := range ct.GradientPoints {
		c := colorOr(cp.Color, ui.Transparent)
		if c.A == 0 {
			continue
		}
		points = append(points, gradientPoint{Color: c, X: clamp01(cp.X), Y: clamp01(cp.Y)})
		if len(points) == maxThemeGradientPoints {
			break
		}
	}
	// Themes saved before the mesh editor kept only two colors.
	if len(points) == 0 {
		if c := colorOr(ct.From, ui.Transparent); c.A != 0 {
			points = append(points, gradientPoint{Color: c, X: 0.18, Y: 0.28})
		}
		if c := colorOr(ct.To, ui.Transparent); c.A != 0 {
			points = append(points, gradientPoint{Color: c, X: 0.82, Y: 0.72})
		}
	}
	return points
}

// customGradientAlpha trusts an explicit opacity, including zero, once a theme
// has mesh points. Themes from before the picker have no points and no stored
// opacity, so they keep the old default.
func customGradientAlpha(ct CustomTheme) float32 {
	if ct.GradientOpacity <= 0 && len(ct.GradientPoints) == 0 {
		return defaultGradientAlpha
	}
	return clampRange(ct.GradientOpacity, 0, 1)
}

func customThemeProblem(ct CustomTheme, accent ui.Color) string {
	p := customPalette(ct)
	if accent.A != 0 {
		p.Accent = accent
	}
	p.Muted = readableMuted(p)
	for _, bg := range gradientBackdrops(p) {
		if contrastRatio(p.Text, bg) < 4.5 {
			return "That theme leaves chrome text hard to read. Choose softer colors, more transparency, or a darker window tint."
		}
	}
	return ""
}

// gradientBackdrops is the worst-case chrome background for text: the tinted
// base, then each mesh point at full strength, before and after the rail's
// text scrim.
func gradientBackdrops(p palette) []ui.Color {
	if !p.HasGradient || len(p.Gradient) < minThemeGradientPoints {
		return []ui.Color{p.Bg}
	}
	bgs := make([]ui.Color, 0, 1+2*len(p.Gradient))
	bgs = append(bgs, p.Bg)
	for _, point := range p.Gradient {
		rendered := p.Bg.Mix(point.Color, p.GradientAlpha)
		bgs = append(bgs, rendered, rendered.Mix(p.Bg, chromeTextScrim))
	}
	return bgs
}

type gradientCacheKey struct {
	count  int
	colors [maxThemeGradientPoints]uint32
	xs     [maxThemeGradientPoints]uint16
	ys     [maxThemeGradientPoints]uint16
	alpha  uint8
	aspect uint16
}

var gradientBitmaps = map[gradientCacheKey]*ui.Bitmap{}

// paintChromeGradient fills r with the smooth mesh behind the chrome. It is
// used for the window background, theme cards, and the editor preview.
func paintChromeGradient(pt *ui.Painter, r ui.Rect, p palette, scale float32) {
	if r.W <= 0 || r.H <= 0 || !p.HasGradient || len(p.Gradient) < minThemeGradientPoints {
		return
	}
	alpha := clamp01(p.GradientAlpha * scale)
	if alpha <= 0 {
		return
	}
	bmp := gradientBitmap(p.Gradient, alpha, r.W/r.H)
	if bmp == nil {
		return
	}
	pt.ImageRegion(bmp, ui.Rect{}, r, ui.ImageOptions{})
	paintGrain(pt, r, p.GradientGrain)
}

// grainBitmap is a small tile of white noise. It is stretched over the chrome
// and tinted down to a whisper, so the gradient reads as a material rather
// than a flat wash without ever competing with text.
var grainTile *ui.Bitmap

func grainBitmap() *ui.Bitmap {
	if grainTile != nil {
		return grainTile
	}
	const size = 128
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	seed := uint32(0x2545f491)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			seed = seed*1664525 + 1013904223
			v := uint8(seed >> 24)
			i := y*img.Stride + x*4
			img.Pix[i+0] = 255
			img.Pix[i+1] = 255
			img.Pix[i+2] = 255
			img.Pix[i+3] = v
		}
	}
	grainTile = ui.NewBitmap(img)
	return grainTile
}

func paintGrain(pt *ui.Painter, r ui.Rect, strength float32) {
	strength = clamp01(strength)
	if strength <= 0 || r.W <= 0 || r.H <= 0 {
		return
	}
	tile := grainBitmap()
	if tile == nil {
		return
	}
	alpha := strength * 0.22
	if alpha <= 0 {
		return
	}
	const step = float32(128)
	for y := r.Y; y < r.Y+r.H; y += step {
		for x := r.X; x < r.X+r.W; x += step {
			w := min(step, r.X+r.W-x)
			h := min(step, r.Y+r.H-y)
			pt.ImageRegion(tile, ui.Rect{}, ui.Rect{X: x, Y: y, W: w, H: h},
				ui.ImageOptions{Tint: ui.RGBA(255, 255, 255, alpha)})
		}
	}
}

// hsvColor converts hue (degrees), saturation and value (both 0..1) to a
// color. The theme editor uses it for its saturation square and hue bar.
func hsvColor(h, s, v float32) ui.Color {
	h = float32(math.Mod(float64(h), 360))
	if h < 0 {
		h += 360
	}
	s = clamp01(s)
	v = clamp01(v)
	c := v * s
	x := c * (1 - float32(math.Abs(math.Mod(float64(h/60), 2)-1)))
	m := v - c
	var r, g, b float32
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return ui.RGBA(
		uint8((r+m)*255+0.5),
		uint8((g+m)*255+0.5),
		uint8((b+m)*255+0.5),
		255,
	)
}

// gradientBitmap renders and caches the mesh at a small size. The bitmap is
// stretched over the target rect, so the result stays smooth on high-DPI
// screens while dragging a point does not allocate a window-sized image.
func gradientBitmap(points []gradientPoint, alpha, aspect float32) *ui.Bitmap {
	count := min(len(points), maxThemeGradientPoints)
	if count < minThemeGradientPoints {
		return nil
	}
	aspect = clampRange(aspect, 0.25, 8)
	key := gradientCacheKey{
		count:  count,
		alpha:  uint8(clamp01(alpha)*255 + 0.5),
		aspect: uint16(math.Round(float64(aspect) * 32)),
	}
	for i := 0; i < count; i++ {
		p := points[i]
		key.colors[i] = packGradientColor(p.Color)
		key.xs[i] = uint16(clamp01(p.X)*math.MaxUint16 + 0.5)
		key.ys[i] = uint16(clamp01(p.Y)*math.MaxUint16 + 0.5)
	}
	if bmp, ok := gradientBitmaps[key]; ok {
		return bmp
	}
	if len(gradientBitmaps) >= gradientBitmapCacheLimit {
		clear(gradientBitmaps)
	}
	bmp := renderGradientBitmap(points[:count], alpha, aspect)
	gradientBitmaps[key] = bmp
	return bmp
}

func renderGradientBitmap(points []gradientPoint, alpha, aspect float32) *ui.Bitmap {
	w, h := gradientBitmapSize, int(math.Round(float64(float32(gradientBitmapSize)/aspect)))
	if aspect < 1 {
		w, h = int(math.Round(float64(float32(gradientBitmapSize)*aspect))), gradientBitmapSize
	}
	w, h = max(w, 32), max(h, 32)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	a := uint16(clamp01(alpha)*255 + 0.5)
	for y := 0; y < h; y++ {
		ny := (float32(y) + 0.5) / float32(h)
		for x := 0; x < w; x++ {
			nx := (float32(x) + 0.5) / float32(w)
			var total, red, green, blue float64
			for _, point := range points {
				dx := (nx - point.X) * aspect
				dy := ny - point.Y
				weight := 1 / (float64(dx*dx+dy*dy) + 0.00001)
				total += weight
				red += weight * float64(point.Color.R)
				green += weight * float64(point.Color.G)
				blue += weight * float64(point.Color.B)
			}
			if total == 0 {
				continue
			}
			r := uint8(red/total + 0.5)
			g := uint8(green/total + 0.5)
			b := uint8(blue/total + 0.5)
			i := y*img.Stride + x*4
			img.Pix[i+0] = uint8(uint16(r) * a / 255)
			img.Pix[i+1] = uint8(uint16(g) * a / 255)
			img.Pix[i+2] = uint8(uint16(b) * a / 255)
			img.Pix[i+3] = uint8(a)
		}
	}
	return ui.NewBitmap(img)
}

func packGradientColor(c ui.Color) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

func clamp01(v float32) float32 { return clampRange(v, 0, 1) }

func clampRange(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// readableMuted nudges the palette's muted color toward its text color until
// the smallest chrome labels keep a 4.5:1 contrast on the gradient field.
func readableMuted(p palette) ui.Color {
	return readableMutedAgainst(p, gradientBackdrops(p))
}

func readableMutedAgainst(p palette, bgs []ui.Color) ui.Color {
	if minContrast(p.Muted, bgs) >= 4.5 {
		return p.Muted
	}
	lo, hi := float32(0), float32(1)
	for i := 0; i < 12; i++ {
		mid := (lo + hi) / 2
		if minContrast(p.Muted.Mix(p.Text, mid), bgs) >= 4.5 {
			hi = mid
		} else {
			lo = mid
		}
	}
	return p.Muted.Mix(p.Text, hi)
}

func minContrast(fg ui.Color, bgs []ui.Color) float64 {
	lowest := math.MaxFloat64
	for _, bg := range bgs {
		if r := contrastRatio(fg, bg); r < lowest {
			lowest = r
		}
	}
	return lowest
}

func contrastRatio(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func customChromeColor(base palette, tint ui.Color) ui.Color {
	return tint.Mix(base.Bg, customTintMix)
}

func normalizeAngle(a float32) float32 {
	a = float32(math.Mod(float64(a), 360))
	if a < 0 {
		a += 360
	}
	return a
}

func colorOr(s string, fallback ui.Color) ui.Color {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 6 {
		return fallback
	}
	if _, err := strconv.ParseUint(h, 16, 32); err != nil {
		return fallback
	}
	return ui.Hex(h)
}

// parseThemeColor accepts the formats people paste into a theme editor: #rgb,
// #rrggbb (the hash is optional) and rgb(r, g, b).
func parseThemeColor(s string) (ui.Color, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ui.Color{}, false
	}
	if strings.HasPrefix(s, "rgb") {
		open := strings.IndexByte(s, '(')
		end := strings.LastIndexByte(s, ')')
		if open < 0 || end <= open {
			return ui.Color{}, false
		}
		parts := strings.FieldsFunc(s[open+1:end], func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		})
		if len(parts) != 3 {
			return ui.Color{}, false
		}
		var rgb [3]uint8
		for i, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 || n > 255 {
				return ui.Color{}, false
			}
			rgb[i] = uint8(n)
		}
		return ui.RGB(rgb[0], rgb[1], rgb[2]), true
	}
	h := strings.TrimPrefix(s, "#")
	switch len(h) {
	case 3:
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	case 6:
	default:
		return ui.Color{}, false
	}
	if _, err := strconv.ParseUint(h, 16, 32); err != nil {
		return ui.Color{}, false
	}
	return ui.Hex(h), true
}

func hexColor(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// resolvePalette follows the desktop for the System choice, resolves custom
// themes, and fills in the chosen accent.
func resolvePalette(settings Settings, systemDark bool, customs []CustomTheme) palette {
	id := settings.Theme
	if id == "" || id == "system" {
		id = "nocturne"
		if !systemDark {
			id = "daylight"
		}
	}
	var p palette
	if ct := customThemeByID(customs, id); ct != nil {
		p = customPalette(*ct)
		if ct.Accent != "" {
			p.Accent = accentColor(ct.Accent)
			if !p.Dark && ct.Accent == "lime" {
				p.Accent = ui.Hex("#6d9e2f")
			}
		} else {
			p.Accent = accentColor(settings.Accent)
			if !p.Dark && settings.Accent == "lime" {
				p.Accent = ui.Hex("#6d9e2f")
			}
		}
	} else {
		p = themeByID(id)
		// Preset themes get their own default accent.
		if accent, ok := presetAccents[id]; ok {
			p.Accent = accentColor(accent)
			if !p.Dark && accent == "lime" {
				p.Accent = ui.Hex("#6d9e2f")
			}
		} else if p.Accent.A == 0 {
			p.Accent = accentColor(settings.Accent)
			if !p.Dark && settings.Accent == "lime" {
				p.Accent = ui.Hex("#6d9e2f")
			}
		}
	}
	if p.HasGradient {
		p.Muted = readableMuted(p)
	}
	return p
}

func applyTheme(c *ui.Context, p palette, s Settings) *ui.Theme {
	dark := p.Dark
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
	}
	t.Dark = dark
	t.Background = p.Bg
	t.Surface = p.Field
	t.SurfaceHover = p.Hover
	t.SurfacePressed = p.Hover.Mix(p.Text, 0.06)
	t.Border = p.Border
	t.Text = p.Text
	t.TextMuted = p.Muted
	t.Accent = p.Accent
	t.AccentHover = p.Accent.Mix(p.Text, 0.10)
	t.AccentPressed = p.Accent.Mix(ui.Hex("#000000"), 0.18)
	t.AccentText = contrastText(p.Accent)
	t.Danger = p.Bad
	t.Warning = p.Warn
	t.Success = p.Good
	t.Selection = p.Accent.Alpha(0.28)
	t.Focus = p.Accent.Alpha(0.7)
	// Tooltips and toasts are MyGo's "inverse" surfaces. Keep them inside the
	// theme instead of the near-white p.Text, which reads as a white popup in
	// every dark palette. A lifted field stays legible over the chrome and the
	// stage, and follows custom main-view colours too.
	t.Inverse = p.Field.Mix(p.Text, 0.04)
	t.InverseText = p.Text
	t.Scrollbar = p.Text.Alpha(0.18)
	t.ScrollbarWidth = 7
	t.Radius = 8
	t.Spacing = 4
	t.FontSize = s.FontSize
	if t.FontSize <= 0 {
		t.FontSize = 14
	}
	if s.Compact {
		t.Spacing = 3.4
		t.Radius = 7
	}
	c.SetTheme(t)
	return t
}

func contrastText(c ui.Color) ui.Color {
	// Pick whichever text has more contrast, rather than guessing from a
	// brightness threshold: the light accents in the palette are much more
	// readable with dark text, while violet and rose can go either way.
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

func alpha(c ui.Color, a float32) ui.Color  { return c.Alpha(a) }
func mix(a, b ui.Color, t float32) ui.Color { return a.Mix(b, t) }
