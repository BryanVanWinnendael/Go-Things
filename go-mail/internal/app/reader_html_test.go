package app

import (
	"strings"
	"testing"
)

// TestMessageHTMLDocForcesReadableText covers the case where a mail hard-codes
// dark text: on the dark reader canvas it was invisible until selected.
func TestMessageHTMLDocForcesReadableText(t *testing.T) {
	a := &App{}
	doc := a.messageHTMLDoc(`<html><body style="color:#000000"><div style="color:#222">Hello</div><a href="#">link</a></body></html>`)
	for _, want := range []string{
		"background:#15161c",
		"color:#e7e7ea!important",
		"a,a *{color:#7db4ff!important;}",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("wrapper lacks %q: %s", want, doc)
		}
	}
}

func TestMessageHTMLDocKeepsLightNewslettersLight(t *testing.T) {
	a := &App{}
	doc := a.messageHTMLDoc(`<body bgcolor="#ffffff"><p style="color:#333">Hello</p></body>`)
	for _, want := range []string{
		"background:#ffffff",
		"color:#1a1a1a!important",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("wrapper lacks %q: %s", want, doc)
		}
	}
}
