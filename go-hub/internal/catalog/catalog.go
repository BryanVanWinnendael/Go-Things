package catalog

import "fmt"

const Version = "1.0.0"

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
	// Featured puts the app in the large hero slot at the top of the store.
	Featured bool `json:"featured"`
	// Icon is the app's mark, shown on its tile when available.
	Icon string `json:"icon"`
	// Accent is the app's own accent colour, used for its tile and mark in
	// the hub instead of the hub's theme-extreme accent.
	Accent string `json:"accent"`
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
			"Local-first: no account, no cloud",
		},
		Version:     "1.0.0",
		Identifier:  "com.example.gopost",
		ExeName:     "go-post.exe",
		Setup:       ReleaseAsset("go-post-setup-1.0.0-amd64.exe"),
		Preview:     AppAsset("gopost", "preview1.png"),
		Hero:        AppAsset("gopost", "preview1.png"),
		Icon:        AppAsset("gopost", "icon.png"),
		Screenshots: []string{AppAsset("gopost", "preview1.png"), AppAsset("gopost", "preview2.png"), AppAsset("gopost", "preview3.png")},
		Accent:      "#10b981",
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
			"Local-first: no account, no cloud",
		},
		Version:     "1.0.0",
		Identifier:  "com.example.gomail",
		ExeName:     "go-mail.exe",
		Setup:       ReleaseAsset("go-mail-setup-1.0.0-amd64.exe"),
		Preview:     AppAsset("gomail", "preview1.png"),
		Hero:        AppAsset("gomail", "preview1.png"),
		Icon:        AppAsset("gomail", "icon.png"),
		Screenshots: []string{AppAsset("gomail", "preview1.png")},
		Accent:      "#2b3b79",
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
