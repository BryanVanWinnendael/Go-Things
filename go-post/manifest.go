// Package manifest exposes the version of the app as recorded in mygo.json.
package manifest

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo"
)

// appManifest is mygo.json, embedded so the app can show the same version the
// installer was built with. `mygo build` reports it from a link-time flag, but
// that is empty when the app runs from source; the manifest works both ways.
//
//go:embed mygo.json
var appManifest []byte

// Version returns the manifest version, or the mygo link-time version when the
// manifest has none.
func Version() string {
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(appManifest, &cfg); err == nil {
		if v := strings.TrimSpace(cfg.Version); v != "" {
			return v
		}
	}
	return strings.TrimSpace(mygo.App.Version())
}

// Label names the app with its version, or without one when none is known, so
// the UI never shows a dangling separator.
func Label() string {
	v := Version()
	if v == "" {
		return "Go Post"
	}
	return "Go Post " + v
}
