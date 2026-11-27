package app

import (
	"testing"
	"time"
)

// TestMergeNewMessagesAppendsOnly checks that a poll keeps every message that
// is already loaded: new mail goes on top, old bodies stay, and the selection
// follows its message instead of its index.
func TestMergeNewMessagesAppendsOnly(t *testing.T) {
	now := time.Now()
	cur := []Message{
		{UID: 3, Subject: "three", Date: now},
		{UID: 2, Subject: "two", Date: now.Add(-time.Hour)},
	}
	cur[0].Body = "body that was already fetched"
	cur[1].Body = "older body"

	fresh := []Message{
		{UID: 4, Subject: "four", Date: now.Add(time.Minute)},
		{UID: 3, Subject: "three", Date: now, Read: true},
	}

	got := mergeNewMessages(cur, fresh, 1)
	if !got.changed || got.added != 1 {
		t.Fatalf("changed=%v added=%d", got.changed, got.added)
	}
	if len(got.msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(got.msgs))
	}
	if got.msgs[0].UID != 4 {
		t.Fatalf("new mail should be first, got UID %d", got.msgs[0].UID)
	}
	if got.msgs[1].Body != "body that was already fetched" {
		t.Fatalf("existing body was dropped: %q", got.msgs[1].Body)
	}
	if got.msgs[2].Body != "older body" {
		t.Fatalf("older body was dropped: %q", got.msgs[2].Body)
	}
	if !got.msgs[1].Read {
		t.Fatal("read state should follow the server")
	}
	if got.sel != 2 || got.msgs[got.sel].UID != 2 {
		t.Fatalf("selection = %d, want the message with UID 2", got.sel)
	}
	// one message became read, one new unread message arrived
	if got.unreadDelta != 0 {
		t.Fatalf("unreadDelta = %d, want 0", got.unreadDelta)
	}
}

// TestMergeNewMessagesNoChange checks that a poll of an unchanged mailbox
// leaves the list and selection alone.
func TestMergeNewMessagesNoChange(t *testing.T) {
	cur := []Message{
		{UID: 2, Subject: "two"},
		{UID: 1, Subject: "one"},
	}
	fresh := []Message{
		{UID: 2, Subject: "two"},
		{UID: 1, Subject: "one"},
	}
	got := mergeNewMessages(cur, fresh, 0)
	if got.changed || got.added != 0 || got.unreadDelta != 0 {
		t.Fatalf("changed=%v added=%d unreadDelta=%d", got.changed, got.added, got.unreadDelta)
	}
	if len(got.msgs) != 2 || got.msgs[0].UID != 2 || got.sel != 0 {
		t.Fatalf("list should be untouched: %+v sel=%d", got.msgs, got.sel)
	}
}

// TestMergeNewMessagesUnreadDelta checks the sidebar badge follows mail that
// arrived unread and mail read somewhere else.
func TestMergeNewMessagesUnreadDelta(t *testing.T) {
	cur := []Message{
		{UID: 2, Read: false},
		{UID: 1, Read: false},
	}
	fresh := []Message{
		{UID: 3, Read: false},
		{UID: 2, Read: true},
	}
	got := mergeNewMessages(cur, fresh, 0)
	// UID 3 adds one unread, UID 2 removes one.
	if got.unreadDelta != 0 {
		t.Fatalf("unreadDelta = %d, want 0", got.unreadDelta)
	}
	if !got.msgs[1].Read {
		t.Fatal("UID 2 should now be read")
	}
}
