package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir  = ".store"
	ConfigFile = "config.yaml"
)

// StoreEntry represents a single store's configuration.
type StoreEntry struct {
	Target   string   `yaml:"target,omitempty"`
	Files    []string `yaml:"files,omitempty"`
	Patterns []string `yaml:"patterns,omitempty"`
}

// HasFileMode returns true if the entry specifies individual files or patterns
// rather than a whole-directory symlink.
func (e StoreEntry) HasFileMode() bool {
	return len(e.Files) > 0 || len(e.Patterns) > 0
}

// Config represents the full .store/config.yaml file.
type Config struct {
	Stores map[string]StoreEntry `yaml:"stores"`
}

// ConfigPath returns the path to the config file given a repo root.
func ConfigPath(root string) string {
	return filepath.Join(root, ConfigDir, ConfigFile)
}

// Load reads and parses the config file from the given repo root.
func Load(root string) (*Config, error) {
	path := ConfigPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Stores == nil {
		cfg.Stores = make(map[string]StoreEntry)
	}

	return &cfg, nil
}

// Save writes the config back to disk.
func Save(root string, cfg *Config) error {
	path := ConfigPath(root)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Exists returns true if the config file exists at the given root.
func Exists(root string) bool {
	_, err := os.Stat(ConfigPath(root))
	return err == nil
}

// ExpandHome expands a leading ~ in a path to the user's home directory.
func ExpandHome(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if len(path) == 1 {
		return home, nil
	}

	if path[1] == '/' {
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

// FindRoot walks up from the current directory to find the repo root
// (the directory containing .store/).
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ConfigDir)); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no %s directory found (run 'store init' first)", ConfigDir)
}
