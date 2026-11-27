package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Doc is the JSON file published in each GitHub release. The order of
// Apps is meaningful: the first one is the featured app on the store page.
type Doc struct {
	SchemaVersion int    `json:"schemaVersion"`
	Updated       string `json:"updated"`
	Apps          []App  `json:"apps"`
}

var (
	catalogClient = &http.Client{Timeout: 25 * time.Second}
	AssetClient   = &http.Client{Timeout: 3 * time.Minute}
	headClient    = &http.Client{Timeout: 15 * time.Second}
	setupClient   = &http.Client{Timeout: 15 * time.Minute}

	appIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	accentPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	CacheClean    = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
)

// loadCatalog refreshes the store from the latest GitHub release. It never
// blocks the UI: the built-in list stays visible until the new one arrives.
func Fetch() (Doc, error) {
	// The timestamp keeps GitHub's CDN from replaying yesterday's
	raw := URL + "?t=" + strconv.FormatInt(time.Now().UnixNano(), 10)
	b, err := GetBytes(catalogClient, raw)
	if err != nil {
		return Doc{}, fmt.Errorf("Could not read the app catalog from GitHub: %w", err)
	}
	var doc Doc
	if err := json.Unmarshal(b, &doc); err != nil {
		return Doc{}, fmt.Errorf("The app catalog from GitHub is not valid JSON: %w", err)
	}
	if err := Validate(doc); err != nil {
		return Doc{}, fmt.Errorf("The app catalog from GitHub is not valid: %w", err)
	}
	return doc, nil
}

func Validate(doc Doc) error {
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
		if !IsHTTPS(app.Setup) {
			return fmt.Errorf("app %q needs an https setup URL", app.ID)
		}
		for label, ref := range map[string]string{
			"preview": app.Preview,
			"hero":    app.Hero,
			"icon":    app.Icon,
		} {
			if ref != "" && !IsHTTPS(ref) {
				return fmt.Errorf("app %q has a %s that is not an https URL", app.ID, label)
			}
		}
		for _, ref := range app.Screenshots {
			if !IsHTTPS(ref) {
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
func SetupSize(ref string) string {
	if !IsHTTPS(ref) {
		return ""
	}
	req, err := http.NewRequest(http.MethodHead, ref, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "GoHub/"+Version)
	resp, err := headClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
		return ""
	}
	return FormatSize(resp.ContentLength)
}

// fetchSetup downloads the catalog's installer into a local cache. Go Hub
// does not ship other apps' installers or pictures.
func URLPath(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Path
}

func Download(raw, dest string) error {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "GoHub/"+Version)
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
	if !SetupOK(dest) {
		return errors.New("GitHub did not return a Windows installer")
	}
	return nil
}

// SetupOK accepts an existing download only when it still looks like the
// NSIS executable the release is expected to carry.
func SetupOK(file string) bool {
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

func GetBytes(client *http.Client, raw string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "GoHub/"+Version)
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

func IsHTTPS(ref string) bool {
	u, err := url.Parse(ref)
	return err == nil && u.Scheme == "https" && u.Host != ""
}
