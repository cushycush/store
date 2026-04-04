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

// TargetEntry represents a single target within a store.
type TargetEntry struct {
	Target   string   `yaml:"target,omitempty"`
	Files    []string `yaml:"files,omitempty"`
	Patterns []string `yaml:"patterns,omitempty"`
}

// HasFileMode returns true if the target specifies individual files or patterns
// rather than a whole-directory symlink.
func (t TargetEntry) HasFileMode() bool {
	return len(t.Files) > 0 || len(t.Patterns) > 0
}

// StoreEntry represents a single store's configuration.
// It supports two formats:
//   - Single-target: uses Target, Files, Patterns fields directly.
//   - Multi-target: uses the Targets list, each with its own Target/Files/Patterns.
//
// Using both Target and Targets on the same entry is invalid.
type StoreEntry struct {
	Target   string   `yaml:"target,omitempty"`
	Files    []string `yaml:"files,omitempty"`
	Patterns []string `yaml:"patterns,omitempty"`

	Targets []TargetEntry `yaml:"targets,omitempty"`
}

// HasFileMode returns true if any resolved target specifies individual files
// or patterns rather than a whole-directory symlink.
func (e StoreEntry) HasFileMode() bool {
	for _, t := range e.ResolvedTargets() {
		if t.HasFileMode() {
			return true
		}
	}
	return false
}

// IsMultiTarget returns true if the entry uses the targets list format.
func (e StoreEntry) IsMultiTarget() bool {
	return len(e.Targets) > 0
}

// ResolvedTargets normalizes both single-target and multi-target formats
// into a slice of TargetEntry.
func (e StoreEntry) ResolvedTargets() []TargetEntry {
	if len(e.Targets) > 0 {
		return e.Targets
	}
	if e.Target == "" {
		return nil
	}
	return []TargetEntry{{
		Target:   e.Target,
		Files:    e.Files,
		Patterns: e.Patterns,
	}}
}

// Validate checks that the entry is well-formed.
func (e StoreEntry) Validate() error {
	if e.Target != "" && len(e.Targets) > 0 {
		return fmt.Errorf("cannot use both 'target' and 'targets' on the same store entry")
	}
	if len(e.Targets) > 0 {
		// Files/Patterns at the top level are invalid with targets.
		if len(e.Files) > 0 || len(e.Patterns) > 0 {
			return fmt.Errorf("cannot use top-level 'files' or 'patterns' with 'targets'; place them inside each target entry")
		}
		for i, t := range e.Targets {
			if t.Target == "" {
				return fmt.Errorf("targets[%d]: target path is required", i)
			}
		}
	}
	return nil
}

// MigrateToMultiTarget converts a single-target entry to multi-target format.
func (e *StoreEntry) MigrateToMultiTarget() {
	if e.Target != "" {
		e.Targets = append(e.Targets, TargetEntry{
			Target:   e.Target,
			Files:    e.Files,
			Patterns: e.Patterns,
		})
		e.Target = ""
		e.Files = nil
		e.Patterns = nil
	}
}

// MigrateToSingleTarget converts back to single-target format if only one target remains.
func (e *StoreEntry) MigrateToSingleTarget() {
	if len(e.Targets) == 1 {
		t := e.Targets[0]
		e.Target = t.Target
		e.Files = t.Files
		e.Patterns = t.Patterns
		e.Targets = nil
	}
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

	for name, entry := range cfg.Stores {
		if err := entry.Validate(); err != nil {
			return nil, fmt.Errorf("store %q: %w", name, err)
		}
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
