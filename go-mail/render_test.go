package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// TestRenderPNG writes the screens to PNGs for visual review. It is a
// development aid, not an assertion.
func TestRenderPNG(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "go-mail-shots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	save := func(name string, img *image.RGBA) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}

	a := &App{
		sel:         -1,
		split:       420,
		listState:   ui.ListState{},
		sidebarSel:  "INBOX",
		folder:      "INBOX",
		dark:        true,
		systemDark:  true,
		settingsTab: "appearance", settingsW: settingsRailDefaultWidth,
		themeSettings: ThemeSettings{
			Theme:    "nocturne",
			FontSize: 14,
			Radius:   9,
		},
		form: Account{
			Name:     "Alex Doe",
			Email:    "alex@example.com",
			IMAPHost: "imap.example.com",
			IMAPPort: "993",
			SMTPHost: "smtp.example.com",
			SMTPPort: "587",
		},
	}
	save("setup.png", ui.NewTester(a.view, 1320, 860).Image())

	a.connected = true
	a.acct = a.form
	a.folders = []Folder{
		{Name: "INBOX", Label: "Inbox", Kind: "inbox", Unread: 2},
		{Name: "Sent", Label: "Sent", Kind: "sent"},
		{Name: "Drafts", Label: "Drafts", Kind: "drafts"},
		{Name: "Trash", Label: "Trash", Kind: "trash"},
	}
	a.msgs = []Message{
		{UID: 42, FromName: "Maya Chen", FromAddr: "maya@example.com", Subject: "Design review on Thursday", Date: time.Now(), Read: false},
		{UID: 41, FromName: "Build bot", FromAddr: "ci@example.com", Subject: "Build #128 passed", Date: time.Now().Add(-2 * time.Hour), Read: true},
		{UID: 40, FromName: "Nadia Patel", FromAddr: "nadia@example.com", Subject: "Re: Q4 roadmap notes", Date: time.Now().Add(-26 * time.Hour), Read: true},
	}
	a.sel = 0
	a.body = "Hi Alex,\n\nCan we move the design review to Thursday at 15:00? The new sidebar mockups are ready, and I would like to walk through the empty states with you.\n\nThanks,\nMaya"
	save("mail.png", ui.NewTester(a.view, 1320, 860).Image())

	a.msgs[0].Attachments = []Attachment{
		{Name: "design-review.pdf", Size: 245760},
		{Name: "sidebar-mockup.png", Size: 912384},
	}
	save("mail-attachments.png", ui.NewTester(a.view, 1320, 860).Image())
	a.msgs[0].Attachments = nil

	a.showCompose = true
	a.composeTo = "maya@example.com"
	a.composeSub = "Re: Design review on Thursday"
	a.composeBody = "Thursday at 15:00 works for me. See you then.\n\nAlex"
	save("compose.png", ui.NewTester(a.view, 1320, 860).Image())

	a.showCompose = false
	a.sel = -1
	save("empty.png", ui.NewTester(a.view, 1320, 860).Image())

	// Settings window (separate screen).
	a.sel = 0
	save("settings.png", ui.NewTester(a.settingsWindowView, 920, 760).Image())

	a.dark = false
	a.themeSettings.Theme = "daylight"
	a.sel = 0
	a.showCompose = false
	save("light-mail.png", ui.NewTester(a.view, 1320, 860).Image())

	a.connected = false
	save("light-setup.png", ui.NewTester(a.view, 1320, 860).Image())

	// Standalone theme editor window.
	ed := testApp()
	ed.palette = resolvePalette(ed.themeSettings, true, nil)
	ed.openThemePicker(nil, "card")
	ed.pendingThemePicker = false
	save("theme-editor.png", ui.NewTester(ed.themePickerWindowView, 430, 820).Image())
	save("theme-editor-tall.png", ui.NewTester(ed.themePickerWindowView, 430, 1180).Image())
	save("theme-editor-small.png", ui.NewTester(ed.themePickerWindowView, 390, 600).Image())

	// The same editor on a light palette.
	lightEd := testApp()
	lightEd.themeSettings.Theme = "linen"
	lightEd.systemDark = false
	lightEd.palette = resolvePalette(lightEd.themeSettings, false, nil)
	lightEd.openThemePicker(nil, "card")
	lightEd.pendingThemePicker = false
	save("theme-editor-light.png", ui.NewTester(lightEd.themePickerWindowView, 430, 820).Image())

	// The rich signature editor in Settings → Mail.
	a.connected = true
	a.settingsTab = "mail"
	a.dark = true
	a.themeSettings.Theme = "nocturne"
	sig := richtext.TextOf(
		richtext.Paragraph{Runs: []richtext.Run{{Text: "Bryan Van Winnendael", Attributes: richtext.Attributes{
			Font: "Georgia, serif", Size: 22, Weight: 700, Color: ui.Hex("#111111"),
		}}}},
		richtext.Paragraph{Runs: []richtext.Run{{Text: "Product designer", Attributes: richtext.Attributes{
			Font: "Arial, Helvetica, sans-serif", Size: 13, Color: ui.Hex("#6b7280"),
		}}}},
		richtext.Paragraph{Runs: []richtext.Run{{Text: "bryan@example.com", Attributes: richtext.Attributes{
			Font: "Arial, Helvetica, sans-serif", Size: 13, Color: ui.Hex("#2563eb"),
		}}}},
	)
	sigJSON, err := json.Marshal(sig)
	if err != nil {
		t.Fatal(err)
	}
	a.acct = Account{
		Name:            "Bryan Van Winnendael",
		Email:           "bryan@example.com",
		SignatureRich:   string(sigJSON),
		SignatureCanvas: []SignatureImage{{Data: testSignatureImage(t), X: 62, Y: 12, Width: 26}},
	}
	a.accounts = []Account{a.acct}
	save("settings-signature.png", ui.NewTester(a.settingsWindowView, 920, 820).Image())
}
