// Package manifest exposes the icon shipped in resources/ from the app root.
package manifest

import _ "embed"

// iconPNG is Go Hub's own logo. MyGo also copies resources/icon.png next to
// the executable, but embedding it keeps the sidebar icon available in a
// source build and in any install layout.
//
//go:embed resources/icon.png
var iconPNG []byte

// IconPNG returns the embedded app icon.
func IconPNG() []byte { return iconPNG }
