package app

import (
	"encoding/json"
	"go-post/internal/core"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

const postmanFixture = `{
  "info": {"name": "Docs API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
  "item": [
    {"name": "Users", "item": [
      {"name": "List users", "request": {
        "method": "GET",
        "header": [{"key": "Accept", "value": "application/json"}],
        "url": {"raw": "{{baseUrl}}/users"}
      }},
      {"name": "Create user", "request": {
        "method": "POST",
        "header": [{"key": "Content-Type", "value": "application/json"}, {"key": "X-Off", "value": "no", "disabled": true}],
        "body": {"mode": "raw", "raw": "{\"name\":\"Ada\"}", "options": {"raw": {"language": "json"}}},
        "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "secret", "type": "string"}]},
        "url": "{{baseUrl}}/users"
      }}
    ]},
    {"name": "Ping", "request": {"method": "GET", "url": "https://example.test/ping"}}
  ],
  "variable": [{"key": "baseUrl", "value": "https://api.test"}]
}`

const yaakFixture = `{
  "yaakSchema": 4,
  "resources": {
    "workspaces": [{"id": "wk_1", "name": "Yaak API"}],
    "folders": [{"id": "fl_1", "workspaceId": "wk_1", "folderId": null, "name": "Orders"}],
    "httpRequests": [
      {"id": "rq_1", "workspaceId": "wk_1", "folderId": null, "name": "Health", "method": "GET", "url": "https://api.test/health", "headers": [{"name": "Accept", "value": "text/plain", "enabled": true}]},
      {"id": "rq_2", "workspaceId": "wk_1", "folderId": "fl_1", "name": "Place order", "method": "POST", "url": "https://api.test/orders", "bodyType": "application/json", "body": {"text": "{\"sku\":1}"}, "authenticationType": "bearer", "authentication": {"token": "abc"}}
    ],
    "environments": [{"id": "ev_1", "name": "Staging", "variables": [{"name": "baseUrl", "value": "https://stage.test", "enabled": true}, {"name": "off", "value": "x", "enabled": false}]}]
  }
}`

const insomniaFixture = `[
  {"_type": "export", "__export_format": 4},
  {"_type": "workspace", "_id": "wrk_1", "name": "Insomnia API"},
  {"_type": "request_group", "_id": "fld_1", "parentId": "wrk_1", "name": "Auth"},
  {"_type": "request", "_id": "req_1", "parentId": "fld_1", "name": "Login", "method": "POST", "url": "https://api.test/login", "headers": [{"name": "Accept", "value": "application/json"}], "body": {"mimeType": "application/json", "text": "{\"ok\":true}"}, "authentication": {"type": "bearer", "token": "insomnia-token"}},
  {"_type": "environment", "_id": "env_1", "name": "Local", "data": {"baseUrl": "https://local.test"}}
]`

func TestImportFormatDetection(t *testing.T) {
	for _, tc := range []struct{ data, want string }{
		{postmanFixture, "postman"},
		{`{"name":"Env","values":[{"key":"a","value":"b"}]}`, "postman-env"},
		{yaakFixture, "yaak"},
		{insomniaFixture, "insomnia"},
		{`{"info":{"name":"H"},"requests":[{"name":"R","method":"GET","endpoint":"https://x.test"}]}`, "hoppscotch"},
		{`curl 'https://api.test/x' -H 'Accept: text/plain'`, "curl"},
		{`not data`, ""},
	} {
		if got := importFormat([]byte(tc.data)); got != tc.want {
			t.Errorf("format of %.30q is %q, want %q", tc.data, got, tc.want)
		}
	}
}

func TestImportPostmanKeepsFoldersHeadersAndAuth(t *testing.T) {
	x, format, err := parseImport([]byte(postmanFixture))
	if err != nil || format != "postman" {
		t.Fatalf("parse: %q, %v", format, err)
	}
	if len(x.Collections) != 2 || x.Collections[1].Parent != 0 {
		t.Fatalf("collections are %+v", x.Collections)
	}
	if len(x.Collections[1].Requests) != 2 {
		t.Fatalf("folder has %d requests", len(x.Collections[1].Requests))
	}
	create := x.Collections[1].Requests[1]
	if create.BodyType != "json" || create.Body != `{"name":"Ada"}` {
		t.Fatalf("body is %q / %q", create.BodyType, create.Body)
	}
	if create.AuthType != "bearer" || create.Bearer != "secret" {
		t.Fatalf("auth is %+v", create)
	}
	if len(create.Headers) != 2 || create.Headers[1].Enabled {
		t.Fatalf("headers are %+v", create.Headers)
	}

	a := testApp(t)
	w := a.ws()
	before := len(w.Collections[0].Requests)
	cols, reqs, envs := a.applyImport(x)
	if cols != 2 || reqs != 3 || envs != 1 {
		t.Fatalf("imported %d/%d/%d", cols, reqs, envs)
	}
	if len(w.Collections) != 3 {
		t.Fatalf("workspace now has %d collections", len(w.Collections))
	}
	children := 0
	for _, c := range w.Collections {
		if c.ParentID != "" {
			children++
		}
	}
	if children != 1 {
		t.Fatalf("found %d nested collections", children)
	}
	if got := len(w.Collections[0].Requests); got != before {
		t.Fatalf("existing collection changed from %d requests to %d", before, got)
	}
	if len(w.Environments[len(w.Environments)-1].Vars) != 1 {
		t.Fatal("collection variables were not imported as an environment")
	}
}

func TestImportYaakAndInsomnia(t *testing.T) {
	a := testApp(t)
	x, format, err := parseImport([]byte(yaakFixture))
	if err != nil || format != "yaak" {
		t.Fatalf("yaak parse: %q, %v", format, err)
	}
	if len(x.Collections) != 2 || len(x.Collections[1].Requests) != 1 {
		t.Fatalf("yaak collections are %+v", x.Collections)
	}
	order := x.Collections[1].Requests[0]
	if order.BodyType != "json" || order.AuthType != "bearer" || order.Bearer != "abc" {
		t.Fatalf("yaak request is %+v", order)
	}
	if len(x.Environments) != 1 || len(x.Environments[0].Vars) != 2 || x.Environments[0].Vars[1].Enabled {
		t.Fatalf("yaak environment is %+v", x.Environments)
	}
	cols, reqs, envs := a.applyImport(x)
	if cols != 2 || reqs != 2 || envs != 1 {
		t.Fatalf("yaak imported %d/%d/%d", cols, reqs, envs)
	}

	x, format, err = parseImport([]byte(insomniaFixture))
	if err != nil || format != "insomnia" {
		t.Fatalf("insomnia parse: %q, %v", format, err)
	}
	if len(x.Collections) != 2 || len(x.Collections[1].Requests) != 1 {
		t.Fatalf("insomnia collections are %+v", x.Collections)
	}
	login := x.Collections[1].Requests[0]
	if login.BodyType != "json" || login.Bearer != "insomnia-token" {
		t.Fatalf("insomnia request is %+v", login)
	}
	if len(x.Environments) != 1 || x.Environments[0].Vars[0].Key != "baseUrl" {
		t.Fatalf("insomnia environment is %+v", x.Environments)
	}
	cols, reqs, envs = a.applyImport(x)
	if cols != 2 || reqs != 1 || envs != 1 {
		t.Fatalf("insomnia imported %d/%d/%d", cols, reqs, envs)
	}
}

func TestImportYaakKeepsNestedFolders(t *testing.T) {
	// Yaak exports resources newest first, so children arrive before their
	// parents. Both the nesting and the sidebar order must survive.
	const src = `{
	  "yaakSchema": 4,
	  "resources": {
	    "workspaces": [{"id": "wk_1", "name": "Yaak API"}],
	    "folders": [
	      {"id": "fl_deep", "workspaceId": "wk_1", "folderId": "fl_child", "name": "Deep", "sortPriority": 2000, "createdAt": "2024-03-01T10:00:00"},
	      {"id": "fl_zed", "workspaceId": "wk_1", "folderId": "fl_child", "name": "Zed", "sortPriority": 500, "createdAt": "2024-02-15T10:00:00"},
	      {"id": "fl_child", "workspaceId": "wk_1", "folderId": "fl_root", "name": "Child", "sortPriority": 2000, "createdAt": "2024-02-01T10:00:00"},
	      {"id": "fl_second", "workspaceId": "wk_1", "folderId": null, "name": "Second root", "sortPriority": 3000, "createdAt": "2024-01-20T10:00:00"},
	      {"id": "fl_root", "workspaceId": "wk_1", "folderId": null, "name": "Root", "sortPriority": 1000, "createdAt": "2024-01-01T10:00:00"}
	    ],
	    "httpRequests": [
	      {"id": "rq_late", "workspaceId": "wk_1", "folderId": "fl_deep", "name": "Late", "method": "GET", "url": "https://api.test/late", "sortPriority": 200},
	      {"id": "rq_early", "workspaceId": "wk_1", "folderId": "fl_deep", "name": "Early", "method": "GET", "url": "https://api.test/early", "sortPriority": 100},
	      {"id": "rq_root", "workspaceId": "wk_1", "folderId": null, "name": "Root request", "method": "GET", "url": "https://api.test/root"}
	    ]
	  }
	}`
	x, format, err := parseImport([]byte(src))
	if err != nil || format != "yaak" {
		t.Fatalf("parse: %q, %v", format, err)
	}
	index := func(name string) int {
		for i := range x.Collections {
			if x.Collections[i].Name == name {
				return i
			}
		}
		t.Fatalf("collection %q is missing from %+v", name, x.Collections)
		return -1
	}
	workspace := index("Yaak API")
	if x.Collections[index("Root")].Parent != workspace ||
		x.Collections[index("Child")].Parent != index("Root") ||
		x.Collections[index("Deep")].Parent != index("Child") {
		t.Fatalf("nested parents are %+v", x.Collections)
	}
	if x.Collections[index("Second root")].Parent != workspace {
		t.Fatalf("second root is nested: %+v", x.Collections[index("Second root")])
	}
	if index("Zed") > index("Deep") || index("Root") > index("Second root") {
		t.Fatalf("sibling order is wrong: %+v", x.Collections)
	}
	deep := x.Collections[index("Deep")].Requests
	if len(deep) != 2 || deep[0].Name != "Early" || deep[1].Name != "Late" {
		t.Fatalf("request order inside the folder is %+v", deep)
	}

	a := testApp(t)
	a.applyImport(x)
	w := a.ws()
	find := func(name string) *core.Collection {
		for i := range w.Collections {
			if w.Collections[i].Name == name {
				return &w.Collections[i]
			}
		}
		t.Fatalf("imported collection %q is missing", name)
		return nil
	}
	root, child, deepCol := find("Root"), find("Child"), find("Deep")
	imported := find("Yaak API")
	if root.ParentID != imported.ID || child.ParentID != root.ID || deepCol.ParentID != child.ID {
		t.Fatalf("imported tree is %+v", w.Collections)
	}
	if find("Second root").ParentID != imported.ID {
		t.Fatal("the second root folder was nested")
	}
	if len(deepCol.Requests) != 2 || deepCol.Requests[0].Name != "Early" {
		t.Fatalf("imported requests are %+v", deepCol.Requests)
	}
}

func TestImportCurlCommands(t *testing.T) {
	src := `curl 'https://api.test/users' -H 'Accept: application/json'
curl -X POST 'https://api.test/users' -H 'Content-Type: application/json' -d '{"name":"Ada"}'`
	x, format, err := parseImport([]byte(src))
	if err != nil || format != "curl" {
		t.Fatalf("curl parse: %q, %v", format, err)
	}
	if len(x.Collections) != 1 || len(x.Collections[0].Requests) != 2 {
		t.Fatalf("curl collections are %+v", x.Collections)
	}
	post := x.Collections[0].Requests[1]
	if post.Method != "POST" || post.BodyType != "json" || len(post.Headers) != 1 {
		t.Fatalf("curl request is %+v", post)
	}
}

func TestExportFormatsRoundTrip(t *testing.T) {
	a := testApp(t)
	r, _ := a.activeRequest()
	r.AuthType = "bearer"
	r.Bearer = "token"

	backup, err := backupJSON(a.data)
	if err != nil {
		t.Fatal(err)
	}
	x, format, err := parseImport(backup)
	if err != nil || format != "gopost" || len(x.Collections) != 0 {
		t.Fatalf("backup format %q: %v", format, err)
	}
	var b backupFile
	if err := json.Unmarshal(backup, &b); err != nil {
		t.Fatal(err)
	}
	if b.Format != "go-post" || b.Data.ActiveWorkspace != a.data.ActiveWorkspace {
		t.Fatalf("backup header is %+v", b)
	}

	pm, err := postmanCollection(a.data)
	if err != nil {
		t.Fatal(err)
	}
	x, format, err = parseImport(pm)
	if err != nil || format != "postman" {
		t.Fatalf("postman export format %q: %v", format, err)
	}
	reqs := 0
	for _, c := range x.Collections {
		reqs += len(c.Requests)
	}
	if reqs == 0 {
		t.Fatal("postman export has no requests")
	}
	found := false
	for _, c := range x.Collections {
		for _, req := range c.Requests {
			if req.URL == r.URL && req.AuthType == "bearer" && req.Bearer == "token" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("postman export lost the request's bearer token")
	}
}

func TestDataSettingsPageShowsImportAndExport(t *testing.T) {
	a := testApp(t)
	a.settingsTab = "data"
	tt := ui.NewTester(a.settingsWindowView, 920, 900)
	tt.Frame()
	tt.Frame()
	for _, label := range []string{"Import file…", "Go Post backup", "Postman collection"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("Data page is missing %q", label)
		}
	}
	if !tt.HasText("Postman, Yaak, Insomnia, Hoppscotch, cURL") {
		t.Fatal("Data page does not name the supported clients")
	}
}

func TestImportSummaryNamesTheClient(t *testing.T) {
	got := importSummary("postman", 2, 5, 1)
	if !strings.Contains(got, "Postman collection") || !strings.Contains(got, "5 requests") {
		t.Fatalf("summary is %q", got)
	}
}
