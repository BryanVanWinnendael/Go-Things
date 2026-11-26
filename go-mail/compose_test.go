package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// TestComposeDocumentShowsSignature checks that the signature is part of the
// document the compose editor edits, not a separate preview, and that send
// turns the editor's styling and inline images into the right MIME message.
func TestComposeDocumentShowsSignature(t *testing.T) {
	b64 := testSignatureImage(t)
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	bmp, err := ui.DecodeBitmap(raw)
	if err != nil {
		t.Fatal(err)
	}
	att := &richtext.Attachment{Image: bmp, Width: 80, Height: 40, Data: b64}
	sig := richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{
		{Text: "Alex", Attributes: richtext.Attributes{Weight: 700, Color: ui.Hex("#2563eb")}},
	}}, richtext.Paragraph{Runs: []richtext.Run{
		{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: att}},
	}})
	data, err := json.Marshal(sig)
	if err != nil {
		t.Fatal(err)
	}

	a := &App{active: 0}
	a.acct = Account{
		Email:           "alex@example.com",
		Signature:       "Alex",
		SignatureRich:   string(data),
		SignatureImages: []string{b64},
	}
	a.composeBody = "Hello there"
	a.startComposeDoc()

	doc := a.composeEditorDoc()
	plain := doc.Text().PlainText()
	bodyAt := strings.Index(plain, "Hello there")
	sigAt := strings.Index(plain, "Alex")
	if bodyAt < 0 || sigAt < 0 || sigAt < bodyAt {
		t.Fatalf("signature should follow the typed body: %q", plain)
	}

	// Styling the body must reach the HTML that is sent.
	doc.SetSelection(0, len("Hello there"))
	doc.Format(func(at *richtext.Attributes) { at.Weight = 700 })
	plainOut, html, imgs, types := a.composeMessageParts()
	if !strings.Contains(html, "<b>Hello there</b>") {
		t.Fatalf("compose HTML lacks the body styling: %s", html)
	}
	if !strings.Contains(html, "cid:signature-image-0") {
		t.Fatalf("compose HTML lacks the signature image: %s", html)
	}
	if !strings.Contains(plainOut, "Hello there") || !strings.Contains(plainOut, "Alex") {
		t.Fatalf("plain part is %q", plainOut)
	}
	if len(imgs) != 1 || len(types) != 1 {
		t.Fatalf("inline images = %d types = %d", len(imgs), len(types))
	}

	out := BuildRichMessage(a.acct, "maya@example.com", "", "Hi", plainOut, html, imgs, types, nil)
	msg := string(out)
	for _, want := range []string{"multipart/related", "Content-ID: <signature-image-0>", "<b>Hello there</b>"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message lacks %q", want)
		}
	}
}

// TestComposeStartsAboveSignature checks that a new message opens with the
// caret on an empty line above the signature, as a normal mail composer does.
func TestComposeStartsAboveSignature(t *testing.T) {
	a := &App{active: 0}
	a.acct = Account{Email: "alex@example.com", Signature: "Alex", SignatureRich: `{"paragraphs":[{"style":{},"runs":[{"text":"Alex","attributes":{}}]}]}`}
	a.startComposeDoc()
	a.composeEditorDoc().InsertText("Hi")
	plain := a.composeEditorDoc().Text().PlainText()
	if strings.Contains(plain, "HiAlex") {
		t.Fatalf("typing joined the signature: %q", plain)
	}
	if i, j := strings.Index(plain, "Hi"), strings.Index(plain, "Alex"); i < 0 || j < 0 || j < i {
		t.Fatalf("typed text should come before the signature: %q", plain)
	}
}
