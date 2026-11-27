package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-post/internal/core"
	"net/url"
	"sort"
	"strings"
)

// exchangeData is the common shape every importer produces. Collections use an
// index as their parent so nested folders survive without depending on any one
// client's ID scheme.
type exchangeData struct {
	Collections  []exchangeCollection
	Environments []exchangeEnvironment
}

type exchangeCollection struct {
	Name     string
	Parent   int // index into Collections, -1 for a root
	Requests []exchangeRequest
	// Authorization on the folder itself. Empty means inherit, as in Postman.
	AuthType, Bearer   string
	Username, Password string
}

type exchangeEnvironment struct {
	Name string
	Vars []exchangeKV
}

type exchangeRequest struct {
	Name, Method, URL, Description string
	Headers, Query                 []exchangeKV
	Body, BodyType                 string
	AuthType, Bearer               string
	Username, Password             string
}

type exchangeKV struct {
	Key, Value string
	Enabled    bool
	Secret     bool
}

func (x exchangeData) counts() (int, int) {
	requests := 0
	for _, c := range x.Collections {
		requests += len(c.Requests)
	}
	return requests, len(x.Environments)
}

// json helpers keep the importers close to each client's loose schema: absent
// or oddly typed fields simply read as empty instead of failing the file.
func jstr(v any) string {
	s, _ := v.(string)
	return s
}

func jmap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func jarr(v any) []any {
	a, _ := v.([]any)
	return a
}

func jbool(v any) bool {
	b, _ := v.(bool)
	return b
}

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")) && json.Valid([]byte(s))
}

// importFormat names the format of a file well enough to pick a parser. A
// client's own export is detected before the generic shapes it happens to use.
func importFormat(data []byte) string {
	t := bytes.TrimSpace(data)
	if len(t) == 0 {
		return ""
	}
	if t[0] == '[' {
		var arr []map[string]any
		if json.Unmarshal(t, &arr) == nil && len(arr) > 0 {
			if _, ok := arr[0]["_type"]; ok {
				return "insomnia"
			}
		}
		return ""
	}
	var root map[string]any
	if json.Unmarshal(t, &root) != nil {
		if bytes.Contains(t, []byte("curl ")) {
			return "curl"
		}
		return ""
	}
	if jstr(root["format"]) == "go-post" {
		return "gopost"
	}
	if res, ok := root["resources"]; ok {
		if _, ok := jmap(res)["httpRequests"]; ok {
			return "yaak"
		}
		if _, ok := res.([]any); ok {
			return "insomnia"
		}
	}
	if _, ok := root["item"]; ok {
		return "postman"
	}
	if _, ok := root["values"]; ok {
		return "postman-env"
	}
	if _, ok := root["requests"]; ok {
		return "hoppscotch"
	}
	return ""
}

// importFormatLabel is what a toast calls the file it just read.
func importFormatLabel(format string) string {
	switch format {
	case "gopost":
		return "Go Post backup"
	case "postman":
		return "Postman collection"
	case "postman-env":
		return "Postman environment"
	case "yaak":
		return "Yaak workspace"
	case "insomnia":
		return "Insomnia collection"
	case "hoppscotch":
		return "Hoppscotch collection"
	case "curl":
		return "cURL commands"
	}
	return "file"
}

