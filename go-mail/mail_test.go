package main

import (
	"strings"
	"testing"
)

func TestFolderKind(t *testing.T) {
	cases := map[string]string{
		"INBOX":               "inbox",
		"[Gmail]/Sent Mail":   "sent",
		"[Gmail]/All Mail":    "archive",
		"[Gmail]/Trash":       "trash",
		"[Gmail]/Spam":        "junk",
		"[Gmail]/Drafts":      "drafts",
		"Sent Items":          "sent",
		"Deleted Items":       "trash",
		"Junk Email":          "junk",
		"Sent Messages":       "sent",
		"Archive":             "archive",
		"Projects":            "",
		"INBOX.Archive":       "archive",
		"Something/Trash Bin": "",
	}
	for name, want := range cases {
		if got := folderKind(name); got != want {
			t.Errorf("folderKind(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestParseBodyPrefersPlainText(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"Subject: Hi",
		"MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="x"`,
		"",
		"--x",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello world",
		"--x",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<p>Hello <b>world</b></p>",
		"--x--",
		"",
	}, "\r\n")
	plain, _ := parseBody([]byte(raw))
	if got := plain; got != "Hello world" {
		t.Errorf("parseBody = %q, want %q", got, "Hello world")
	}
}

func TestParseBodyFallsBackToHTML(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=utf-8",
		"",
		"<div>Hello&nbsp;<b>world</b></div><br><div>Second line</div>",
		"",
	}, "\r\n")
	plain, _ := parseBody([]byte(raw))
	got := plain
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") || !strings.Contains(got, "Second line") {
		t.Errorf("parseBody = %q", got)
	}
	if strings.Contains(got, "<") {
		t.Errorf("parseBody kept HTML tags: %q", got)
	}
}

func TestBuildMessage(t *testing.T) {
	acct := Account{Name: "Alex Doe", Email: "alex@example.com"}
	raw := string(buildMessage(acct, "bob@example.com", "", "Grüße", "line one\nline two", nil))
	for _, want := range []string{
		"From: \"Alex Doe\" <alex@example.com>",
		"To: bob@example.com",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"line one\r\nline two",
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("message lacks %q:\n%s", want, raw)
		}
	}
	if !strings.Contains(raw, "Subject: =?utf-8?") {
		t.Errorf("subject not encoded: %s", raw)
	}
}

func TestBuildMessageCc(t *testing.T) {
	acct := Account{Name: "Alex Doe", Email: "alex@example.com"}
	raw := string(buildMessage(acct, "bob@example.com", "carol@example.com", "subj", "hello", nil))
	for _, want := range []string{"To: bob@example.com", "Cc: carol@example.com"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("message lacks %q:\n%s", want, raw)
		}
	}

	plain := string(buildMessage(acct, "bob@example.com", "", "subj", "hello", nil))
	if strings.Contains(plain, "Cc:") {
		t.Fatalf("empty cc should not add a header:\n%s", plain)
	}
}
