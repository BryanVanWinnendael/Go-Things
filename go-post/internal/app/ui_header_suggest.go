package app

import (
	"go-post/internal/core"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// headerSuggestDuration is the popover's entrance. It is short because the
// popover opens as the user types, a surface they look at many times in a
// minute: enough to read as arriving, not enough to wait for.
const headerSuggestDuration = 150 * time.Millisecond

// headerSuggestState is the autocomplete of one header field. Go Post edits
// one field at a time, so one state is enough; row and field say which.
type headerSuggestState struct {
	row   string
	field string
	query string
	index int
	open  bool
	// muted keeps the popover closed after Escape or a click away, until the
	// text changes again.
	muted bool
}

// headerFieldSuggestions returns the suggestions for a focused header field
// and keeps the shared state in step with what is typed.
func (a *app) headerFieldSuggestions(row *core.KV, field, query string) []core.Suggestion {
	if a.headerSuggest.row != row.ID || a.headerSuggest.field != field || a.headerSuggest.query != query {
		a.headerSuggest.row = row.ID
		a.headerSuggest.field = field
		a.headerSuggest.query = query
		a.headerSuggest.index = 0
		a.headerSuggest.muted = false
	}
	items := core.HeaderSuggestions(field, row.Key, query)
	if a.headerSuggest.index >= len(items) {
		a.headerSuggest.index = 0
	}
	a.headerSuggest.open = !a.headerSuggest.muted && len(items) > 0
	return items
}

// headerTextKey takes the keys that move or accept a Suggestion before the
// text input does. Every other key is left to the input.
func (a *app) headerTextKey(c *ui.Context, row *core.KV, field string) func(ui.TextKey) bool {
	return func(k ui.TextKey) bool {
		if k.Mods != 0 || !a.headerSuggest.open ||
			a.headerSuggest.row != row.ID || a.headerSuggest.field != field {
			return false
		}
		items := core.HeaderSuggestions(field, row.Key, a.headerSuggest.query)
		if len(items) == 0 {
			return false
		}
		switch k.Key {
		case ui.KeyDown:
			a.headerSuggest.index = (a.headerSuggest.index + 1) % len(items)
			c.Invalidate()
			return true
		case ui.KeyUp:
			a.headerSuggest.index = (a.headerSuggest.index - 1 + len(items)) % len(items)
			c.Invalidate()
			return true
		case ui.KeyEnter, ui.KeyTab:
			choice := items[min(a.headerSuggest.index, len(items)-1)].Text
			k.SetText(choice)
			a.acceptHeaderSuggestion(c, row, field, choice)
			return true
		case ui.KeyEscape:
			a.headerSuggest.muted = true
			a.headerSuggest.open = false
			c.Invalidate()
			return true
		}
		return false
	}
}

// acceptHeaderSuggestion writes a choice into the row and closes the popover.
// A key accepted into an empty row hands the focus to its value, which is the
// step that follows.
func (a *app) acceptHeaderSuggestion(c *ui.Context, row *core.KV, field, choice string) {
	if field == "key" {
		row.Key = choice
		if strings.TrimSpace(row.Value) == "" {
			a.headerFocus = row.ID
		}
	} else {
		row.Value = choice
	}
	a.headerSuggest.query = choice
	a.headerSuggest.index = 0
	a.headerSuggest.muted = true
	a.headerSuggest.open = false
	a.save()
	c.Invalidate()
}

// headerSuggestAnimKey is one field's entrance animation. The row and field
// keep the key and value fields from sharing a position.
func headerSuggestAnimKey(row *core.KV, field string) string {
	return "header-suggest:" + row.ID + ":" + field
}

// headerSuggestPopover draws the suggestions under a header field. anim is the
// field's entrance, driven every frame by the caller so that closing the
// popover puts it back at its start for the next open.
func (a *app) headerSuggestPopover(c *ui.Context, anchor ui.Element, row *core.KV, field string, items []core.Suggestion, anim float32) {
	if !a.headerSuggest.open || len(items) == 0 {
		return
	}
	p := a.palette
	panel := ui.PopoverBase(c, anchor, &a.headerSuggest.open, func(panel ui.Element) {
		a.menuPanel(panel, "header-suggest", 324)
		panel.AttachTo(anchor, ui.AnchorBottomStart, ui.AnchorTopStart)
		// A press on a Suggestion keeps the caret in the field it completes.
		panel.KeepFocus()
		panel.Opacity(anim).Translate(0, (1-anim)*-4)
		panel.Children(func() {
			for i := range items {
				item := items[i]
				selected := i == a.headerSuggest.index
				b := ui.ButtonBase(c).FillWidth().Height(32).Padding(0, 8).Radius(7).Gap(6).Label(item.Text)
				if selected {
					b.Background(p.Accent.Alpha(0.12))
				} else if b.Hovered() {
					b.Background(p.Hover)
				}
				b.Children(func() {
					a.suggestionText(c, item)
					ui.Spacer(c).Grow(1)
					if selected {
						ui.Text(c, "↵").FontSize(10).TextColor(p.Muted)
					}
				})
				if b.Clicked() {
					a.acceptHeaderSuggestion(c, row, field, item.Text)
				}
			}
		})
	})
	if panel.Dismissed() {
		a.headerSuggest.muted = true
		a.headerSuggest.open = false
	}
}

// suggestionText draws a completion as one continuous run of text, with the
// part that matched the query colored. Splitting it into separate text
// elements put the button's gap between the pieces, which read as a space
// inside a word ("Acc ept"); spans keep the word whole.
func (a *app) suggestionText(c *ui.Context, s core.Suggestion) {
	p := a.palette
	r := []rune(s.Text)
	var spans []ui.Span
	add := func(text string, color ui.Color, weight int) {
		if text != "" {
			spans = append(spans, ui.Span{Text: text, Color: color, Weight: weight})
		}
	}
	add(string(r[:s.MatchStart]), p.Text, 0)
	add(string(r[s.MatchStart:s.MatchEnd]), p.Accent, 650)
	add(string(r[s.MatchEnd:]), p.Text, 0)
	ui.RichText(c, spans...).FontSize(11.5).NoWrap()
}
