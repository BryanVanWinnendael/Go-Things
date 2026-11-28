package catalog

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is the Go Hub running now. When the catalog lists a newer version
// for itself, the store offers to update the hub.
const Version = "1.0.0"

// SelfID names Go Hub when a synthetic app is needed to download its setup.
const SelfID = "gohub"

// HubUpdate is the only information the catalog carries about Go Hub itself:
// the version to compare with the running one, and its installer URL.
type HubUpdate struct {
	Version string `json:"version"`
	Setup   string `json:"setup"`
}

// DefaultHub is used before the first catalog arrives and when GitHub cannot
// be reached.
func DefaultHub() HubUpdate {
	return HubUpdate{Version: Version, Setup: ReleaseAsset("go-hub-setup-1.0.0-amd64.exe")}
}

// The store is driven by catalog.json in this project's latest GitHub
// release. Attach a new catalog.json and the installers it names to a
// release and every installed Go Hub picks the change up on its next check.
const (
	repoOwner = "BryanVanWinnendael"
	repoName  = "Go-Things"
	repoURL   = "https://github.com/" + repoOwner + "/" + repoName
	// releaseBase is the stable address of the newest published release.
	releaseBase = repoURL + "/releases/latest/download/"
	URL         = releaseBase + "catalog.json"

	// Pictures are read from the repository, not attached to a release.
	repoBranch = "main"
	rawBase    = "https://raw.githubusercontent.com/" + repoOwner + "/" + repoName + "/" + repoBranch + "/"
)

func ReleaseAsset(name string) string {
	return releaseBase + name
}

// AppAsset points at an app's own resources/ folder in the repository.
func AppAsset(id, name string) string {
	folder := map[string]string{"gopost": "go-post", "gomail": "go-mail"}[id]
	if folder == "" {
		folder = id
	}
	return rawBase + folder + "/resources/" + name
}

// App is one product the hub can install. The values are the facts the
// storefront is allowed to state: what the app does, which version ships,
// and where its setup and preview live. The published catalog uses absolute
// https URLs for all of them; Go Hub itself keeps none of these files.
type App struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Short       string   `json:"short"`
	Tagline     string   `json:"tagline"`
	Facts       []string `json:"facts"`
	Features    []string `json:"features"`
	Version     string   `json:"version"`
	Identifier  string   `json:"identifier"`
	ExeName     string   `json:"exe"`
	Setup       string   `json:"setup"`
	Preview     string   `json:"preview"`
	Hero        string   `json:"hero"`
	Screenshots []string `json:"screenshots"`
	// Light variants of the same pictures. They are optional: a catalog with
	// only the original fields still works in both themes.
	PreviewLight     string   `json:"previewLight,omitempty"`
	HeroLight        string   `json:"heroLight,omitempty"`
	ScreenshotsLight []string `json:"screenshotsLight,omitempty"`
	// Featured puts the app in the large hero slot at the top of the store.
	Featured bool `json:"featured"`
	// Icon is the app's mark, shown on its tile when available.
	Icon string `json:"icon"`
	// Accent is the app's own accent colour, used for its tile and mark in
	// the hub instead of the hub's theme-extreme accent.
	Accent string `json:"accent"`
}

// PreviewRef picks the preview for the active theme. The light fields are
// optional, so an older catalog keeps its single image in both themes.
func (a App) PreviewRef(dark bool) string {
	if !dark && a.PreviewLight != "" {
		return a.PreviewLight
	}
	return a.Preview
}

// HeroRef picks the hero image for the active theme.
func (a App) HeroRef(dark bool) string {
	if !dark && a.HeroLight != "" {
		return a.HeroLight
	}
	return a.Hero
}

// ScreenshotRefs picks the gallery for the active theme.
func (a App) ScreenshotRefs(dark bool) []string {
	if !dark && len(a.ScreenshotsLight) > 0 {
		return a.ScreenshotsLight
	}
	return a.Screenshots
}

