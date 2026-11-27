package app

import (
	"go-post/internal/core"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestThemedInputMenu(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	tt.SetClipboard("https://example.test/pasted")

	if err := tt.Click("Request URL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.RightClick("Request URL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()

	if native := tt.Menu(); native != nil {
		t.Fatalf("text input still opened the native menu: %v", native)
	}
	if !a.ctxMenu.open {
		t.Fatal("text input did not open the themed menu")
	}
	menu := menuLabels(a.ctxMenu.items)
	for _, item := range []string{"Cut", "Copy", "Paste", "Delete", "Select all"} {
		if !contains(menu, item) {
			t.Fatalf("input menu %v lacks %q", menu, item)
		}
	}

	if err := tt.Click("Paste"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r, _ := a.activeRequest()
	if r == nil || !strings.Contains(r.URL, "https://example.test/pasted") {
		t.Fatalf("paste did not reach the input: %q", r.URL)
	}
	if a.ctxMenu.open {
		t.Fatal("choosing an input menu item left the menu open")
	}
}

// Right-clicking the response body opens the app's themed menu, not MyGo's
// native one, and its Copy all puts the whole body on the clipboard.
func TestThemedResponseMenu(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	body := `{"name":"Go Post","ok":true}`
	a.responses[r.ID] = core.SendResult{StatusCode: 200, Status: "200 OK", Body: body, JSON: true}

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	rect, ok := tt.Find(body)
	if !ok {
		t.Fatal("response body was not drawn")
	}
	tt.RightClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	tt.Frame()
	if native := tt.Menu(); native != nil {
		t.Fatalf("response body still opened the native menu: %v", native)
	}
	if !a.ctxMenu.open {
		t.Fatal("response body did not open the themed menu")
	}
	menu := menuLabels(a.ctxMenu.items)
	for _, item := range []string{"Copy", "Copy all"} {
		if !contains(menu, item) {
			t.Fatalf("response menu %v lacks %q", menu, item)
		}
	}

	if err := tt.Click("Copy all"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got != body {
		t.Fatalf("clipboard has %q, want the response body", got)
	}
	if a.ctxMenu.open {
		t.Fatal("choosing a response menu item left the menu open")
	}
}

func TestSettingsAppearanceHasNoTextSizeOrCompact(t *testing.T) {
	a := testApp(t)
	stt := ui.NewTester(a.settingsWindowView, 920, 760)
	stt.Frame()
	stt.Frame()
	if _, ok := stt.Find("Text size"); ok {
		t.Fatal("settings still shows Text size")
	}
	if _, ok := stt.Find("Compact spacing"); ok {
		t.Fatal("settings still shows Compact spacing")
	}
}
