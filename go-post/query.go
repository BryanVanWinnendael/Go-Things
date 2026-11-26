package main

import (
	"net/url"
	"strings"
)

// The URL bar and the Params tab are two views of one thing. This file keeps
// them in step: editing either side rewrites the other, so the URL always
// shows the enabled rows and the rows always show the URL's query.

// splitURLQuery separates a URL into its base, its query string and its
// fragment. The fragment is split first, because a "#" ends the query even when
// it contains a "?".
func splitURLQuery(raw string) (base, query, fragment string) {
	base = raw
	if i := strings.Index(base, "#"); i >= 0 {
		base, fragment = base[:i], base[i:]
	}
	if i := strings.Index(base, "?"); i >= 0 {
		base, query = base[:i], base[i+1:]
	}
	return base, query, fragment
}

// queryParams reads a query string into rows. Values are decoded so the tab
// shows what will be sent, including any {{variables}} a value was written
// with.
func queryParams(query string) []KV {
	if query == "" {
		return nil
	}
	var rows []KV
	for _, part := range strings.Split(query, "&") {
		if part == "" {
			continue
		}
		key, value := part, ""
		if i := strings.Index(part, "="); i >= 0 {
			key, value = part[:i], part[i+1:]
		}
		rows = append(rows, KV{
			ID:      newID("q"),
			Key:     unescapeQuery(key),
			Value:   unescapeQuery(value),
			Enabled: true,
		})
	}
	return rows
}

// escapeQuery keeps {{variables}} readable in the URL bar. They are the one
// thing Go's escaping would otherwise turn into "%7B%7B…%7D%7D", which would
// hide their color and their hover.
func escapeQuery(s string) string {
	s = url.QueryEscape(s)
	s = strings.ReplaceAll(s, "%7B", "{")
	s = strings.ReplaceAll(s, "%7D", "}")
	return s
}

func unescapeQuery(s string) string {
	if v, err := url.QueryUnescape(s); err == nil {
		return v
	}
	return s
}

// queryString renders the enabled rows as a query string.
func queryString(rows []KV) string {
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		if !r.Enabled || strings.TrimSpace(r.Key) == "" {
			continue
		}
		parts = append(parts, escapeQuery(r.Key)+"="+escapeQuery(r.Value))
	}
	return strings.Join(parts, "&")
}

// syncURLQuery writes the rows into the URL. Disabled rows stay in the tab but
// leave the URL, which is what gets sent.
func syncURLQuery(r *Request) {
	base, query, fragment := splitURLQuery(r.URL)
	next := queryString(r.Query)
	if next == query {
		return
	}
	if next != "" {
		base += "?" + next
	}
	r.URL = base + fragment
}

// syncQueryFromURL replaces the enabled rows with the URL's query. Disabled
// rows keep their place, so switching a row off does not make it jump.
func syncQueryFromURL(r *Request) {
	_, query, _ := splitURLQuery(r.URL)
	parsed := queryParams(query)
	prev := r.Query
	out := make([]KV, 0, len(prev)+len(parsed))
	p := 0
	for _, row := range prev {
		if !row.Enabled {
			out = append(out, row)
			continue
		}
		if p < len(parsed) {
			row.Key, row.Value = parsed[p].Key, parsed[p].Value
			out = append(out, row)
			p++
		}
	}
	for ; p < len(parsed); p++ {
		out = append(out, parsed[p])
	}
	if sameRows(out, prev) {
		return
	}
	r.Query = out
}

func sameRows(a, b []KV) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].Value != b[i].Value || a[i].Enabled != b[i].Enabled {
			return false
		}
	}
	return true
}

// absorbInlineQuery turns a URL's query into rows for data saved before the
// Params tab existed. It runs once, at load, and only when no rows are stored.
func absorbInlineQuery(r *Request) {
	if len(r.Query) > 0 {
		return
	}
	_, query, _ := splitURLQuery(r.URL)
	if query == "" {
		return
	}
	r.Query = queryParams(query)
}

// applyQueryParams builds the URL that is actually sent from the enabled rows.
// The URL's own copy of the query is dropped first so no parameter is sent
// twice; with no rows the URL is used exactly as written.
func applyQueryParams(rawURL string, rows []KV, vars map[string]string) string {
	values := url.Values{}
	enabled := 0
	for _, p := range rows {
		if !p.Enabled || strings.TrimSpace(p.Key) == "" {
			continue
		}
		values.Add(interpolate(p.Key, vars), interpolate(p.Value, vars))
		enabled++
	}
	if enabled == 0 {
		return rawURL
	}
	base, _, fragment := splitURLQuery(rawURL)
	return base + "?" + values.Encode() + fragment
}
