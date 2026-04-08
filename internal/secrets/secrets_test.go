package secrets

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSecretsPath(t *testing.T) {
	tests := []struct {
		name string
		root string
		want string
	}{
		{
			name: "joins root and store path",
			root: "/repo",
			want: filepath.Join("/repo", ".store", SecretsFile),
		},
		{
			name: "empty root still uses store directory",
			root: "",
			want: filepath.Join("", ".store", SecretsFile),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SecretsPath(tt.root); got != tt.want {
				t.Fatalf("SecretsPath(%q) = %q, want %q", tt.root, got, tt.want)
			}
		})
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		want  bool
	}{
		{
			name:  "false when file missing",
			setup: func(t *testing.T, root string) {},
			want:  false,
		},
		{
			name: "true when file exists",
			setup: func(t *testing.T, root string) {
				t.Helper()
				if err := Save(root, "passphrase", map[string]string{"token": "abc"}); err != nil {
					t.Fatalf("Save() error = %v", err)
				}
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			if got := Exists(root); got != tt.want {
				t.Fatalf("Exists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	largeSecrets := make(map[string]string, 64)
	for i := range 64 {
		largeSecrets[fmt.Sprintf("key-%02d", i)] = strings.Repeat(fmt.Sprintf("value-%02d", i), 3)
	}

	tests := []struct {
		name       string
		secrets    map[string]string
		passphrase string
	}{
		{
			name: "round trip preserves key values",
			secrets: map[string]string{
				"api_token": "secret-token",
				"username":  "cush",
				"nested":    "a=b;c=d",
			},
			passphrase: "correct horse battery staple",
		},
		{
			name:       "empty map round trip",
			secrets:    map[string]string{},
			passphrase: "empty-pass",
		},
		{
			name:       "large map round trip",
			secrets:    largeSecrets,
			passphrase: "large-pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			if err := Save(root, tt.passphrase, tt.secrets); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			got, err := Load(root, tt.passphrase)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if !reflect.DeepEqual(got, tt.secrets) {
				t.Fatalf("Load() = %#v, want %#v", got, tt.secrets)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "returns empty map without error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			got, err := Load(root, "passphrase")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("Load() returned %d secrets, want 0", len(got))
			}
		})
	}
}

func TestLoadWrongPassphrase(t *testing.T) {
	tests := []struct {
		name            string
		passphrase      string
		wrongPassphrase string
		wantErr         string
	}{
		{
			name:            "returns helpful error",
			passphrase:      "correct-passphrase",
			wrongPassphrase: "wrong-passphrase",
			wantErr:         "wrong passphrase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := Save(root, tt.passphrase, map[string]string{"token": "secret"}); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			_, err := Load(root, tt.wrongPassphrase)
			if err == nil {
				t.Fatal("Load() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSaveCreatesStoreDirectory(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "creates .store directory automatically"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			storeDirPath := filepath.Join(root, ".store")

			if _, err := os.Stat(storeDirPath); !os.IsNotExist(err) {
				t.Fatalf("expected %q to not exist before Save(), err=%v", storeDirPath, err)
			}

			if err := Save(root, "passphrase", map[string]string{"token": "secret"}); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			fi, err := os.Stat(storeDirPath)
			if err != nil {
				t.Fatalf("Stat(%q): %v", storeDirPath, err)
			}
			if !fi.IsDir() {
				t.Fatalf("%q is not a directory", storeDirPath)
			}
		})
	}
}

func TestMultipleSaveLoadCycles(t *testing.T) {
	tests := []struct {
		name   string
		cycles []map[string]string
	}{
		{
			name: "sequential saves overwrite prior data",
			cycles: []map[string]string{
				{"first": "one"},
				{"second": "two", "third": "three"},
				{},
				{"final": "done"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			passphrase := "cycle-passphrase"

			for i, want := range tt.cycles {
				if err := Save(root, passphrase, want); err != nil {
					t.Fatalf("cycle %d Save() error = %v", i, err)
				}

				got, err := Load(root, passphrase)
				if err != nil {
					t.Fatalf("cycle %d Load() error = %v", i, err)
				}

				if !reflect.DeepEqual(got, want) {
					t.Fatalf("cycle %d Load() = %#v, want %#v", i, got, want)
				}
			}
		})
	}
}

func TestDifferentPassphrasesProduceDifferentCiphertext(t *testing.T) {
	tests := []struct {
		name        string
		passphraseA string
		passphraseB string
		secrets     map[string]string
	}{
		{
			name:        "same data encrypted with different passphrases differs",
			passphraseA: "alpha-pass",
			passphraseB: "beta-pass",
			secrets: map[string]string{
				"shared": "value",
				"token":  "secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootA := t.TempDir()
			rootB := t.TempDir()

			if err := Save(rootA, tt.passphraseA, tt.secrets); err != nil {
				t.Fatalf("Save(rootA) error = %v", err)
			}
			if err := Save(rootB, tt.passphraseB, tt.secrets); err != nil {
				t.Fatalf("Save(rootB) error = %v", err)
			}

			dataA, err := os.ReadFile(SecretsPath(rootA))
			if err != nil {
				t.Fatalf("ReadFile(rootA): %v", err)
			}
			dataB, err := os.ReadFile(SecretsPath(rootB))
			if err != nil {
				t.Fatalf("ReadFile(rootB): %v", err)
			}

			if bytes.Equal(dataA, dataB) {
				t.Fatal("ciphertexts are equal, want different bytes")
			}
		})
	}
}
