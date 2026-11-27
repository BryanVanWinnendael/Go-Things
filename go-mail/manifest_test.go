package manifest

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestVersionComesFromManifest checks that the label in Settings and About is
// built from the version in mygo.json, not a hardcoded string.
func TestVersionComesFromManifest(t *testing.T) {
	var cfg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(appManifest, &cfg); err != nil {
		t.Fatalf("mygo.json does not parse: %v", err)
	}
	want := strings.TrimSpace(cfg.Version)
	if want == "" {
		t.Fatal("mygo.json has no version")
	}
	if got := Version(); got != want {
		t.Fatalf("Version() is %q, want %q", got, want)
	}
	if got := Label(); got != "Go Mail "+want {
		t.Fatalf("Label() is %q, want %q", got, "Go Mail "+want)
	}
}
