package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Data is everything Go Post keeps between launches. It lives in JSON in the
// app's user data directory; certificates keep their bytes beside it.
type Data struct {
	Version         int           `json:"version"`
	Workspaces      []Workspace   `json:"workspaces"`
	ActiveWorkspace string        `json:"activeWorkspace"`
	Certificates    []Certificate `json:"certificates"`
	CustomThemes    []CustomTheme `json:"customThemes,omitempty"`
	Settings        Settings      `json:"settings"`
}

// CustomTheme starts from one of the built-in palettes, then tints the window
// chrome and optionally paints a soft gradient. The request workspace keeps a
// solid panel, so the gradient never sits behind the response body. Surface is
// that panel's color; an empty value keeps the base palette's panel.
type CustomTheme struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Base            string                `json:"base"`
	Background      string                `json:"background"`
	Surface         string                `json:"surface,omitempty"`
	Accent          string                `json:"accent,omitempty"`
	Gradient        bool                  `json:"gradient"`
	GradientPoints  []CustomGradientPoint `json:"gradientPoints,omitempty"`
	GradientOpacity float32               `json:"gradientOpacity,omitempty"`
	GradientGrain   float32               `json:"gradientGrain,omitempty"`
	GradientMode    string                `json:"gradientMode,omitempty"` // "light", "dark", "auto"
	TextColor       string                `json:"textColor,omitempty"`    // "auto", "black", "white"
	// From, To and Angle are the pre-mesh format. They are still written so
	// older builds can open a new theme, and still read so old themes load.
	From  string  `json:"gradientFrom,omitempty"`
	To    string  `json:"gradientTo,omitempty"`
	Angle float32 `json:"gradientAngle,omitempty"`
}

// CustomGradientPoint is a color and its place in the chrome, from 0 to 1
// across and down the window. The editor drags these points directly.
type CustomGradientPoint struct {
	Color string  `json:"color"`
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
}

type Workspace struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Collections  []Collection  `json:"collections"`
	Environments []Environment `json:"environments"`
	ActiveEnv    string        `json:"activeEnv"`
	OpenTabs     []string      `json:"openTabs"`
	ActiveTab    string        `json:"activeTab"`
	History      []History     `json:"history"`
	// Theme and Material are the workspace's look: the selected built-in or
	// custom theme, and the window background. The active workspace's values
	// are mirrored into Settings, which is what every theme read uses.
	Theme    string `json:"theme,omitempty"`
	Material string `json:"material,omitempty"`
}

type Collection struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	ParentID string    `json:"parentId,omitempty"`
	Requests []Request `json:"requests"`
	// Authorization set on a folder or collection. "" and "inherit" mean the
	// parent's authorization is used; a concrete type applies to every request
	// below that inherits from it.
	AuthType string `json:"authType,omitempty"`
	Bearer   string `json:"bearer,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type Request struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Query       []KV   `json:"query,omitempty"`
	Headers     []KV   `json:"headers"`
	BodyType    string `json:"bodyType"` // none, json, text, xml
	Body        string `json:"body"`
	AuthType    string `json:"authType"` // inherit, none, bearer, basic
	Bearer      string `json:"bearer"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Description string `json:"description"`
}

type KV struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	Secret  bool   `json:"secret"`
}

type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Vars []KV   `json:"vars"`
}

type History struct {
	ID        string  `json:"id"`
	RequestID string  `json:"requestId"`
	Method    string  `json:"method"`
	URL       string  `json:"url"`
	Status    int     `json:"status"`
	Duration  int64   `json:"durationMs"`
	Size      int64   `json:"size"`
	At        string  `json:"at"`
	Body      string  `json:"body"`
	Headers   string  `json:"headers"`
	Request   Request `json:"request"`
}

type Settings struct {
	Theme       string  `json:"theme"`
	Accent      string  `json:"accent"`
	TimeoutMS   int     `json:"timeoutMs"`
	Follow      bool    `json:"followRedirects"`
	SkipVerify  bool    `json:"skipVerify"`
	Compact     bool    `json:"compact"`
	FontSize    float32 `json:"fontSize"`
	PersistBody bool    `json:"persistBody"`
	Acrylic     bool    `json:"acrylic"`
	Material    string  `json:"material,omitempty"` // "none", "acrylic", "mica"
}

