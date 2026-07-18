package storage

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "value.json")
	want := map[string]int{"value": 1}
	if err := SaveJSON(path, want); err != nil {
		t.Fatal(err)
	}
	var got map[string]int
	if err := LoadJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if got["value"] != 1 {
		t.Fatalf("got %#v", got)
	}
}
