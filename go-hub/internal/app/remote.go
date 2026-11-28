package app

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
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
			if doc.Hub.Version != "" {
				a.hub = doc.Hub
			}
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
		previewsLight := map[string]*ui.Bitmap{}
		logos := map[string]*ui.Bitmap{}
		shots := map[string][]*ui.Bitmap{}
		shotsLight := map[string][]*ui.Bitmap{}
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
		// Both themes are loaded up front, so switching the theme only repaints
		// the window and never has to wait for a download.
		loadCards := func(dst map[string]*ui.Bitmap, app catalog.App, dark bool) {
			if bmp := load(app.PreviewRef(dark)); bmp != nil {
				dst[app.ID] = bmp
			}
			if bmp := load(app.HeroRef(dark)); bmp != nil {
				dst[app.ID+heroSuffix] = bmp
			}
			if _, ok := dst[app.ID+heroSuffix]; !ok {
				if bmp := dst[app.ID]; bmp != nil {
					dst[app.ID+heroSuffix] = bmp
				}
			}
		}
		loadShots := func(refs []string) []*ui.Bitmap {
			var list []*ui.Bitmap
			for _, ref := range refs {
				if bmp := load(ref); bmp != nil {
					list = append(list, bmp)
				}
			}
			return list
		}
		for _, app := range apps {
			loadCards(previews, app, true)
			loadCards(previewsLight, app, false)
			if bmp := load(app.Icon); bmp != nil {
				logos[app.ID] = bmp
			}
			if list := loadShots(app.ScreenshotRefs(true)); len(list) > 0 {
				shots[app.ID] = list
			}
			if list := loadShots(app.ScreenshotRefs(false)); len(list) > 0 {
				shotsLight[app.ID] = list
			}
			sizes[app.ID] = catalog.SetupSize(app.Setup)
		}
		a.win.Update(func() {
			if gen != a.assetsGen {
				return
			}
			a.previews = previews
			a.previewsLight = previewsLight
			a.logos = logos
			a.shots = shots
			a.shotsLight = shotsLight
			a.sizes = sizes
			a.win.Invalidate()
		})
	}()
}

// setupCacheName includes the catalog revision so a new release cannot reuse
// an installer cached under an older catalog.
func setupCacheName(app catalog.App, updated string) string {
	base := path.Base(catalog.URLPath(app.Setup))
	if base == "." || base == "/" || base == "" {
		base = app.ID + ".exe"
	}
	parts := []string{app.ID, catalog.CacheClean.ReplaceAllString(app.Version, "-")}
	if rev := catalog.CacheClean.ReplaceAllString(updated, "-"); rev != "" {
		parts = append(parts, rev)
	}
	parts = append(parts, base)
	return strings.Join(parts, "-")
}

// fetchSetup downloads the catalog's installer into a local cache.
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

	name := setupCacheName(app, a.catalogUpdated)
	dest := filepath.Join(dir, name)
	if catalog.SetupOK(dest) {
		// A catalog can name the same version and file name while the asset
		// itself was replaced (a corrected identifier, for instance). Check
		// the size the host reports before trusting the cached copy.
		keep := true
		if n, ok := catalog.SetupLength(app.Setup); ok {
			if st, err := os.Stat(dest); err != nil || st.Size() != n {
				keep = false
			}
		}
		if keep {
			return dest, nil
		}
	}

	part := dest + ".part"
	if err := catalog.Download(app.Setup, part); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	_ = os.Remove(dest)
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
