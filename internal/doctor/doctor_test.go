package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/linker"
	"github.com/cushycush/store/v2/internal/platform"
)

func TestCheckSymlinkCapabilityNonWindows(t *testing.T) {
	root := t.TempDir()
	// Non-windows info: should be a no-op regardless of actual host.
	info := platform.Info{OS: "linux"}
	if issues := checkSymlinkCapability(root, info); len(issues) != 0 {
		t.Fatalf("expected no issues on non-windows, got %#v", issues)
	}
}

func TestCheckSymlinkCapabilityWindows(t *testing.T) {
	// We can only verify the success path on a host that supports symlinks,
	// which covers Linux and macOS CI runners. On Windows without Developer
	// Mode the probe exercises the warning path — either outcome is valid.
	root := t.TempDir()
	info := platform.Info{OS: "windows"}
	issues := checkSymlinkCapability(root, info)

	switch runtime.GOOS {
	case "linux", "darwin":
		if len(issues) != 0 {
			t.Fatalf("expected symlink probe to succeed on %s, got %#v", runtime.GOOS, issues)
		}
	}
	for _, issue := range issues {
		if issue.Level != "warning" {
			t.Fatalf("unexpected issue level %q", issue.Level)
		}
	}
}

func TestCheckOrphanedConfigEntry(t *testing.T) {
	root := t.TempDir()
	targetRoot := t.TempDir()
	writeConfig(t, root, map[string]config.StoreEntry{
		"old-configs": {Target: filepath.Join(targetRoot, "old-configs")},
	})

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level:   "warning",
		Message: `store "old-configs" has no directory — did you delete it?`,
	}})
}

func TestCheckUnconfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, map[string]config.StoreEntry{})
	makeDir(t, filepath.Join(root, "scratch"))

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level:   "info",
		Message: `directory "scratch" exists but is not configured as a store`,
	}})
}

func TestCheckBrokenSymlink(t *testing.T) {
	root := t.TempDir()
	storeDir := filepath.Join(root, "nvim")
	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "nvim")

	writeConfig(t, root, map[string]config.StoreEntry{
		"nvim": {Target: target},
	})
	writeFile(t, filepath.Join(storeDir, "init.lua"), "return {}\n")
	mustSymlink(t, filepath.Join(targetRoot, "missing-source"), target)

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level: "error",
		// Doctor formats paths with %q, which escapes backslashes on Windows;
		// mirror that here so this test works on every platform.
		Message: fmt.Sprintf("store %q target %q has a broken symlink", "nvim", target),
	}})
}

func TestCheckConflictingTargets(t *testing.T) {
	root := t.TempDir()
	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "shared")

	writeConfig(t, root, map[string]config.StoreEntry{
		"nvim":        {Target: target},
		"nvim-custom": {Target: target},
	})
	writeFile(t, filepath.Join(root, "nvim", "init.lua"), "return {}\n")
	writeFile(t, filepath.Join(root, "nvim-custom", "init.lua"), "return {}\n")

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level:   "error",
		Message: fmt.Sprintf("target %q is claimed by both store %q and store %q", target, "nvim", "nvim-custom"),
	}})
}

func TestCheckMissingSecretsWithoutSecretsFile(t *testing.T) {
	root := t.TempDir()
	targetRoot := t.TempDir()

	writeConfig(t, root, map[string]config.StoreEntry{
		"git": {Target: filepath.Join(targetRoot, "git")},
	})
	writeFile(t, filepath.Join(root, "git", ".gitconfig"), "token = {{ secret \"api_key\" }}\n")

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level:   "warning",
		Message: "secrets file not found but templates reference secrets",
	}})
}

func TestCheckEmptyStore(t *testing.T) {
	root := t.TempDir()
	targetRoot := t.TempDir()

	writeConfig(t, root, map[string]config.StoreEntry{
		"empty": {Target: filepath.Join(targetRoot, "empty")},
	})
	makeDir(t, filepath.Join(root, "empty"))

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level:   "info",
		Message: `store "empty" directory is empty`,
	}})
}

func TestCheckPlatformSkippedStore(t *testing.T) {
	root := t.TempDir()
	targetRoot := t.TempDir()

	writeConfig(t, root, map[string]config.StoreEntry{
		"linux-only": {
			Target: filepath.Join(targetRoot, "linux-only"),
			When:   &config.WhenClause{OS: platformMismatchOS()},
		},
	})
	writeFile(t, filepath.Join(root, "linux-only", "config.txt"), "ok\n")

	issues := Check(root)
	assertIssues(t, issues, []Issue{{
		Level:   "info",
		Message: `store "linux-only" will be skipped on this platform (when: os=` + platformMismatchOS() + `, current: ` + runtime.GOOS + `)`,
	}})
}

func TestCheckCleanState(t *testing.T) {
	root := t.TempDir()
	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "nvim")

	writeConfig(t, root, map[string]config.StoreEntry{
		"nvim": {Target: target},
	})
	writeFile(t, filepath.Join(root, "nvim", "init.lua"), "return {}\n")
	if err := linker.Link(filepath.Join(root, "nvim"), target); err != nil {
		t.Fatalf("Link() error = %v", err)
	}

	issues := Check(root)
	if len(issues) != 0 {
		t.Fatalf("Check() returned %#v, want no issues", issues)
	}
}

func writeConfig(t *testing.T, root string, stores map[string]config.StoreEntry) {
	t.Helper()
	if err := config.Save(root, &config.Config{Stores: stores}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func makeDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustSymlink(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(linkPath), err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink(%q, %q) error = %v", target, linkPath, err)
	}
}

func assertIssues(t *testing.T, got, want []Issue) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(issues) = %d, want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("issues[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func platformMismatchOS() string {
	if runtime.GOOS == "linux" {
		return "darwin"
	}
	return "linux"
}
