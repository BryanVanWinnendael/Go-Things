package app

import (
	"encoding/json"
	"fmt"
	"go-post/internal/core"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type app struct {
	store          *core.Store
	data           core.Data
	win            *mygo.Window
	settingsWin    *mygo.Window
	themePickerWin *mygo.Window
	settingsTab    string

	page                 string
	systemDark           bool
	split                float32
	sidebarW             float32
	sidebarAnimW         float32
	sidebarExpandedW     float32
	sidebarAnimating     bool
	sidebarTogglePending bool
	sidebarCollapsed     bool
	settingsW            float32
	reqTab               int
	respTab              int
	envOpen              bool
	wsOpen               bool
	materialOpen         bool
	headerSuggest        headerSuggestState
	headerFocus          string
	varHover             *varHoverInfo
	varSuggest           variableSuggestState
	varCaretField        string
	varCaretPos          int
	envReveal            string
	envRevealDone        bool
	dialogAnim           float32
	ctxMenuAnim          float32
	ctxMenu              contextMenuState
	viewToken            int

	inputHandles map[uint64]*ui.Handle

	// Colored runs of JSON, kept per request so a body is tokenized when it
	// changes rather than on every frame it is drawn.
	bodySyntax     map[string]*syntaxEntry
	responseSyntax map[string]*syntaxEntry

	responses map[string]core.SendResult
	sending   map[string]bool

	// Modal state. A modal's fields are plain strings bound by the view.
	modal       string
	modalTitle  string
	modalError  string
	modalInfo   string
	modalTarget string
	modalParent string
	modalAuth   string
	input1      string
	input2      string
	input3      string
	confirm     func()

	themePickerOpen      bool
	themePickerAnchor    string
	themePickerTitleRef  ui.Handle
	themePickerCardRef   ui.Handle
	themePickerDraft     core.CustomTheme
	themePickerOriginal  core.CustomTheme
	themePickerIndex     int
	themePickerPoint     int
	themePickerHex       string
	themePickerSurface   string
	themePickerTarget    string
	themePickerAccentHex string

	pendingCertPath string
	pendingCertInfo *core.Certificate
	certTestOK      bool

	search           string
	searchThemeMode  bool
	themePreviewBase string
	themePreviewLast string
	reveal           map[string]bool
	last             time.Time
	lastJS           string

	collapsed map[string]bool

	palette  palette
	themeKey string
}

func newApp(store *core.Store, d core.Data) *app {
	a := &app{
		store:            store,
		data:             d,
		page:             "request",
		settingsTab:      "appearance",
		split:            54,
		sidebarW:         sidebarDefaultWidth,
		sidebarAnimW:     sidebarDefaultWidth,
		sidebarExpandedW: sidebarDefaultWidth,
		settingsW:        settingsRailDefaultWidth,
		collapsed:        map[string]bool{},
		systemDark:       true,
		inputHandles:     map[uint64]*ui.Handle{},
		responses:        map[string]core.SendResult{},
		sending:          map[string]bool{},
		reveal:           map[string]bool{},
	}
	return a
}

func (a *app) ws() *core.Workspace { return a.data.Workspace(a.data.ActiveWorkspace) }

// refreshWindows makes every open window rebuild with the current settings,
// which matters when a theme or accent changes while both are visible.
func (a *app) refreshWindows() {
	if a.win != nil && !a.win.IsDestroyed() {
		a.win.Invalidate()
	}
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Invalidate()
	}
	if a.themePickerWin != nil && !a.themePickerWin.IsDestroyed() {
		a.themePickerWin.Invalidate()
	}
}

func (a *app) setTheme(id string) {
	a.data.Settings.Theme = id
	if w := a.ws(); w != nil {
		w.Theme = id
	}
	a.themeKey = ""
	a.save()
	a.refreshWindows()
}

func (a *app) setAccent(id string) {
	a.data.Settings.Accent = id
	a.themeKey = ""
	a.save()
	a.refreshWindows()
}

func isPresetTheme(id string) bool {
	return core.IsPresetTheme(id)
}

// validTheme reports whether id names the System choice, a built-in theme, or a
// custom theme that still exists.
func validTheme(id string, customs []core.CustomTheme) bool {
	return core.ValidTheme(id, customs)
}