// IsNewer compares two dotted version numbers, so 1.10.0 beats 1.9.0 as
// expected. Missing or non-numeric parts count as zero.
func IsNewer(latest, current string) bool {
	l, c := versionParts(latest), versionParts(current)
	for i := 0; i < len(l) || i < len(c); i++ {
		x, y := 0, 0
		if i < len(l) {
			x = l[i]
		}
		if i < len(c) {
			y = c[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func versionParts(v string) []int {
	var out []int
	for _, part := range strings.Split(strings.TrimSpace(v), ".") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// Fallback keeps the store useful before the first catalog arrives,
// and when GitHub cannot be reached. It points at the same release URLs as a
// published catalog, so nothing has to be bundled with Go Hub.
var Fallback = []App{
	{
		ID:       "gopost",
		Name:     "Go Post",
		Short:    "HTTP workspace",
		Featured: true,
		Tagline:  "Send requests, keep them in collections, and switch environments without leaving the desk.",
		Facts:    []string{"Requests & collections", "Environments", "Local-first"},
		Features: []string{
			"Send HTTP requests: GET, POST, PUT, PATCH, DELETE and more",
			"Organise requests in collections and folders",
			"Switch between environments with variables",
			"Headers, params, body and auth per request",
			"Request history",
			"Import and export workspaces",
			"Customise the theme and look: palettes, your own themes, workspace colour and window material",
			"Local-first: no account, no cloud",
		},
		Version:          "1.0.0",
		Identifier:       "com.bryanvanwinnendael.gopost",
		ExeName:          "go-post.exe",
		Setup:            ReleaseAsset("go-post-setup-1.0.0-amd64.exe"),
		Preview:          AppAsset("gopost", "preview1.png"),
		Hero:             AppAsset("gopost", "preview1.png"),
		Screenshots:      []string{AppAsset("gopost", "preview1.png"), AppAsset("gopost", "preview2.png"), AppAsset("gopost", "preview3.png")},
		PreviewLight:     AppAsset("gopost", "preview-light1.png"),
		HeroLight:        AppAsset("gopost", "preview-light1.png"),
		ScreenshotsLight: []string{AppAsset("gopost", "preview-light1.png"), AppAsset("gopost", "preview-light2.png"), AppAsset("gopost", "preview-light3.png")},
		Icon:             AppAsset("gopost", "icon.png"),
		Accent:           "#10b981",
	},
	{
		ID:      "gomail",
		Name:    "Go Mail",
		Short:   "Mail client",
		Tagline: "A focused IMAP and SMTP client with several accounts and provider presets that set themselves up.",
		Facts:   []string{"IMAP & SMTP", "Multiple accounts", "Provider presets"},
		Features: []string{
			"IMAP and SMTP mail",
			"Several accounts at once",
			"Provider presets that set themselves up",
			"Folders, search and attachments",
			"Customise the theme and look: palettes, your own themes, stage colour and window material",
			"Local-first: no account, no cloud",
		},
		Version:          "1.0.0",
		Identifier:       "com.bryanvanwinnendael.gomail",
		ExeName:          "go-mail.exe",
		Setup:            ReleaseAsset("go-mail-setup-1.0.0-amd64.exe"),
		Preview:          AppAsset("gomail", "preview1.png"),
		Hero:             AppAsset("gomail", "preview1.png"),
		Screenshots:      []string{AppAsset("gomail", "preview1.png"), AppAsset("gomail", "preview2.png"), AppAsset("gomail", "preview3.png")},
		PreviewLight:     AppAsset("gomail", "preview-light1.png"),
		HeroLight:        AppAsset("gomail", "preview-light1.png"),
		ScreenshotsLight: []string{AppAsset("gomail", "preview-light1.png"), AppAsset("gomail", "preview-light2.png"), AppAsset("gomail", "preview-light3.png")},
		Icon:             AppAsset("gomail", "icon.png"),
		Accent:           "#2b3b79",
	},
}

// Featured is the catalog's featured section, in catalog order.
func Featured(apps []App) []App {
	var out []App
	for _, app := range apps {
		if app.Featured {
			out = append(out, app)
		}
	}
	return out
}

// FormatSize reports the size of a setup, the way a store lists a download:
// one decimal below ten megabytes.
func FormatSize(n int64) string {
	mb := float64(n) / (1024 * 1024)
	if mb >= 9.95 {
		return fmt.Sprintf("%.0f MB", mb)
	}
	return fmt.Sprintf("%.1f MB", mb)
}
