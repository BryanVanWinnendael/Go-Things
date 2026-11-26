package main

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

type settings struct {
	Theme string `json:"theme"`
}

func loadSettings() settings {
	s := settings{Theme: "nocturne"}
	path := settingsPath()
	if path == "" {
		return s
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	var onDisk settings
	if json.Unmarshal(b, &onDisk) != nil {
		return s
	}
	switch onDisk.Theme {
	case "system", "nocturne", "daylight":
		s.Theme = onDisk.Theme
	}
	return s
}

func (a *App) saveSettings() {
	path := settingsPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	b, err := json.MarshalIndent(settings{Theme: a.themeSetting}, "", "  ")
	if err != nil {
		return
	}
	b = append(b, '\n')
	_ = os.WriteFile(path, b, 0o644)
}