func (a *app) deleteCustomTheme(id string) {
	for i := range a.data.CustomThemes {
		if a.data.CustomThemes[i].ID == id {
			a.data.CustomThemes = append(a.data.CustomThemes[:i], a.data.CustomThemes[i+1:]...)
			break
		}
	}
	// The theme lived in the shared library, so every workspace using it must
	// fall back, not just the active one.
	for i := range a.data.Workspaces {
		if a.data.Workspaces[i].Theme == id {
			a.data.Workspaces[i].Theme = "system"
		}
	}
	if a.data.Settings.Theme == id {
		a.data.Settings.Theme = "system"
	}
	a.themeKey = ""
	a.save()
	a.refreshWindows()
}

// copyTheme duplicates a custom theme, or turns a built-in preset into a
// custom theme seeded from it.
func (a *app) copyTheme(id string) {
	if ct := customThemeByID(a.data.CustomThemes, id); ct != nil {
		cp := *ct
		cp.ID = core.NewID("theme")
		cp.Name = a.copyThemeName(ct.Name)
		cp.GradientPoints = append([]core.CustomGradientPoint(nil), ct.GradientPoints...)
		a.data.CustomThemes = append(a.data.CustomThemes, cp)
		a.setTheme(cp.ID)
		return
	}
	if !isPresetTheme(id) {
		return
	}
	base := themeByID(id)
	cp := core.CustomTheme{
		ID:              core.NewID("theme"),
		Name:            a.copyThemeName(base.Name),
		Base:            base.ID,
		Background:      hexColor(base.Bg),
		Gradient:        base.HasGradient,
		GradientOpacity: base.GradientAlpha,
		GradientGrain:   base.GradientGrain,
		GradientMode:    base.GradientMode,
		TextColor:       "auto",
	}
	// Copy the mesh itself, not just the base palette: the editor reads these
	// points, so leaving them out would show a different gradient as soon as
	// the copy was opened for editing.
	for _, point := range base.Gradient {
		cp.GradientPoints = append(cp.GradientPoints, core.CustomGradientPoint{
			Color: hexColor(point.Color),
			X:     point.X,
			Y:     point.Y,
		})
	}
	if actual := resolvePalette(core.Settings{Theme: id}, a.systemDark, a.data.CustomThemes); actual.Accent.A != 0 {
		cp.Accent = hexColor(actual.Accent)
	}
	a.data.CustomThemes = append(a.data.CustomThemes, cp)
	a.setTheme(cp.ID)
}

func (a *app) copyThemeName(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "Theme"
	}
	// Copied copies read better as "Name copy 2" than "Name copy copy".
	if idx := strings.LastIndex(strings.ToLower(base), " copy"); idx > 0 {
		tail := strings.TrimSpace(base[idx+len(" copy"):])
		if tail == "" || digitsOnly(tail) {
			base = strings.TrimSpace(base[:idx])
		}
	}
	used := make(map[string]bool, len(a.data.CustomThemes))
	for _, ct := range a.data.CustomThemes {
		used[normalizedThemeName(ct.Name)] = true
	}
	name := base + " copy"
	if !used[normalizedThemeName(name)] {
		return name
	}
	for i := 2; ; i++ {
		name = fmt.Sprintf("%s copy %d", base, i)
		if !used[normalizedThemeName(name)] {
			return name
		}
	}
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (a *app) setMaterial(material string) {
	a.data.Settings.Material = material
	a.data.Settings.Acrylic = material == "acrylic"
	if w := a.ws(); w != nil {
		w.Material = material
	}
	a.save()
	a.applyWindowMaterials()
	a.refreshWindows()
}

// applyWindowMaterials puts the workspace's material behind every window when
// the setting is on. Each window is created with a material so it can switch
// off and on without being recreated. The settings and theme editor windows
// float over the main one, but they follow the same material: their rail sits
// on the chrome, while the rounded stage stays solid.
func (a *app) applyWindowMaterials() {
	material := mygo.VibrancyNone
	switch a.data.Settings.Material {
	case "acrylic":
		material = mygo.VibrancyAcrylic
	case "mica":
		material = mygo.VibrancyMica
	}
	for _, w := range []*mygo.Window{a.win, a.settingsWin, a.themePickerWin} {
		if w != nil && !w.IsDestroyed() {
			w.SetVibrancy(material)
		}
	}
}

