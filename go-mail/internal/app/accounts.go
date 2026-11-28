package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// openAccounts brings the settings window forward on the account manager.
func (a *App) openAccounts() {
	a.settingsTab = "accounts"
	a.pendingSettings = true
}

// accountName is the label shown on a manager card: the account's display name
// when it has one, otherwise the part before the @ in its address.
func accountName(acc Account) string {
	if name := strings.TrimSpace(acc.Name); name != "" {
		return name
	}
	if i := strings.Index(acc.Email, "@"); i > 0 {
		return acc.Email[:i]
	}
	return acc.Email
}

func firstLetter(s string) string {
	for _, r := range strings.TrimSpace(s) {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// accountServer is the small subtitle under an account's name.
func accountServer(acc Account) string {
	if host := strings.TrimSpace(acc.IMAPHost); host != "" {
		return host
	}
	if i := strings.LastIndex(acc.Email, "@"); i >= 0 {
		return acc.Email[i+1:]
	}
	return "No server configured"
}

// settingsAccounts lists every saved account with its state and actions. It is
// built like Go Post's workspace manager: one card per account, an active
// badge and border, and small Open / Log out / Delete actions on each row.
func (a *App) settingsAccounts(c *ui.Context) {
	p := a.palette
	ui.Text(c, "Each account keeps its own signature, theme, mailboxes and cached mail.").
		FontSize(10.8).TextColor(p.Muted).Wrap()

	for i := range a.accounts {
		acc := a.accounts[i]
		i, acc := i, acc
		active := i == a.active
		card := a.card(c).FillWidth().Key("account:" + acc.Email).
			Transition(ui.ElementTransition{
				Duration: 160 * time.Millisecond,
				Colors:   true,
				Enter:    &ui.Motion{Collapse: true, Opacity: 0},
				Exit:     &ui.Motion{Collapse: true, Opacity: 0},
			})
		if active {
			card.BorderColor(p.Accent.Alpha(0.28))
		}
		card.Children(func() {
			ui.Row(c).FillWidth().Height(74).Padding(0, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(38, 38).Radius(10).Background(p.Accent.Alpha(0.14)).Center().Children(func() {
					ui.Text(c, firstLetter(accountName(acc))).FontSize(15).FontWeight(750).TextColor(p.Accent)
				})
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, accountName(acc)).FontSize(13).FontWeight(650).SingleLine()
						if active {
							a.badge(c, "active", p.Good)
						}
					})
					ui.Text(c, fmt.Sprintf("%s · %s", acc.Email, accountServer(acc))).
						FontSize(9.8).TextColor(p.Muted).SingleLine()
				})
				if !active {
					if a.plainButton(c, "Open", "").Clicked() {
						a.closeSettings()
						go a.switchAccount(i)
					}
				}
				if a.iconButton(c, "logout", "Log out", 30).Clicked() {
					email, name := acc.Email, accountName(acc)
					a.openConfirmAction("Log out of "+name+"?",
						"“"+email+"” will be signed out and its cached mail removed. The account stays in this list.",
						"Log out", "logout", func() { a.logoutAccount(email) })
				}
				if a.iconButton(c, "trash", "Delete account", 30).Clicked() {
					email, name := acc.Email, accountName(acc)
					a.openConfirmAction("Delete account?",
						"“"+name+"” and its saved mail will be removed from this device.",
						"Delete", "trash", func() { a.deleteAccount(email) })
				}
			})
		})
	}

	add := ui.ButtonBase(c).Key("account:new").FillWidth().Height(66).Padding(0, 16).
		Radius(14).Gap(12).Background(p.Panel).Border(1, p.Border).Label("Add account")
	if add.Hovered() {
		add.Background(p.Hover)
	}
	add.Children(func() {
		ui.Box(c).Size(30, 30).Radius(9).Background(p.Accent.Alpha(0.14)).Center().Children(func() {
			ui.Icon(c, icon("plus", 14)).Size(14, 14).TextColor(p.Accent)
		})
		ui.Text(c, "Add account").FontSize(12.5).FontWeight(600)
	})
	if add.Clicked() {
		a.closeSettings()
		go a.startAddAccount()
	}
}

// forgetCacheLocked drops an account's cached mail. The active folder cache is
// keyed by folder rather than address, so it is only valid for the old account
// and is cleared too.
func (a *App) forgetCacheLocked(email string) {
	if a.diskCache.Accounts != nil {
		delete(a.diskCache.Accounts, email)
	}
	if len(a.cache) > 0 {
		a.cache = map[string]*folderCache{}
	}
}

// stopKeepAliveLocked ends the connection's background polling.
func (a *App) stopKeepAliveLocked() {
	a.keepAliveGen = 0
	if a.keepAliveStop != nil {
		close(a.keepAliveStop)
		a.keepAliveStop = nil
	}
}

// clearMailState resets everything that belongs to the signed-in mailbox.
func (a *App) clearMailState() {
	a.sess = nil
	a.connected = false
	a.connecting = false
	a.switching = false
	a.msgs = nil
	a.folders = nil
	a.sel = -1
	a.viewingMsg = nil
	a.resetSearch()
	a.body = ""
	a.bodyHTML = ""
	a.bodyUID = 0
	a.showCompose = false
	a.errMsg = ""
	a.setupErr = ""
	a.note = ""
	a.acct = Account{}
	a.folder = ""
}

// logoutAccount signs an account out without removing it. Its saved password
// and cached mail are dropped; if it is the active account the setup form comes
// back with the address filled in, ready for the password to be entered again.
func (a *App) logoutAccount(email string) {
	a.mu.Lock()
	idx := -1
	for i := range a.accounts {
		if a.accounts[i].Email == email {
			idx = i
			break
		}
	}
	if idx < 0 {
		a.mu.Unlock()
		return
	}
	a.accounts[idx].Password = ""
	acct := a.accounts[idx]
	active := idx == a.active
	a.forgetCacheLocked(email)
	if active {
		a.stopKeepAliveLocked()
	}
	a.mu.Unlock()

	var sess *Session
	if active {
		sess = a.sess
		a.clearMailState()
		a.form = acct
	}
	a.saveConfig()
	a.saveDiskCache()
	a.invalidateAll()
	if sess != nil {
		go func() {
			a.netMu.Lock()
			sess.Close()
			a.netMu.Unlock()
		}()
	}
}

// deleteAccount removes an account entirely, along with its cached mail. When
// it was the active account the next one is shown, or the setup form if none
// are left.
func (a *App) deleteAccount(email string) {
	a.mu.Lock()
	idx := -1
	for i := range a.accounts {
		if a.accounts[i].Email == email {
			idx = i
			break
		}
	}
	if idx < 0 {
		a.mu.Unlock()
		return
	}
	active := idx == a.active
	a.accounts = append(a.accounts[:idx], a.accounts[idx+1:]...)
	a.forgetCacheLocked(email)
	if active {
		a.stopKeepAliveLocked()
	}
	if a.active >= len(a.accounts) && len(a.accounts) > 0 {
		a.active = len(a.accounts) - 1
	}
	if a.active < 0 || a.active >= len(a.accounts) {
		a.active = -1
	}
	a.mu.Unlock()

	var sess *Session
	if active {
		sess = a.sess
		a.clearMailState()
		a.form = Account{IMAPPort: "993", SMTPPort: "587"}
		if a.active >= 0 {
			a.form = a.accounts[a.active]
		}
	}
	a.saveConfig()
	a.saveDiskCache()
	a.invalidateAll()
	if sess != nil {
		go func() {
			a.netMu.Lock()
			sess.Close()
			a.netMu.Unlock()
		}()
	}
}
