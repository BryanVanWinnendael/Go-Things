package app

import (
	"encoding/json"
	"go-post/internal/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Enabled query rows are appended to the URL when the request is sent, are
// interpolated from the active environment, and are encoded once.
func TestSendIncludesEnabledQueryParams(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st := &core.Store{}
	req := core.Request{
		Method: "GET", URL: srv.URL + "/search",
		Query: []core.KV{
			{ID: "q1", Key: "q", Value: "{{term}}", Enabled: true},
			{ID: "q2", Key: "limit", Value: "10", Enabled: true},
			{ID: "q3", Key: "off", Value: "x", Enabled: false},
			{ID: "q4", Key: "", Value: "ignored", Enabled: true},
		},
	}
	res := st.Send(req, map[string]string{"term": "a b&c"}, core.Settings{TimeoutMS: 5000}, nil)
	if res.Error != "" {
		t.Fatalf("send failed: %s", res.Error)
	}
	if !strings.Contains(gotURL, "/search?") {
		t.Fatalf("query string is missing: %q", gotURL)
	}
	if !strings.Contains(gotURL, "q=a+b%26c") || !strings.Contains(gotURL, "limit=10") {
		t.Fatalf("query is wrong: %q", gotURL)
	}
	if strings.Contains(gotURL, "off=") {
		t.Fatalf("disabled parameter was sent: %q", gotURL)
	}
}

func TestQueryParamsSurviveImportAndExport(t *testing.T) {
	src := `{"info":{"name":"Q"},"item":[{"name":"Search","request":{
		"method":"GET","url":{"raw":"https://api.test/search?q=hello&page=2","query":[
			{"key":"q","value":"hello"},{"key":"page","value":"2"},{"key":"off","value":"x","disabled":true}
		]}
	}}]}`
	x, format, err := parseImport([]byte(src))
	if err != nil || format != "postman" {
		t.Fatalf("parse: %q, %v", format, err)
	}
	r := x.Collections[0].Requests[0]
	if len(r.Query) != 3 || r.Query[2].Enabled {
		t.Fatalf("query is %+v", r.Query)
	}

	a := testApp(t)
	a.applyImport(x)
	got := a.ws().Collections[len(a.ws().Collections)-1].Requests[0]
	if got.URL != "https://api.test/search?q=hello&page=2" {
		t.Fatalf("URL is %q", got.URL)
	}
	if len(got.Query) != 3 || got.Query[0].Value != "hello" {
		t.Fatalf("request query is %+v", got.Query)
	}

	pm, err := postmanCollection(a.data)
	if err != nil {
		t.Fatal(err)
	}
	var root struct {
		Item []json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(pm, &root); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range root.Item {
		var m map[string]any
		if json.Unmarshal(item, &m) != nil {
			continue
		}
		for _, child := range jarr(m["item"]) {
			req := jmap(jmap(child)["request"])
			query := jarr(jmap(req["url"])["query"])
			if len(query) > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("Postman export lost the query parameters")
	}
}

func TestParamsTabShowsEditableRows(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.Query = []core.KV{{ID: "q1", Key: "page", Value: "1", Enabled: true}}
	a.reqTab = 0

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Add parameter"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(r.Query) != 2 {
		t.Fatalf("now have %d parameters, want 2", len(r.Query))
	}
	if _, ok := tt.Find("Parameter key"); !ok {
		t.Fatal("parameter key field is not shown")
	}
}

// The URL and the Params tab are one value: an edit on either side lands on
// the other, a disabled row leaves the URL without leaving the tab, and a
// fragment survives the rewrite.
func TestURLAndParamsStayInSync(t *testing.T) {
	r := &core.Request{URL: "https://api.test/search"}
	r.URL = "https://api.test/search?q=a%20b&page=2#top"
	core.SyncQueryFromURL(r)
	if len(r.Query) != 2 || r.Query[0].Value != "a b" || r.Query[1].Key != "page" {
		t.Fatalf("rows from the URL are %+v", r.Query)
	}

	r.Query[1].Value = "3"
	core.SyncURLQuery(r)
	if r.URL != "https://api.test/search?q=a+b&page=3#top" {
		t.Fatalf("URL is %q", r.URL)
	}

	r.Query[0].Enabled = false
	core.SyncURLQuery(r)
	if r.URL != "https://api.test/search?page=3#top" {
		t.Fatalf("disabled row is still in the URL: %q", r.URL)
	}
	if len(r.Query) != 2 || r.Query[0].Enabled {
		t.Fatalf("disabled row left the tab: %+v", r.Query)
	}

	// Editing the URL keeps disabled rows where they are.
	r.URL = "https://api.test/search?page=9"
	core.SyncQueryFromURL(r)
	if len(r.Query) != 2 || r.Query[0].Enabled || r.Query[1].Value != "9" {
		t.Fatalf("rows after a URL edit are %+v", r.Query)
	}

	r.Query = r.Query[1:]
	core.SyncURLQuery(r)
	if r.URL != "https://api.test/search?page=9" {
		t.Fatalf("URL after removing a row is %q", r.URL)
	}

	// A URL with no query clears the enabled rows but keeps disabled ones.
	r.URL = "https://api.test/search"
	core.SyncQueryFromURL(r)
	if len(r.Query) != 0 {
		t.Fatalf("rows after clearing the URL are %+v", r.Query)
	}
}

func TestQuerySyncKeepsVariablesReadable(t *testing.T) {
	r := &core.Request{
		URL:   "https://api.test/items",
		Query: []core.KV{{ID: "q1", Key: "q", Value: "{{term}}", Enabled: true}},
	}
	core.SyncURLQuery(r)
	if r.URL != "https://api.test/items?q={{term}}" {
		t.Fatalf("URL is %q", r.URL)
	}
	if tokens := variableTokens(r.URL, map[string]string{"term": "x"}, nil); len(tokens) != 1 || tokens[0].name != "term" {
		t.Fatalf("tokens in the URL are %+v", tokens)
	}
}

func TestSendDoesNotSendAParamTwice(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req := core.Request{
		Method: "GET", URL: srv.URL + "/search?q=one",
		Query: []core.KV{{ID: "q1", Key: "q", Value: "one", Enabled: true}},
	}
	res := (&core.Store{}).Send(req, nil, core.Settings{TimeoutMS: 5000}, nil)
	if res.Error != "" {
		t.Fatalf("send failed: %s", res.Error)
	}
	if got != "q=one" {
		t.Fatalf("sent query is %q", got)
	}
}

// Typing in the URL bar fills the Params tab before the tab is even clicked.
func TestTypingInURLFillsParamsTab(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.URL = ""
	r.Query = nil
	a.reqTab = 0

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Request URL"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("https://api.test/search?q=hello&page=2")
	tt.Frame()

	if len(r.Query) != 2 || r.Query[0].Value != "hello" || r.Query[1].Value != "2" {
		t.Fatalf("rows after typing a URL are %+v", r.Query)
	}
	if _, ok := tt.Find("Parameter value"); !ok {
		t.Fatal("the Params tab did not show the new rows")
	}
}

// Typing in the Params tab rewrites the URL bar.
func TestTypingInParamsRewritesURL(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.URL = "https://api.test/search"
	r.Query = []core.KV{{ID: "q1", Key: "q", Value: "", Enabled: true}}
	a.reqTab = 0

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if err := tt.Click("Parameter value"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("hello")
	tt.Frame()

	if r.URL != "https://api.test/search?q=hello" {
		t.Fatalf("URL after typing a value is %q", r.URL)
	}
}

// Data saved before the Params tab keeps its query; loading splits it without
// changing what the URL bar shows.
func TestLoadAbsorbsInlineQuery(t *testing.T) {
	r := &core.Request{URL: "https://api.test/search?q=hello&off=x"}
	core.AbsorbInlineQuery(r)
	if r.URL != "https://api.test/search?q=hello&off=x" {
		t.Fatalf("the URL changed to %q", r.URL)
	}
	if len(r.Query) != 2 || r.Query[0].Value != "hello" {
		t.Fatalf("rows are %+v", r.Query)
	}
	// Rows already stored win; nothing is split twice.
	core.SyncQueryFromURL(r)
	if len(r.Query) != 2 {
		t.Fatalf("rows after a sync are %+v", r.Query)
	}
}
