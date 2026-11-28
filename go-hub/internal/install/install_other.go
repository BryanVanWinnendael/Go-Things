//go:build !windows

package install

import (
	"errors"

	"go-hub/internal/catalog"
)

// Go Hub manages per-user Windows installs; on other platforms it still
// builds so the store can be looked at, but no app can be installed.

var errUnsupported = errors.New("Go Hub installs Windows apps. Run it on Windows 10 or 11.")

func ReadInstalled(app catalog.App) (Info, bool) { return Info{}, false }

func RunSetup(path string) error { return errUnsupported }

func StartSetup(path string) error { return errUnsupported }

func RunUninstaller(info Info) error { return errUnsupported }

func LaunchApp(app catalog.App, info Info) error { return errUnsupported }
