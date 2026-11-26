package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	_ "github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
)

// SignatureImage is a picture placed anywhere on the signature canvas.
// X, Y and Width are percentages of the canvas, so a signature looks the
// same at any window size and in the sent message.
type SignatureImage struct {
	Data  string  `json:"data"`
	X     float32 `json:"x,omitempty"`
	Y     float32 `json:"y,omitempty"`
	Width float32 `json:"width,omitempty"`
}

// Account is what the setup screen collects. Password is kept in memory only.
type Account struct {
	Name            string           `json:"name"`
	Email           string           `json:"email"`
	Signature       string           `json:"signature,omitempty"`
	SignatureRich   string           `json:"signatureRich,omitempty"`
	SignatureHTML   string           `json:"signatureHtml,omitempty"`
	SignatureImage  string           `json:"signatureImage,omitempty"`
	SignatureImages []string         `json:"signatureImages,omitempty"`
	SignatureCanvas []SignatureImage `json:"signatureCanvas,omitempty"`
	IMAPHost        string           `json:"imap_host"`
	IMAPPort        string           `json:"imap_port"`
	SMTPHost        string           `json:"smtp_host"`
	SMTPPort        string           `json:"smtp_port"`
	Password        string           `json:"password"`
	Theme           ThemeSettings    `json:"theme,omitempty"`
	FolderOrder     []string         `json:"folderOrder,omitempty"`
}

// Folder is a mailbox shown in the sidebar.
type Folder struct {
	Name   string
	Label  string
	Kind   string // inbox, sent, drafts, junk, trash, archive
	Unread int
}

