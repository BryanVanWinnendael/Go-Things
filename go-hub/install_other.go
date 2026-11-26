//go:build !windows

package main

import "errors"

// Go Hub manages per-user Windows installs; on other platforms it still
// builds so the store can be looked at, but no app can be installed.

var errUnsupported = errors.New("Go Hub installs Windows apps. Run it on Windows 10 or 11.")

func readInstalled(app suiteApp) (installedInfo, bool) { return installedInfo{}, false }

func runSetup(path string) error { return errUnsupported }

func runUninstaller(info installedInfo) error { return errUnsupported }

func launchApp(app suiteApp, info installedInfo) error { return errUnsupported }