func parseImport(data []byte) (exchangeData, string, error) {
	data = bytes.TrimSpace(data)
	format := importFormat(data)
	if format == "" {
		return exchangeData{}, "", errors.New("this does not look like a supported HTTP client export")
	}
	var x exchangeData
	var err error
	switch format {
	case "gopost":
		var b backupFile
		err = json.Unmarshal(data, &b)
		if err == nil {
			if len(b.Data.Workspaces) == 0 || b.Data.ActiveWorkspace == "" {
				err = errors.New("the backup has no workspaces")
			}
		}
		return x, format, err
	case "curl":
		return parseCurl(string(data)), format, nil
	}
	var root map[string]any
	if data[0] == '[' {
		var arr []any
		if err := json.Unmarshal(data, &arr); err != nil {
			return x, format, err
		}
		root = map[string]any{"resources": arr}
	} else if err := json.Unmarshal(data, &root); err != nil {
		return x, format, err
	}
	switch format {
	case "postman":
		x = parsePostman(root)
	case "postman-env":
		x = parsePostmanEnvironment(root)
	case "yaak":
		x = parseYaak(root)
	case "insomnia":
		x = parseInsomnia(jarr(root["resources"]))
	case "hoppscotch":
		x = parseHoppscotch(root)
	}
	return x, format, nil
}

// parsePostman reads the collection v2.1 tree: folders are `item`s with their
// own `item` array, requests are `item`s holding a `request` object.
func parsePostman(root map[string]any) exchangeData {
	info := jmap(root["info"])
	name := jstr(info["name"])
	if name == "" {
		name = "Imported collection"
	}
	x := exchangeData{Collections: []exchangeCollection{{Name: name, Parent: -1}}}
	authType, bearer, username, password := parseGenericAuth(jmap(root["auth"]))
	x.Collections[0].AuthType = authType
	x.Collections[0].Bearer = bearer
	x.Collections[0].Username = username
	x.Collections[0].Password = password
	for _, item := range jarr(root["item"]) {
		parsePostmanItem(item, &x, 0)
	}
	vars := parseKVs(jarr(root["variable"]), "key", "value", "disabled")
	if len(vars) > 0 {
		x.Environments = append(x.Environments, exchangeEnvironment{Name: name + " variables", Vars: vars})
	}
	return x
}

func parsePostmanItem(item any, x *exchangeData, parent int) {
	m := jsonMapOrNil(item)
	if m == nil {
		return
	}
	if len(jarr(m["item"])) > 0 {
		idx := len(x.Collections)
		name := jstr(m["name"])
		if name == "" {
			name = "Folder"
		}
		authType, bearer, username, password := parseGenericAuth(jmap(m["auth"]))
		x.Collections = append(x.Collections, exchangeCollection{
			Name: name, Parent: parent, AuthType: authType,
			Bearer: bearer, Username: username, Password: password,
		})
		for _, child := range jarr(m["item"]) {
			parsePostmanItem(child, x, idx)
		}
		return
	}
	req := jmap(m["request"])
	if len(req) == 0 {
		return
	}
	r := exchangeRequest{
		Name:        jstr(m["name"]),
		Method:      strings.ToUpper(jstr(req["method"])),
		Description: jstr(req["description"]),
	}
	switch u := req["url"].(type) {
	case string:
		r.URL = u
	case map[string]any:
		r.URL = jstr(u["raw"])
		for _, q := range jarr(u["query"]) {
			m := jsonMapOrNil(q)
			if m == nil || jstr(m["key"]) == "" {
				continue
			}
			r.Query = append(r.Query, exchangeKV{
				Key: jstr(m["key"]), Value: jstr(m["value"]),
				Enabled: !jbool(m["disabled"]),
			})
		}
	}
	r.Headers = parseKVs(jarr(req["header"]), "key", "value", "disabled")
	body := jmap(req["body"])
	switch jstr(body["mode"]) {
	case "raw":
		r.Body = jstr(body["raw"])
	case "urlencoded":
		r.Body = encodeKVs(parseKVs(jarr(body["urlencoded"]), "key", "value", "disabled"))
	case "formdata":
		r.Body = encodeKVs(parseKVs(jarr(body["formdata"]), "key", "value", "disabled"))
	}
	language := jstr(jmap(jmap(body["options"])["raw"])["language"])
	if language == "json" || language == "graphql" || looksLikeJSON(r.Body) {
		r.BodyType = "json"
	}
	auth := jmap(req["auth"])
	r.AuthType, r.Bearer, r.Username, r.Password = parseGenericAuth(auth)
	if r.Name == "" {
		r.Name = r.Method + " " + r.URL
	}
	x.Collections[parent].Requests = append(x.Collections[parent].Requests, r)
}

