package app

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"

	"go-hub/internal/catalog"
)

// loadCatalog refreshes the store from the latest GitHub release. It never
// blocks the UI: the built-in list stays visible until the new one arrives.
func (a *App) loadCatalog() {
	if a.catalogLoading || a.win == nil {
		return
	}
	a.catalogLoading = true
	go func() {
		doc, err := catalog.Fetch()
		a.win.Update(func() {
			a.catalogLoading = false
			if err != nil {
				a.catalogErr = err.Error()
				a.win.Invalidate()
				return
			}
			a.catalogErr = ""
			a.catalogUpdated = doc.Updated
			a.lastCatalog = time.Now()
			a.apps = doc.Apps
			if a.detailID != "" {
				if _, ok := a.appByID(a.detailID); !ok {
					a.goPage(pageStore)
				}
			}
			a.loadAssets()
			a.refresh()
			a.win.Invalidate()
		})
	}()
}

// refreshAll checks the catalog and the installed state together.
func (a *App) refreshAll() {
	a.refresh()
	a.loadCatalog()
}

// loadAssets downloads the pictures and measures the setups in the background.
func (a *App) loadAssets() {
	if a.win == nil {
		return
	}
	a.assetsGen++
	gen := a.assetsGen
	apps := append([]catalog.App(nil), a.apps...)
	go func() {
		previews := map[string]*ui.Bitmap{}
		logos := map[string]*ui.Bitmap{}
		shots := map[string][]*ui.Bitmap{}
		sizes := map[string]string{}
		cache := map[string]*ui.Bitmap{}
		load := func(ref string) *ui.Bitmap {
			if ref == "" {
				return nil
			}
			if bmp, ok := cache[ref]; ok {
				return bmp
			}
			if !catalog.IsHTTPS(ref) {
				return nil
			}
			b, err := catalog.GetBytes(catalog.AssetClient, ref)
			if err != nil {
				return nil
			}
			bmp, err := ui.DecodeBitmap(b)
			if err != nil {
				return nil
			}
			cache[ref] = bmp
			return bmp
		}
		for _, app := range apps {
			if bmp := load(app.Preview); bmp != nil {
				previews[app.ID] = bmp
			}
			if bmp := load(app.Hero); bmp != nil {
				previews[app.ID+heroSuffix] = bmp
			}
			if _, ok := previews[app.ID+heroSuffix]; !ok {
				if bmp := previews[app.ID]; bmp != nil {
					previews[app.ID+heroSuffix] = bmp
				}
			}
			if bmp := load(app.Icon); bmp != nil {
				logos[app.ID] = bmp
			}
			var list []*ui.Bitmap
			for _, ref := range app.Screenshots {
				if bmp := load(ref); bmp != nil {
					list = append(list, bmp)
				}
			}
			if len(list) > 0 {
				shots[app.ID] = list
			}
			sizes[app.ID] = catalog.SetupSize(app.Setup)
		}
		a.win.Update(func() {
			if gen != a.assetsGen {
				return
			}
			a.previews = previews
			a.logos = logos
			a.shots = shots
			a.sizes = sizes
			a.win.Invalidate()
		})
	}()
}

// catalog.SetupSize asks GitHub how large the installer is without downloading it.
func (a *App) fetchSetup(app catalog.App) (string, error) {
	if !catalog.IsHTTPS(app.Setup) {
		return "", fmt.Errorf("%s's setup is not hosted at an https URL.", app.Name)
	}

	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	} else {
		dir = filepath.Join(dir, "go-hub", "downloads")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("Could not create the download folder: %w", err)
	}

	base := path.Base(catalog.URLPath(app.Setup))
	if base == "." || base == "/" || base == "" {
		base = app.ID + ".exe"
	}
	name := app.ID + "-" + catalog.CacheClean.ReplaceAllString(app.Version, "-") + "-" + base
	dest := filepath.Join(dir, name)
	if catalog.SetupOK(dest) {
		return dest, nil
	}

	part := dest + ".part"
	if err := catalog.Download(app.Setup, part); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	if err := os.Rename(part, dest); err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("Could not finish the download: %w", err)
	}
	return dest, nil
}

func (a *App) appByID(id string) (catalog.App, bool) {
	for _, app := range a.apps {
		if app.ID == id {
			return app, true
		}
	}
	return catalog.App{}, false
}
