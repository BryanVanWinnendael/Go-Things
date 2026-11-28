package app

import (
	"strings"
	"testing"

	"go-hub/internal/catalog"
)

func TestSetupCacheNameIncludesCatalogRevision(t *testing.T) {
	app := catalog.App{
		ID:      "gopost",
		Version: "1.0.0",
		Setup:   "https://example.com/go-post-setup-1.0.0-amd64.exe",
	}
	old := setupCacheName(app, "2026-11-26")
	current := setupCacheName(app, "2026-11-28")
	if old == current {
		t.Fatalf("a new catalog revision must get a new cache name: %q", old)
	}
	if !strings.Contains(current, "2026-11-28") || !strings.HasSuffix(current, "go-post-setup-1.0.0-amd64.exe") {
		t.Fatalf("unexpected cache name: %q", current)
	}
}
