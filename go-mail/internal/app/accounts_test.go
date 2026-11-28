package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSwitcherLeadsToManager checks the account menu's footer opens the manager,
// matching Go Post's "Manage workspaces" action.
func TestSwitcherLeadsToManager(t *testing.T) {
	a := testApp()
	a.accounts = []Account{{Email: "alex@example.com"}, {Email: "sam@work.test"}}
	a.active = 0
	a.acct = a.accounts[0]
	a.connected = true
	a.accountOpen = true

	tt := ui.NewTester(a.view, 1320, 860)
	if err := tt.Click("Manage accounts"); err != nil {
		t.Fatalf("account menu lacks its manage action: %v", err)
	}
	if a.settingsTab != "accounts" || !a.pendingSettings {
		t.Fatalf("manage action did not open the accounts page: tab=%q pending=%v", a.settingsTab, a.pendingSettings)
	}
}

// TestSettingsAccountsRenders drives the manager page so a broken card or a
// missing icon is caught without opening the real window.
func TestSettingsAccountsRenders(t *testing.T) {
	a := testApp()
	a.settingsTab = "accounts"
	a.accounts = []Account{
		{Name: "Alex", Email: "alex@example.com", IMAPHost: "imap.example.com", Password: "secret"},
		{Email: "sam@work.test", IMAPHost: "mail.work.test"},
	}
	a.active = 0
	ui.NewTester(a.settingsWindowView, 920, 760).Frame()
	if got := a.settingsTitle(); got != "Accounts" {
		t.Fatalf("settings title is %q, want Accounts", got)
	}
}

// TestAccountConfirmAction checks the shared confirmation can show a sign-out
// action instead of always saying Delete.
func TestAccountConfirmAction(t *testing.T) {
	a := testApp()
	called := false
	a.openConfirmAction("Log out?", "The account stays in the list.", "Log out", "logout", func() { called = true })
	if a.confirmAction != "Log out" || a.confirmIcon != "logout" {
		t.Fatalf("confirm action is %q / %q", a.confirmAction, a.confirmIcon)
	}
	a.runConfirm()
	if !called {
		t.Fatal("confirm action did not run")
	}
	if a.confirmFn != nil || a.confirmAction != "" {
		t.Fatal("confirmation was not cleared after running")
	}
}

// TestLogoutAccountKeepsAccount checks that signing out forgets credentials and
// cached mail but leaves the account in the manager.
func TestLogoutAccountKeepsAccount(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_MAIL_CONFIG", dir+"/config.json")
	t.Setenv("GO_MAIL_CACHE", dir+"/cache.json")

	a := testApp()
	a.accounts = []Account{
		{Email: "alex@example.com", Password: "secret", IMAPHost: "imap.example.com"},
		{Email: "sam@work.test", Password: "secret2"},
	}
	a.active = 0
	a.acct = a.accounts[0]
	a.connected = true
	a.diskCache.Accounts = map[string]map[string][]Message{
		"alex@example.com": {"INBOX": {{UID: 1}}},
		"sam@work.test":    {"INBOX": {{UID: 2}}},
	}

	a.logoutAccount("alex@example.com")

	if len(a.accounts) != 2 {
		t.Fatalf("got %d accounts after logout, want 2", len(a.accounts))
	}
	if a.accounts[0].Password != "" {
		t.Fatal("logout left the password saved")
	}
	if a.accounts[0].Email != "alex@example.com" {
		t.Fatal("logout changed the account's address")
	}
	if a.connected || a.sess != nil {
		t.Fatal("logout left the mailbox connected")
	}
	if a.form.Email != "alex@example.com" || a.form.Password != "" {
		t.Fatalf("setup form is %+v, want the signed-out account with no password", a.form)
	}
	if a.diskCache.Accounts["alex@example.com"] != nil {
		t.Fatal("logout left the account's cached mail behind")
	}
	if a.diskCache.Accounts["sam@work.test"] == nil {
		t.Fatal("logout removed another account's cache")
	}
}

// TestDeleteAccountRemovesIt checks that deleting the active account removes
// it from the list and cache, and selects the next one.
func TestDeleteAccountRemovesIt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_MAIL_CONFIG", dir+"/config.json")
	t.Setenv("GO_MAIL_CACHE", dir+"/cache.json")

	a := testApp()
	a.accounts = []Account{
		{Email: "alex@example.com", Password: "secret"},
		{Email: "sam@work.test", Password: "secret2"},
	}
	a.active = 0
	a.acct = a.accounts[0]
	a.connected = true
	a.diskCache.Accounts = map[string]map[string][]Message{
		"alex@example.com": {"INBOX": {{UID: 1}}},
		"sam@work.test":    {"INBOX": {{UID: 2}}},
	}

	a.deleteAccount("alex@example.com")

	if len(a.accounts) != 1 || a.accounts[0].Email != "sam@work.test" {
		t.Fatalf("accounts after delete: %+v", a.accounts)
	}
	if a.active != 0 || a.form.Email != "sam@work.test" {
		t.Fatalf("active is %d, form is %+v; want the remaining account", a.active, a.form)
	}
	if a.connected || a.sess != nil {
		t.Fatal("deleting the active account left the old session connected")
	}
	if a.diskCache.Accounts["alex@example.com"] != nil {
		t.Fatal("delete left the account's cached mail behind")
	}
}
