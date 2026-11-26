package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestVariableTokensFindEveryReference(t *testing.T) {
	values := map[string]string{"baseUrl": "https://api.test", "id": "42"}
	secrets := map[string]bool{"id": true}
	s := "{{baseUrl}}/users/{{ id }}/{{missing}} and {{unclosed"
	toks := variableTokens(s, values, secrets)
	if len(toks) != 3 {
		t.Fatalf("found %d tokens, want 3", len(toks))
	}
	want := []struct {
		name       string
		start, end int
		known      bool
		secret     bool
	}{
		{"baseUrl", 0, 11, true, false},
		{"id", 18, 26, true, true},
		{"missing", 27, 38, false, false},
	}
	for i, w := range want {
		got := toks[i]
		if got.name != w.name || got.start != w.start || got.end != w.end ||
			got.known != w.known || got.secret != w.secret {
			t.Fatalf("token %d is %+v, want %+v", i, got, w)
		}
	}
}

func TestVariableRangesColorKnownAndUnknownDifferently(t *testing.T) {
	p := themeByID("nocturne")
	values := map[string]string{"baseUrl": "https://api.test"}
	toks := variableTokens("{{baseUrl}}/{{nope}}", values, map[string]bool{})
	ranges := variableRanges(toks, p)
	if len(ranges) != 2 {
		t.Fatalf("got %d ranges", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 11 || ranges[0].Color != p.Accent {
		t.Fatalf("known range is %+v", ranges[0])
	}
	if ranges[1].Start != 12 || ranges[1].End != 20 || ranges[1].Color != p.Bad {
		t.Fatalf("unknown range is %+v", ranges[1])
	}
}

func TestVariableHitsFindTokenUnderPointer(t *testing.T) {
	h := variableHits{starts: []float32{10, 50}, ends: []float32{30, 70}}
	if got := h.indexAt(15, 5); got != 0 {
		t.Fatalf("pointer in the first token gave %d", got)
	}
	if got := h.indexAt(55, 5); got != 1 {
		t.Fatalf("pointer in the second token gave %d", got)
	}
	if got := h.indexAt(7, 5); got != -1 {
		t.Fatalf("pointer before the first token gave %d", got)
	}
	if got := h.indexAt(40, 5); got != -1 {
		t.Fatalf("pointer between tokens gave %d", got)
	}
}

func TestVariableQueryFindsTheReferenceBeingTyped(t *testing.T) {
	if _, _, _, ok := variableQuery("{{base}}", 8); ok {
		t.Fatal("a finished reference should not be a query")
	}
	start, end, query, ok := variableQuery("{{base", 6)
	if !ok || start != 0 || end != 6 || query != "base" {
		t.Fatalf("query is (%d, %d, %q, %v)", start, end, query, ok)
	}
	start, end, query, ok = variableQuery("x {{ base ", 10)
	if !ok || start != 2 || end != 10 || query != "base" {
		t.Fatalf("spaced query is (%d, %d, %q, %v)", start, end, query, ok)
	}
	// A closing brace already typed is part of what a completion replaces.
	start, end, query, ok = variableQuery("{{base}}", 6)
	if !ok || start != 0 || end != 8 || query != "base" {
		t.Fatalf("query before braces is (%d, %d, %q, %v)", start, end, query, ok)
	}
}

func TestSuggestCaretOffsetFollowsTheTypedToken(t *testing.T) {
	var x1, y1, x2, y2 float32
	tt := ui.NewTester(func(c *ui.Context) {
		x1, y1 = suggestCaretOffset(c, ui.Element{}, "abc{{", 3, "monospace", 12)
		x2, y2 = suggestCaretOffset(c, ui.Element{}, "abc\ndef{{", 7, "monospace", 12)
	}, 480, 320)
	tt.Frame()
	if x1 <= 0 || y1 <= 0 {
		t.Fatalf("first-line caret is (%v, %v)", x1, y1)
	}
	if x2 <= 0 {
		t.Fatalf("second-line caret x is %v", x2)
	}
	if y2 <= y1 {
		t.Fatalf("caret on the second line is y=%v, not below y=%v", y2, y1)
	}
}

func TestOpenVariableSwitchesEnvironmentAndMarksRow(t *testing.T) {
	a := testApp(t)
	w := a.ws()
	w.ActiveEnv = "env_local" // token lives in env_dev only
	a.openVariable(nil, "token")
	if w.ActiveEnv != "env_dev" {
		t.Fatalf("active environment is %q", w.ActiveEnv)
	}
	if a.page != "environments" {
		t.Fatalf("page is %q", a.page)
	}
	if a.envReveal != "token" {
		t.Fatalf("revealed variable is %q", a.envReveal)
	}
}

// Typing inside {{ offers the variables of the active environment, and Enter
// completes the reference with its closing braces.
func TestVariableSuggestionCompletesTypedName(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.URL = ""

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Request URL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("{{base")
	tt.Frame()
	tt.Frame()

	if !a.varSuggest.open {
		t.Fatal("typing {{base did not open suggestions")
	}
	if !tt.HasText("baseUrl") {
		t.Fatal("baseUrl was not suggested")
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if r.URL != "{{baseUrl}}" {
		t.Fatalf("URL after Enter is %q", r.URL)
	}
	if a.varSuggest.open {
		t.Fatal("suggestions stayed open after the completion")
	}
}

// The arrow keys move the choice, and Escape closes without changing the text.
func TestVariableSuggestionKeys(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.URL = ""

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Request URL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("{{")
	tt.Frame()
	tt.Frame()
	if !a.varSuggest.open {
		t.Fatal("an empty {{ did not suggest any variable")
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.varSuggest.open {
		t.Fatal("Escape left the suggestions open")
	}
	if r.URL != "{{" {
		t.Fatalf("Escape changed the URL to %q", r.URL)
	}

	tt.Type("b")
	tt.Frame()
	tt.Frame()
	if !a.varSuggest.open {
		t.Fatal("typing after Escape did not reopen the suggestions")
	}
	if r.URL != "{{b" {
		t.Fatalf("URL after typing is %q", r.URL)
	}
}

// Hovering a variable in the URL bar shows its value; a plain click leaves
// the caret alone and the shortcut plus click lands in the environment manager.
func TestVariableHoverAndClickInRequest(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.URL = "{{baseUrl}}"

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	rect, ok := tt.Find("Request URL")
	if !ok {
		t.Fatal("no URL field")
	}
	// The text starts after the border and padding; the token is the whole
	// short URL, so a point a little in is inside it.
	x := rect.X + 11 + 30
	y := rect.Y + rect.H/2
	tt.Move(x, y)
	tt.Frame()
	tt.Frame()

	if a.varHover == nil || a.varHover.name != "baseUrl" {
		t.Fatalf("hovered variable is %+v", a.varHover)
	}
	if !tt.HasText("https://httpbin.org") {
		t.Fatal("the tooltip does not show the variable's value")
	}

	// A plain click only places the caret; it must not navigate.
	tt.ClickAt(x, y)
	tt.Frame()
	tt.Frame()
	if a.page != "request" {
		t.Fatalf("a plain click navigated to %q", a.page)
	}

	// The platform's shortcut plus click opens the variable.
	tt.ClickAtWith(ui.Cmd, x, y)
	tt.Frame()
	tt.Frame()
	if a.page != "environments" {
		t.Fatalf("Ctrl+click the variable went to %q", a.page)
	}
	if a.envReveal != "baseUrl" {
		t.Fatalf("revealed variable is %q", a.envReveal)
	}
}