func parsePostmanEnvironment(root map[string]any) exchangeData {
	name := jstr(root["name"])
	if name == "" {
		name = "Imported variables"
	}
	vars := parseKVs(jarr(root["values"]), "key", "value", "disabled")
	return exchangeData{Environments: []exchangeEnvironment{{Name: name, Vars: vars}}}
}

// parseYaak reads the workspace export's resources: httpRequests point at a
// folderId, folders at a folderId or a workspace, and environments at variables.
//
// The export lists resources newest first, so a nested folder usually appears
// before the folder that contains it. Every folder therefore gets an index in
// the first pass, and parents are resolved once all IDs are known.
func parseYaak(root map[string]any) exchangeData {
	res := jmap(root["resources"])
	x := exchangeData{}
	byID := map[string]int{}
	for _, w := range jarr(res["workspaces"]) {
		m := jsonMapOrNil(w)
		if m == nil {
			continue
		}
		name := jstr(m["name"])
		if name == "" {
			name = "Imported workspace"
		}
		byID[jstr(m["id"])] = len(x.Collections)
		x.Collections = append(x.Collections, exchangeCollection{Name: name, Parent: -1})
	}

	type folderRow struct {
		index, workspace int
		m                map[string]any
	}
	var folderMaps []map[string]any
	for _, f := range jarr(res["folders"]) {
		if m := jsonMapOrNil(f); m != nil {
			folderMaps = append(folderMaps, m)
		}
	}
	// Yaak shows siblings by sortPriority, then creation date. Assigning indexes
	// in that order keeps each folder's children in the original sidebar order.
	sort.SliceStable(folderMaps, func(i, j int) bool {
		return yaakBefore(folderMaps[i], folderMaps[j])
	})
	var folders []folderRow
	for _, m := range folderMaps {
		name := jstr(m["name"])
		if name == "" {
			name = "Folder"
		}
		idx := len(x.Collections)
		byID[jstr(m["id"])] = idx
		x.Collections = append(x.Collections, exchangeCollection{Name: name, Parent: -1})
		workspace, _ := byID[jstr(m["workspaceId"])]
		folders = append(folders, folderRow{index: idx, workspace: workspace, m: m})
	}
	for _, f := range folders {
		parent, ok := byID[jstr(f.m["folderId"])]
		if !ok || parent == f.index {
			parent = f.workspace
		}
		x.Collections[f.index].Parent = parent
	}

	folderOf := func(m map[string]any) (int, bool) {
		if idx, ok := byID[jstr(m["folderId"])]; ok {
			return idx, true
		}
		idx, ok := byID[jstr(m["workspaceId"])]
		return idx, ok
	}
	var requests []struct {
		index int
		m     map[string]any
	}
	for _, r := range jarr(res["httpRequests"]) {
		m := jsonMapOrNil(r)
		if m == nil {
			continue
		}
		idx, ok := folderOf(m)
		if !ok {
			continue
		}
		requests = append(requests, struct {
			index int
			m     map[string]any
		}{idx, m})
	}
	sort.SliceStable(requests, func(i, j int) bool {
		if requests[i].index != requests[j].index {
			return requests[i].index < requests[j].index
		}
		return yaakBefore(requests[i].m, requests[j].m)
	})
	for _, row := range requests {
		m := row.m
		body := jmap(m["body"])
		req := exchangeRequest{
			Name:        jstr(m["name"]),
			Method:      strings.ToUpper(jstr(m["method"])),
			URL:         jstr(m["url"]),
			Description: jstr(m["description"]),
			Body:        jstr(body["text"]),
			Headers:     parseKVs(jarr(m["headers"]), "name", "value", "disabled"),
		}
		for _, p := range jarr(m["urlParameters"]) {
			pm := jsonMapOrNil(p)
			if pm == nil || jstr(pm["name"]) == "" {
				continue
			}
			enabled := true
			if b, ok := pm["enabled"].(bool); ok {
				enabled = b
			}
			req.Query = append(req.Query, exchangeKV{Key: jstr(pm["name"]), Value: jstr(pm["value"]), Enabled: enabled})
		}
		if jstr(m["bodyType"]) == "application/json" || looksLikeJSON(req.Body) {
			req.BodyType = "json"
		}
		req.AuthType, req.Bearer, req.Username, req.Password = parseGenericAuth(jmap(m["authentication"]))
		if req.AuthType == "" {
			req.AuthType = normalizeAuth(jstr(m["authenticationType"]))
		}
		if req.Name == "" {
			req.Name = req.Method + " " + req.URL
		}
		x.Collections[row.index].Requests = append(x.Collections[row.index].Requests, req)
	}
	for _, e := range jarr(res["environments"]) {
		m := jsonMapOrNil(e)
		if m == nil {
			continue
		}
		vars := parseEnabledKVs(jarr(m["variables"]), "name", "value")
		name := jstr(m["name"])
		if name == "" {
			name = "Imported environment"
		}
		x.Environments = append(x.Environments, exchangeEnvironment{Name: name, Vars: vars})
	}
	return x
}

