package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo"
)

// installedInfo is what Windows knows about a suite app after it is
// installed: read from the per-user uninstall key the setup writes.
type installedInfo struct {
	Version   string
	Location  string
	Uninstall string
	Icon      string
}

// refresh reads the installed state of every app in the background and
// hands the result to the UI thread.
func (a *App) refresh() {
	if a.refreshing || a.win == nil {
		return
	}
	a.refreshing = true
	apps := append([]suiteApp(nil), a.apps...)
	go func() {
		found := map[string]installedInfo{}
		for _, app := range apps {
			if info, ok := readInstalled(app); ok {
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
func (a *App) startInstall(app suiteApp) {
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
		if err := runSetup(setup); err != nil {
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
func (a *App) startUninstall(app suiteApp) {
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
		if err := runUninstaller(info); err != nil {
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

func (a *App) launch(app suiteApp) {
	info, ok := a.installs[app.ID]
	if !ok {
		return
	}
	if err := launchApp(app, info); err != nil {
		a.errors[app.ID] = err.Error()
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

// openFolder reveals an installed app's folder in File Explorer.
func (a *App) openFolder(app suiteApp) {
	info, ok := a.installs[app.ID]
	if !ok || info.Location == "" {
		return
	}
	if err := mygo.Shell.OpenPath(info.Location); err != nil {
		a.errors[app.ID] = "Could not open the install folder."
	}
}

// fail records an error for one app without disturbing the others.
func (a *App) fail(app suiteApp, err error) {
	a.win.Update(func() {
		a.busy[app.ID] = ""
		a.errors[app.ID] = err.Error()
	})
}

// waitInstalled polls the uninstall key until the app appears.
func waitInstalled(app suiteApp, timeout time.Duration) (installedInfo, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if info, ok := readInstalled(app); ok && info.Version != "" {
			return info, true
		}
		if time.Now().After(deadline) {
			return installedInfo{}, false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// waitGone polls the uninstall key until the app is no longer listed.
func waitGone(app suiteApp, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, ok := readInstalled(app); !ok {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}
