package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/emersion/go-imap/v2"
)

// TestViewsRender builds every screen in a headless window. NewTester runs
// frames and panics on a duplicate key or a broken layout, which the app
// itself would only log.
func TestViewsRender(t *testing.T) {
	a := &App{
		sel:         -1,
		split:       420,
		listState:   ui.ListState{},
		sidebarSel:  "INBOX",
		folder:      "INBOX",
		dark:        true,
		systemDark:  true,
		settingsTab: "appearance", settingsW: settingsRailDefaultWidth,
		themeSettings: ThemeSettings{Theme: "nocturne", FontSize: 14, Radius: 9},
		form: Account{
			Name:     "Alex Doe",
			Email:    "alex@example.com",
			IMAPHost: "imap.example.com",
			IMAPPort: "993",
			SMTPHost: "smtp.example.com",
			SMTPPort: "587",
		},
	}
	// Setup screen, light and dark.
	ui.NewTester(a.view, 1320, 860)
	a.dark = false
	ui.NewTester(a.view, 1320, 860)
	a.dark = true

	// Mailbox with a few messages.
	a.connected = true
	a.acct = a.form
	a.acct.Password = "secret"
	a.folders = []Folder{
		{Name: "INBOX", Label: "Inbox", Kind: "inbox", Unread: 2},
		{Name: "Sent", Label: "Sent", Kind: "sent"},
		{Name: "Drafts", Label: "Drafts", Kind: "drafts"},
		{Name: "Trash", Label: "Trash", Kind: "trash"},
	}
	a.msgs = []Message{
		{UID: imap.UID(42), FromName: "Maya Chen", FromAddr: "maya@example.com", Subject: "Design review on Thursday", Date: time.Now(), Read: false},
		{UID: imap.UID(41), FromName: "Build bot", FromAddr: "ci@example.com", Subject: "Build #128 passed", Date: time.Now().Add(-2 * time.Hour), Read: true},
	}
	a.sel = 0
	a.body = "Hi Alex,\n\nCan we move the design review to Thursday?\n\nThanks,\nMaya"
	ui.NewTester(a.view, 1320, 860)

	// Reading pane empty, then compose.
	a.sel = -1
	ui.NewTester(a.view, 1320, 860)
	a.showCompose = true
	a.composeTo = "maya@example.com"
	a.composeSub = "Re: Design review on Thursday"
	ui.NewTester(a.view, 1320, 860)

	// An error banner and a long subject must not break layout.
	a.showCompose = false
	a.sel = 0
	a.msgs[0].Subject = "A very long subject line that should be cut off with an ellipsis rather than push the pane wider than the window allows"
	a.errMsg = "cannot reach imap.example.com: dial tcp: connection refused"
	ui.NewTester(a.view, 980, 620)

	// The filter panel and its date picker popover build cleanly.
	a.filterPanelOpen = true
	a.filterDateFromOpen = true
	ui.NewTester(a.view, 1320, 860)
}