// yaakBefore orders two resources the way Yaak's sidebar does: explicit
// sortPriority first, then creation time, both oldest first.
func yaakBefore(a, b map[string]any) bool {
	ap, _ := a["sortPriority"].(float64)
	bp, _ := b["sortPriority"].(float64)
	if ap != bp {
		return ap < bp
	}
	return jstr(a["createdAt"]) < jstr(b["createdAt"])
}

// parseInsomnia reads its flat resource list. Workspaces and request groups
// become collections; requests are placed by their parentId.
func parseInsomnia(resources []any) exchangeData {
	x := exchangeData{}
	byID := map[string]int{}
	for _, item := range resources {
		m := jsonMapOrNil(item)
		if m == nil || jstr(m["_type"]) != "workspace" {
			continue
		}
		byID[jstr(m["_id"])] = len(x.Collections)
		x.Collections = append(x.Collections, exchangeCollection{Name: insomniaName(m), Parent: -1})
	}
	for _, item := range resources {
		m := jsonMapOrNil(item)
		if m == nil || jstr(m["_type"]) != "request_group" {
			continue
		}
		parent, ok := byID[jstr(m["parentId"])]
		if !ok {
			parent = 0
		}
		byID[jstr(m["_id"])] = len(x.Collections)
		x.Collections = append(x.Collections, exchangeCollection{Name: insomniaName(m), Parent: parent})
	}
	if len(x.Collections) == 0 {
		x.Collections = append(x.Collections, exchangeCollection{Name: "Imported collection", Parent: -1})
	}
	for _, item := range resources {
		m := jsonMapOrNil(item)
		if m == nil || jstr(m["_type"]) != "request" {
			continue
		}
		idx, ok := byID[jstr(m["parentId"])]
		if !ok {
			idx = 0
		}
		body := jmap(m["body"])
		req := exchangeRequest{
			Name:        insomniaName(m),
			Method:      strings.ToUpper(jstr(m["method"])),
			URL:         jstr(m["url"]),
			Description: jstr(m["description"]),
			Body:        jstr(body["text"]),
			Headers:     parseKVs(jarr(m["headers"]), "name", "value", "disabled"),
		}
		if strings.Contains(jstr(body["mimeType"]), "json") || looksLikeJSON(req.Body) {
			req.BodyType = "json"
		}
		req.AuthType, req.Bearer, req.Username, req.Password = parseGenericAuth(jmap(m["authentication"]))
		if req.AuthType == "" {
			req.AuthType = normalizeAuth(jstr(jmap(m["authentication"])["type"]))
		}
		x.Collections[idx].Requests = append(x.Collections[idx].Requests, req)
	}
	for _, item := range resources {
		m := jsonMapOrNil(item)
		if m == nil || jstr(m["_type"]) != "environment" {
			continue
		}
		vars := []exchangeKV{}
		data := jmap(m["data"])
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vars = append(vars, exchangeKV{Key: k, Value: jstr(data[k]), Enabled: true})
		}
		for _, v := range jarr(m["variables"]) {
			vm := jsonMapOrNil(v)
			if vm == nil {
				continue
			}
			vars = append(vars, exchangeKV{
				Key: jstr(vm["name"]), Value: jstr(vm["value"]),
				Enabled: jbool(vm["enabled"]),
			})
		}
		x.Environments = append(x.Environments, exchangeEnvironment{Name: insomniaName(m), Vars: vars})
	}
	return x
}

