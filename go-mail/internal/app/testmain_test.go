package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points the app's files at a temporary directory, so a test can
// never overwrite the configuration, message cache or logos of the person
// running it. Tests that need their own paths still set them with
// t.Setenv, which restores these values afterwards.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "go-mail-test-")
	if err != nil {
		panic(err)
	}
	set := func(key, file string) {
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, filepath.Join(dir, file))
		}
	}
	set("GO_MAIL_CONFIG", "config.json")
	set("GO_MAIL_CACHE", "cache.json")
	set("GO_MAIL_LOGOS", "logos")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
