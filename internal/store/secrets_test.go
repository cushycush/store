package store

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/render"
)

const (
	testTemplateContent = "token = {{ secret \"test_key\" }}\nother = plaintext\n"
	renderedContent     = "token = secret_value\nother = plaintext\n"
)

func assertSymlinkExists(t *testing.T, path string) {
	t.Helper()

	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%q): %v", path, err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%q is not a symlink", path)
	}
}

func stagingStoreDir(t *testing.T, root, name string) string {
	t.Helper()

	stagingBase, err := render.StagingDir(root)
	if err != nil {
		t.Fatalf("StagingDir(%q): %v", root, err)
	}

	return filepath.Join(stagingBase, name)
}

func TestStoreTargetWithSecrets(t *testing.T) {
	t.Run("no secrets creates direct symlink to repo", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{"config.toml": testTemplateContent})

		target := filepath.Join(root, "targets", "app")
		te := config.TargetEntry{Target: target}

		if err := StoreTargetWithSecrets(root, "app", te, nil); err != nil {
			t.Fatalf("StoreTargetWithSecrets() error = %v", err)
		}

		assertSymlinkExists(t, target)
		assertSymlinkPointsTo(t, target, filepath.Join(root, "app"))
		assertFileContent(t, filepath.Join(target, "config.toml"), testTemplateContent)
	})

	t.Run("secrets with no templates links repo directly", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{"config.toml": "plain = true\n"})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		target := filepath.Join(root, "targets", "app")
		te := config.TargetEntry{Target: target}

		if err := StoreTargetWithSecrets(root, "app", te, &RenderContext{Secrets: map[string]string{"test_key": "secret_value"}}); err != nil {
			t.Fatalf("StoreTargetWithSecrets() error = %v", err)
		}

		assertSymlinkExists(t, target)
		assertSymlinkPointsTo(t, target, filepath.Join(root, "app"))
		assertMissing(t, stagingStoreDir(t, root, "app"))
	})

	t.Run("whole directory mode stages rendered templates", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{
			"config.toml": testTemplateContent,
			"plain.txt":   "plain\n",
		})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		target := filepath.Join(root, "targets", "app")
		te := config.TargetEntry{Target: target}
		stagedSource := stagingStoreDir(t, root, "app")

		if err := StoreTargetWithSecrets(root, "app", te, &RenderContext{Secrets: map[string]string{"test_key": "secret_value"}}); err != nil {
			t.Fatalf("StoreTargetWithSecrets() error = %v", err)
		}

		assertSymlinkExists(t, target)
		assertSymlinkPointsTo(t, target, stagedSource)
		assertFileContent(t, filepath.Join(target, "config.toml"), renderedContent)
		assertSymlinkPointsTo(t, filepath.Join(stagedSource, "plain.txt"), filepath.Join(root, "app", "plain.txt"))
	})

	t.Run("file mode links rendered and plain files through staging", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{
			"config.toml": testTemplateContent,
			"plain.txt":   "plain\n",
		})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		target := filepath.Join(root, "targets", "files")
		te := config.TargetEntry{Target: target, Files: []string{"config.toml", "plain.txt"}}
		stagedSource := stagingStoreDir(t, root, "app")

		if err := StoreTargetWithSecrets(root, "app", te, &RenderContext{Secrets: map[string]string{"test_key": "secret_value"}}); err != nil {
			t.Fatalf("StoreTargetWithSecrets() error = %v", err)
		}

		renderedTarget := filepath.Join(target, "config.toml")
		plainTarget := filepath.Join(target, "plain.txt")
		stagedRendered := filepath.Join(stagedSource, "config.toml")
		stagedPlain := filepath.Join(stagedSource, "plain.txt")

		assertSymlinkExists(t, renderedTarget)
		assertSymlinkExists(t, plainTarget)
		assertSymlinkPointsTo(t, renderedTarget, stagedRendered)
		assertSymlinkPointsTo(t, plainTarget, stagedPlain)
		assertFileContent(t, renderedTarget, renderedContent)
		assertFileContent(t, plainTarget, "plain\n")

		if fi, err := os.Lstat(stagedRendered); err != nil {
			t.Fatalf("Lstat(%q): %v", stagedRendered, err)
		} else if fi.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%q should be a rendered file, not a symlink", stagedRendered)
		}
		assertSymlinkPointsTo(t, stagedPlain, filepath.Join(root, "app", "plain.txt"))
	})

	t.Run("missing secret returns error with secret name", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{"config.toml": testTemplateContent})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		target := filepath.Join(root, "targets", "app")
		te := config.TargetEntry{Target: target}

		err := StoreTargetWithSecrets(root, "app", te, &RenderContext{Secrets: map[string]string{"other_key": "value"}})
		if err == nil {
			t.Fatal("StoreTargetWithSecrets() error = nil, want missing secret error")
		}
		if !strings.Contains(err.Error(), "test_key") {
			t.Fatalf("StoreTargetWithSecrets() error = %v, want missing secret name", err)
		}
		assertMissing(t, target)
	})
}

