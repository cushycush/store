package main

import (
	"reflect"
	"testing"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/platform"
)

func TestFilterStoresByPlatform(t *testing.T) {
	falseValue := false
	stores := map[string]config.StoreEntry{
		"always": {},
		"linux": {
			When: &config.WhenClause{OS: "linux"},
		},
		"darwin": {
			When: &config.WhenClause{OS: "darwin"},
		},
		"not-wsl": {
			When: &config.WhenClause{WSL: &falseValue},
		},
	}

	got := filterStoresByPlatform(stores, platform.Info{OS: "linux", WSL: false})
	want := map[string]config.StoreEntry{
		"always":  stores["always"],
		"linux":   stores["linux"],
		"not-wsl": stores["not-wsl"],
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterStoresByPlatform() = %#v, want %#v", got, want)
	}
}
