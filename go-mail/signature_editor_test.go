package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

func testSignatureImage(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 120, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 120; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 2), G: uint8(y * 4), B: 180, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// TestSignatureEditorLoadsRichText checks that an account's saved rich text
// is what the page opens with, so fonts, sizes and colors survive a restart.
func TestSignatureEditorLoadsRichText(t *testing.T) {
	text := richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{
		{Text: "Alex Doe", Attributes: richtext.Attributes{
			Font: "Georgia, serif", Size: 20, Weight: 700, Color: ui.Hex("#2563eb"),
		}},
	}})
	data, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{dark: true, systemDark: true}
	a.palette = resolvePalette(ThemeSettings{Theme: "nocturne"}, true, nil)
	a.acct = Account{Email: "alex@example.com", SignatureRich: string(data)}

	ed := a.signatureDoc()
	if got := ed.doc.Text().PlainText(); got != "Alex Doe" {
		t.Fatalf("plain text is %q", got)
	}
	html := ed.doc.Text().HTML()
	for _, want := range []string{"Georgia", "#2563eb", "font-size:20px"} {
		if !bytes.Contains([]byte(html), []byte(want)) {
			t.Fatalf("reloaded signature lacks %q: %s", want, html)
		}
	}
}

// TestSignatureImageSurvivesReload checks that an inline signature image is
// still there after the app restarts, both when its bytes were saved with
// the account and when it comes from an old canvas signature.
func TestSignatureImageSurvivesReload(t *testing.T) {
	b64 := testSignatureImage(t)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	bmp, err := ui.DecodeBitmap(raw)
	if err != nil {
		t.Fatal(err)
	}
	att := &richtext.Attachment{Image: bmp, Width: 120, Height: 60, Data: b64}
	text := richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{
		{Text: "Hi "},
		{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: att}},
	}})
	data, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		acct Account
	}{
		{"saved bytes", Account{Email: "a@b.c", SignatureRich: string(data), SignatureImages: []string{b64}}},
		{"canvas fallback", Account{Email: "a@b.c", SignatureRich: string(data), SignatureCanvas: []SignatureImage{{Data: b64, Width: 22}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{active: 0}
			a.acct = tc.acct
			ed := a.signatureDoc()
			n, withImage := 0, 0
			for _, p := range ed.doc.Text().Paragraphs() {
				for _, r := range p.Runs {
					if r.Attributes.Attachment != nil {
						n++
						if r.Attributes.Attachment.Image != nil {
							withImage++
						}
					}
				}
			}
			if n != 1 || withImage != 1 {
				t.Fatalf("attachments=%d with image=%d", n, withImage)
			}
		})
	}
}

// TestSignatureEditorSavesTyping checks that text typed on the page reaches
// the account without another click, which is what a restart reads.
func TestSignatureEditorSavesTyping(t *testing.T) {
	t.Setenv("GO_MAIL_CONFIG", t.TempDir()+"/config.json")
	a := &App{dark: true, systemDark: true, active: 0}
	a.themeSettings = ThemeSettings{Theme: "nocturne"}
	a.palette = resolvePalette(a.themeSettings, true, nil)
	a.acct = Account{Email: "alex@example.com", Signature: "Doe"}
	a.accounts = []Account{a.acct}

	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() { a.signatureEditorView(c) })
	}, 760, 480)
	tester.Frame()

	a.sigEd.doc.InsertText("Alex ")
	tester.Frame()

	if got := a.acct.Signature; got == "Doe" || !strings.Contains(got, "Alex") {
		t.Fatalf("typed text was not saved: %q", got)
	}
	if a.acct.SignatureRich == "" {
		t.Fatal("rich text was not saved")
	}
	if a.accounts[0].Signature != a.acct.Signature {
		t.Fatal("account was not updated")
	}
}
