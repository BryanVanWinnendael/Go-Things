package app

import (
	"go-post/internal/core"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func suggestionTexts(items []core.Suggestion) []string {
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = s.Text
	}
	return out
}

func TestHeaderKeySuggestionsFollowTyping(t *testing.T) {
	got := suggestionTexts(core.HeaderSuggestions("key", "", "acc"))
	want := []string{"Accept", "Accept-Charset", "Accept-Encoding", "Accept-Language"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("suggestions for acc are %v", got)
	}
	// An exact match is offered before the names that only start with it.
	got = suggestionTexts(core.HeaderSuggestions("key", "", "accept"))
	if len(got) == 0 || got[0] != "Accept" {
		t.Fatalf("suggestions for accept are %v", got)
	}
	// A word within a name counts, as the encoding of Accept-Encoding.
	got = suggestionTexts(core.HeaderSuggestions("key", "", "encoding"))
	if strings.Join(got, ",") != "Accept-Encoding,Content-Encoding,Transfer-Encoding" {
		t.Fatalf("suggestions for encoding are %v", got)
	}
}

func TestHeaderValueSuggestionsFollowTheKey(t *testing.T) {
	got := suggestionTexts(core.HeaderSuggestions("value", "Accept", "t"))
	if len(got) < 2 || got[0] != "text/plain" || got[1] != "text/html" {
		t.Fatalf("suggestions for Accept/t are %v", got)
	}
	// The key is matched without regard to case.
	got = suggestionTexts(core.HeaderSuggestions("value", "content-type", "j"))
	if len(got) == 0 || got[0] != "application/json" {
		t.Fatalf("suggestions for Content-Type/j are %v", got)
	}
	// A header the catalog does not know stays quiet.
	if got := core.HeaderSuggestions("value", "X-Unknown", "t"); len(got) != 0 {
		t.Fatalf("an unknown header suggested %v", suggestionTexts(got))
	}
	// An empty query is not a Suggestion.
	if got := core.HeaderSuggestions("key", "", "   "); len(got) != 0 {
		t.Fatalf("an empty query suggested %v", suggestionTexts(got))
	}
}

func TestSuggestionMatchPointsAtWhatWasTyped(t *testing.T) {
	items := core.HeaderSuggestions("value", "Content-Type", "json")
	if len(items) == 0 {
		t.Fatal("json did not match a media type")
	}
	s := items[0]
	if s.Text != "application/json" {
		t.Fatalf("matched %q", s.Text)
	}
	if got := string([]rune(s.Text)[s.MatchStart:s.MatchEnd]); got != "json" {
		t.Fatalf("highlighted %q, want json", got)
	}
}

func TestSuggestionsAreCapped(t *testing.T) {
	got := core.HeaderSuggestions("key", "", "a")
	if len(got) > core.MaxHeaderSuggestions {
		t.Fatalf("%d suggestions, more than the cap", len(got))
	}
	if len(got) == 0 {
		t.Fatal("a matched nothing")
	}
}

// The whole header flow: type a key, take the Suggestion, land in the value,
// type a value, take its Suggestion.
func TestHeaderAutocompleteFillsKeyAndValue(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.Headers = []core.KV{{ID: "h_test", Key: "", Value: "", Enabled: true}}
	a.reqTab = 1

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Header key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("acc")
	tt.Frame()
	tt.Frame()

	if !a.headerSuggest.open {
		t.Fatal("typing acc did not open the suggestions")
	}
	if !tt.HasText("Accept") {
		t.Fatal("the popover does not offer Accept")
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()

	if r.Headers[0].Key != "Accept" {
		t.Fatalf("key after Enter is %q", r.Headers[0].Key)
	}
	if !tt.Focused("Value") {
		t.Fatal("accepting the key did not move the focus to the value")
	}

	tt.Type("t")
	tt.Frame()
	tt.Frame()
	if !a.headerSuggest.open || !tt.HasText("text/plain") {
		t.Fatalf("value suggestions are missing (open=%v)", a.headerSuggest.open)
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if r.Headers[0].Value != "text/plain" {
		t.Fatalf("value after Enter is %q", r.Headers[0].Value)
	}
}

// The arrows move the choice, and Enter takes the one that is highlighted.
func TestHeaderAutocompleteArrowKeysChoose(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.Headers = []core.KV{{ID: "h_test", Key: "", Value: "", Enabled: true}}
	a.reqTab = 1

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Header key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("acc")
	tt.Frame()
	tt.Frame()

	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.headerSuggest.index != 1 {
		t.Fatalf("Down chose index %d", a.headerSuggest.index)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if r.Headers[0].Key != "Accept-Charset" {
		t.Fatalf("Enter took %q, want Accept-Charset", r.Headers[0].Key)
	}
}

// Escape closes the popover without changing the text, and the next keystroke
// brings it back.
func TestHeaderAutocompleteEscapeClosesAndTypingReopens(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.Headers = []core.KV{{ID: "h_test", Key: "", Value: "", Enabled: true}}
	a.reqTab = 1

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Header key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("acc")
	tt.Frame()
	tt.Frame()
	if !a.headerSuggest.open {
		t.Fatal("the popover did not open")
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if a.headerSuggest.open {
		t.Fatal("Escape left the popover open")
	}
	if r.Headers[0].Key != "acc" {
		t.Fatalf("Escape changed the key to %q", r.Headers[0].Key)
	}

	tt.Type("e")
	tt.Frame()
	tt.Frame()
	if !a.headerSuggest.open {
		t.Fatal("typing after Escape did not reopen the popover")
	}
}

// A click on a Suggestion fills the field, the other way to accept one.
func TestHeaderAutocompleteClickFillsTheField(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.Headers = []core.KV{{ID: "h_test", Key: "", Value: "", Enabled: true}}
	a.reqTab = 1

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Header key"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("acc")
	tt.Frame()
	tt.Frame()

	if err := tt.Click("Accept"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if r.Headers[0].Key != "Accept" {
		t.Fatalf("clicking Accept filled %q", r.Headers[0].Key)
	}
}