func insomniaName(m map[string]any) string {
	if n := jstr(m["name"]); n != "" {
		return n
	}
	return "Imported item"
}

// parseHoppscotch covers the simple collection export with a requests array.
func parseHoppscotch(root map[string]any) exchangeData {
	name := jstr(jmap(root["info"])["name"])
	if name == "" {
		name = jstr(root["name"])
	}
	if name == "" {
		name = "Imported collection"
	}
	x := exchangeData{Collections: []exchangeCollection{{Name: name, Parent: -1}}}
	for _, item := range jarr(root["requests"]) {
		m := jsonMapOrNil(item)
		if m == nil {
			continue
		}
		url := jstr(m["endpoint"])
		if url == "" {
			url = jstr(m["url"])
		}
		req := exchangeRequest{
			Name:    jstr(m["name"]),
			Method:  strings.ToUpper(jstr(m["method"])),
			URL:     url,
			Headers: parseKVs(jarr(m["headers"]), "key", "value", "disabled"),
		}
		body := jmap(m["body"])
		req.Body = jstr(body["raw"])
		if looksLikeJSON(req.Body) {
			req.BodyType = "json"
		}
		if req.Name == "" {
			req.Name = req.Method + " " + req.URL
		}
		x.Collections[0].Requests = append(x.Collections[0].Requests, req)
	}
	return x
}

func jsonMapOrNil(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// parseKVs reads a list of key/value rows. enabledKey may mean either "live"
// or "disabled", so callers pass the name and we invert it for disabled.
func parseKVs(rows []any, keyField, valueField, flagField string) []exchangeKV {
	var out []exchangeKV
	for _, row := range rows {
		m := jsonMapOrNil(row)
		if m == nil {
			continue
		}
		key := jstr(m[keyField])
		if key == "" {
			continue
		}
		enabled := true
		if flag, ok := m[flagField]; ok {
			if b, ok := flag.(bool); ok {
				enabled = !b
			}
		}
		out = append(out, exchangeKV{Key: key, Value: jstr(m[valueField]), Enabled: enabled})
	}
	return out
}

// parseEnabledKVs reads rows whose flag means "enabled" rather than "disabled".
func parseEnabledKVs(rows []any, keyField, valueField string) []exchangeKV {
	var out []exchangeKV
	for _, row := range rows {
		m := jsonMapOrNil(row)
		if m == nil {
			continue
		}
		key := jstr(m[keyField])
		if key == "" {
			continue
		}
		enabled := true
		if b, ok := m["enabled"].(bool); ok {
			enabled = b
		}
		out = append(out, exchangeKV{Key: key, Value: jstr(m[valueField]), Enabled: enabled})
	}
	return out
}

func encodeKVs(rows []exchangeKV) string {
	parts := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Enabled {
			parts = append(parts, r.Key+"="+r.Value)
		}
	}
	return strings.Join(parts, "&")
}

