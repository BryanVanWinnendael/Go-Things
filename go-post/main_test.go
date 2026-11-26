package main

import (
	"testing"
)

func TestInterpolate(t *testing.T) {
	got := interpolate("{{baseUrl}}/users/{{id}}", map[string]string{"baseUrl": "https://api.example.com", "id": "42"})
	want := "https://api.example.com/users/42"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultData(t *testing.T) {
	d := defaultData()
	w := d.workspace(d.ActiveWorkspace)
	if w == nil {
		t.Fatal("active workspace missing")
	}
	if len(w.Collections) == 0 {
		t.Fatal("starter collection missing")
	}
	if w.activeVars()["baseUrl"] == "" {
		t.Fatal("environment variable missing")
	}
}
