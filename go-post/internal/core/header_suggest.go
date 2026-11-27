package core

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// headerNames are the request headers Go Post offers as a key is typed. They
// are stored in their canonical casing, which is what the field takes when a
// Suggestion is accepted.
var headerNames = []string{
	"Accept",
	"Accept-Charset",
	"Accept-Encoding",
	"Accept-Language",
	"Authorization",
	"Cache-Control",
	"Connection",
	"Content-Disposition",
	"Content-Encoding",
	"Content-Language",
	"Content-Length",
	"Content-Type",
	"Cookie",
	"Date",
	"Expect",
	"Forwarded",
	"From",
	"Host",
	"If-Match",
	"If-Modified-Since",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
	"Max-Forwards",
	"Origin",
	"Pragma",
	"Proxy-Authorization",
	"Range",
	"Referer",
	"Referrer-Policy",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
	"User-Agent",
	"Vary",
	"Via",
	"Warning",
	"X-Api-Key",
	"X-Correlation-ID",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Real-IP",
	"X-Request-ID",
}

// headerValues are the values a known header usually takes, in the order they
// are offered: the first that matches is the one Enter takes. Values may use
// the environment variables the rest of the app uses.
var headerValues = map[string][]string{
	"accept": {
		"application/json", "text/plain", "text/html", "application/xml",
		"application/x-www-form-urlencoded", "multipart/form-data", "*/*",
	},
	"accept-charset":   {"utf-8", "iso-8859-1", "*"},
	"accept-encoding":  {"gzip", "deflate", "br", "identity", "gzip, deflate, br"},
	"accept-language":  {"en-US", "en-GB", "en", "nl", "de", "fr", "es", "*"},
	"authorization":    {"Bearer {{token}}", "Basic {{credentials}}"},
	"cache-control":    {"no-cache", "no-store", "max-age=0", "must-revalidate", "public", "private"},
	"connection":       {"keep-alive", "close"},
	"content-encoding": {"gzip", "deflate", "br", "identity"},
	"content-type": {
		"application/json", "application/x-www-form-urlencoded", "multipart/form-data",
		"text/plain", "text/html", "application/xml", "application/octet-stream",
	},
	"expect":              {"100-continue"},
	"if-match":            {"*"},
	"if-none-match":       {"*"},
	"origin":              {"*", "{{baseUrl}}"},
	"pragma":              {"no-cache"},
	"proxy-authorization": {"Bearer {{token}}", "Basic {{credentials}}"},
	"range":               {"bytes=0-1023", "bytes=0-"},
	"referer":             {"{{baseUrl}}"},
	"te":                  {"trailers"},
	"transfer-encoding":   {"chunked"},
	"user-agent":          {"Go Post/1.0", "Mozilla/5.0", "curl/8.0"},
	"vary":                {"Accept", "Accept-Encoding", "Origin"},
	"x-api-key":           {"{{apiKey}}"},
	"x-correlation-id":    {"{{requestId}}"},
	"x-request-id":        {"{{requestId}}"},
}

// MaxHeaderSuggestions keeps the popover a glance, not a list to read.
const MaxHeaderSuggestions = 6

// Suggestion is one completion and the run of it that matched the query, in
// runes, so the popover can color the part that was typed.
type Suggestion struct {
	Text       string
	MatchStart int
	MatchEnd   int
}

// HeaderSuggestions returns what to offer for a header field. A value is only
// offered for a key the catalog knows, so an unknown header stays quiet.
func HeaderSuggestions(field, key, query string) []Suggestion {
	if field == "key" {
		return MatchSuggestions(headerNames, query)
	}
	values, ok := headerValues[strings.ToLower(strings.TrimSpace(key))]
	if !ok {
		return nil
	}
	return MatchSuggestions(values, query)
}

// MatchSuggestions finds the catalog entries a query starts, or that start a
// word within them: "acc" finds Accept, and "plain" finds text/plain. An
// exact match comes first, then prefix matches in catalog order, which is the
// order of preference.
func MatchSuggestions(catalog []string, query string) []Suggestion {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []Suggestion
	for _, item := range catalog {
		if start, ok := suggestionMatch(item, q); ok {
			out = append(out, Suggestion{
				Text:       item,
				MatchStart: start,
				MatchEnd:   start + utf8.RuneCountInString(q),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		exactI := strings.EqualFold(out[i].Text, q)
		exactJ := strings.EqualFold(out[j].Text, q)
		if exactI != exactJ {
			return exactI
		}
		return out[i].MatchStart < out[j].MatchStart
	})
	if len(out) > MaxHeaderSuggestions {
		out = out[:MaxHeaderSuggestions]
	}
	return out
}

// suggestionMatch returns where query matches label: at its start, or after a
// separator such as the slash of a media type. Offsets are in runes.
func suggestionMatch(label, query string) (int, bool) {
	lower := strings.ToLower(label)
	if strings.HasPrefix(lower, query) {
		return 0, true
	}
	for i := 0; i < len(lower); i++ {
		switch lower[i] {
		case '/', '-', '_', ' ':
			if strings.HasPrefix(lower[i+1:], query) {
				return utf8.RuneCountInString(lower[:i+1]), true
			}
		}
	}
	return 0, false
}
