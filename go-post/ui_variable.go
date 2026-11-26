package main

import (
	"strings"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

// varHoverInfo is the variable under the pointer, kept between frames because
// pointer events arrive before the next build.
type varHoverInfo struct {
	field  string
	name   string
	value  string
	known  bool
	secret bool
	x      float32
}

// variableSuggestState is the {{name}} being completed in one field.
type variableSuggestState struct {
	field      string
	start, end int // rune range a completion replaces
	query      string
	index      int
	open       bool
	muted      bool
}

// variableField makes the {{variables}} of a field alive: they are colored,
// completed as they are typed, and can be opened with the platform's shortcut
// (Ctrl, or Cmd on macOS) plus click. Plain typing and clicks are untouched.
// fallback is the field's own OnTextKey, which runs for keys the completion
// does not take.
func (a *app) variableField(c *ui.Context, el ui.Element, value *string, font string, size float32, field string, fallback func(ui.TextKey) bool, interactive bool) ui.Element {
	values, secrets := a.activeVarInfo()
	tokens := variableTokens(*value, values, secrets)
	if len(tokens) > 0 {
		el = el.TextRanges(variableRanges(tokens, a.palette)...)
	}

	items := a.updateVariableSuggestions(el, *value, field)
	el = el.OnTextKey(a.variableTextKey(c, field, fallback))
	if a.varCaretField == field {
		el.SetTextSelection(a.varCaretPos, a.varCaretPos)
		a.varCaretField = ""
	}
	a.variableSuggestPopover(c, el, value, font, size, field, items)

	if interactive {
		hits := measureVariableHits(c, *value, tokens, font, size)
		// The text starts after the input's border and horizontal padding.
		pad := c.Theme().Space(2.5) + 1
		el = el.HandleInput(func(ev ui.InputEvent) bool {
			switch ev.Kind {
			case ui.InputPointerMove:
				i := hits.indexAt(ev.X, pad)
				if i < 0 {
					if a.varHover != nil && a.varHover.field == field {
						a.varHover = nil
						c.Invalidate()
					}
					return false
				}
				t := tokens[i]
				if a.varHover == nil || a.varHover.field != field || a.varHover.name != t.name {
					a.varHover = &varHoverInfo{
						field: field, name: t.name, value: t.value,
						known: t.known, secret: t.secret, x: hits.starts[i],
					}
					c.Invalidate()
				}
				return false
			case ui.InputPointerDown:
				// A plain click places the caret as usual; only the platform's
				// shortcut opens the variable's definition.
				if ev.Button != 0 || ev.Mods&ui.Cmd == 0 {
					return false
				}
				i := hits.indexAt(ev.X, pad)
				if i < 0 {
					return false
				}
				if a.varHover != nil {
					a.varHover = nil
				}
				a.openVariable(c, tokens[i].name)
				return true
			}
			return false
		})
		a.variableTooltip(c, el, field)
	}
	return el
}

// variableSuggestions matches names for a query. Just after an opening {{ the
// query is empty, and every available variable is offered.
func (a *app) variableSuggestions(query string) []suggestion {
	if strings.TrimSpace(query) == "" {
		names := a.variableNames()
		if len(names) > maxHeaderSuggestions {
			names = names[:maxHeaderSuggestions]
		}
		out := make([]suggestion, 0, len(names))
		for _, name := range names {
			out = append(out, suggestion{Text: name})
		}
		return out
	}
	return matchSuggestions(a.variableNames(), query)
}

// updateVariableSuggestions reads the caret and keeps the shared state in
// step with the {{ being typed. Only the focused field completes.
func (a *app) updateVariableSuggestions(el ui.Element, text, field string) []suggestion {
	if !el.Focused() {
		if a.varSuggest.field == field {
			a.varSuggest = variableSuggestState{}
		}
		return nil
	}
	start, caret := el.TextSelection()
	if start != caret {
		if a.varSuggest.field == field {
			a.varSuggest = variableSuggestState{}
		}
		return nil
	}
	s, e, query, ok := variableQuery(text, start)
	if !ok {
		if a.varSuggest.field == field {
			a.varSuggest = variableSuggestState{}
		}
		return nil
	}
	if a.varSuggest.field != field || a.varSuggest.query != query || a.varSuggest.start != s {
		a.varSuggest = variableSuggestState{field: field, start: s, end: e, query: query}
	}
	items := a.variableSuggestions(query)
	if a.varSuggest.index >= len(items) {
		a.varSuggest.index = 0
	}
	a.varSuggest.open = !a.varSuggest.muted && len(items) > 0
	return items
}

// variableTextKey takes the keys that move or accept a completion before the
// input does, then hands the rest to the field's own handler.
func (a *app) variableTextKey(c *ui.Context, field string, fallback func(ui.TextKey) bool) func(ui.TextKey) bool {
	return func(k ui.TextKey) bool {
		if a.varSuggest.open && a.varSuggest.field == field {
			items := matchSuggestions(a.variableNames(), a.varSuggest.query)
			if len(items) > 0 {
				switch k.Key {
				case ui.KeyDown:
					a.varSuggest.index = (a.varSuggest.index + 1) % len(items)
					c.Invalidate()
					return true
				case ui.KeyUp:
					a.varSuggest.index = (a.varSuggest.index - 1 + len(items)) % len(items)
					c.Invalidate()
					return true
				case ui.KeyEnter, ui.KeyTab:
					choice := items[min(a.varSuggest.index, len(items)-1)].Text
					k.SetText(a.acceptVariableSuggestion(field, choice, k.Text()))
					c.Invalidate()
					return true
				case ui.KeyEscape:
					a.varSuggest.open = false
					a.varSuggest.muted = true
					c.Invalidate()
					return true
				}
			}
		}
		if fallback != nil {
			return fallback(k)
		}
		return false
	}
}

// acceptVariableSuggestion replaces the partial reference with the chosen
// name and leaves the caret just after its closing braces.
func (a *app) acceptVariableSuggestion(field, name, text string) string {
	s := a.varSuggest
	r := []rune(text)
	end := min(s.end, len(r))
	inserted := "{{" + name + "}}"
	a.varCaretField = field
	a.varCaretPos = s.start + len([]rune(inserted))
	a.varSuggest = variableSuggestState{field: field, muted: true}
	return string(r[:s.start]) + inserted + string(r[end:])
}

// suggestCaretOffset returns where the {{ being typed sits inside the field,
// relative to the field's top-left. Shaping the prefix with the field's own
// font and width accounts for wrapped lines, so a completion in a tall,
// scrolling request body lands under the token instead of at the field's
// bottom edge.
func suggestCaretOffset(c *ui.Context, el ui.Element, text string, start int, font string, size float32) (float32, float32) {
	pad := c.Theme().Space(2.5) + 1
	r := []rune(text)
	if start > len(r) {
		start = len(r)
	}
	prefix := string(r[:start])
	// Leave room for the input's border, padding and scrollbar gutter so the
	// shaped line breaks match the control's.
	width := el.Bounds().W - pad*2 - 12
	if width < 40 {
		width = 0
	}
	layout := ui.ShapeText(prefix, ui.Font{Family: font, Size: size}, width)
	idx := 0
	for _, ch := range prefix {
		idx += utf16.RuneLen(ch)
	}
	caret := layout.CaretAt(ui.TextCaretPosition{Index: idx})
	x := pad + caret.X
	y := pad + caret.Y
	// A textarea keeps its caret visible while scrolling, so clamp a document
	// position that runs past the bottom to the last fully visible line.
	if h := el.Bounds().H; h > 0 {
		y = min(y, h-caret.H)
	}
	return x, max(y, 0)
}

// variableSuggestPopover draws the completions under the {{ being typed. anim
// is driven every frame by the caller so a closed popover is at the start of
// its entrance the next time it opens.
func (a *app) variableSuggestPopover(c *ui.Context, anchor ui.Element, value *string, font string, size float32, field string, items []suggestion) {
	target := float32(0)
	if a.varSuggest.open {
		target = 1
	}
	anim := anchor.Animate("variable-suggest:"+field, target, headerSuggestDuration)
	if !a.varSuggest.open || len(items) == 0 {
		return
	}
	p := a.palette
	panel := ui.PopoverBase(c, anchor, &a.varSuggest.open, func(panel ui.Element) {
		a.menuPanel(panel, "variable-suggest", 280)
		panel.AttachTo(anchor, ui.AnchorBottomStart, ui.AnchorTopStart)
		// Offset the popover from the field's bottom-left to the {{ token,
		// including its line, so a multi-line body stays anchored to what the
		// user is typing.
		x, y := suggestCaretOffset(c, anchor, *value, a.varSuggest.start, font, size)
		panel.Margin(y, 0, 0, 0).MarginStart(x)
		panel.KeepFocus()
		panel.Opacity(anim).Translate(0, (1-anim)*-4)
		panel.Children(func() {
			for i := range items {
				item := items[i]
				selected := i == a.varSuggest.index
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
					*value = a.acceptVariableSuggestion(field, item.Text, *value)
				}
			}
		})
	})
	if panel.Dismissed() {
		a.varSuggest.muted = true
		a.varSuggest.open = false
	}
}

