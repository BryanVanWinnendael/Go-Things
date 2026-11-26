package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestFolderDragReorder drags a mailbox above another one the way Go Post
// reorders request tree rows, and checks the saved order.
func TestFolderDragReorder(t *testing.T) {
	a := &App{
		sel:         -1,
		split:       420,
		sidebarSel:  "INBOX",
		folder:      "INBOX",
		dark:        true,
		systemDark:  true,
		settingsTab: "appearance",
		themeSettings: ThemeSettings{
			Theme: "nocturne", FontSize: 14, Radius: 9,
		},
		form: Account{Name: "Alex Doe", Email: "alex@example.com"},
	}
	a.connected = true
	a.acct = a.form
	a.accounts = []Account{a.acct}
	a.active = 0
	a.folders = []Folder{
		{Name: "INBOX", Label: "Inbox", Kind: "inbox"},
		{Name: "Sent", Label: "Sent", Kind: "sent"},
		{Name: "Drafts", Label: "Drafts", Kind: "drafts"},
		{Name: "Trash", Label: "Trash", Kind: "trash"},
	}

	tt := ui.NewTester(a.view, 1320, 860)
	tt.Frame()
	tt.Frame()
	src, ok := tt.Find("Drafts")
	if !ok {
		t.Fatal("no Drafts row")
	}
	dst, ok := tt.Find("Inbox")
	if !ok {
		t.Fatal("no Inbox row")
	}
	// Press the Drafts row, move in steps, and release on the upper half of
	// Inbox so Drafts becomes the first folder.
	fx, fy := src.X+src.W/2, src.Y+src.H/2
	tx, ty := dst.X+dst.W/2, dst.Y+2
	tt.Press(fx, fy)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(fx+(tx-fx)*f, fy+(ty-fy)*f)
	}
	tt.Release(tx, ty)
	tt.Advance(250 * time.Millisecond)
	tt.Frame()

	if len(a.folders) == 0 || a.folders[0].Name != "Drafts" {
		t.Fatalf("folder order after drag: %v", a.folders)
	}
	if len(a.acct.FolderOrder) == 0 || a.acct.FolderOrder[0] != "Drafts" {
		t.Fatalf("saved folder order: %v", a.acct.FolderOrder)
	}
}