func TestStoreWithSecrets(t *testing.T) {
	t.Run("hooks use repo root while linking staged source", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// The hook commands use POSIX `printf` and `$VAR` expansion which
			// aren't native to cmd.exe. The platform-agnostic behavior (hooks
			// receive STORE_ROOT/NAME/TARGET/ACTION) is covered by
			// internal/hooks/hooks_windows_test.go.
			t.Skip("uses POSIX shell syntax; equivalent coverage lives in hooks_windows_test.go")
		}
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{"config.toml": testTemplateContent})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		preLog := filepath.Join(root, "pre.log")
		postLog := filepath.Join(root, "post.log")
		target := filepath.Join(root, "targets", "app")
		entry := config.StoreEntry{
			Target: target,
			Hooks: &config.HookEntry{
				Pre:  `printf '%s|%s|%s|%s\n' "$STORE_ROOT" "$STORE_NAME" "$STORE_TARGET" "$STORE_ACTION" >> pre.log`,
				Post: `printf '%s|%s|%s|%s\n' "$STORE_ROOT" "$STORE_NAME" "$STORE_TARGET" "$STORE_ACTION" >> post.log`,
			},
		}

		if err := StoreWithSecrets(root, "app", entry, &RenderContext{Secrets: map[string]string{"test_key": "secret_value"}}); err != nil {
			t.Fatalf("StoreWithSecrets() error = %v", err)
		}

		assertSymlinkExists(t, target)
		assertSymlinkPointsTo(t, target, stagingStoreDir(t, root, "app"))
		assertFileContent(t, filepath.Join(target, "config.toml"), renderedContent)

		wantHookLine := fmt.Sprintf("%s|app|%s|link\n", root, target)
		assertFileContent(t, preLog, wantHookLine)
		assertFileContent(t, postLog, wantHookLine)
	})

	t.Run("no secrets behaves like regular store", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "app", map[string]string{"config.toml": testTemplateContent})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		target := filepath.Join(root, "targets", "app")
		entry := config.StoreEntry{Target: target}

		if err := StoreWithSecrets(root, "app", entry, nil); err != nil {
			t.Fatalf("StoreWithSecrets() error = %v", err)
		}

		assertSymlinkExists(t, target)
		assertSymlinkPointsTo(t, target, filepath.Join(root, "app"))
		assertFileContent(t, filepath.Join(target, "config.toml"), testTemplateContent)
		assertMissing(t, stagingStoreDir(t, root, "app"))
	})
}

func TestStoreAllWithSecrets(t *testing.T) {
	t.Run("mixed stores only stage templated store", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "templated", map[string]string{"config.toml": testTemplateContent})
		createStore(t, root, "plain", map[string]string{"plain.txt": "plain\n"})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		cfg := &config.Config{Stores: map[string]config.StoreEntry{
			"templated": {Target: filepath.Join(root, "targets", "templated")},
			"plain":     {Target: filepath.Join(root, "targets", "plain")},
		}}

		if err := StoreAllWithSecrets(root, cfg, &RenderContext{Secrets: map[string]string{"test_key": "secret_value"}}); err != nil {
			t.Fatalf("StoreAllWithSecrets() error = %v", err)
		}

		assertSymlinkPointsTo(t, cfg.Stores["templated"].Target, stagingStoreDir(t, root, "templated"))
		assertFileContent(t, filepath.Join(cfg.Stores["templated"].Target, "config.toml"), renderedContent)
		assertSymlinkPointsTo(t, cfg.Stores["plain"].Target, filepath.Join(root, "plain"))
		assertMissing(t, stagingStoreDir(t, root, "plain"))
	})

	t.Run("no secrets links all stores directly to repo", func(t *testing.T) {
		root := t.TempDir()
		createStore(t, root, "templated", map[string]string{"config.toml": testTemplateContent})
		createStore(t, root, "plain", map[string]string{"plain.txt": "plain\n"})

		stateHome := t.TempDir()
		t.Setenv("XDG_STATE_HOME", stateHome)

		cfg := &config.Config{Stores: map[string]config.StoreEntry{
			"templated": {Target: filepath.Join(root, "targets", "templated")},
			"plain":     {Target: filepath.Join(root, "targets", "plain")},
		}}

		if err := StoreAllWithSecrets(root, cfg, nil); err != nil {
			t.Fatalf("StoreAllWithSecrets() error = %v", err)
		}

		assertSymlinkPointsTo(t, cfg.Stores["templated"].Target, filepath.Join(root, "templated"))
		assertFileContent(t, filepath.Join(cfg.Stores["templated"].Target, "config.toml"), testTemplateContent)
		assertSymlinkPointsTo(t, cfg.Stores["plain"].Target, filepath.Join(root, "plain"))
		assertMissing(t, stagingStoreDir(t, root, "templated"))
		assertMissing(t, stagingStoreDir(t, root, "plain"))
	})
}
