package app

import manifest "go-post"

// appVersion is the version the manifest was built with.
var appVersion = manifest.Version()

// versionLabel names the app with its version, or without one when none is
// known, so the UI never shows a dangling separator.
func versionLabel() string {
	if appVersion == "" {
		return "Go Post"
	}
	return "Go Post " + appVersion
}