type Certificate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	FileName    string `json:"fileName"`
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	NotBefore   string `json:"notBefore"`
	NotAfter    string `json:"notAfter"`
	Serial      string `json:"serial"`
	Fingerprint string `json:"fingerprint"`
	Host        string `json:"host"`
	CreatedAt   string `json:"createdAt"`
	HasPassword bool   `json:"hasPassword"`
}

func NewID(prefix string) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func BlankRequest(name string) Request {
	return Request{
		ID:       NewID("req"),
		Name:     name,
		Method:   "GET",
		URL:      "https://httpbin.org/get",
		BodyType: "none",
		AuthType: "inherit",
		Headers: []KV{
			{ID: NewID("h"), Key: "Accept", Value: "application/json", Enabled: true},
		},
	}
}

func DefaultData() Data {
	personal := Workspace{
		ID:   "ws_personal",
		Name: "Personal",
		Collections: []Collection{
			{
				ID:   "col_starter",
				Name: "Starter kit",
				Requests: []Request{
					{
						ID: "req_httpbin_get", Name: "HTTPBin / Get", Method: "GET",
						URL: "{{baseUrl}}/get", BodyType: "none", AuthType: "inherit",
						Headers:     []KV{{ID: "h1", Key: "Accept", Value: "application/json", Enabled: true}},
						Description: "A first request with an environment variable.",
					},
					{
						ID: "req_httpbin_post", Name: "HTTPBin / Post", Method: "POST",
						URL: "{{baseUrl}}/post", BodyType: "json", AuthType: "inherit",
						Headers: []KV{{ID: "h2", Key: "Content-Type", Value: "application/json", Enabled: true}},
						Body:    "{\n  \"hello\": \"world\",\n  \"from\": \"Go Post\"\n}",
					},
					{
						ID: "req_github_zen", Name: "GitHub / Zen", Method: "GET",
						URL: "https://api.github.com/zen", BodyType: "none", AuthType: "inherit",
						Headers: []KV{{ID: "h3", Key: "Accept", Value: "text/plain", Enabled: true}},
					},
				},
			},
		},
		Environments: []Environment{
			{
				ID: "env_dev", Name: "Development",
				Vars: []KV{
					{ID: "v1", Key: "baseUrl", Value: "https://httpbin.org", Enabled: true},
					{ID: "v2", Key: "token", Value: "paste-token-here", Enabled: false, Secret: true},
				},
			},
			{
				ID: "env_local", Name: "Local",
				Vars: []KV{
					{ID: "v3", Key: "baseUrl", Value: "http://localhost:8080", Enabled: true},
				},
			},
		},
		ActiveEnv: "env_dev",
		OpenTabs:  []string{"req_httpbin_get", "req_github_zen"},
		ActiveTab: "req_httpbin_get",
		Theme:     "nocturne",
		Material:  "none",
	}
	acme := Workspace{
		ID:   "ws_acme",
		Name: "Acme API",
		Collections: []Collection{
			{
				ID:   "col_acme",
				Name: "Users",
				Requests: []Request{
					{ID: "req_acme_list", Name: "List users", Method: "GET", URL: "{{baseUrl}}/users", BodyType: "none", AuthType: "bearer", Bearer: "{{token}}"},
					{ID: "req_acme_create", Name: "Create user", Method: "POST", URL: "{{baseUrl}}/users", BodyType: "json", AuthType: "bearer", Bearer: "{{token}}", Body: "{\n  \"name\": \"Ada Lovelace\",\n  \"role\": \"admin\"\n}"},
				},
			},
		},
		Environments: []Environment{
			{ID: "env_acme_stage", Name: "Staging", Vars: []KV{
				{ID: "v4", Key: "baseUrl", Value: "https://api.example.com", Enabled: true},
				{ID: "v5", Key: "token", Value: "", Enabled: true, Secret: true},
			}},
		},
		ActiveEnv: "env_acme_stage",
		OpenTabs:  []string{"req_acme_list"},
		ActiveTab: "req_acme_list",
		Theme:     "nocturne",
		Material:  "none",
	}
	return Data{
		Version:         2,
		Workspaces:      []Workspace{personal, acme},
		ActiveWorkspace: personal.ID,
		Certificates:    []Certificate{},
		Settings: Settings{
			Theme:       "nocturne",
			Accent:      "emerald",
			TimeoutMS:   30000,
			Follow:      true,
			SkipVerify:  false,
			Compact:     false,
			FontSize:    14,
			PersistBody: true,
			Acrylic:     false,
			Material:    "none",
		},
	}
}

