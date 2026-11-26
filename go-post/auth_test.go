package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestResolveAuthWalksFolders(t *testing.T) {
	w := &Workspace{Collections: []Collection{
		{ID: "root", AuthType: "bearer", Bearer: "root-token"},
		{ID: "child", ParentID: "root"},
		{ID: "grandchild", ParentID: "child"},
		{ID: "closed", ParentID: "root", AuthType: "none"},
		{ID: "basic", ParentID: "child", AuthType: "basic", Username: "ada", Password: "secret"},
	}}
	inherit := &Request{AuthType: "inherit"}

	res := w.resolveAuth(inherit, w.collection("grandchild"))
	if res.Type != "bearer" || res.Bearer != "root-token" || res.Source == nil || res.Source.ID != "root" {
		t.Fatalf("inherited auth is %+v", res)
	}

	// An explicit "No auth" stops the walk.
	res = w.resolveAuth(inherit, w.collection("closed"))
	if res.Type != "none" || res.Source == nil || res.Source.ID != "closed" {
		t.Fatalf("closed auth is %+v", res)
	}

	// The nearest folder wins.
	res = w.resolveAuth(inherit, w.collection("basic"))
	if res.Type != "basic" || res.Username != "ada" || res.Password != "secret" || res.Source.ID != "basic" {
		t.Fatalf("nearest auth is %+v", res)
	}

	// A request's own choice wins, even inside a protected folder.
	own := &Request{AuthType: "bearer", Bearer: "own-token"}
	res = w.resolveAuth(own, w.collection("basic"))
	if res.Type != "bearer" || res.Bearer != "own-token" || res.Source != nil {
		t.Fatalf("own auth is %+v", res)
	}
	none := &Request{AuthType: "none"}
	if res = w.resolveAuth(none, w.collection("root")); res.Type != "none" || res.Source != nil {
		t.Fatalf("explicit no-auth is %+v", res)
	}

	// Nothing above the request means no auth.
	if res = w.resolveAuth(inherit, nil); res.Type != "none" || res.Source != nil {
		t.Fatalf("orphan auth is %+v", res)
	}
	if blankRequest("x").AuthType != "inherit" {
		t.Fatalf("a new request should inherit, got %q", blankRequest("x").AuthType)
	}
}

// The request the HTTP client receives carries the folder's header while the
// stored request keeps its inherit choice.
func TestSendUsesFolderAuthorization(t *testing.T) {
	type seenAuth struct {
		value string
		done  chan struct{}
	}
	seen := &seenAuth{done: make(chan struct{})}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.value = r.Header.Get("Authorization")
		close(seen.done)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := testApp(t)
	col := a.ws().collection("col_starter")
	col.AuthType = "bearer"
	col.Bearer = "folder-{{token}}"
	r, _ := a.activeRequest()
	r.URL = srv.URL
	r.AuthType = "inherit"
	a.data.Workspaces[0].Environments = append(a.data.Workspaces[0].Environments, Environment{
		ID: "env_auth", Name: "Auth", Vars: []KV{{ID: "v1", Key: "token", Value: "secret", Enabled: true}},
	})
	a.ws().ActiveEnv = "env_auth"

	a.sendActive()
	select {
	case <-seen.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the request was never sent")
	}
	if seen.value != "Bearer folder-secret" {
		t.Fatalf("Authorization is %q", seen.value)
	}
	if r.AuthType != "inherit" || r.Bearer != "" {
		t.Fatalf("sending rewrote the stored request: %+v", r)
	}
}

func TestFolderAuthorizationModalSaves(t *testing.T) {
	a := testApp(t)
	col := a.ws().collection("col_starter")
	a.openCollectionAuth(col)
	if a.modal != "collection_auth" {
		t.Fatalf("modal is %q", a.modal)
	}
	a.modalAuth = "bearer"
	a.input1 = "{{token}}"

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Collection authorization") {
		t.Fatal("the modal title is missing")
	}
	if _, ok := tt.Find("Authorization type"); !ok {
		t.Fatal("the authorization type picker is missing")
	}
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if col.AuthType != "bearer" || col.Bearer != "{{token}}" {
		t.Fatalf("saved auth is %q / %q", col.AuthType, col.Bearer)
	}
	if a.modal != "" {
		t.Fatalf("the modal stayed open: %q", a.modal)
	}
}

func TestFolderModalOffersInheritForFolders(t *testing.T) {
	a := testApp(t)
	col := a.ws().collection("col_starter")
	folder := &Collection{ID: "fld_auth", Name: "Admin", ParentID: col.ID}
	a.ws().Collections = append(a.ws().Collections, *folder)
	a.openCollectionAuth(a.ws().collection("fld_auth"))

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Folder authorization") {
		t.Fatal("a folder should say folder, not collection")
	}
	if !tt.HasText("Inherit from parent") {
		t.Fatal("a folder should offer inheritance")
	}
}

