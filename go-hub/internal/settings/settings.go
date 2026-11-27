package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// settingsPath is where the hub keeps its own preferences, next to the
// suite's other apps under the user's roaming profile.
func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "go-hub", "settings.json")
}

// Settings is the hub's own preference file.
type Settings struct {
	Theme string `json:"theme"`
}

// Load reads the saved theme, falling back to Nocturne.
func Load() Settings {
	s := Settings{Theme: "nocturne"}
	path := settingsPath()
	if path == "" {
		return s
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	var onDisk Settings
	if json.Unmarshal(b, &onDisk) != nil {
		return s
	}
	switch onDisk.Theme {
	case "system", "nocturne", "daylight":
		s.Theme = onDisk.Theme
	}
	return s
}

// Save writes the theme choice.
func Save(theme string) {
	path := settingsPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	b, err := json.MarshalIndent(Settings{Theme: theme}, "", "  ")
	if err != nil {
		return
	}
	b = append(b, '\n')
	_ = os.WriteFile(path, b, 0o644)
}
