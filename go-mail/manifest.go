// Package manifest exposes the app's mygo.json manifest and the icon shipped
// in resources/, both embedded from the app root.
package manifest

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo"
)

// appManifest is mygo.json, embedded so the app can show the same version the
// installer was built with. mygo build reports it from a link-time flag, but
// that is empty when the app runs from source; the manifest works both ways.
//
//go:embed mygo.json
var appManifest []byte

// appIconPNG is the app icon used by the loading screen.
//
//go:embed resources/icon.png
var appIconPNG []byte

// IconPNG returns the embedded app icon.
func IconPNG() []byte {
	return appIconPNG
}

// Version is the version in the manifest at build time. It falls back to the
// link-time version mygo sets when the manifest has none.
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
	if v := Version(); v != "" {
		return "Go Mail " + v
	}
	return "Go Mail"
}