func TestRequestAuthTabShowsInheritedSource(t *testing.T) {
	a := testApp(t)
	col := a.ws().collection("col_starter")
	col.AuthType = "bearer"
	col.Bearer = "{{token}}"
	r, _ := a.activeRequest()
	r.AuthType = "inherit"
	r.Bearer = ""
	a.reqTab = 3

	tt := ui.NewTester(a.view, 1280, 820)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Bearer token from Starter kit") {
		t.Fatal("the inherited source is not shown")
	}
	if _, ok := tt.Find("Collection authorization"); !ok {
		t.Fatal("the edit shortcut for the folder is missing")
	}
}

func TestPostmanFolderAuthRoundTrip(t *testing.T) {
	src := `{
	  "info": {"name": "Admin API"},
	  "item": [
	    {"name": "Admin", "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "{{adminToken}}", "type": "string"}]},
	     "item": [
	       {"name": "Inherits", "request": {"method": "GET", "url": "https://api.test/a"}},
	       {"name": "No auth", "request": {"method": "GET", "url": "https://api.test/b", "auth": {"type": "noauth"}}}
	     ]}
	  ]
	}`
	x, format, err := parseImport([]byte(src))
	if err != nil || format != "postman" {
		t.Fatalf("parse: %q, %v", format, err)
	}
	if len(x.Collections) != 2 {
		t.Fatalf("collections are %+v", x.Collections)
	}
	folder := x.Collections[1]
	if folder.AuthType != "bearer" || folder.Bearer != "{{adminToken}}" {
		t.Fatalf("folder auth is %+v", folder)
	}
	if folder.Requests[0].AuthType != "" || folder.Requests[1].AuthType != "none" {
		t.Fatalf("request auth is %+v", folder.Requests)
	}

	a := testApp(t)
	a.applyImport(x)
	var imported *Collection
	for i := range a.ws().Collections {
		if a.ws().Collections[i].Name == "Admin" {
			imported = &a.ws().Collections[i]
		}
	}
	if imported == nil {
		t.Fatal("the Admin folder was not imported")
	}
	if imported.AuthType != "bearer" || imported.Bearer != "{{adminToken}}" {
		t.Fatalf("imported folder auth is %+v", imported)
	}
	if imported.Requests[0].AuthType != "inherit" || imported.Requests[1].AuthType != "none" {
		t.Fatalf("imported request auth is %+v", imported.Requests)
	}
	if res := a.ws().resolveAuth(&imported.Requests[0], imported); res.Type != "bearer" || res.Source == nil {
		t.Fatalf("imported inherit does not resolve: %+v", res)
	}

	pm, err := postmanCollection(a.data)
	if err != nil {
		t.Fatal(err)
	}
	x2, _, err := parseImport(pm)
	if err != nil {
		t.Fatal(err)
	}
	var exported *exchangeCollection
	for i := range x2.Collections {
		if x2.Collections[i].Name == "Admin" {
			exported = &x2.Collections[i]
		}
	}
	if exported == nil || exported.AuthType != "bearer" || exported.Bearer != "{{adminToken}}" {
		t.Fatalf("exported folder auth is %+v", exported)
	}
	if exported.Requests[0].AuthType != "" || exported.Requests[1].AuthType != "none" {
		t.Fatalf("exported request auth is %+v", exported.Requests)
	}
}

func TestLoadMigratesOldRequestAuthToInherit(t *testing.T) {
	d := Data{
		Version:         1,
		ActiveWorkspace: "w",
		Workspaces: []Workspace{{
			ID: "w", Name: "W", Theme: "nocturne", Material: "none",
			Collections: []Collection{{ID: "c", Name: "C", Requests: []Request{
				{ID: "r_old", Name: "Old", Method: "GET", URL: "https://x.test", AuthType: "none"},
				{ID: "r_bearer", Name: "Bearer", Method: "GET", URL: "https://x.test", AuthType: "bearer"},
			}}},
			OpenTabs: []string{"r_old"}, ActiveTab: "r_old",
		}},
		Settings: Settings{Theme: "nocturne", TimeoutMS: 30000, FontSize: 14, Material: "none"},
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "gopost.json")
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0o600); err != nil {
		t.Fatal(err)
	}
	load := func() Data {
		st := &Store{dir: dir, file: file, fallback: map[string]string{}}
		got, err := st.Load()
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	got := load()
	r := got.Workspaces[0].Collections[0].Requests
	if got.Version != 2 {
		t.Fatalf("version is %d", got.Version)
	}
	if r[0].AuthType != "inherit" || r[1].AuthType != "bearer" {
		t.Fatalf("migrated auth is %q / %q", r[0].AuthType, r[1].AuthType)
	}

	// A choice made after the migration is not converted again.
	got.Workspaces[0].Collections[0].Requests[0].AuthType = "none"
	if err := (&Store{dir: dir, file: file, fallback: map[string]string{}}).Save(got); err != nil {
		t.Fatal(err)
	}
	if again := load(); again.Workspaces[0].Collections[0].Requests[0].AuthType != "none" {
		t.Fatalf("explicit no-auth was migrated again: %q", again.Workspaces[0].Collections[0].Requests[0].AuthType)
	}
}