func normalizeAuth(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "bearer", "oauth2":
		return "bearer"
	case "basic":
		return "basic"
	case "none", "noauth", "no_auth":
		return "none"
	}
	return ""
}

func parseGenericAuth(auth map[string]any) (authType, bearer, username, password string) {
	authType = normalizeAuth(jstr(auth["type"]))
	for _, item := range jarr(auth[authType]) {
		m := jsonMapOrNil(item)
		if m == nil {
			continue
		}
		switch jstr(m["key"]) {
		case "token":
			bearer = jstr(m["value"])
		case "username":
			username = jstr(m["value"])
		case "password":
			password = jstr(m["value"])
		}
	}
	for k, v := range auth {
		switch strings.ToLower(k) {
		case "token":
			if bearer == "" {
				bearer = jstr(v)
			}
		case "username", "user":
			if username == "" {
				username = jstr(v)
			}
		case "password":
			if password == "" {
				password = jstr(v)
			}
		}
	}
	return
}

// applyImport adds the imported collections and environments to the active
// workspace. Existing requests are never touched, so an import is safe to run
// more than once.
func (a *app) applyImport(x exchangeData) (int, int, int) {
	w := a.ws()
	if w == nil {
		return 0, 0, 0
	}
	// Reserve every ID first: an importer may list a nested folder before its
	// parent (Yaak exports newest first), and the parent index must resolve.
	ids := make([]string, len(x.Collections))
	for i := range ids {
		ids[i] = core.NewID("col")
	}
	collections, requests := 0, 0
	for i, col := range x.Collections {
		name := strings.TrimSpace(col.Name)
		if name == "" {
			name = "Imported collection"
		}
		parent := ""
		if col.Parent >= 0 && col.Parent < len(ids) {
			parent = ids[col.Parent]
		}
		c := core.Collection{ID: ids[i], Name: name, ParentID: parent,
			AuthType: col.AuthType, Bearer: col.Bearer,
			Username: col.Username, Password: col.Password}
		for _, r := range col.Requests {
			c.Requests = append(c.Requests, importRequest(r))
			requests++
		}
		w.Collections = append(w.Collections, c)
		collections++
	}
	envs := 0
	for _, e := range x.Environments {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = "Imported environment"
		}
		env := core.Environment{ID: core.NewID("env"), Name: name}
		for _, v := range e.Vars {
			if strings.TrimSpace(v.Key) == "" {
				continue
			}
			env.Vars = append(env.Vars, core.KV{
				ID: core.NewID("v"), Key: v.Key, Value: v.Value,
				Enabled: v.Enabled, Secret: v.Secret,
			})
		}
		w.Environments = append(w.Environments, env)
		envs++
	}
	return collections, requests, envs
}

func importRequest(r exchangeRequest) core.Request {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		name = strings.TrimSpace(r.Method + " " + r.URL)
	}
	if name == "" {
		name = "Imported request"
	}
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = "GET"
	}
	query := r.Query
	if len(query) == 0 {
		query = queryFromURL(r.URL)
	}
	bodyType := r.BodyType
	if bodyType == "" {
		switch {
		case looksLikeJSON(r.Body):
			bodyType = "json"
		case strings.TrimSpace(r.Body) != "":
			bodyType = "text"
		default:
			bodyType = "none"
		}
	}
	authType := r.AuthType
	if authType == "" {
		// No auth object in the file means "inherit from the folder", which is
		// how Postman leaves requests that follow their collection.
		authType = "inherit"
	}
	req := core.Request{
		ID: core.NewID("req"), Name: name, Method: method, URL: r.URL,
		BodyType: bodyType, Body: r.Body, AuthType: authType,
		Bearer: r.Bearer, Username: r.Username, Password: r.Password,
		Description: r.Description,
	}
	for _, h := range r.Headers {
		req.Headers = append(req.Headers, core.KV{
			ID: core.NewID("h"), Key: h.Key, Value: h.Value,
			Enabled: h.Enabled, Secret: h.Secret,
		})
	}
	for _, q := range query {
		req.Query = append(req.Query, core.KV{ID: core.NewID("q"), Key: q.Key, Value: q.Value, Enabled: q.Enabled})
	}
	// Show the rows in the URL too, so an imported request looks the same on
	// both sides from the first frame.
	core.SyncURLQuery(&req)
	return req
}

