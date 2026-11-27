package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/emersion/go-imap/v2"
)

func TestContextMenuAtPointer(t *testing.T) {
	a := &App{
		sel:         0,
		split:       420,
		sidebarSel:  "INBOX",
		folder:      "INBOX",
		dark:        true,
		systemDark:  true,
		settingsTab: "appearance",
		themeSettings: ThemeSettings{
			Theme: "nocturne", FontSize: 14, Radius: 9,
		},
		form: Account{
			Name:  "Alex Doe",
			Email: "alex@example.com",
		},
	}
	a.connected = true
	a.acct = a.form
	a.folders = []Folder{{Name: "INBOX", Label: "Inbox", Kind: "inbox"}}
	a.msgs = []Message{
		{UID: imap.UID(42), FromName: "Maya Chen", FromAddr: "maya@example.com", Subject: "Design review on Thursday", Date: time.Now()},
	}
	tt := ui.NewTester(a.view, 1320, 860)
	row, ok := tt.Find("Maya Chen")
	if !ok {
		t.Fatal("no message row")
	}
	x, y := row.X+row.W-40, row.Y+row.H/2
	tt.RightClickAt(x, y)
	// The panel scales in from 96%, so let its open motion settle before
	// checking that it is anchored at the pointer.
	tt.Advance(150 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("Reply"); !ok {
		t.Fatalf("no context menu; texts %q", tt.Texts())
	}
	panel, ok := tt.Find("Context menu")
	if !ok {
		t.Fatalf("no context menu panel; texts %q", tt.Texts())
	}
	if panel.X < x-4 || panel.X > x+4 || panel.Y < y-4 || panel.Y > y+4 {
		t.Fatalf("menu at %v, pointer at (%v, %v)", panel, x, y)
	}
}