func (a *app) useTheme(c *ui.Context, p palette) *ui.Theme {
	applyTheme(c, p, a.data.Settings)
	t := c.Theme()
	if t.Dark != p.Dark || t.Background != p.Bg || t.Accent != p.Accent || t.Surface != p.Field || t.SurfaceHover != p.Hover || t.Text != p.Text {
		// A window can take a frame to pick up a new theme; make sure the
		// next one does instead of leaving a control with the old colors.
		c.Invalidate()
	}
	return t
}

func (a *app) activeRequest() (*core.Request, *core.Collection) {
	w := a.ws()
	if w == nil {
		return nil, nil
	}
	w.EnsureTabs()
	if w.ActiveTab == "" {
		return nil, nil
	}
	return w.Request(w.ActiveTab)
}

func (a *app) save() {
	_ = a.store.Save(a.data)
	b, _ := json.Marshal(a.data)
	a.lastJS = string(b)
	a.last = time.Now()
}

// autosave runs at the end of each frame. Text inputs write into the model as
// the view builds, so comparing the encoded model catches every edit without
// saving on each key press.
func (a *app) autosave(c *ui.Context) {
	b, err := json.Marshal(a.data)
	if err != nil || string(b) == a.lastJS {
		return
	}
	now := time.Now()
	if now.Sub(a.last) < 350*time.Millisecond {
		c.After(400 * time.Millisecond)
		return
	}
	a.lastJS = string(b)
	a.last = now
	_ = a.store.Save(a.data)
}

func (a *app) toast(c *ui.Context, title, description, kind string) {
	c.AddToast(ui.Toast{Title: title, Description: description, Type: kind})
}

func (a *app) closePopovers() {
	a.envOpen = false
	a.wsOpen = false
	a.materialOpen = false
	a.themePickerOpen = false
	a.closeContextMenu()
}

func (a *app) setPage(page string) {
	a.closePopovers()
	a.page = page
	a.save()
}

func (a *app) openRequest(id string) {
	if w := a.ws(); w != nil {
		a.closePopovers()
		w.OpenTab(id)
		a.page = "request"
		a.save()
	}
}

func (a *app) newRequest() {
	w := a.ws()
	if w == nil {
		return
	}
	a.closePopovers()
	var col *core.Collection
	for i := range w.Collections {
		if w.Collections[i].Name == "Scratch" && w.Collections[i].ParentID == "" {
			col = &w.Collections[i]
		}
	}
	if col == nil {
		w.Collections = append(w.Collections, core.Collection{ID: core.NewID("col"), Name: "Scratch"})
		col = &w.Collections[len(w.Collections)-1]
	}
	r := core.BlankRequest("Untitled request")
	col.Requests = append(col.Requests, r)
	delete(a.collapsed, col.ID)
	w.OpenTab(r.ID)
	a.page = "request"
	a.reqTab = 0
	a.save()
}

func (a *app) newWorkspace(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Untitled workspace"
	}
	w := core.Workspace{
		ID: core.NewID("ws"), Name: name, OpenTabs: []string{},
		// A new workspace starts with the look you are already in, not a reset.
		Theme:    a.data.Settings.Theme,
		Material: a.data.Settings.Material,
	}
	a.data.Workspaces = append(a.data.Workspaces, w)
	a.data.ActiveWorkspace = w.ID
	a.closePopovers()
	a.page = "request"
	a.modal = ""
	a.save()
}

func (a *app) newEnvironment(name string) {
	w := a.ws()
	if w == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "New environment"
	}
	e := core.Environment{ID: core.NewID("env"), Name: name, Vars: []core.KV{{ID: core.NewID("v"), Key: "baseUrl", Value: "https://", Enabled: true}}}
	w.Environments = append(w.Environments, e)
	w.ActiveEnv = e.ID
	a.modal = ""
	a.save()
}

func (a *app) newCollection(name string) {
	a.newCollectionIn(name, "")
}

func (a *app) newCollectionIn(name, parentID string) {
	w := a.ws()
	if w == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if parentID != "" {
			name = "New folder"
		} else {
			name = "New collection"
		}
	}
	if parentID != "" && w.Collection(parentID) == nil {
		parentID = ""
	}
	w.Collections = append(w.Collections, core.Collection{ID: core.NewID("col"), Name: name, ParentID: parentID})
	if parentID != "" {
		delete(a.collapsed, parentID)
	}
	a.modalParent = ""
	a.modal = ""
	a.save()
}

