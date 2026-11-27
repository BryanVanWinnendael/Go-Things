package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestEnvironmentDropdownAndSearchPlacement(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	search, ok := tt.Find("Command")
	if !ok {
		t.Fatal("search field not found")
	}
	if search.Y < 700 {
		t.Fatalf("search field is at y=%v, want it at the bottom of the sidebar", search.Y)
	}
	if _, ok := tt.Find("ENVIRONMENT"); ok {
		t.Fatal("sidebar still shows its environment switcher")
	}

	if err := tt.Click("Environment"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(180 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Manage environments") {
		t.Fatal("environment dropdown lacks its manage action")
	}
	if err := tt.Click("Local"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ws().ActiveEnv != "env_local" {
		t.Fatalf("active environment is %q, want env_local", a.ws().ActiveEnv)
	}
}

func TestSearchOpensWithControlSpace(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	tt.Key(ui.Cmd, ui.KeySpace)
	tt.Frame()
	if a.modal != "palette" {
		t.Fatalf("Ctrl+Space opened modal %q, want the command palette", a.modal)
	}

	a.modal = ""
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if a.modal == "palette" {
		t.Fatal("Ctrl+K still opens the command palette")
	}
}

func TestControlSTogglesSidebar(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	tt.Key(ui.Cmd, ui.KeyS)
	tt.Frame()
	if !a.sidebarCollapsed {
		t.Fatal("Ctrl+S did not collapse the sidebar")
	}
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("Settings"); ok {
		t.Fatal("collapsed sidebar still renders its Settings button")
	}

	tt.Key(ui.Cmd, ui.KeyS)
	tt.Frame()
	if a.sidebarCollapsed {
		t.Fatal("Ctrl+S did not expand the sidebar again")
	}
	tt.Advance(250 * time.Millisecond)
	tt.Frame()
	if _, ok := tt.Find("Settings"); !ok {
		t.Fatal("expanded sidebar does not render its Settings button")
	}
}

func TestWorkspaceDropdownLeadsToWorkspacePage(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Switch workspace"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Advance(180 * time.Millisecond)
	tt.Frame()
	if !tt.HasText("Manage workspaces") {
		t.Fatal("workspace dropdown lacks its manage action")
	}
	if _, ok := tt.Find("Acme API"); !ok {
		t.Fatal("workspace dropdown does not list the other workspace")
	}
	if err := tt.Click("Manage workspaces"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.page != "workspaces" {
		t.Fatalf("page is %q, want workspaces", a.page)
	}
	if _, ok := tt.Find("New workspace"); !ok {
		t.Fatal("workspaces page lacks its create action")
	}
	if _, ok := tt.Find("Each workspace keeps its own collections, environments, open tabs and history."); !ok {
		t.Fatal("workspaces page did not render")
	}
	if _, ok := tt.Find("Environment"); !ok {
		t.Fatal("top bar environment switcher is missing from the workspaces page")
	}
}
