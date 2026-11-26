package main

import "github.com/egoist/mygo/ui"

// mustIcon builds a 24×24 line icon whose strokes follow the text color,
// matching the icon set Go Post and Go Mail draw with.
func mustIcon(inner string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + inner + `</svg>`))
}

// mustSolidIcon builds a filled icon, for glyphs such as play that read
// poorly as an outline.
func mustSolidIcon(inner string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="none">` + inner + `</svg>`))
}

var (
	iconTiles    = mustIcon(`<rect x="3.5" y="3.5" width="7" height="7" rx="2"/><rect x="13.5" y="3.5" width="7" height="7" rx="2"/><rect x="3.5" y="13.5" width="7" height="7" rx="2"/><rect x="13.5" y="13.5" width="7" height="7" rx="2"/>`)
	iconStore    = mustIcon(`<path d="M5.2 8h13.6l-1 10.9a2 2 0 0 1-2 1.8H8.2a2 2 0 0 1-2-1.8Z"/><path d="M9 8V6.2a3 3 0 0 1 6 0V8"/>`)
	iconLibrary  = mustIcon(`<path d="M12 3.2 20 7.6l-8 4.4-8-4.4Z"/><path d="m4 12.2 8 4.4 8-4.4"/><path d="m4 16.6 8 4.4 8-4.4"/>`)
	iconInfo     = mustIcon(`<circle cx="12" cy="12" r="8.4"/><path d="M12 11.2v5.2"/><path d="M12 7.9h.01"/>`)
	iconRefresh  = mustIcon(`<path d="M19.6 12a7.6 7.6 0 1 1-2.4-5.5"/><path d="M19.8 4.4v4.6h-4.6"/>`)
	iconDownload = mustIcon(`<path d="M12 4.2v10.4"/><path d="m7.9 10.6 4.1 4.1 4.1-4.1"/><path d="M5.2 19h13.6"/>`)
	iconPlay     = mustSolidIcon(`<path d="M8.6 5.4v13.2L19.2 12z"/>`)
	iconTrash    = mustIcon(`<path d="M4.6 7h14.8"/><path d="M9.4 7V5.3A1.3 1.3 0 0 1 10.7 4h2.6a1.3 1.3 0 0 1 1.3 1.3V7"/><path d="m6.6 7 .8 12a1.9 1.9 0 0 0 1.9 1.8h5.4a1.9 1.9 0 0 0 1.9-1.8l.8-12"/>`)
	iconFolder   = mustIcon(`<path d="M3.4 7.2a2 2 0 0 1 2-2h3.4l2 2.4h7.8a2 2 0 0 1 2 2v8.2a2 2 0 0 1-2 2H5.4a2 2 0 0 1-2-2Z"/>`)
	iconCheck    = mustIcon(`<path d="m5 12.4 4.4 4.4L19 7.4"/>`)
	iconAlert    = mustIcon(`<circle cx="12" cy="12" r="8.4"/><path d="M12 7.8v5.2"/><path d="M12 16.4h.01"/>`)
	iconX        = mustIcon(`<path d="M6 6l12 12"/><path d="M18 6 6 18"/>`)
	iconMinimize = mustIcon(`<path d="M5 12h14"/>`)
	iconMaximize = mustIcon(`<rect x="5.5" y="5.5" width="13" height="13" rx="1.6"/>`)
	iconRestore  = mustIcon(`<rect x="8" y="8" width="11" height="11" rx="1.6"/><path d="M5.5 15.5V6.8a1.3 1.3 0 0 1 1.3-1.3h8.7"/>`)
	iconMonitor  = mustIcon(`<rect x="3.4" y="4.8" width="17.2" height="11.6" rx="2"/><path d="M9 20.2h6"/><path d="M12 16.4v3.8"/>`)
	iconMoon     = mustIcon(`<path d="M19.8 14.6A8.2 8.2 0 0 1 9.4 4.2a8.4 8.4 0 1 0 10.4 10.4Z"/>`)
	iconSun      = mustIcon(`<circle cx="12" cy="12" r="4"/><path d="M12 3v2"/><path d="M12 19v2"/><path d="M3 12h2"/><path d="M19 12h2"/><path d="m5.6 5.6 1.4 1.4"/><path d="m17 17 1.4 1.4"/><path d="M18.4 5.6 17 7"/><path d="m7 17-1.4 1.4"/>`)
	iconSend     = mustIcon(`<path d="M20.5 3.5 3.7 9.9l6.7 3.7 3.7 6.7Z"/><path d="m10.4 13.6 4.2-4.2"/>`)
	iconMail     = mustIcon(`<rect x="2.6" y="4.6" width="18.8" height="14.8" rx="2.5"/><path d="m3 7 8.2 5.5a1.5 1.5 0 0 0 1.6 0L21 7"/>`)
)

var iconBack = mustIcon(`<path d="m15 18-6-6 6-6"/>`)

// appGlyph is the outline drawn when no real icon could be read from the
// app's setup.
func appGlyph(id string) *ui.SVG {
	if id == "gomail" {
		return iconMail
	}
	return iconSend
}
