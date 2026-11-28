package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo"

	"go-hub/internal/catalog"
	"go-hub/internal/install"
)

// refresh reads the installed state of every app in the background and
// hands the result to the UI thread.
func (a *App) refresh() {
	if a.refreshing || a.win == nil {
		return
	}
	a.refreshing = true
	apps := append([]catalog.App(nil), a.apps...)
	go func() {
		found := map[string]install.Info{}
		for _, app := range apps {
			if info, ok := install.ReadInstalled(app); ok {
				found[app.ID] = info
			}
		}
		a.win.Update(func() {
			a.installs = found
			a.refreshing = false
		})
	}()
}

// startInstall downloads the setup named by the latest catalog, runs it
// silently, and waits until Windows lists the app. The card turns installed
// only when the app is actually there.
func (a *App) startInstall(app catalog.App) {
	if a.busy[app.ID] != "" {
		return
	}
	a.busy[app.ID] = "download"
	a.errors[app.ID] = ""
	a.confirm = ""
	if a.win != nil {
		a.win.Invalidate()
	}
	go func() {
		setup, err := a.fetchSetup(app)
		if err != nil {
			a.fail(app, fmt.Errorf("Could not download %s: %w", app.Name, err))
			return
		}
		a.win.Update(func() {
			a.busy[app.ID] = "install"
			a.win.Invalidate()
		})
		if err := install.RunSetup(setup); err != nil {
			a.fail(app, err)
			return
		}
		info, ok := waitInstalled(app, 30*time.Second)
		if !ok {
			a.fail(app, errors.New("The setup finished, but Windows did not list "+app.Name+". Run the install again, or check Windows' installed apps."))
			return
		}
		a.win.Update(func() {
			a.busy[app.ID] = ""
			a.installs[app.ID] = info
			a.errors[app.ID] = ""
		})
	}()
}

// startUninstall removes the app and waits until the uninstall key is gone.
func (a *App) startUninstall(app catalog.App) {
	info, ok := a.installs[app.ID]
	if !ok || a.busy[app.ID] != "" {
		return
	}
	a.busy[app.ID] = "uninstall"
	a.errors[app.ID] = ""
	a.confirm = ""
	if a.win != nil {
		a.win.Invalidate()
	}
	go func() {
		if err := install.RunUninstaller(info); err != nil {
			a.fail(app, err)
			return
		}
		if !waitGone(app, 30*time.Second) {
			a.fail(app, errors.New(app.Name+" still appears in Windows' installed apps. Windows may still be removing it; refresh in a moment."))
			return
		}
		a.win.Update(func() {
			a.busy[app.ID] = ""
			delete(a.installs, app.ID)
			a.errors[app.ID] = ""
		})
	}()
}

func (a *App) launch(app catalog.App) {
	info, ok := a.installs[app.ID]
	if !ok {
		return
	}
	if err := install.LaunchApp(app, info); err != nil {
		a.errors[app.ID] = err.Error()
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

// openFolder reveals an installed app's folder in File Explorer.
func (a *App) openFolder(app catalog.App) {
	info, ok := a.installs[app.ID]
	if !ok || info.Location == "" {
		return
	}
	if err := mygo.Shell.OpenPath(info.Location); err != nil {
		a.errors[app.ID] = "Could not open the install folder."
	}
}

// fail records an error for one app without disturbing the others.
func (a *App) fail(app catalog.App, err error) {
	a.win.Update(func() {
		a.busy[app.ID] = ""
		a.errors[app.ID] = err.Error()
	})
}

// hubUpdateAvailable compares the catalog's hub version with the build that
// is running now. Go Hub is not one of the listed apps; it only has this
// version/setup pair in the catalog.
func (a *App) hubUpdateAvailable() bool {
	return a.hub.Version != "" && catalog.IsNewer(a.hub.Version, catalog.Version)
}

// hubApp makes a catalog.App just for the download helper. The hub is never
// added to the store list or the library.
func (a *App) hubApp() catalog.App {
	return catalog.App{ID: catalog.SelfID, Name: "Go Hub", Version: a.hub.Version, Setup: a.hub.Setup}
}

// startSelfUpdate downloads the hub's own setup and lets it run. The NSIS
// installer closes this process before replacing the files, so there is no
// registry poll here: the new version is visible after the hub reopens.
func (a *App) startSelfUpdate() {
	if a.busy[catalog.SelfID] != "" {
		return
	}
	a.busy[catalog.SelfID] = "download"
	if a.win != nil {
		a.win.Invalidate()
	}
	app := a.hubApp()
	go func() {
		setup, err := a.fetchSetup(app)
		if err != nil {
			a.fail(app, fmt.Errorf("Could not download the Go Hub update: %w", err))
			return
		}
		a.win.Update(func() {
			a.busy[catalog.SelfID] = "install"
			a.win.Invalidate()
		})
		if err := install.StartSetup(setup); err != nil {
			a.fail(app, err)
		}
	}()
}

// waitInstalled polls the uninstall key until the app appears.
func waitInstalled(app catalog.App, timeout time.Duration) (install.Info, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if info, ok := install.ReadInstalled(app); ok && info.Version != "" {
			return info, true
		}
		if time.Now().After(deadline) {
			return install.Info{}, false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// waitGone polls the uninstall key until the app is no longer listed.
func waitGone(app catalog.App, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, ok := install.ReadInstalled(app); !ok {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}