// queryFromURL reads the query string of a URL that a client kept inline, so
// that go-post can show it as editable parameter rows.
func queryFromURL(raw string) []exchangeKV {
	i := strings.Index(raw, "?")
	if i < 0 {
		return nil
	}
	values, err := url.ParseQuery(raw[i+1:])
	if err != nil {
		return nil
	}
	out := []exchangeKV{}
	for k, vs := range values {
		for _, v := range vs {
			out = append(out, exchangeKV{Key: k, Value: v, Enabled: true})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// backupFile is the Go Post exchange format. It is the same Data the app
// persists, wrapped with a marker so an import can offer a full restore.
type backupFile struct {
	Format  string    `json:"format"`
	Version int       `json:"version"`
	Data    core.Data `json:"data"`
}

func backupJSON(d core.Data) ([]byte, error) {
	return json.MarshalIndent(backupFile{Format: "go-post", Version: 1, Data: d}, "", "  ")
}

// postmanCollection writes v2.1, which Postman, Yaak, Insomnia and others all
// read. The active environment becomes collection variables so values travel.
func postmanCollection(d core.Data) ([]byte, error) {
	w := d.Workspace(d.ActiveWorkspace)
	if w == nil {
		return nil, errors.New("no active workspace to export")
	}
	info := map[string]any{
		"name":   w.Name + " (Go Post)",
		"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
	}
	items := make([]any, 0, len(w.Collections))
	for _, c := range w.Collections {
		if c.ParentID == "" {
			items = append(items, postmanItem(c, w.Collections))
		}
	}
	root := map[string]any{"info": info, "item": items}
	if env := w.Environment(w.ActiveEnv); env != nil {
		vars := make([]any, 0, len(env.Vars))
		for _, v := range env.Vars {
			if v.Enabled && v.Key != "" {
				vars = append(vars, map[string]any{"key": v.Key, "value": v.Value, "type": "string"})
			}
		}
		if len(vars) > 0 {
			root["variable"] = vars
		}
	}
	return json.MarshalIndent(root, "", "  ")
}

func postmanItem(c core.Collection, all []core.Collection) map[string]any {
	items := make([]any, 0, len(c.Requests))
	for _, parent := range all {
		if parent.ParentID == c.ID {
			items = append(items, postmanItem(parent, all))
		}
	}
	for _, r := range c.Requests {
		items = append(items, map[string]any{"name": r.Name, "request": postmanRequest(r)})
	}
	item := map[string]any{"name": c.Name, "item": items}
	if auth := postmanAuth(c.AuthType, c.Bearer, c.Username, c.Password); auth != nil {
		item["auth"] = auth
	}
	return item
}

func postmanRequest(r core.Request) map[string]any {
	u := map[string]any{"raw": r.URL}
	query := make([]any, 0, len(r.Query))
	for _, p := range r.Query {
		if p.Key == "" {
			continue
		}
		query = append(query, map[string]any{"key": p.Key, "value": p.Value, "disabled": !p.Enabled})
	}
	if len(query) > 0 {
		u["query"] = query
	}
	req := map[string]any{
		"method":      r.Method,
		"header":      []any{},
		"url":         u,
		"description": r.Description,
	}
	headers := make([]any, 0, len(r.Headers))
	for _, h := range r.Headers {
		if h.Key == "" {
			continue
		}
		headers = append(headers, map[string]any{
			"key": h.Key, "value": h.Value, "disabled": !h.Enabled,
		})
	}
	req["header"] = headers
	if r.Body != "" {
		req["body"] = map[string]any{
			"mode": "raw", "raw": r.Body,
			"options": map[string]any{"raw": map[string]any{"language": r.BodyType}},
		}
	}
	if auth := postmanAuth(r.AuthType, r.Bearer, r.Username, r.Password); auth != nil {
		req["auth"] = auth
	}
	return req
}

// postmanAuth renders one authorization block. An inheriting request or folder
// has no block, so it follows its parent, as in Postman.
func postmanAuth(authType, bearer, username, password string) map[string]any {
	switch authType {
	case "none":
		return map[string]any{"type": "noauth"}
	case "bearer":
		return map[string]any{
			"type":   "bearer",
			"bearer": []any{map[string]any{"key": "token", "value": bearer, "type": "string"}},
		}
	case "basic":
		return map[string]any{
			"type": "basic",
			"basic": []any{
				map[string]any{"key": "username", "value": username, "type": "string"},
				map[string]any{"key": "password", "value": password, "type": "string"},
			},
		}
	}
	return nil
}

// parseCurl reads one or more curl commands, including backslash continuations.
func parseCurl(data string) exchangeData {
	data = strings.ReplaceAll(data, "\\\n", " ")
	col := exchangeCollection{Name: "cURL import", Parent: -1}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "curl ") {
			continue
		}
		r := parseCurlCommand(shellTokens(line))
		if r.URL != "" {
			col.Requests = append(col.Requests, r)
		}
	}
	if len(col.Requests) == 0 {
		return exchangeData{}
	}
	return exchangeData{Collections: []exchangeCollection{col}}
}

func shellTokens(s string) []string {
	var out []string
	var cur strings.Builder
	var quote rune
	started := false
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case r == '\\':
			// Skip the escape; the next rune is taken literally.
		case r == ' ' || r == '\t':
			if started {
				out = append(out, cur.String())
				cur.Reset()
				started = false
			}
		default:
			cur.WriteRune(r)
			started = true
		}
	}
	if started {
		out = append(out, cur.String())
	}
	return out
}

