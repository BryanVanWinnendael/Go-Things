package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	gomail "github.com/emersion/go-message/mail"
)

func TestBuildMessageSignatureImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	acct := Account{Name: "Alex", Email: "alex@example.com", SignatureCanvas: []SignatureImage{
		{Data: b64, X: 10, Y: 20, Width: 25},
	}}
	raw := buildMessage(acct, "bob@example.com", "", "subj", "hello", nil)
	if !bytes.Contains(raw, []byte("multipart/alternative")) {
		t.Fatal("expected multipart/alternative")
	}
	if !bytes.Contains(raw, []byte("Content-ID: <signature-image-0>")) {
		t.Fatal("missing inline signature image")
	}
	r, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var plain, html string
	var inline bool
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		ct := p.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(ct, "text/plain"):
			b, _ := io.ReadAll(p.Body)
			plain = string(b)
		case strings.HasPrefix(ct, "text/html"):
			b, _ := io.ReadAll(p.Body)
			html = string(b)
		case strings.HasPrefix(ct, "image/"):
			inline = true
		}
		io.Copy(io.Discard, p.Body)
	}
	if plain == "" || html == "" {
		t.Fatalf("missing parts: plain=%q html=%q", plain, html)
	}
	if !inline {
		t.Fatal("inline image part not found")
	}
	if !strings.Contains(html, "cid:signature-image") {
		t.Fatalf("html lacks cid reference: %s", html)
	}
	if !strings.Contains(html, `width="150"`) {
		t.Fatalf("image placement missing: %s", html)
	}
}

func TestBuildMessageRichSignature(t *testing.T) {
	acct := Account{
		Name:          "Alex",
		Email:         "alex@example.com",
		Signature:     "Alex",
		SignatureHTML: `<meta charset="utf-8"><p><span style="font-family:Georgia, serif;color:#2563eb">Alex</span></p>`,
	}
	raw := buildMessage(acct, "bob@example.com", "", "subj", "hello\n\nAlex", nil)
	if !bytes.Contains(raw, []byte("multipart/alternative")) {
		t.Fatal("rich signature without images still needs HTML")
	}
	r, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var html string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") {
			b, _ := io.ReadAll(p.Body)
			html = string(b)
		}
		io.Copy(io.Discard, p.Body)
	}
	for _, want := range []string{"font-family:Georgia", "color:#2563eb"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html lacks %q: %s", want, html)
		}
	}
}

func TestBuildMessageRichSignatureJSON(t *testing.T) {
	text := richtext.TextOf(richtext.Paragraph{Runs: []richtext.Run{
		{Text: "Alex", Attributes: richtext.Attributes{
			Font: "Georgia, serif", Size: 18, Weight: 700, Color: ui.Hex("#2563eb"),
		}},
	}})
	data, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	acct := Account{Name: "Alex", Email: "alex@example.com", Signature: "Alex", SignatureRich: string(data)}
	raw := buildMessage(acct, "bob@example.com", "", "subj", "hello\n\nAlex", nil)
	if !bytes.Contains(raw, []byte("multipart/alternative")) {
		t.Fatal("rich signature without images still needs HTML")
	}
	r, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var html string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") {
			b, _ := io.ReadAll(p.Body)
			html = string(b)
		}
		io.Copy(io.Discard, p.Body)
	}
	for _, want := range []string{"Georgia, serif", "color:#2563eb", "font-size:18px", "<b>"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html lacks %q: %s", want, html)
		}
	}
}

func TestBuildMessageAttachments(t *testing.T) {
	acct := Account{Name: "Alex", Email: "alex@example.com"}
	payload := []byte("hello world")
	raw := buildMessage(acct, "bob@example.com", "", "with attachments", "body text", []OutgoingAttachment{
		{Name: "hello.txt", Type: "text/plain", Data: payload},
		{Name: "data.bin", Type: "application/octet-stream", Data: []byte{0x00, 0xff, 0x01}},
	})
	if !bytes.Contains(raw, []byte("multipart/mixed")) {
		t.Fatal("message is not multipart")
	}
	r, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	names := []string{}
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if ah, ok := p.Header.(*gomail.AttachmentHeader); ok {
			name, err := ah.Filename()
			if err != nil {
				t.Fatal(err)
			}
			names = append(names, name)
			data, err := io.ReadAll(p.Body)
			if err != nil {
				t.Fatal(err)
			}
			if name == "hello.txt" && !bytes.Equal(data, payload) {
				t.Errorf("attachment data mismatch: %q", data)
			}
		} else {
			io.Copy(io.Discard, p.Body)
		}
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 attachments, got %d: %v", len(names), names)
	}
}
