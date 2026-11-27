package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The catalog checked here is the exact file that gets attached to a release.
func TestPublishedCatalogIsValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "catalog.json"))
	if err != nil {
		t.Fatalf("read catalog.json: %v", err)
	}
	var doc Doc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("decode catalog.json: %v", err)
	}
	if err := Validate(doc); err != nil {
		t.Fatalf("catalog.json is not publishable: %v", err)
	}
	if len(doc.Apps) == 0 {
		t.Fatal("catalog.json contains no apps")
	}
	featured := Featured(doc.Apps)
	if len(featured) == 0 {
		t.Fatal("catalog.json should mark at least one app as featured")
	}
	if featured[0].ID != "gopost" {
		t.Fatalf("expected Go Post to be featured, got %q", featured[0].ID)
	}
}