// moveRequest moves a request into another collection or folder, appending it.
// The tab stays open so the request does not disappear from under the editor.
func (a *app) moveRequest(sourceID, requestID, targetID string) {
	a.reorderRequest(sourceID, requestID, targetID, -1)
}

// reorderRequest moves a request to a position in a collection. before is the
// index it should land at; -1 or len appends. Moving within one collection
// adjusts the index after removal so dropping below a row lands below it.
func (a *app) reorderRequest(sourceID, requestID, targetID string, before int) {
	w := a.ws()
	if w == nil {
		return
	}
	source, target := w.Collection(sourceID), w.Collection(targetID)
	if source == nil || target == nil {
		return
	}
	from := -1
	for i := range source.Requests {
		if source.Requests[i].ID == requestID {
			from = i
			break
		}
	}
	if from < 0 {
		return
	}
	r := source.Requests[from]
	source.Requests = append(source.Requests[:from], source.Requests[from+1:]...)
	if source == target && before > from {
		before--
	}
	if before < 0 || before > len(target.Requests) {
		before = len(target.Requests)
	}
	target.Requests = append(target.Requests, core.Request{})
	copy(target.Requests[before+1:], target.Requests[before:])
	target.Requests[before] = r
	delete(a.collapsed, targetID)
	a.save()
}

func (a *app) canMoveCollection(id, parentID string) bool {
	w := a.ws()
	if w == nil {
		return false
	}
	col := w.Collection(id)
	if col == nil || col.ParentID == parentID {
		return false
	}
	if parentID == "" {
		return true
	}
	if parentID == id || w.Collection(parentID) == nil {
		return false
	}
	return !w.InSubtree(id, parentID)
}

// moveCollection nests a folder under another collection, or moves it back to
// the top level when parentID is "".
func (a *app) moveCollection(id, parentID string) {
	if !a.canMoveCollection(id, parentID) {
		return
	}
	w := a.ws()
	if parentID != "" {
		delete(a.collapsed, parentID)
	}
	w.Collection(id).ParentID = parentID
	a.save()
}

func (a *app) addRequestTo(col *core.Collection) {
	if col == nil {
		return
	}
	a.closePopovers()
	r := core.BlankRequest("Untitled request")
	col.Requests = append(col.Requests, r)
	delete(a.collapsed, col.ID)
	a.ws().OpenTab(r.ID)
	a.page = "request"
	a.save()
}

func (a *app) deleteRequest(id string) {
	w := a.ws()
	for ci := range w.Collections {
		rs := w.Collections[ci].Requests
		for i := range rs {
			if rs[i].ID == id {
				w.Collections[ci].Requests = append(rs[:i], rs[i+1:]...)
				w.CloseTab(id)
				delete(a.responses, id)
				delete(a.bodySyntax, id)
				delete(a.responseSyntax, id)
				a.save()
				return
			}
		}
	}
}

func (a *app) duplicateRequest(id string) {
	w := a.ws()
	r, col := w.Request(id)
	if r == nil {
		return
	}
	cp := *r
	cp.ID = core.NewID("req")
	cp.Name = r.Name + " copy"
	cp.Headers = append([]core.KV(nil), r.Headers...)
	for i := range cp.Headers {
		cp.Headers[i].ID = core.NewID("h")
	}
	idx := len(col.Requests)
	for i := range col.Requests {
		if col.Requests[i].ID == id {
			idx = i + 1
		}
	}
	col.Requests = append(col.Requests, core.Request{})
	copy(col.Requests[idx+1:], col.Requests[idx:])
	col.Requests[idx] = cp
	w.OpenTab(cp.ID)
	a.save()
}

