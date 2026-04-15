package importer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cushycush/store/internal/config"
)

func TestScanWholeDirectorySymlink(t *testing.T) {
	repoRoot := t.TempDir()
	storeDir := filepath.Join(repoRoot, "nvim")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeDir): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, config.ConfigDir), 0o755); err != nil {
		t.Fatalf("MkdirAll(config dir): %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".hidden"), 0o755); err != nil {
		t.Fatalf("MkdirAll(hidden dir): %v", err)
	}

	scanDir := t.TempDir()
	linkPath := filepath.Join(scanDir, "nvim")
	if err := os.Symlink(storeDir, linkPath); err != nil {
		t.Fatalf("Symlink(): %v", err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, config.ConfigDir), filepath.Join(scanDir, ".store-link")); err != nil {
		t.Fatalf("Symlink(config dir): %v", err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, ".hidden"), filepath.Join(scanDir, "hidden-link")); err != nil {
		t.Fatalf("Symlink(hidden dir): %v", err)
	}

	got, err := Scan(repoRoot, []string{scanDir})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	// Scan resolves symlinks and 8.3 short names; mirror that in the expected
	// values so macOS (/private/var/folders) and Windows (RUNNER~1 vs
	// runneradmin) comparisons succeed.
	wantSource := resolvePath(t, storeDir)
	want := []DiscoveredLink{{
		StoreName: "nvim",
		Source:    wantSource,
		Target:    linkPath,
		File:      "",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() = %#v, want %#v", got, want)
	}
}

// resolvePath returns the canonical path with symlinks and (on Windows) 8.3
// short names resolved. Scan applies the same transformation internally, so
// tests that build expected values from t.TempDir() paths must do the same.
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", path, err)
	}
	return resolved
}

// setTestHome points both HOME and USERPROFILE at the given directory so that
// os.UserHomeDir() (which prefers USERPROFILE on Windows) agrees with the
// POSIX-style HOME that the tool's config reads.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestScanFileModeSymlinks(t *testing.T) {
	repoRoot := t.TempDir()
	storeDir := filepath.Join(repoRoot, "shells")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(storeDir): %v", err)
	}
	zshrc := filepath.Join(storeDir, ".zshrc")
	bashrc := filepath.Join(storeDir, ".bashrc")
	for _, path := range []string{zshrc, bashrc} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}

	scanDir := t.TempDir()
	links := map[string]string{
		filepath.Join(scanDir, ".zshrc"):  zshrc,
		filepath.Join(scanDir, ".bashrc"): bashrc,
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("Symlink(%q, %q): %v", target, link, err)
		}
	}

	got, err := Scan(repoRoot, []string{scanDir})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := []DiscoveredLink{
		{StoreName: "shells", Source: resolvePath(t, bashrc), Target: filepath.Join(scanDir, ".bashrc"), File: ".bashrc"},
		{StoreName: "shells", Source: resolvePath(t, zshrc), Target: filepath.Join(scanDir, ".zshrc"), File: ".zshrc"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() = %#v, want %#v", got, want)
	}
}

func TestScanSkipsNonRepoSymlinksAndRegularFiles(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(repo store): %v", err)
	}

	scanDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll(outside): %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(scanDir, "elsewhere")); err != nil {
		t.Fatalf("Symlink(outside): %v", err)
	}
	if err := os.WriteFile(filepath.Join(scanDir, "regular.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(regular): %v", err)
	}

	got, err := Scan(repoRoot, []string{scanDir})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Scan() = %#v, want no links", got)
	}
}

func TestToConfigGroupingAndMixedMode(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	repoRoot := t.TempDir()

	links := []DiscoveredLink{
		{
			StoreName: "shells",
			Source:    filepath.Join(repoRoot, "shells", ".zshrc"),
			Target:    filepath.Join(home, ".zshrc"),
			File:      ".zshrc",
		},
		{
			StoreName: "shells",
			Source:    filepath.Join(repoRoot, "shells", ".bashrc"),
			Target:    filepath.Join(home, ".bashrc"),
			File:      ".bashrc",
		},
		{
			StoreName: "git",
			Source:    filepath.Join(repoRoot, "git"),
			Target:    filepath.Join(home, ".config", "git"),
			File:      "",
		},
		{
			StoreName: "mixed",
			Source:    filepath.Join(repoRoot, "mixed"),
			Target:    filepath.Join(home, ".config", "mixed"),
			File:      "",
		},
		{
			StoreName: "mixed",
			Source:    filepath.Join(repoRoot, "mixed", "extra.conf"),
			Target:    filepath.Join(home, ".config", "mixed", "extra.conf"),
			File:      "extra.conf",
		},
	}

	got := ToConfig(links, repoRoot)
	want := map[string]config.StoreEntry{
		"shells": {
			Target: "~",
			Files:  []string{".bashrc", ".zshrc"},
		},
		"git": {
			Target: "~/.config/git",
		},
		"mixed": {
			Target: "~/.config/mixed",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToConfig() = %#v, want %#v", got, want)
	}
}

func TestToConfigMultiTargetAndHomePortability(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	repoRoot := t.TempDir()

	links := []DiscoveredLink{
		{
			StoreName: "shells",
			Source:    filepath.Join(repoRoot, "shells", ".zshrc"),
			Target:    filepath.Join(home, ".zshrc"),
			File:      ".zshrc",
		},
		{
			StoreName: "shells",
			Source:    filepath.Join(repoRoot, "shells", "config.fish"),
			Target:    filepath.Join(home, ".config", "fish", "config.fish"),
			File:      "config.fish",
		},
		{
			StoreName: "shells",
			Source:    filepath.Join(repoRoot, "shells", "config.nu"),
			Target:    filepath.Join(home, ".config", "nushell", "config.nu"),
			File:      "config.nu",
		},
	}

	got := ToConfig(links, repoRoot)
	want := map[string]config.StoreEntry{
		"shells": {
			Targets: []config.TargetEntry{
				{Target: "~", Files: []string{".zshrc"}},
				{Target: "~/.config/fish", Files: []string{"config.fish"}},
				{Target: "~/.config/nushell", Files: []string{"config.nu"}},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToConfig() = %#v, want %#v", got, want)
	}
}
