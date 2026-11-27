package app

import "go-mail"

// appIconPNG is the icon embedded at the app root, which go:embed cannot reach
// from this package directory.
var appIconPNG = manifest.IconPNG()

// versionLabel names the app with its version for the Settings and About UI.
func versionLabel() string {
	return manifest.Label()
}
