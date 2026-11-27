package manifest

import (
	"encoding/json"
	"testing"
)

// The app shows the version its manifest was built with, so bumping mygo.json
// is enough to change what the UI says.
func TestAppVersionComesFromManifest(t *testing.T) {
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(appManifest, &cfg); err != nil {
		t.Fatalf("mygo.json is not readable: %v", err)
	}
	if cfg.Version == "" {
		t.Fatal("mygo.json has no version")
	}
	if Version() != cfg.Version {
		t.Fatalf("app version is %q, want the manifest's %q", Version(), cfg.Version)
	}
	if Label() != "Go Post "+cfg.Version {
		t.Fatalf("version label is %q", Label())
	}
}
