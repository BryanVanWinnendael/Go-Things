package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// catalogDoc is the JSON file published in each GitHub release. The order of
// Apps is meaningful: the first one is the featured app on the store page.
type catalogDoc struct {
	SchemaVersion int        `json:"schemaVersion"`
	Updated       string     `json:"updated"`
	Apps          []suiteApp `json:"apps"`
}

var (
	catalogClient = &http.Client{Timeout: 25 * time.Second}
	assetClient   = &http.Client{Timeout: 3 * time.Minute}
	headClient    = &http.Client{Timeout: 15 * time.Second}
	setupClient   = &http.Client{Timeout: 15 * time.Minute}

	appIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	accentPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	cacheClean    = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
)

// loadCatalog refreshes the store from the latest GitHub release. It never
// blocks the UI: the built-in list stays visible until the new one arrives.
func (a *App) loadCatalog() {
	if a.catalogLoading || a.win == nil {
		return
	}
	a.catalogLoading = true
	go func() {
		doc, err := fetchCatalog()
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

func fetchCatalog() (catalogDoc, error) {
	// The timestamp keeps GitHub's CDN from replaying yesterday's catalog.
	raw := catalogURL + "?t=" + strconv.FormatInt(time.Now().UnixNano(), 10)
	b, err := getBytes(catalogClient, raw)
	if err != nil {
		return catalogDoc{}, fmt.Errorf("Could not read the app catalog from GitHub: %w", err)
	}
	var doc catalogDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return catalogDoc{}, fmt.Errorf("The app catalog from GitHub is not valid JSON: %w", err)
	}
	if err := validateCatalog(doc); err != nil {
		return catalogDoc{}, fmt.Errorf("The app catalog from GitHub is not valid: %w", err)
	}
	return doc, nil
}

func validateCatalog(doc catalogDoc) error {
	if doc.SchemaVersion != 1 {
		return fmt.Errorf("it uses schema %d, but this Go Hub understands schema 1", doc.SchemaVersion)
	}
	if len(doc.Apps) == 0 {
		return errors.New("it contains no apps")
	}
	seen := map[string]bool{}
	for i := range doc.Apps {
		app := &doc.Apps[i]
		app.ID = strings.TrimSpace(app.ID)
		app.Name = strings.TrimSpace(app.Name)
		app.Version = strings.TrimSpace(app.Version)
		if !appIDPattern.MatchString(app.ID) {
			return fmt.Errorf("app %d has an id that is not lowercase letters, digits or hyphens", i+1)
		}
		if seen[app.ID] {
			return fmt.Errorf("app %q appears more than once", app.ID)
		}
		seen[app.ID] = true
		if app.Name == "" || app.Version == "" {
			return fmt.Errorf("app %q needs a name and a version", app.ID)
		}
		if app.Identifier == "" || app.ExeName == "" {
			return fmt.Errorf("app %q needs an identifier and an exe name", app.ID)
		}
		if !isHTTPS(app.Setup) {
			return fmt.Errorf("app %q needs an https setup URL", app.ID)
		}
		for label, ref := range map[string]string{
			"preview": app.Preview,
			"hero":    app.Hero,
			"icon":    app.Icon,
		} {
			if ref != "" && !isHTTPS(ref) {
				return fmt.Errorf("app %q has a %s that is not an https URL", app.ID, label)
			}
		}
		for _, ref := range app.Screenshots {
			if !isHTTPS(ref) {
				return fmt.Errorf("app %q has a screenshot that is not an https URL", app.ID)
			}
		}
		if app.Accent != "" && !accentPattern.MatchString(app.Accent) {
			return fmt.Errorf("app %q has an accent that is not a #rrggbb colour", app.ID)
		}
	}
	return nil
}

// loadAssets downloads the pictures and measures the setups in the background.
func (a *App) loadAssets() {
	if a.win == nil {
		return
	}
	a.assetsGen++
	gen := a.assetsGen
	apps := append([]suiteApp(nil), a.apps...)
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
			if !isHTTPS(ref) {
				return nil
			}
			b, err := getBytes(assetClient, ref)
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
			sizes[app.ID] = setupSize(app.Setup)
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

// setupSize asks GitHub how large the installer is without downloading it.
func setupSize(ref string) string {
	if !isHTTPS(ref) {
		return ""
	}
	req, err := http.NewRequest(http.MethodHead, ref, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "GoHub/"+hubVersion)
	resp, err := headClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
		return ""
	}
	return formatSize(resp.ContentLength)
}

// fetchSetup downloads the catalog's installer into a local cache. Go Hub
// does not ship other apps' installers or pictures.
func (a *App) fetchSetup(app suiteApp) (string, error) {
	if !isHTTPS(app.Setup) {
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

	base := path.Base(mustURLPath(app.Setup))
	if base == "." || base == "/" || base == "" {
		base = app.ID + ".exe"
	}
	name := app.ID + "-" + cacheClean.ReplaceAllString(app.Version, "-") + "-" + base
	dest := filepath.Join(dir, name)
	if setupOK(dest) {
		return dest, nil
	}

	part := dest + ".part"
	if err := downloadFile(app.Setup, part); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	if err := os.Rename(part, dest); err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("Could not finish the download: %w", err)
	}
	return dest, nil
}

func mustURLPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Path
}

func downloadFile(raw, dest string) error {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "GoHub/"+hubVersion)
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := setupClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub did not answer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub returned %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); strings.Contains(strings.ToLower(ct), "text/html") {
		return errors.New("GitHub returned a web page instead of the installer")
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("Could not save the installer: %w", err)
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("The download stopped early: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("Could not save the installer: %w", closeErr)
	}
	if n < 1024 {
		return errors.New("the downloaded file is too small to be an installer")
	}
	if !setupOK(dest) {
		return errors.New("GitHub did not return a Windows installer")
	}
	return nil
}

// setupOK accepts an existing download only when it still looks like the
// NSIS executable the release is expected to carry.
func setupOK(file string) bool {
	st, err := os.Stat(file)
	if err != nil || st.IsDir() || st.Size() < 1024 {
		return false
	}
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 2)
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return head[0] == 'M' && head[1] == 'Z'
}

func getBytes(client *http.Client, raw string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "GoHub/"+hubVersion)
	req.Header.Set("Cache-Control", "no-cache, no-store")
	req.Header.Set("Pragma", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func isHTTPS(ref string) bool {
	u, err := url.Parse(ref)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
