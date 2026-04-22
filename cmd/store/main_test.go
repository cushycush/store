package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/platform"
	storeops "github.com/cushycush/store/v2/internal/store"
)

func TestFilterStoresByPlatform(t *testing.T) {
	falseValue := false
	stores := map[string]config.StoreEntry{
		"always": {},
		"linux": {
			When: &config.WhenClause{OS: config.Strings{"linux"}},
		},
		"darwin": {
			When: &config.WhenClause{OS: config.Strings{"darwin"}},
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

func TestBuildDiffReport(t *testing.T) {
	root := t.TempDir()
	createStore := func(name string, files map[string]string) {
		t.Helper()
		storeDir := filepath.Join(root, name)
		if err := os.MkdirAll(storeDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", storeDir, err)
		}
		for rel, content := range files {
			path := filepath.Join(storeDir, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatalf("WriteFile(%q): %v", path, err)
			}
		}
	}
	mustSymlink := func(target, linkPath string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(linkPath), err)
		}
		if err := os.Symlink(target, linkPath); err != nil {
			t.Fatalf("Symlink(%q, %q): %v", target, linkPath, err)
		}
	}

	createStore("nvim", map[string]string{"init.lua": "init"})
	createStore("shells", map[string]string{".zshrc": "zsh", ".bashrc": "bash"})
	createStore("git", map[string]string{".gitconfig": "git"})
	createStore("old", map[string]string{"config": "old"})
	createStore("broken", map[string]string{"ghost": "gone soon"})
	createStore("bad", map[string]string{"noop": "noop"})

	nvimTarget := filepath.Join(root, "targets", "nvim")
	if err := os.MkdirAll(filepath.Dir(nvimTarget), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(nvimTarget), err)
	}
	if err := os.Symlink(filepath.Join(root, "nvim"), nvimTarget); err != nil {
		t.Fatalf("Symlink(nvim): %v", err)
	}

	gitTarget := filepath.Join(root, "targets", "git")
	if err := os.MkdirAll(gitTarget, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", gitTarget, err)
	}

	brokenTarget := filepath.Join(root, "targets", "broken", "ghost")
	mustSymlink(filepath.Join(root, "nowhere", "ghost"), brokenTarget)

	cfg := &config.Config{Stores: map[string]config.StoreEntry{
		"nvim": {Target: nvimTarget},
		"shells": {
			Target: filepath.Join(root, "targets", "home"),
			Files:  []string{".zshrc", ".bashrc"},
		},
		"git": {
			Target: gitTarget,
		},
		"old": {
			Target: filepath.Join(root, "targets", "old"),
		},
		"broken": {
			Target: filepath.Join(root, "targets", "broken"),
			Files:  []string{"ghost"},
		},
		"bad": {},
	}}

	rows, summary := buildDiffReport(storeops.GetStatusAll(root, cfg))

	if summary != (diffSummary{OK: 1, Create: 3, Conflict: 1, Replace: 1, Error: 1}) {
		t.Fatalf("summary = %+v, want %+v", summary, diffSummary{OK: 1, Create: 3, Conflict: 1, Replace: 1, Error: 1})
	}

	got := make(map[string]diffRow, len(rows))
	for _, row := range rows {
		got[fmt.Sprintf("%s|%s", row.Name, row.Display)] = row
	}

	checks := map[string]struct {
		label string
		err   bool
	}{
		fmt.Sprintf("%s|%s", "nvim", nvimTarget):                                                       {label: "ok"},
		fmt.Sprintf("%s|%s", "shells", ".bashrc → "+filepath.Join(root, "targets", "home", ".bashrc")): {label: "create"},
		fmt.Sprintf("%s|%s", "shells", ".zshrc → "+filepath.Join(root, "targets", "home", ".zshrc")):   {label: "create"},
		fmt.Sprintf("%s|%s", "git", gitTarget):                                                         {label: "conflict"},
		fmt.Sprintf("%s|%s", "old", filepath.Join(root, "targets", "old")):                             {label: "create"},
		fmt.Sprintf("%s|%s", "broken", "ghost → "+filepath.Join(root, "targets", "broken", "ghost")):   {label: "replace"},
		"bad|": {label: "error", err: true},
	}

	for key, want := range checks {
		row, ok := got[key]
		if !ok {
			t.Fatalf("missing diff row for %q", key)
		}
		if row.Label != want.label {
			t.Fatalf("row %q label = %q, want %q", key, row.Label, want.label)
		}
		if (row.Error != nil) != want.err {
			t.Fatalf("row %q error presence = %v, want %v", key, row.Error != nil, want.err)
		}
	}

	if got := formatDiffSummary(summary); got != "Summary: 1 ok, 3 to create, 1 conflict, 1 to replace, 1 error" {
		t.Fatalf("formatDiffSummary() = %q", got)
	}
}