func (d *Data) Workspace(id string) *Workspace {
	for i := range d.Workspaces {
		if d.Workspaces[i].ID == id {
			return &d.Workspaces[i]
		}
	}
	if len(d.Workspaces) > 0 {
		return &d.Workspaces[0]
	}
	return nil
}

func (w *Workspace) Collection(id string) *Collection {
	for i := range w.Collections {
		if w.Collections[i].ID == id {
			return &w.Collections[i]
		}
	}
	return nil
}

// collectionChildren returns the folders directly inside parentID. A parent of
// "" means the top level of the workspace.
func (w *Workspace) CollectionChildren(parentID string) []*Collection {
	var out []*Collection
	for i := range w.Collections {
		if w.Collections[i].ParentID == parentID {
			out = append(out, &w.Collections[i])
		}
	}
	return out
}

// inSubtree reports whether id is ancestorID or sits below it. The seen guard
// makes a damaged tree safe to walk.
func (w *Workspace) InSubtree(ancestorID, id string) bool {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		if id == ancestorID {
			return true
		}
		seen[id] = true
		col := w.Collection(id)
		if col == nil {
			return false
		}
		id = col.ParentID
	}
	return false
}

func (w *Workspace) CollectionPath(id string) string {
	var names []string
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		col := w.Collection(id)
		if col == nil {
			break
		}
		names = append(names, col.Name)
		id = col.ParentID
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return strings.Join(names, " / ")
}

func (w *Workspace) CollectionRequestCount(id string) int {
	seen := map[string]bool{}
	var count func(string) int
	count = func(cid string) int {
		if seen[cid] {
			return 0
		}
		seen[cid] = true
		col := w.Collection(cid)
		if col == nil {
			return 0
		}
		n := len(col.Requests)
		for _, child := range w.CollectionChildren(cid) {
			n += count(child.ID)
		}
		return n
	}
	return count(id)
}

func (w *Workspace) Request(id string) (*Request, *Collection) {
	for ci := range w.Collections {
		for ri := range w.Collections[ci].Requests {
			if w.Collections[ci].Requests[ri].ID == id {
				return &w.Collections[ci].Requests[ri], &w.Collections[ci]
			}
		}
	}
	return nil, nil
}

func (w *Workspace) Environment(id string) *Environment {
	for i := range w.Environments {
		if w.Environments[i].ID == id {
			return &w.Environments[i]
		}
	}
	return nil
}

func (w *Workspace) ActiveVars() map[string]string {
	out := map[string]string{}
	if e := w.Environment(w.ActiveEnv); e != nil {
		for _, v := range e.Vars {
			if v.Enabled && v.Key != "" {
				out[v.Key] = v.Value
			}
		}
	}
	return out
}

func Interpolate(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
		s = strings.ReplaceAll(s, "{{ "+k+" }}", v)
	}
	return s
}

func (w *Workspace) EnsureTabs() {
	seen := map[string]bool{}
	tabs := w.OpenTabs[:0]
	for _, id := range w.OpenTabs {
		if !seen[id] {
			if r, _ := w.Request(id); r != nil {
				tabs = append(tabs, id)
				seen[id] = true
			}
		}
	}
	w.OpenTabs = tabs
	if w.ActiveTab != "" {
		if _, ok := seen[w.ActiveTab]; !ok {
			w.ActiveTab = ""
		}
	}
	if w.ActiveTab == "" && len(w.OpenTabs) > 0 {
		w.ActiveTab = w.OpenTabs[0]
	}
}

func (w *Workspace) OpenTab(id string) {
	w.EnsureTabs()
	for _, t := range w.OpenTabs {
		if t == id {
			w.ActiveTab = id
			return
		}
	}
	w.OpenTabs = append(w.OpenTabs, id)
	w.ActiveTab = id
}

func (w *Workspace) CloseTab(id string) {
	w.EnsureTabs()
	for i, t := range w.OpenTabs {
		if t == id {
			w.OpenTabs = append(w.OpenTabs[:i], w.OpenTabs[i+1:]...)
			break
		}
	}
	if w.ActiveTab == id {
		w.ActiveTab = ""
		if len(w.OpenTabs) > 0 {
			w.ActiveTab = w.OpenTabs[0]
		}
	}
}