func (a *app) sendActive() {
	r, col := a.activeRequest()
	if r == nil || a.sending[r.ID] {
		return
	}
	w := a.ws()
	// The history entry keeps the request as written, so replaying it still
	// inherits. Only the copy handed to the client gets the resolved headers.
	rec := *r
	rec.Headers = append([]core.KV(nil), r.Headers...)
	rec.Query = append([]core.KV(nil), r.Query...)
	send := rec
	w.ApplyAuth(&send, col)
	vars := w.ActiveVars()
	settings := a.data.Settings
	var cert *core.Certificate
	if cp := core.CertificateForURL(a.data.Certificates, core.Interpolate(send.URL, vars)); cp != nil {
		c := *cp
		cert = &c
	}
	id := send.ID
	a.sending[id] = true
	delete(a.responses, id)
	a.save()
	go func() {
		res := a.store.Send(send, vars, settings, cert)
		if a.win == nil {
			return
		}
		a.win.Update(func() {
			delete(a.sending, id)
			a.responses[id] = res
			a.addHistory(rec, res)
			a.save()
		})
	}()
}

func (a *app) addHistory(req core.Request, res core.SendResult) {
	w := a.ws()
	if w == nil {
		return
	}
	body := res.Body
	if !a.data.Settings.PersistBody {
		body = ""
	} else if len(body) > 200_000 {
		body = body[:200_000] + "\n…"
	}
	h := core.History{
		ID: core.NewID("hist"), RequestID: req.ID, Method: req.Method, URL: res.FinalURL,
		Status: res.StatusCode, Duration: res.Duration, Size: res.Size,
		At: time.Now().Format(time.RFC3339), Body: body,
		Headers: core.HeadersText(res.Headers), Request: req,
	}
	if h.URL == "" {
		h.URL = req.URL
	}
	w.History = append([]core.History{h}, w.History...)
	if len(w.History) > 80 {
		w.History = w.History[:80]
	}
}

func (a *app) importCertificate(path, password, host, name string) {
	info, err := a.store.ImportPFX(path, password, host)
	if err != nil {
		a.modalError = err.Error()
		return
	}
	if strings.TrimSpace(name) != "" {
		info.Name = strings.TrimSpace(name)
	}
	a.data.Certificates = append(a.data.Certificates, info)
	a.modal = ""
	a.pendingCertPath = ""
	a.pendingCertInfo = nil
	a.save()
	if a.win != nil {
		a.win.Update(func() {})
	}
}

func (a *app) deleteCertificate(id string) {
	for i := range a.data.Certificates {
		if a.data.Certificates[i].ID == id {
			_ = core.RemoveFile(filepath.Join(a.store.CertDir(), a.data.Certificates[i].FileName))
			a.store.DeletePassword(id)
			a.data.Certificates = append(a.data.Certificates[:i], a.data.Certificates[i+1:]...)
			a.save()
			return
		}
	}
}

func (a *app) switchWorkspace(id string) {
	w := a.data.Workspace(id)
	if w == nil {
		return
	}
	a.data.ActiveWorkspace = w.ID
	a.applyWorkspaceAppearance()
	a.closePopovers()
	a.page = "request"
	a.modal = ""
	a.save()
}

// applyWorkspaceAppearance makes the active workspace's theme and window
// material the effective ones. Settings is the single place the rest of the
// app reads, so it is synced here instead of at every call site.
func (a *app) applyWorkspaceAppearance() {
	w := a.ws()
	if w == nil {
		return
	}
	a.data.Settings.Theme = w.Theme
	a.data.Settings.Material = w.Material
	a.data.Settings.Acrylic = w.Material == "acrylic"
	a.themeKey = ""
	a.applyWindowMaterials()
	a.refreshWindows()
}

func (a *app) currentResponse() (core.SendResult, bool) {
	r, _ := a.activeRequest()
	if r == nil {
		return core.SendResult{}, false
	}
	res, ok := a.responses[r.ID]
	return res, ok
}

func (a *app) openCollectionAuth(col *core.Collection) {
	if col == nil {
		return
	}
	a.modal = "collection_auth"
	a.modalTarget = col.ID
	a.modalAuth = col.AuthType
	if a.modalAuth == "" {
		a.modalAuth = "inherit"
	}
	a.input1 = col.Bearer
	a.input2 = col.Username
	a.input3 = col.Password
	a.modalError = ""
}

func (a *app) openConfirm(title, message string, fn func()) {
	a.modal = "confirm"
	a.modalTitle = title
	a.modalInfo = message
	a.confirm = fn
	a.modalError = ""
}

func (a *app) runConfirm() {
	if a.confirm != nil {
		fn := a.confirm
		a.confirm = nil
		a.modal = ""
		fn()
	}
}

func (a *app) sendSummary() string {
	return fmt.Sprintf("%d requests", len(a.ws().History))
}
