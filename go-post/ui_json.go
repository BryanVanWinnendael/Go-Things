package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// maxHighlightedBody is the largest body Go Post colors. Past it the body
// already has thousands of tokens, and coloring every frame would cost more
// than it helps: the text still shows, just plain.
const maxHighlightedBody = 64 << 10

// jsonTokenKind is what a run of JSON is.
type jsonTokenKind uint8

const (
	jsonKey jsonTokenKind = iota
	jsonString
	jsonNumber
	jsonLiteral
	jsonPunct
)

// jsonToken is a colored run of a body. Offsets are in runes, which is what
// ui.TextRange counts.
type jsonToken struct {
	start, end int
	kind       jsonTokenKind
}

// jsonTokens finds the colored runs of JSON. It is a lexer, not a parser:
// text that is not valid JSON still colors the parts that look right, which
// is what an editor shows while a body is being typed.
func jsonTokens(s string) []jsonToken {
	r := []rune(s)
	var out []jsonToken
	for i := 0; i < len(r); {
		switch c := r[i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '"':
			start := i
			i++
			for i < len(r) {
				if r[i] == '\\' && i+1 < len(r) {
					i += 2
					continue
				}
				if r[i] == '"' {
					i++
					break
				}
				i++
			}
			kind := jsonString
			// A string followed by a colon is a key, not a value.
			if next := skipJSONSpace(r, i); next < len(r) && r[next] == ':' {
				kind = jsonKey
			}
			out = append(out, jsonToken{start: start, end: i, kind: kind})
		case c == '-' || (c >= '0' && c <= '9'):
			start := i
			for i < len(r) && isJSONNumberRune(r[i]) {
				i++
			}
			out = append(out, jsonToken{start: start, end: i, kind: jsonNumber})
		case c == 't' || c == 'f' || c == 'n':
			start := i
			for i < len(r) && r[i] >= 'a' && r[i] <= 'z' {
				i++
			}
			out = append(out, jsonToken{start: start, end: i, kind: jsonLiteral})
		case c == '{' || c == '}' || c == '[' || c == ']' || c == ',' || c == ':':
			out = append(out, jsonToken{start: i, end: i + 1, kind: jsonPunct})
			i++
		default:
			i++
		}
	}
	return out
}

func skipJSONSpace(r []rune, i int) int {
	for i < len(r) && (r[i] == ' ' || r[i] == '\t' || r[i] == '\n' || r[i] == '\r') {
		i++
	}
	return i
}

func isJSONNumberRune(c rune) bool {
	return c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'
}

// jsonColors are the colors of JSON's parts, chosen for the panel the text
// sits on. A custom theme can be dark with a light panel, so the theme's own
// dark flag is not enough to pick them.
type jsonColors struct {
	dark                      bool
	key, str, num, lit, punct ui.Color
}

func syntaxColors(p palette) jsonColors {
	if relativeLuminance(p.Field) < 0.45 {
		return jsonColors{
			dark:  true,
			key:   ui.Hex("#79c0ff"),
			str:   ui.Hex("#a5d6ff"),
			num:   ui.Hex("#d2a8ff"),
			lit:   ui.Hex("#ff7b72"),
			punct: ui.Hex("#8b949e"),
		}
	}
	return jsonColors{
		key:   ui.Hex("#0550ae"),
		str:   ui.Hex("#0a3069"),
		num:   ui.Hex("#8250df"),
		lit:   ui.Hex("#cf222e"),
		punct: ui.Hex("#6e7781"),
	}
}

func (c jsonColors) color(k jsonTokenKind) ui.Color {
	switch k {
	case jsonKey:
		return c.key
	case jsonString:
		return c.str
	case jsonNumber:
		return c.num
	case jsonLiteral:
		return c.lit
	}
	return c.punct
}

// jsonWeight makes keys stand out from the string values beside them.
func jsonWeight(k jsonTokenKind) int {
	if k == jsonKey {
		return 600
	}
	return 0
}

// jsonSpans returns the body as spans for a RichText, one per run. Every rune
// of the body is in a span, so the text drawn is exactly the response.
func jsonSpans(body string, cols jsonColors) []ui.Span {
	toks := jsonTokens(body)
	runes := []rune(body)
	spans := make([]ui.Span, 0, len(toks)*2+1)
	last := 0
	add := func(from, to int, col ui.Color, weight int) {
		if to <= from {
			return
		}
		spans = append(spans, ui.Span{Text: string(runes[from:to]), Color: col, Weight: weight})
	}
	for _, t := range toks {
		add(last, t.start, ui.Color{}, 0)
		add(t.start, t.end, cols.color(t.kind), jsonWeight(t.kind))
		last = t.end
	}
	add(last, len(runes), ui.Color{}, 0)
	return spans
}

// jsonRanges is jsonSpans for an editable text area, which colors runs of its
// text rather than taking spans.
func jsonRanges(body string, cols jsonColors) []ui.TextRange {
	toks := jsonTokens(body)
	out := make([]ui.TextRange, 0, len(toks))
	for _, t := range toks {
		out = append(out, ui.TextRange{
			Start: t.start, End: t.end,
			Color: cols.color(t.kind), Weight: jsonWeight(t.kind),
		})
	}
	return out
}

// prettyJSONBody indents a JSON body, keeping its key order, and says why it
// could not when the text is not JSON yet.
func prettyJSONBody(body string) (string, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", fmt.Errorf("the body is empty")
	}
	if !json.Valid([]byte(trimmed)) {
		return "", fmt.Errorf("the body is not valid JSON")
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// syntaxEntry is the colored form of one body, kept while the text and the
// panel behind it stay the same.
type syntaxEntry struct {
	body   string
	dark   bool
	spans  []ui.Span
	ranges []ui.TextRange
}

// bodyRanges colors the request body being typed, or returns nil when it is
// too large to color.
func (a *app) bodyRanges(id, body string, cols jsonColors) []ui.TextRange {
	if len(body) > maxHighlightedBody {
		return nil
	}
	if a.bodySyntax == nil {
		a.bodySyntax = map[string]*syntaxEntry{}
	}
	e := a.bodySyntax[id]
	if e == nil || e.body != body || e.dark != cols.dark {
		e = &syntaxEntry{body: body, dark: cols.dark, ranges: jsonRanges(body, cols)}
		a.bodySyntax[id] = e
	}
	return e.ranges
}

// responseSpans colors a JSON response body, or returns nil when it is too
// large to color.
func (a *app) responseSpans(id, body string, cols jsonColors) []ui.Span {
	if len(body) > maxHighlightedBody {
		return nil
	}
	if a.responseSyntax == nil {
		a.responseSyntax = map[string]*syntaxEntry{}
	}
	e := a.responseSyntax[id]
	if e == nil || e.body != body || e.dark != cols.dark {
		e = &syntaxEntry{body: body, dark: cols.dark, spans: jsonSpans(body, cols)}
		a.responseSyntax[id] = e
	}
	return e.spans
}
