package main

// resolvedAuth is the authorization a request actually sends. When a request
// inherits, Source is the folder or collection that supplied it, so the UI can
// say where it comes from.
type resolvedAuth struct {
	Type     string
	Bearer   string
	Username string
	Password string
	Source   *Collection
}

// resolveAuth walks from the request outwards: its own choice wins, otherwise
// the nearest folder with a concrete type does, and an explicit "No auth" on a
// folder stops the walk.
func (w *Workspace) resolveAuth(r *Request, col *Collection) resolvedAuth {
	switch r.AuthType {
	case "none", "bearer", "basic":
		return resolvedAuth{
			Type: r.AuthType, Bearer: r.Bearer,
			Username: r.Username, Password: r.Password,
		}
	}
	seen := map[string]bool{}
	for c := col; c != nil && !seen[c.ID]; c = w.collection(c.ParentID) {
		seen[c.ID] = true
		switch c.AuthType {
		case "bearer", "basic":
			return resolvedAuth{
				Type: c.AuthType, Bearer: c.Bearer,
				Username: c.Username, Password: c.Password, Source: c,
			}
		case "none":
			return resolvedAuth{Type: "none", Source: c}
		}
	}
	return resolvedAuth{Type: "none"}
}

// applyAuth writes the resolved authorization into a copy of the request, which
// is what the HTTP client sends. The stored request keeps its "inherit" choice.
func (w *Workspace) applyAuth(r *Request, col *Collection) {
	a := w.resolveAuth(r, col)
	r.AuthType, r.Bearer, r.Username, r.Password = a.Type, a.Bearer, a.Username, a.Password
}

func authLabel(t string) string {
	switch t {
	case "bearer":
		return "Bearer token"
	case "basic":
		return "Basic auth"
	}
	return "No auth"
}

// authOptions are the request and folder authorization choices, in menu order.
var authOptions = []struct{ key, label string }{
	{"inherit", "Inherit from parent"},
	{"none", "No auth"},
	{"bearer", "Bearer token"},
	{"basic", "Basic auth"},
}

func authOptionIndex(key string) int {
	for i, opt := range authOptions {
		if opt.key == key {
			return i
		}
	}
	return 0
}