func parseCurlCommand(tokens []string) exchangeRequest {
	r := exchangeRequest{Method: "GET"}
	headers := []exchangeKV{}
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok {
		case "-X", "--request":
			i++
			if i < len(tokens) {
				r.Method = strings.ToUpper(tokens[i])
			}
		case "-H", "--header":
			i++
			if i < len(tokens) {
				if k, v, ok := strings.Cut(tokens[i], ":"); ok {
					headers = append(headers, exchangeKV{Key: strings.TrimSpace(k), Value: strings.TrimSpace(v), Enabled: true})
				}
			}
		case "-d", "--data", "--data-raw", "--data-binary":
			i++
			if i < len(tokens) {
				r.Body = tokens[i]
				if r.Method == "GET" {
					r.Method = "POST"
				}
			}
		case "-u", "--user":
			i++
			if i < len(tokens) {
				if u, p, ok := strings.Cut(tokens[i], ":"); ok {
					r.Username, r.Password, r.AuthType = u, p, "basic"
				}
			}
		default:
			if !strings.HasPrefix(tok, "-") && r.URL == "" {
				r.URL = tok
			}
		}
	}
	r.Headers = headers
	if looksLikeJSON(r.Body) {
		r.BodyType = "json"
	}
	r.Name = r.Method + " " + r.URL
	return r
}

func importSummary(format string, collections, requests, envs int) string {
	parts := []string{}
	if collections > 0 {
		parts = append(parts, fmt.Sprintf("%d collections", collections))
	}
	if requests > 0 {
		parts = append(parts, fmt.Sprintf("%d requests", requests))
	}
	if envs > 0 {
		parts = append(parts, fmt.Sprintf("%d environments", envs))
	}
	if len(parts) == 0 {
		return "Nothing to import"
	}
	return fmt.Sprintf("Imported %s from %s", strings.Join(parts, ", "), importFormatLabel(format))
}