// Attachment is a downloadable part of a message.
type Attachment struct {
	Part     []int  `json:"part"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     uint32 `json:"size"`
	Encoding string `json:"encoding,omitempty"`
}

// OutgoingAttachment is a file to include when sending a message.
type OutgoingAttachment struct {
	Name string
	Type string
	Data []byte
}

// Message is one row of the message list.
type Message struct {
	UID         imap.UID
	Folder      string
	MessageID   string
	Attachments []Attachment `json:"attachments,omitempty"`
	FromName    string
	FromAddr    string
	To          []string
	Cc          []string
	Subject     string
	Date        time.Time
	Read        bool
	Flagged     bool
	Body        string
	BodyErr     string
}

// Session is one logged-in IMAP connection. Its methods are not safe for
// concurrent use; the app serialises them.
type Session struct {
	c     *imapclient.Client
	acct  Account
	trash string
}

// Open logs in and returns the folders to show.
func Open(a Account) (*Session, []Folder, error) {
	c, err := dialIMAP(a)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot reach %s:%s: %w", a.IMAPHost, a.IMAPPort, err)
	}
	if err := c.Login(a.Email, a.Password).Wait(); err != nil {
		c.Close()
		return nil, nil, fmt.Errorf("login failed: %w%s", err, authHint(a, err))
	}
	s := &Session{c: c, acct: a}
	folders, err := s.loadFolders()
	if err != nil {
		c.Close()
		return nil, nil, fmt.Errorf("cannot list mailboxes: %w", err)
	}
	return s, folders, nil
}

// authHint explains the usual fix when a provider rejects the password.
func authHint(a Account, err error) string {
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "auth") && !strings.Contains(msg, "credential") && !strings.Contains(msg, "password") {
		return ""
	}
	host := strings.ToLower(a.IMAPHost)
	switch {
	case strings.Contains(host, "gmail") || strings.Contains(host, "google"):
		return " — Gmail needs an App Password, not your normal password. Turn on 2-Step Verification, create one at https://myaccount.google.com/apppasswords, then paste the 16-character password here (no spaces). Also enable IMAP in Gmail: Settings → Forwarding and POP/IMAP → Enable IMAP."
	case strings.Contains(host, "icloud") || strings.Contains(host, "me.com") || strings.Contains(host, "mac.com"):
		return " — iCloud needs an App Password from https://account.apple.com (Sign-In and Security → App-Specific Passwords)."
	case strings.Contains(host, "yahoo"):
		return " — Yahoo needs an App Password from https://login.yahoo.com/account/security (Generate app password)."
	}
	return ""
}

func dialIMAP(a Account) (*imapclient.Client, error) {
	host := strings.TrimSpace(a.IMAPHost)
	port := strings.TrimSpace(a.IMAPPort)
	if port == "" {
		port = "993"
	}
	addr := net.JoinHostPort(host, port)
	opts := &imapclient.Options{TLSConfig: &tls.Config{ServerName: host}}
	if port == "143" {
		return imapclient.DialStartTLS(addr, opts)
	}
	c, err := imapclient.DialTLS(addr, opts)
	if err == nil {
		return c, nil
	}
	// A server on a non-standard port may offer STARTTLS instead.
	if c2, err2 := imapclient.DialStartTLS(addr, opts); err2 == nil {
		return c2, nil
	}
	return nil, err
}

func (s *Session) Close() {
	if s == nil || s.c == nil {
		return
	}
	_ = s.c.Logout().Wait()
	s.c.Close()
}

// loadFolders lists the mailboxes worth showing and their unread counts.
func (s *Session) loadFolders() ([]Folder, error) {
	list, err := s.c.List("", "*", nil).Collect()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{"INBOX": true}
	seenKind := map[string]bool{"inbox": true}
	out := []Folder{{Name: "INBOX", Label: "Inbox", Kind: "inbox"}}
	for _, d := range list {
		if hasAttr(d.Attrs, imap.MailboxAttrNoSelect) || hasAttr(d.Attrs, imap.MailboxAttrNonExistent) {
			continue
		}
		if seen[d.Mailbox] {
			continue
		}
		kind := folderKindFromAttrs(d.Attrs)
		if kind == "" {
			kind = folderKind(d.Mailbox)
		}
		if kind == "inbox" {
			continue
		}
		seen[d.Mailbox] = true
		label := d.Mailbox
		if kind != "" {
			if seenKind[kind] {
				label = d.Mailbox
			} else {
				label = kindLabel(kind)
			}
			seenKind[kind] = true
		}
		folderKindName := kind
		if folderKindName == "" {
			folderKindName = "folder"
		}
		out = append(out, Folder{Name: d.Mailbox, Label: label, Kind: folderKindName})
	}
	slices.SortStableFunc(out, func(a, b Folder) int {
		if ra, rb := folderRank(a.Kind), folderRank(b.Kind); ra != rb {
			return ra - rb
		}
		return strings.Compare(a.Label, b.Label)
	})
	for i := range out {
		if out[i].Kind == "trash" && s.trash == "" {
			s.trash = out[i].Name
		}
		st, err := s.c.Status(out[i].Name, &imap.StatusOptions{NumUnseen: true}).Wait()
		if err == nil && st.NumUnseen != nil {
			out[i].Unread = int(*st.NumUnseen)
		}
	}
	return out, nil
}

func hasAttr(attrs []imap.MailboxAttr, want imap.MailboxAttr) bool {
	return slices.Contains(attrs, want)
}

// folderKindFromAttrs derives a standard folder kind from IMAP special-use
// attributes (RFC 6154), which is more reliable than matching localized names.
func folderKindFromAttrs(attrs []imap.MailboxAttr) string {
	for _, a := range attrs {
		switch a {
		case imap.MailboxAttrSent:
			return "sent"
		case imap.MailboxAttrDrafts:
			return "drafts"
		case imap.MailboxAttrTrash:
			return "trash"
		case imap.MailboxAttrJunk:
			return "junk"
		case imap.MailboxAttrFlagged:
			return "flagged"
		case imap.MailboxAttrArchive, imap.MailboxAttrAll:
			return "archive"
		}
	}
	return ""
}

// folderKind maps a mailbox name to one of the standard folders.
func folderKind(name string) string {
	lower := strings.ToLower(name)
	leaf := lower
	if i := strings.LastIndexAny(lower, "/."); i >= 0 {
		leaf = lower[i+1:]
	}
	switch {
	case lower == "inbox":
		return "inbox"
	case leaf == "sent", leaf == "sent mail", leaf == "sent items", leaf == "sent messages", leaf == "sent e-mail":
		return "sent"
	case leaf == "drafts", leaf == "draft", leaf == "draft messages":
		return "drafts"
	case leaf == "trash", leaf == "deleted items", leaf == "deleted messages", leaf == "bin":
		return "trash"
	case leaf == "junk", leaf == "spam", leaf == "junk e-mail", leaf == "junk email", leaf == "bulk mail":
		return "junk"
	case leaf == "starred", leaf == "flagged", leaf == "starred messages":
		return "flagged"
	case leaf == "archive", leaf == "archives", leaf == "all mail":
		return "archive"
	}
	return ""
}

func folderRank(kind string) int {
	switch kind {
	case "inbox":
		return 0
	case "sent":
		return 1
	case "drafts":
		return 2
	case "junk":
		return 3
	case "flagged":
		return 4
	case "archive":
		return 5
	case "trash":
		return 6
	default:
		return 100
	}
}

func kindLabel(kind string) string {
	switch kind {
	case "inbox":
		return "Inbox"
	case "sent":
		return "Sent"
	case "drafts":
		return "Drafts"
	case "junk":
		return "Junk"
	case "flagged":
		return "Flagged"
	case "trash":
		return "Trash"
	case "archive":
		return "Archive"
	}
	return kind
}

// Messages selects a mailbox and returns its newest messages, newest first.
// limit is the page size (0 = all); offset skips that many newest messages.
func (s *Session) Messages(name string, limit, offset int) ([]Message, error) {
	data, err := s.c.Select(name, nil).Wait()
	if err != nil {
		return nil, err
	}
	if data.NumMessages == 0 {
		return nil, nil
	}
	total := int(data.NumMessages)
	if limit <= 0 {
		limit = total
	}
	if offset < 0 {
		offset = 0
	}
	end := total - offset
	if end < 1 {
		return nil, nil
	}
	start := end - limit + 1
	if start < 1 {
		start = 1
	}
	var set imap.SeqSet
	set.AddRange(uint32(start), uint32(end))
	return s.fetchEnvelopes(name, set)
}

// fetchEnvelopes fetches envelopes for a set of messages in the selected
// mailbox and returns them newest first.
func (s *Session) fetchEnvelopes(name string, set imap.NumSet) ([]Message, error) {
	opts := &imap.FetchOptions{Envelope: true, Flags: true, UID: true, InternalDate: true, BodyStructure: &imap.FetchItemBodyStructure{}}
	bufs, err := s.c.Fetch(set, opts).Collect()
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(bufs))
	for _, b := range bufs {
		m := Message{
			UID:     b.UID,
			Folder:  name,
			Read:    slices.Contains(b.Flags, imap.FlagSeen),
			Flagged: slices.Contains(b.Flags, imap.FlagFlagged),
		}
		if b.Envelope != nil {
			m.Subject = strings.TrimSpace(b.Envelope.Subject)
			m.MessageID = b.Envelope.MessageID
			m.Date = b.Envelope.Date
			if len(b.Envelope.From) > 0 {
				from := b.Envelope.From[0]
				m.FromAddr = from.Addr()
				m.FromName = strings.TrimSpace(from.Name)
			}
			for _, a := range b.Envelope.To {
				if addr := a.Addr(); addr != "" {
					m.To = append(m.To, addr)
				}
			}
			for _, a := range b.Envelope.Cc {
				if addr := a.Addr(); addr != "" {
					m.Cc = append(m.Cc, addr)
				}
			}
		}
		if m.Date.IsZero() {
			m.Date = b.InternalDate
		}
		if m.FromName == "" {
			m.FromName = displayName(m.FromAddr)
		}
		if m.Subject == "" {
			m.Subject = "(no subject)"
		}
		if b.BodyStructure != nil {
			b.BodyStructure.Walk(func(path []int, part imap.BodyStructure) bool {
				if sp, ok := part.(*imap.BodyStructureSinglePart); ok {
					if name := sp.Filename(); name != "" {
						m.Attachments = append(m.Attachments, Attachment{
							Part:     append([]int(nil), path...),
							Name:     name,
							Type:     sp.MediaType(),
							Size:     sp.Size,
							Encoding: sp.Encoding,
						})
					}
				}
				return true
			})
		}
		msgs = append(msgs, m)
	}
	slices.Reverse(msgs)
	return msgs, nil
}

// Search returns the messages in one mailbox matching query, newest first.
// It uses the server's SEARCH so it also finds mail that was never fetched.
func (s *Session) Search(name, query string) ([]Message, error) {
	return s.searchMailbox(name, query, false)
}

// SearchFlagged returns every flagged message across all mailboxes, newest
// first. query narrows the result like Search.
func (s *Session) SearchFlagged(query string) ([]Message, error) {
	list, err := s.c.List("", "*", nil).Collect()
	if err != nil {
		return nil, err
	}
	var out []Message
	seen := map[string]bool{}
	for _, d := range list {
		if d.Mailbox == "" || hasAttr(d.Attrs, imap.MailboxAttrNoSelect) || hasAttr(d.Attrs, imap.MailboxAttrNonExistent) {
			continue
		}
		msgs, err := s.searchMailbox(d.Mailbox, query, true)
		if err != nil {
			continue
		}
		for _, m := range msgs {
			key := "id:" + m.MessageID
			if m.MessageID == "" {
				key = m.Folder + "|" + fmt.Sprint(m.UID)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(a, b Message) int {
		switch {
		case a.Date.After(b.Date):
			return -1
		case a.Date.Before(b.Date):
			return 1
		default:
			return 0
		}
	})
	return out, nil
}

// searchMailbox runs a UID SEARCH in one mailbox and fetches only the
// envelopes it matched.
func (s *Session) searchMailbox(name, query string, flaggedOnly bool) ([]Message, error) {
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return nil, err
	}
	criteria := &imap.SearchCriteria{}
	if flaggedOnly {
		criteria.Flag = []imap.Flag{imap.FlagFlagged}
	}
	if q := strings.TrimSpace(query); q != "" {
		criteria.Text = []string{q}
	}
	data, err := s.c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, err
	}
	set, ok := data.All.(imap.UIDSet)
	if !ok {
		return nil, nil
	}
	uids, _ := set.Nums()
	if len(uids) == 0 {
		return nil, nil
	}
	return s.fetchEnvelopes(name, imap.UIDSetNum(uids...))
}

// Body fetches and decodes the text of one message. It returns the plain text
// (for quoting), the HTML body (for rich rendering) and any attachments.
func (s *Session) Body(name string, uid imap.UID) (string, string, []Attachment, error) {
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return "", "", nil, err
	}
	opts := &imap.FetchOptions{
		UID:           true,
		BodySection:   []*imap.FetchItemBodySection{{Peek: true}},
		BodyStructure: &imap.FetchItemBodyStructure{},
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(uid), opts).Collect()
	if err != nil {
		return "", "", nil, err
	}
	if len(bufs) == 0 || len(bufs[0].BodySection) == 0 {
		return "", "", nil, fmt.Errorf("no body returned")
	}
	plain, htmlBody := parseBody(bufs[0].BodySection[0].Bytes)
	var attachments []Attachment
	if bufs[0].BodyStructure != nil {
		bufs[0].BodyStructure.Walk(func(path []int, part imap.BodyStructure) bool {
			sp, ok := part.(*imap.BodyStructureSinglePart)
			if !ok {
				return true
			}
			if name := sp.Filename(); name != "" {
				attachments = append(attachments, Attachment{
					Part:     append([]int(nil), path...),
					Name:     name,
					Type:     sp.MediaType(),
					Size:     sp.Size,
					Encoding: sp.Encoding,
				})
			}
			return true
		})
	}
	return plain, htmlBody, attachments, nil
}

// Attachment fetches one attachment's decoded bytes.
func (s *Session) Attachment(name string, uid imap.UID, att Attachment) ([]byte, error) {
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return nil, err
	}
	opts := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{{Part: att.Part}},
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(uid), opts).Collect()
	if err != nil {
		return nil, err
	}
	if len(bufs) == 0 || len(bufs[0].BodySection) == 0 {
		return nil, fmt.Errorf("no attachment data")
	}
	raw := bufs[0].BodySection[0].Bytes
	switch strings.ToLower(att.Encoding) {
	case "base64":
		clean := strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(raw))
		dec, err := base64.StdEncoding.DecodeString(clean)
		if err != nil {
			return nil, err
		}
		return dec, nil
	case "quoted-printable":
		r := quotedprintable.NewReader(bytes.NewReader(raw))
		return io.ReadAll(r)
	default:
		return append([]byte(nil), raw...), nil
	}
}

// SetFlagged marks a message flagged or unflagged.
func (s *Session) SetFlagged(name string, uid imap.UID, flagged bool) error {
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return err
	}
	op := imap.StoreFlagsAdd
	if !flagged {
		op = imap.StoreFlagsDel
	}
	return s.c.Store(imap.UIDSetNum(uid),
		&imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{imap.FlagFlagged}}, nil).Close()
}
func (s *Session) SetSeen(name string, uid imap.UID, seen bool) error {
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return err
	}
	op := imap.StoreFlagsAdd
	if !seen {
		op = imap.StoreFlagsDel
	}
	return s.c.Store(imap.UIDSetNum(uid),
		&imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{imap.FlagSeen}}, nil).Close()
}

// Delete moves a message to Trash, or removes it when it is already there.
// Delete moves a message to Trash (or permanently removes it when already
// there) and returns its UID in Trash when it was moved.
func (s *Session) Delete(name string, uid imap.UID) (imap.UID, error) {
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return uid, err
	}
	if s.trash != "" && !strings.EqualFold(name, s.trash) {
		if data, err := s.c.Move(imap.UIDSetNum(uid), s.trash).Wait(); err == nil {
			newUID := uid
			if nu := firstUIDFromNumSet(data.DestUIDs); nu != 0 {
				newUID = nu
			}
			return newUID, nil
		}
	}
	if err := s.c.Store(imap.UIDSetNum(uid),
		&imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
		return uid, err
	}
	if err := s.c.Expunge().Close(); err != nil {
		return uid, err
	}
	return 0, nil
}

// Move moves a message to another mailbox (copy + delete).
func firstUIDFromNumSet(set imap.NumSet) imap.UID {
	if set == nil {
		return 0
	}
	switch v := set.(type) {
	case imap.UIDSet:
		if nums, ok := v.Nums(); ok && len(nums) > 0 {
			return nums[0]
		}
	case *imap.UIDSet:
		if nums, ok := v.Nums(); ok && len(nums) > 0 {
			return nums[0]
		}
	case imap.SeqSet:
		if nums, ok := v.Nums(); ok && len(nums) > 0 {
			return imap.UID(nums[0])
		}
	case *imap.SeqSet:
		if nums, ok := v.Nums(); ok && len(nums) > 0 {
			return imap.UID(nums[0])
		}
	}
	return 0
}

// Move moves a message to another mailbox and returns its UID in the
// destination (or the original UID when the server does not report one).
func (s *Session) Move(name string, uid imap.UID, dest string) (imap.UID, error) {
	if dest == "" || strings.EqualFold(name, dest) {
		return uid, nil
	}
	if _, err := s.c.Select(name, nil).Wait(); err != nil {
		return uid, err
	}
	newUID := uid
	// Try MOVE first.
	if data, err := s.c.Move(imap.UIDSetNum(uid), dest).Wait(); err == nil {
		if nu := firstUIDFromNumSet(data.DestUIDs); nu != 0 {
			newUID = nu
		}
		return newUID, nil
	}
	// Fallback: copy then delete.
	data, err := s.c.Copy(imap.UIDSetNum(uid), dest).Wait()
	if err != nil {
		return uid, err
	}
	if nu := firstUIDFromNumSet(data.DestUIDs); nu != 0 {
		newUID = nu
	}
	if err := s.c.Store(imap.UIDSetNum(uid),
		&imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
		return uid, err
	}
	if err := s.c.Expunge().Close(); err != nil {
		return uid, err
	}
	return newUID, nil
}
func (s *Session) Append(name string, raw []byte) error {
	if name == "" {
		return nil
	}
	cmd := s.c.Append(name, int64(len(raw)), nil)
	if _, err := cmd.Write(raw); err != nil {
		cmd.Close()
		return err
	}
	return cmd.Close()
}

// Send delivers a raw message to the recipients in to over SMTP.
func Send(a Account, to string, raw []byte) error {
	addrs, err := mail.ParseAddressList(to)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("invalid recipient %q", to)
	}

	host := strings.TrimSpace(a.SMTPHost)
	port := strings.TrimSpace(a.SMTPPort)
	if port == "" {
		port = "587"
	}
	addr := net.JoinHostPort(host, port)
	tlsCfg := &tls.Config{ServerName: host}

	var c *smtp.Client
	if port == "465" {
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("cannot reach %s: %w", addr, err)
		}
		c, err = smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return err
		}
	} else {
		c, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("cannot reach %s: %w", addr, err)
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				c.Close()
				return fmt.Errorf("TLS failed: %w", err)
			}
		}
	}
	defer c.Close()

	if ok, _ := c.Extension("AUTH"); ok {
		if err := c.Auth(smtp.PlainAuth("", a.Email, a.Password, host)); err != nil {
			return fmt.Errorf("SMTP login failed: %w", err)
		}
	}
	if err := c.Mail(a.Email); err != nil {
		return err
	}
	for _, addr := range addrs {
		if err := c.Rcpt(addr.Address); err != nil {
			return fmt.Errorf("cannot send to %s: %w", addr.Address, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// BuildMessage returns the raw RFC 5322 message, also kept in Sent.
func BuildMessage(a Account, to, cc, subject, body string, attachments []OutgoingAttachment) []byte {
	return buildMessage(a, to, cc, subject, body, attachments)
}

// BuildRichMessage builds a message from an editor's rendered body, where
// html may refer to inline images by cid:signature-image-N.
func BuildRichMessage(a Account, to, cc, subject, plain, html string, inline [][]byte, inlineTypes []string, attachments []OutgoingAttachment) []byte {
	from := mail.Address{Name: a.Name, Address: a.Email}
	hasInline := len(inline) > 0

	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	if to = strings.TrimSpace(to); to != "" {
		b.WriteString("To: " + to + "\r\n")
	}
	if cc = strings.TrimSpace(cc); cc != "" {
		b.WriteString("Cc: " + cc + "\r\n")
	}
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: " + messageID(a.Email) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	text := strings.ReplaceAll(plain, "\n", "\r\n")

	if len(attachments) > 0 {
		topBoundary := newBoundary()
		b.WriteString("Content-Type: multipart/mixed; boundary=\"" + topBoundary + "\"\r\n")
		b.WriteString("\r\n")
		b.WriteString("--" + topBoundary + "\r\n")
		if hasInline {
			writeRelatedPart(&b, text, html, inline, inlineTypes)
		} else {
			writeAlternativePart(&b, text, html, nil, nil)
		}
		for _, att := range attachments {
			writeFileAttachment(&b, topBoundary, att)
		}
		b.WriteString("--" + topBoundary + "--\r\n")
	} else if hasInline {
		writeRelatedPart(&b, text, html, inline, inlineTypes)
	} else {
		writeAlternativePart(&b, text, html, nil, nil)
	}
	return []byte(b.String())
}

func buildMessage(a Account, to, cc, subject, body string, attachments []OutgoingAttachment) []byte {
	from := mail.Address{Name: a.Name, Address: a.Email}
	sigImgData, sigImgTypes := signatureImageParts(a)
	hasSigImg := len(sigImgData) > 0

	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	if to = strings.TrimSpace(to); to != "" {
		b.WriteString("To: " + to + "\r\n")
	}
	if cc = strings.TrimSpace(cc); cc != "" {
		b.WriteString("Cc: " + cc + "\r\n")
	}
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: " + messageID(a.Email) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	text := strings.ReplaceAll(body, "\n", "\r\n")
	htmlBody, richSig := composeHTML(a, body)

	if !hasSigImg && !richSig && len(attachments) == 0 {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(text)
		return []byte(b.String())
	}

	if len(attachments) > 0 {
		topBoundary := newBoundary()
		b.WriteString("Content-Type: multipart/mixed; boundary=\"" + topBoundary + "\"\r\n")
		b.WriteString("\r\n")
		b.WriteString("--" + topBoundary + "\r\n")
		if hasSigImg {
			writeRelatedPart(&b, text, htmlBody, sigImgData, sigImgTypes)
		} else {
			writeAlternativePart(&b, text, htmlBody, nil, nil)
		}
		for _, att := range attachments {
			writeFileAttachment(&b, topBoundary, att)
		}
		b.WriteString("--" + topBoundary + "--\r\n")
	} else if hasSigImg {
		writeRelatedPart(&b, text, htmlBody, sigImgData, sigImgTypes)
	} else {
		writeAlternativePart(&b, text, htmlBody, nil, nil)
	}
	return []byte(b.String())
}

func writeAlternativePart(b *strings.Builder, plain, html string, sigImgData [][]byte, sigImgTypes []string) {
	hasSigImg := len(sigImgData) > 0
	altBoundary := newBoundary()
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + altBoundary + "\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("--" + altBoundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(plain)
	b.WriteString("\r\n")
	b.WriteString("--" + altBoundary + "\r\n")
	if hasSigImg {
		relBoundary := newBoundary()
		b.WriteString("Content-Type: multipart/related; boundary=\"" + relBoundary + "\"\r\n")
		b.WriteString("\r\n")
		b.WriteString("--" + relBoundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(html)
		b.WriteString("\r\n")
		for i := range sigImgData {
			b.WriteString("--" + relBoundary + "\r\n")
			b.WriteString("Content-Type: " + sigImgTypes[i] + "; name=\"signature-image-" + strconv.Itoa(i) + "\"\r\n")
			b.WriteString("Content-Transfer-Encoding: base64\r\n")
			b.WriteString(fmt.Sprintf("Content-ID: <signature-image-%d>\r\n", i))
			b.WriteString(fmt.Sprintf("Content-Location: signature-image-%d\r\n", i))
			b.WriteString(fmt.Sprintf("Content-Disposition: inline; filename=\"signature-image-%d\"\r\n", i))
			b.WriteString("\r\n")
			writeBase64(b, sigImgData[i])
			b.WriteString("\r\n")
		}
		b.WriteString("--" + relBoundary + "--\r\n")
	} else {
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(html)
		b.WriteString("\r\n")
	}
	b.WriteString("--" + altBoundary + "--\r\n")
}

func writeRelatedPart(b *strings.Builder, plain, html string, sigImgData [][]byte, sigImgTypes []string) {
	relBoundary := newBoundary()
	b.WriteString("Content-Type: multipart/related; boundary=\"" + relBoundary + "\"\r\n")
	b.WriteString("\r\n")
	altBoundary := newBoundary()
	b.WriteString("--" + relBoundary + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + altBoundary + "\"\r\n")
	b.WriteString("\r\n")
	b.WriteString("--" + altBoundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(plain)
	b.WriteString("\r\n")
	b.WriteString("--" + altBoundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(html)
	b.WriteString("\r\n")
	b.WriteString("--" + altBoundary + "--\r\n")
	for i := range sigImgData {
		b.WriteString("--" + relBoundary + "\r\n")
		b.WriteString("Content-Type: " + sigImgTypes[i] + "; name=\"signature-image-" + strconv.Itoa(i) + "\"\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n")
		b.WriteString(fmt.Sprintf("Content-ID: <signature-image-%d>\r\n", i))
		b.WriteString(fmt.Sprintf("Content-Location: signature-image-%d\r\n", i))
		b.WriteString(fmt.Sprintf("Content-Disposition: inline; filename=\"signature-image-%d\"\r\n", i))
		b.WriteString("\r\n")
		writeBase64(b, sigImgData[i])
		b.WriteString("\r\n")
	}
	b.WriteString("--" + relBoundary + "--\r\n")
}
func writeFileAttachment(b *strings.Builder, boundary string, att OutgoingAttachment) {
	ct := att.Type
	if ct == "" {
		ct = "application/octet-stream"
	}
	if parsed, _, err := mime.ParseMediaType(ct); err == nil {
		ct = parsed
	}
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: " + mime.FormatMediaType(ct, map[string]string{"name": att.Name}) + "\r\n")
	b.WriteString("Content-Disposition: " + mime.FormatMediaType("attachment", map[string]string{"filename": att.Name}) + "\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("\r\n")
	writeBase64(b, att.Data)
	b.WriteString("\r\n")
}

// signatureImageParts returns the bytes and content types of the images
// sent inline with the signature. Inline (richtext) images come first, then
// legacy canvas images.
func signatureImageParts(a Account) ([][]byte, []string) {
	var data [][]byte
	var types []string
	// Inline images from the rich text document.
	if rich := strings.TrimSpace(a.SignatureRich); rich != "" {
		var text richtext.Text
		if err := json.Unmarshal([]byte(rich), &text); err == nil {
			// Count inline attachments.
			n := 0
			for _, p := range text.Paragraphs() {
				for _, r := range p.Runs {
					if r.Attributes.Attachment != nil {
						n++
					}
				}
			}
			// Use persisted image bytes (SignatureImages), matched by order.
			// Signatures saved by the old canvas editor may have none yet;
			// fall back to the canvas bytes.
			for i := 0; i < n; i++ {
				raw := ""
				if i < len(a.SignatureImages) {
					raw = a.SignatureImages[i]
				}
				if raw == "" && i < len(a.SignatureCanvas) {
					raw = a.SignatureCanvas[i].Data
				}
				d, ct := decodeSignatureImage(raw)
				if len(d) > 0 {
					data = append(data, d)
					types = append(types, ct)
				}
			}
		}
	}
	// Legacy canvas images (old signatures) only if no inline images.
	if len(data) == 0 {
		for _, im := range a.SignatureCanvas {
			raw, ct := decodeSignatureImage(im.Data)
			if len(raw) > 0 {
				data = append(data, raw)
				types = append(types, ct)
			}
		}
		if len(data) == 0 {
			for _, b64 := range legacySignatureImages(a) {
				raw, ct := decodeSignatureImage(b64)
				if len(raw) > 0 {
					data = append(data, raw)
					types = append(types, ct)
				}
			}
		}
	}
	return data, types
}

func decodeSignatureImage(b64 string) ([]byte, string) {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return nil, ""
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(data) == 0 {
		return nil, ""
	}
	ct := http.DetectContentType(data)
	if !strings.HasPrefix(ct, "image/") {
		return nil, ""
	}
	return data, ct
}

func textToHTML(s string) string {
	s = html.EscapeString(s)
	return strings.ReplaceAll(s, "\n", "<br>\n")
}

func newBoundary() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return "go-mail-" + hex.EncodeToString(buf[:])
}

// writeBase64 writes data as base64 wrapped at 76 columns, as MIME requires.
func writeBase64(b *strings.Builder, data []byte) {
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		b.WriteString(enc[:76])
		b.WriteString("\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc)
}

func messageID(email string) string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	domain := email
	if i := strings.LastIndex(email, "@"); i >= 0 {
		domain = email[i+1:]
	}
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), hex.EncodeToString(buf[:]), domain)
}

// collectMIMEParts walks a MIME reader, including nested multiparts, and
// collects the first plain-text part, the first HTML part and any inline
// images keyed by their Content-ID.
func collectMIMEParts(r *gomail.Reader, plain, htmlBody *string, images map[string]string) {
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		h, ok := p.Header.(*gomail.InlineHeader)
		if !ok {
			io.Copy(io.Discard, p.Body)
			continue
		}
		ct, _, _ := h.ContentType()
		if ct == "" {
			ct = "text/plain"
		}
		if strings.HasPrefix(ct, "multipart/") {
			data, _ := io.ReadAll(io.LimitReader(p.Body, 4<<20))
			nr, err := gomail.CreateReader(bytes.NewReader(data))
			if err == nil && nr != nil {
				collectMIMEParts(nr, plain, htmlBody, images)
				nr.Close()
			}
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(p.Body, 4<<20))
		switch {
		case strings.HasPrefix(ct, "text/plain") && strings.TrimSpace(*plain) == "":
			*plain = string(data)
		case strings.HasPrefix(ct, "text/html") && *htmlBody == "":
			*htmlBody = string(data)
		}
		if strings.HasPrefix(ct, "image/") {
			cid := strings.Trim(h.Get("Content-ID"), "<>")
			if cid != "" {
				images[cid] = "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
			}
		}
	}
}
func parseBody(raw []byte) (string, string) {
	r, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil || r == nil {
		body := string(raw)
		if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
			body = string(raw[i+4:])
		} else if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
			body = string(raw[i+2:])
		}
		return tidyText(body), ""
	}
	defer r.Close()
	var plain, htmlBody string
	images := map[string]string{}
	collectMIMEParts(r, &plain, &htmlBody, images)
	for cid, dataURI := range images {
		htmlBody = strings.ReplaceAll(htmlBody, "cid:"+cid, dataURI)
	}
	// Sanitize: drop scripts.
	htmlBody = scriptRE.ReplaceAllString(htmlBody, "")
	if strings.TrimSpace(plain) != "" {
		return tidyText(plain), htmlBody
	}
	if htmlBody != "" {
		return tidyText(htmlToText(htmlBody)), htmlBody
	}
	// Last resort: return the raw body text.
	body := string(raw)
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		body = string(raw[i+4:])
	} else if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
		body = string(raw[i+2:])
	}
	return tidyText(body), ""
}

var (
	tagRE    = regexp.MustCompile(`(?s)<[^>]*>`)
	blockRE  = regexp.MustCompile(`(?i)</?(p|div|br|tr|li|h[1-6]|blockquote|table)[^>]*>`)
	scriptRE = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	styleRE  = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	headRE   = regexp.MustCompile(`(?is)<head\b[^>]*>.*?</head>`)
	spaceRE  = regexp.MustCompile(`[ \t\x{00a0}]+`)
	blankRE  = regexp.MustCompile(`\n{3,}`)
)

func htmlToText(s string) string {
	s = scriptRE.ReplaceAllString(s, " ")
	s = styleRE.ReplaceAllString(s, " ")
	s = headRE.ReplaceAllString(s, " ")
	s = blockRE.ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return s
}

func tidyText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(spaceRE.ReplaceAllString(line, " "), " ")
	}
	s = strings.Join(lines, "\n")
	s = blankRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// displayName is a sender's name from an address when the envelope has none.
func displayName(addr string) string {
	if addr == "" {
		return "Unknown sender"
	}
	name := addr
	if i := strings.Index(addr, "@"); i > 0 {
		name = addr[:i]
	}
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return addr
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
