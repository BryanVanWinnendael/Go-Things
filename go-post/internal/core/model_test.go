package core

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	got := Interpolate("{{baseUrl}}/users/{{id}}", map[string]string{"baseUrl": "https://api.example.com", "id": "42"})
	want := "https://api.example.com/users/42"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultData(t *testing.T) {
	d := DefaultData()
	w := d.Workspace(d.ActiveWorkspace)
	if w == nil {
		t.Fatal("active workspace missing")
	}
	if len(w.Collections) == 0 {
		t.Fatal("starter collection missing")
	}
	if w.ActiveVars()["baseUrl"] == "" {
		t.Fatal("environment variable missing")
	}
}