// variableTooltip shows the hovered variable's value in a themed card just
// under the token, and names the shortcut that opens it.
func (a *app) variableTooltip(c *ui.Context, anchor ui.Element, field string) {
	hover := a.varHover
	open := hover != nil && hover.field == field
	if open && !anchor.Hovered() {
		a.varHover = nil
		hover = nil
		open = false
	}
	target := float32(0)
	if open {
		target = 1
	}
	anim := anchor.Animate("variable-tip:"+field, target, 110*time.Millisecond)
	if !open {
		return
	}
	p := a.palette
	panel := ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		a.menuPanel(panel, "variable-tip", 252)
		panel.AttachTo(anchor, ui.AnchorBottomStart, ui.AnchorTopStart)
		panel.MarginStart(min(hover.x, 180))
		panel.Opacity(anim).Translate(0, (1-anim)*-3)
		panel.Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					a.dot(c, func() ui.Color {
						if hover.known {
							return p.Good
						}
						return p.Bad
					}(), 6)
					ui.Text(c, hover.name).FontSize(11).FontWeight(650)
				})
				if hover.known {
					shown := hover.value
					if hover.secret {
						shown = "••••••  (secret value)"
					}
					ui.Text(c, shown).FontSize(10.5).TextColor(func() ui.Color {
						if hover.secret {
							return p.Muted
						}
						return p.Text
					}()).SingleLine()
				} else {
					ui.Text(c, "Not defined in this workspace").FontSize(10.5).TextColor(p.Muted)
				}
				ui.Text(c, modLabel()+"+click to open").FontSize(9.5).TextColor(p.Muted)
			})
		})
	})
	if panel.Dismissed() {
		a.varHover = nil
	}
}

// openVariable follows a variable to its definition: the environment that
// holds it becomes active, its row is marked, and the page changes. An
// unknown variable still lands on the manager, where it can be added.
func (a *app) openVariable(c *ui.Context, name string) {
	w := a.ws()
	if w == nil {
		return
	}
	envID := w.ActiveEnv
	found := false
	if e := w.environment(envID); e != nil {
		for _, v := range e.Vars {
			if v.Key == name {
				found = true
				break
			}
		}
	}
	if !found {
		for _, e := range w.Environments {
			for _, v := range e.Vars {
				if v.Key == name {
					envID, found = e.ID, true
					break
				}
			}
			if found {
				break
			}
		}
	}
	if found && envID != w.ActiveEnv {
		w.ActiveEnv = envID
		a.save()
	}
	a.envReveal = name
	a.envRevealDone = false
	a.setPage("environments")
	if !found {
		a.toast(c, "Variable not defined", "Add “"+name+"” in the environment manager.", "info")
	}
}
