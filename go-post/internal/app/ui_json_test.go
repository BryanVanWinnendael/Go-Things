package app

import (
	"go-post/internal/core"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// colored returns the text of each token of the kind, so a test reads what the
// highlighter found rather than offsets.
func colored(body string, kind jsonTokenKind) []string {
	runes := []rune(body)
	var out []string
	for _, t := range jsonTokens(body) {
		if t.kind == kind {
			out = append(out, string(runes[t.start:t.end]))
		}
	}
	return out
}

func TestJSONTokensSeparateKeysFromStrings(t *testing.T) {
	body := `{"name": "Go Post", "count": 12, "ok": true, "none": null, "list": ["a", "b"]}`
	if got := colored(body, jsonKey); strings.Join(got, ",") != `"name","count","ok","none","list"` {
		t.Fatalf("keys are %v", got)
	}
	if got := colored(body, jsonString); strings.Join(got, ",") != `"Go Post","a","b"` {
		t.Fatalf("strings are %v", got)
	}
	if got := colored(body, jsonNumber); strings.Join(got, ",") != "12" {
		t.Fatalf("numbers are %v", got)
	}
	if got := colored(body, jsonLiteral); strings.Join(got, ",") != "true,null" {
		t.Fatalf("literals are %v", got)
	}
}

// Escaped quotes must not end a string early, or the rest of the body would be
// colored as if it were still inside one.
func TestJSONTokensSurviveEscapes(t *testing.T) {
	body := `{"quote": "say \"hi\"", "after": 1}`
	if got := colored(body, jsonString); strings.Join(got, ",") != `"say \"hi\""` {
		t.Fatalf("strings are %v", got)
	}
	if got := colored(body, jsonKey); strings.Join(got, ",") != `"quote","after"` {
		t.Fatalf("keys are %v", got)
	}
}

// The response pane draws the spans as the body, so they have to add up to it
// exactly, whitespace and all.
func TestJSONSpansCoverTheWholeBody(t *testing.T) {
	body := "{\n  \"name\": \"Go Post\",\n  \"nested\": {\"a\": [1, 2.5, -3e2]},\n  \"ok\": false\n}"
	var got strings.Builder
	for _, s := range jsonSpans(body, syntaxColors(themeByID("nocturne"))) {
		got.WriteString(s.Text)
	}
	if got.String() != body {
		t.Fatalf("spans build\n%q\nwant\n%q", got.String(), body)
	}
}

// The range colors follow the panel, not the theme's dark flag: a custom dark
// theme can have a light panel behind the body.
func TestSyntaxColorsFollowThePanelNotTheDarkFlag(t *testing.T) {
	dark := themeByID("nocturne")
	light := themeByID("daylight")
	if !syntaxColors(dark).dark {
		t.Fatal("dark panel picked the light colors")
	}
	if syntaxColors(light).dark {
		t.Fatal("light panel picked the dark colors")
	}
	// A dark base with a white workspace panel: the panel wins.
	custom := customPalette(core.CustomTheme{Base: "nocturne", Surface: "#ffffff"})
	if syntaxColors(custom).dark {
		t.Fatal("light panel of a dark theme picked the dark colors")
	}
}

func TestPrettyJSONBodyFormatsAndKeepsKeyOrder(t *testing.T) {
	pretty, err := prettyJSONBody(`{"z":1,"a":[true,null]}`)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"z\": 1,\n  \"a\": [\n    true,\n    null\n  ]\n}"
	if pretty != want {
		t.Fatalf("pretty body is\n%s\nwant\n%s", pretty, want)
	}
	if _, err := prettyJSONBody(`{"a":`); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	if _, err := prettyJSONBody("   "); err == nil {
		t.Fatal("an empty body was accepted")
	}
}

// DisplayBody flags the response as JSON only when it is, since that flag is
// what the pane uses to color it.
func TestDisplayBodyFlagsJSON(t *testing.T) {
	body, isJSON := core.DisplayBody([]byte(`{"a":1}`), "application/json")
	if !isJSON {
		t.Fatal("JSON body was not flagged")
	}
	if body != "{\n  \"a\": 1\n}" {
		t.Fatalf("body is %q", body)
	}
	if _, isJSON := core.DisplayBody([]byte("plain text"), "text/plain"); isJSON {
		t.Fatal("plain text was flagged as JSON")
	}
}

// The Pretty button formats what is typed in the body, and the body keeps its
// key order. An invalid body leaves the text alone.
func TestPrettyButtonFormatsTheBody(t *testing.T) {
	a := testApp(t)
	a.reqTab = 2
	r, _ := a.activeRequest()
	r.BodyType = "json"
	r.Body = `{"z":1,"a":2}`

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Pretty"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if r.Body != "{\n  \"z\": 1,\n  \"a\": 2\n}" {
		t.Fatalf("body after Pretty is %q", r.Body)
	}

	r.Body = `{"broken":`
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Pretty"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if r.Body != `{"broken":` {
		t.Fatalf("invalid body was changed to %q", r.Body)
	}
}

// The body's runs stay inside the text and are reused while it does not
// change, so a frame that only repaints does not tokenize again.
func TestBodyRangesCoverTheBodyAndAreCached(t *testing.T) {
	a := testApp(t)
	cols := syntaxColors(a.palette)
	body := `{"a": 1}`
	ranges := a.bodyRanges("req_x", body, cols)
	if len(ranges) == 0 {
		t.Fatal("JSON body has no ranges")
	}
	for _, r := range ranges {
		if r.Start < 0 || r.End > len([]rune(body)) || r.End <= r.Start {
			t.Fatalf("range %d-%d is outside the body", r.Start, r.End)
		}
	}
	// The same body is not tokenized twice.
	if again := a.bodyRanges("req_x", body, cols); &again[0] != &ranges[0] {
		t.Fatal("body was tokenized again although nothing changed")
	}
	// A body past the cap stays plain rather than making a huge range list.
	big := strings.Repeat(" ", maxHighlightedBody) + body
	if got := a.bodyRanges("req_big", big, cols); got != nil {
		t.Fatalf("oversized body got %d ranges", len(got))
	}
}

// A JSON response is drawn from colored spans, so its text is still exactly
// the body and nothing is dropped on the way to the screen.
func TestResponsePaneShowsJSONBody(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	body, isJSON := core.DisplayBody([]byte(`{"name":"Go Post","ok":true}`), "application/json")
	if !isJSON {
		t.Fatal("test body was not flagged as JSON")
	}
	a.responses[r.ID] = core.SendResult{StatusCode: 200, Status: "200 OK", Body: body, JSON: true}

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	for _, want := range []string{"Go Post", `"name"`, "true"} {
		if !tt.HasText(want) {
			t.Fatalf("response pane does not show %q", want)
		}
	}
	if spans := a.responseSpans(r.ID, body, syntaxColors(a.palette)); len(spans) < 5 {
		t.Fatalf("JSON response has %d spans, too few to be colored", len(spans))
	}
}
