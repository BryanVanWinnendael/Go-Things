package app

import (
	"testing"
	"time"
)

func TestMessageCacheKey(t *testing.T) {
	a := &App{acct: Account{Email: "me@example.com"}, folder: "INBOX"}
	if email, folder := a.messageCacheKey(Message{UID: 1, Folder: "INBOX"}); email != "me@example.com" || folder != "INBOX" {
		t.Fatalf("normal = %q/%q", email, folder)
	}
	if email, folder := a.messageCacheKey(Message{UID: 1, Folder: "Archive"}); email != "me@example.com" || folder != "Archive" {
		t.Fatalf("flagged = %q/%q", email, folder)
	}
	a.folder = "SHARED"
	if email, folder := a.messageCacheKey(Message{UID: 1, Folder: "other@example.com"}); email != "other@example.com" || folder != "INBOX" {
		t.Fatalf("shared = %q/%q", email, folder)
	}
}

func TestCacheMoveAndFlagHelpers(t *testing.T) {
	const me = "me@example.com"
	a := &App{
		acct:   Account{Email: me},
		folder: "INBOX",
		cache: map[string]*folderCache{
			"Archive": {msgs: []Message{{UID: 7, Folder: "Archive"}}},
			"FLAGGED": {msgs: []Message{{UID: 42, Folder: "INBOX", Flagged: true}}},
		},
		diskCache: diskCache{Accounts: map[string]map[string][]Message{
			me: {
				"INBOX":   {{UID: 42, Folder: "INBOX", Subject: "Hello", Date: time.Now()}},
				"Archive": {{UID: 7, Folder: "Archive"}},
			},
		}},
	}
	src := Message{UID: 42, Folder: "INBOX", Subject: "Hello", Date: time.Now()}

	a.cacheRemoveMessageAt(me, "INBOX", src)
	a.cacheAddMessageAt(me, "Archive", src)

	if got := a.diskCache.Accounts[me]["INBOX"]; len(got) != 0 {
		t.Fatalf("source still cached: %+v", got)
	}
	if got := a.diskCache.Accounts[me]["Archive"]; len(got) != 2 || got[0].Folder != "Archive" {
		t.Fatalf("destination cache = %+v", got)
	}
	if ce := a.cache["Archive"]; len(ce.msgs) != 2 {
		t.Fatalf("in-memory destination = %+v", ce.msgs)
	}
	if ce := a.cache["FLAGGED"]; len(ce.msgs) != 0 {
		t.Fatalf("flagged view still lists moved mail: %+v", ce.msgs)
	}

	a.cache["FLAGGED"] = &folderCache{msgs: []Message{{UID: 42, Folder: "INBOX", Flagged: true}}}
	a.diskCache.Accounts[me]["INBOX"] = []Message{{UID: 42, Folder: "INBOX", Flagged: true}}
	a.cacheUpdateMessageAt(me, "INBOX", Message{UID: 42, Folder: "INBOX"}, func(m *Message) { m.Flagged = false })
	if a.diskCache.Accounts[me]["INBOX"][0].Flagged {
		t.Fatal("disk cache flag not cleared")
	}
	if a.cache["FLAGGED"].msgs[0].Flagged {
		t.Fatal("flagged view flag not cleared")
	}
	a.cacheRemoveFromFlagged(me, Message{UID: 42, Folder: "INBOX"})
	if got := a.cache["FLAGGED"].msgs; len(got) != 0 {
		t.Fatalf("flagged view not cleared: %+v", got)
	}
}

func TestCacheSharedMoveDoesNotTouchOtherAccountFlagged(t *testing.T) {
	const me = "me@example.com"
	a := &App{
		acct:   Account{Email: me},
		folder: "SHARED",
		cache: map[string]*folderCache{
			"FLAGGED": {msgs: []Message{{UID: 42, Folder: "INBOX"}}},
			"SHARED":  {msgs: []Message{{UID: 42, Folder: "other@example.com"}}},
		},
		diskCache: diskCache{Accounts: map[string]map[string][]Message{
			"other@example.com": {"INBOX": {{UID: 42, Folder: "INBOX"}}},
		}},
	}
	m := Message{UID: 42, Folder: "other@example.com"}
	a.cacheRemoveMessageAt("other@example.com", "INBOX", m)
	if got := a.diskCache.Accounts["other@example.com"]["INBOX"]; len(got) != 0 {
		t.Fatalf("shared source not removed: %+v", got)
	}
	if got := a.cache["FLAGGED"].msgs; len(got) != 1 {
		t.Fatalf("unrelated flagged mail removed: %+v", got)
	}
	if got := a.cache["SHARED"].msgs; len(got) != 0 {
		t.Fatalf("shared view not updated: %+v", got)
	}
}
