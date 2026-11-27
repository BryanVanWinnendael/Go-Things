package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

var iconAttachment = mustIcon(`<path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>`)

func (a *App) previewAttachment(m Message, att Attachment) {
	go func() {
		var sess *Session
		folder := m.Folder
		if strings.Contains(m.Folder, "@") {
			acct, ok := a.accountByEmail(m.Folder)
			if !ok {
				a.mu.Lock()
				a.errMsg = "Unknown account."
				a.mu.Unlock()
				a.invalidateAll()
				return
			}
			s, _, err := Open(acct)
			if err != nil {
				a.mu.Lock()
				a.errMsg = err.Error()
				a.mu.Unlock()
				a.invalidateAll()
				return
			}
			sess = s
			folder = "INBOX"
			defer sess.Close()
		} else {
			a.mu.Lock()
			sess = a.sess
			a.mu.Unlock()
			if folder == "" || folder == "SHARED" || folder == "FLAGGED" {
				folder = "INBOX"
			}
		}
		if sess == nil {
			a.mu.Lock()
			a.errMsg = "Not connected."
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		data, err := sess.Attachment(folder, m.UID, att)
		if err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		dir := os.TempDir()
		path := filepath.Join(dir, att.Name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		if err := mygo.Shell.OpenPath(path); err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
		}
		a.invalidateAll()
	}()
}

func (a *App) downloadAttachment(m Message, att Attachment) {
	go func() {
		var sess *Session
		folder := m.Folder
		if strings.Contains(m.Folder, "@") {
			acct, ok := a.accountByEmail(m.Folder)
			if !ok {
				a.mu.Lock()
				a.errMsg = "Unknown account."
				a.mu.Unlock()
				a.invalidateAll()
				return
			}
			s, _, err := Open(acct)
			if err != nil {
				a.mu.Lock()
				a.errMsg = err.Error()
				a.mu.Unlock()
				a.invalidateAll()
				return
			}
			sess = s
			folder = "INBOX"
			defer sess.Close()
		} else {
			a.mu.Lock()
			sess = a.sess
			a.mu.Unlock()
			if folder == "" || folder == "SHARED" || folder == "FLAGGED" {
				folder = "INBOX"
			}
		}
		if sess == nil {
			a.mu.Lock()
			a.errMsg = "Not connected."
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		data, err := sess.Attachment(folder, m.UID, att)
		if err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
			a.invalidateAll()
			return
		}
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{DefaultPath: att.Name})
		if err != nil || path == "" {
			a.invalidateAll()
			return
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			a.mu.Lock()
			a.errMsg = err.Error()
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.note = "Saved " + filepath.Base(path)
			a.mu.Unlock()
		}
		a.invalidateAll()
	}()
}

var iconChevronsUpDown = mustIcon(`<path d="m7 15 5 5 5-5"/><path d="m7 9 5-5 5 5"/>`)
