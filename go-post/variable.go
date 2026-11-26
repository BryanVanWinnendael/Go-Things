package main

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// variableToken is one {{name}} in a field. Offsets are in runes, which is
// what ui.TextRange and TextSelection count.
type variableToken struct {
	start, end int
	name       string
	known      bool
	value      string
	secret     bool
}

// activeVarInfo returns the values of the active environment's variables and
// which of them are secret, so a token can say whether it resolves and what
// it resolves to.
func (a *app) activeVarInfo() (map[string]string, map[string]bool) {
	values := map[string]string{}
	secrets := map[string]bool{}
	w := a.ws()
	if w == nil {
		return values, secrets
	}
	if e := w.environment(w.ActiveEnv); e != nil {
		for _, v := range e.Vars {
			if v.Enabled && v.Key != "" {
				values[v.Key] = v.Value
				secrets[v.Key] = v.Secret
			}
		}
	}
	return values, secrets
}

// variableTokens finds {{name}} and {{ name }} in a field. An opening brace
// without a closing one is left as typed; the rest of the text still parses.
func variableTokens(s string, values map[string]string, secrets map[string]bool) []variableToken {
	r := []rune(s)
	var out []variableToken
	for i := 0; i < len(r); {
		if r[i] != '{' || i+1 >= len(r) || r[i+1] != '{' {
			i++
			continue
		}
		end := i + 2
		for end < len(r) && r[end] != '}' {
			end++
		}
		if end >= len(r) || end+1 >= len(r) || r[end+1] != '}' {
			i += 2
			continue
		}
		name := strings.TrimSpace(string(r[i+2 : end]))
		if name != "" {
			value, known := values[name]
			out = append(out, variableToken{
				start: i, end: end + 2, name: name,
				known: known, value: value, secret: secrets[name],
			})
		}
		i = end + 2
	}
	return out
}

// variableNames are the names available to complete while a {{ is typed.
func (a *app) variableNames() []string {
	values, _ := a.activeVarInfo()
	out := make([]string, 0, len(values))
	for name := range values {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// variableQuery finds the {{...}} being typed at the caret. The returned
// range is what accepting a completion replaces: it reaches to the caret, or
// past a closing }} already typed. A closed reference before the caret is not
// a query, so a finished {{name}} does not keep suggesting.
func variableQuery(text string, caret int) (start, end int, query string, ok bool) {
	r := []rune(text)
	if caret < 0 || caret > len(r) {
		return 0, 0, "", false
	}
	open := -1
	for i := caret - 2; i >= 0; i-- {
		if r[i] == '{' && r[i+1] == '{' {
			open = i
			break
		}
	}
	if open < 0 {
		return 0, 0, "", false
	}
	for i := open + 2; i < caret; i++ {
		if r[i] == '}' && i+1 < len(r) && r[i+1] == '}' {
			return 0, 0, "", false
		}
	}
	start = open
	end = caret
	if caret+1 < len(r) && r[caret] == '}' && r[caret+1] == '}' {
		end = caret + 2
	}
	return start, end, strings.TrimSpace(string(r[open+2 : caret])), true
}

// variableRanges colors known variables with the accent and unresolved ones
// with the error color, so a typo is visible before the request is sent.
func variableRanges(tokens []variableToken, p palette) []ui.TextRange {
	out := make([]ui.TextRange, 0, len(tokens))
	for _, t := range tokens {
		col := p.Accent
		if !t.known {
			col = p.Bad
		}
		out = append(out, ui.TextRange{Start: t.start, End: t.end, Color: col, Weight: 600})
	}
	return out
}

// variableHits is where each token sits in a field's text, measured with the
// field's own font so hit testing matches what is drawn.
type variableHits struct {
	starts, ends []float32
}

func measureVariableHits(c *ui.Context, text string, tokens []variableToken, font string, size float32) variableHits {
	r := []rune(text)
	h := variableHits{
		starts: make([]float32, len(tokens)),
		ends:   make([]float32, len(tokens)),
	}
	for i, t := range tokens {
		h.starts[i], _ = c.MeasureText(0, ui.Span{Text: string(r[:t.start]), Font: font, Size: size})
		h.ends[i], _ = c.MeasureText(0, ui.Span{Text: string(r[:t.end]), Font: font, Size: size})
	}
	return h
}

// indexAt returns the token under x, or -1 for plain text. pad is the space
// before the field's first glyph: its border and padding.
func (h variableHits) indexAt(x, pad float32) int {
	x -= pad
	for i := range h.starts {
		if x >= h.starts[i] && x <= h.ends[i] {
			return i
		}
	}
	return -1
}
