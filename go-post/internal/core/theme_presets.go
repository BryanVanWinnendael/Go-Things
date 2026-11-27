package core

// ThemePresetIDs are the built-in theme IDs. They mirror the palettes in the
// app's theme catalog; keeping the IDs here lets the store validate saved data
// without depending on the UI package.
var ThemePresetIDs = []string{
	"nocturne", "daylight", "slate", "cocoa", "linen", "dusk", "tide", "bloom",
}

// IsPresetTheme reports whether id names a built-in theme.
func IsPresetTheme(id string) bool {
	for _, preset := range ThemePresetIDs {
		if preset == id {
			return true
		}
	}
	return false
}

// CustomThemeByID finds a custom theme by ID.
func CustomThemeByID(themes []CustomTheme, id string) *CustomTheme {
	for i := range themes {
		if themes[i].ID == id {
			return &themes[i]
		}
	}
	return nil
}

// ValidTheme reports whether id names the System choice, a built-in theme, or
// a custom theme that still exists.
func ValidTheme(id string, customs []CustomTheme) bool {
	return id == "system" || IsPresetTheme(id) || CustomThemeByID(customs, id) != nil
}
