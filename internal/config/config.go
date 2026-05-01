package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	core "github.com/cushycush/store-core/config"
	"github.com/cushycush/store-core/platform"
	"gopkg.in/yaml.v3"
)

// ConfigDir is the .store directory name, sourced from store-core.
const ConfigDir = core.ConfigDir

// ConfigFile is store's per-repo config filename.
const ConfigFile = "config.yaml"

// WhenClause and Strings are re-exported so callers within store keep their
// existing import path. WhenClause's fields are scalar-or-list slices; write
// either `os: linux` or `os: [linux, darwin]` in YAML.
type (
	WhenClause = core.WhenClause
	Strings    = core.Strings
)

// ExpandHome expands a leading ~ to the user's home directory.
func ExpandHome(path string) (string, error) { return core.ExpandHome(path) }

// FindRoot walks up from the current working directory to locate the repo
// root (the directory containing .store/).
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	root, err := core.FindRoot(dir)
	if err != nil {
		return "", fmt.Errorf("%w (run 'store init' first)", err)
	}
	return root, nil
}

// TargetEntry represents a single target within a store.
type TargetEntry struct {
	Target   string      `yaml:"target,omitempty"`
	Files    []string    `yaml:"files,omitempty"`
	Patterns []string    `yaml:"patterns,omitempty"`
	Ignore   []string    `yaml:"ignore,omitempty"`
	When     *WhenClause `yaml:"when,omitempty"`
}

// HookEntry defines pre and post shell commands to run around store operations.
type HookEntry struct {
	Pre  string `yaml:"pre,omitempty"`
	Post string `yaml:"post,omitempty"`
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
	Target   string        `yaml:"target,omitempty"`
	Files    []string      `yaml:"files,omitempty"`
	Patterns []string      `yaml:"patterns,omitempty"`
	Ignore   []string      `yaml:"ignore,omitempty"`
	Targets  []TargetEntry `yaml:"targets,omitempty"`
	Hooks    *HookEntry    `yaml:"hooks,omitempty"`
	When     *WhenClause   `yaml:"when,omitempty"`
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
// into a slice of TargetEntry. Per-target when: clauses are not consulted here;
// use ApplicableTargets when you only want targets that apply on the current
// platform.
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
		Ignore:   e.Ignore,
	}}
}

// ApplicableTargets returns the resolved targets that match the current platform
// according to each target's when: clause. Targets without a when: clause always
// apply. The store-level when: is filtered separately at the store level
// (see filterStoresByPlatform), so this method assumes the store itself is in
// scope and only narrows by target.
func (e StoreEntry) ApplicableTargets(info platform.Info) []TargetEntry {
	all := e.ResolvedTargets()
	out := make([]TargetEntry, 0, len(all))
	for _, t := range all {
		if t.When == nil || t.When.Matches(info) {
			out = append(out, t)
		}
	}
	return out
}

// Validate checks that the entry is well-formed.
func (e StoreEntry) Validate() error {
	if e.Target != "" && len(e.Targets) > 0 {
		return fmt.Errorf("cannot use both 'target' and 'targets' on the same store entry")
	}
	if len(e.Targets) > 0 {
		// Files/Patterns/Ignore at the top level are invalid with targets.
		if len(e.Files) > 0 || len(e.Patterns) > 0 || len(e.Ignore) > 0 {
			return fmt.Errorf("cannot use top-level 'files', 'patterns', or 'ignore' with 'targets'; place them inside each target entry")
		}
		for i, t := range e.Targets {
			if t.Target == "" {
				return fmt.Errorf("targets[%d]: target path is required", i)
			}
		}
	}

	// Check if this entry has no targets at all (YAML null parsed as empty string)
	if e.Target == "" && len(e.Targets) == 0 {
		fmt.Fprintf(os.Stderr, "warning: store entry has no target configured -- did you mean `target: \"~\"`?\n")
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
			Ignore:   e.Ignore,
		})
		e.Target = ""
		e.Files = nil
		e.Patterns = nil
		e.Ignore = nil
	}
}

// MigrateToSingleTarget converts back to single-target format if only one target remains.
// Single-target form has no slot for a per-target when: clause, so a remaining target
// that carries one stays in multi-target form.
func (e *StoreEntry) MigrateToSingleTarget() {
	if len(e.Targets) == 1 {
		t := e.Targets[0]
		if t.When != nil {
			return
		}
		e.Target = t.Target
		e.Files = t.Files
		e.Patterns = t.Patterns
		e.Ignore = t.Ignore
		e.Targets = nil
	}
}

// Config represents the full .store/config.yaml file.
//
// The on-disk YAML may write `stores:` as either a flat mapping where keys
// embed slashes ("desktop/hyprland: ...") or as a nested mapping where
// groups contain stores ("desktop:\n  hyprland: ..."). Both forms parse
// into the same flat `Stores` map keyed by full slash path; the tree view
// in the TUI derives its hierarchy from those slashes. On Save, the
// config writes back nested when there are no name collisions, and falls
// back to flat keys when a store name is also a prefix of another name.
type Config struct {
	Stores map[string]StoreEntry `yaml:"-"`
	Vars   map[string]string     `yaml:"-"`
}

// storeFieldSet enumerates the keys that mark a YAML mapping as a store
// entry rather than a nested group. Anything else at that level is a group
// containing more stores.
var storeFieldSet = map[string]bool{
	"target":   true,
	"targets":  true,
	"files":    true,
	"patterns": true,
	"ignore":   true,
	"hooks":    true,
	"when":     true,
}

// UnmarshalYAML parses either flat or nested `stores:` mappings.
func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	c.Stores = make(map[string]StoreEntry)
	c.Vars = nil
	if node.Kind == yaml.ScalarNode && node.Value == "" {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("config: expected mapping, got %s", yamlKindName(node.Kind))
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		switch k.Value {
		case "stores":
			if err := unmarshalStores(v, "", c.Stores); err != nil {
				return err
			}
		case "vars":
			if v.Kind == yaml.ScalarNode && v.Value == "" {
				continue
			}
			if err := v.Decode(&c.Vars); err != nil {
				return fmt.Errorf("vars: %w", err)
			}
		}
	}
	return nil
}

// MarshalYAML emits a nested `stores:` mapping where possible. When a store
// name is also a prefix of another (e.g. both "shells" and "shells/fish"
// exist), the whole tree falls back to flat slash-paths so the output is
// unambiguous.
func (c Config) MarshalYAML() (any, error) {
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	storesNode, err := buildStoresNode(c.Stores)
	if err != nil {
		return nil, err
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "stores"},
		storesNode,
	)
	if len(c.Vars) > 0 {
		varsNode := &yaml.Node{}
		if err := varsNode.Encode(c.Vars); err != nil {
			return nil, fmt.Errorf("vars: %w", err)
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "vars"},
			varsNode,
		)
	}
	return root, nil
}

func unmarshalStores(node *yaml.Node, prefix string, out map[string]StoreEntry) error {
	if node.Kind == yaml.ScalarNode && node.Value == "" {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("stores: expected mapping at %q", prefix)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i]
		v := node.Content[i+1]
		name := k.Value
		if prefix != "" {
			name = prefix + "/" + name
		}
		if v.Kind == yaml.MappingNode && !isStoreEntryNode(v) {
			if err := unmarshalStores(v, name, out); err != nil {
				return err
			}
			continue
		}
		var entry StoreEntry
		if v.Kind == yaml.MappingNode {
			if err := v.Decode(&entry); err != nil {
				return fmt.Errorf("store %q: %w", name, err)
			}
		}
		// Scalar null leaves entry zero-valued; Validate emits the warning.
		out[name] = entry
	}
	return nil
}

func isStoreEntryNode(node *yaml.Node) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if storeFieldSet[node.Content[i].Value] {
			return true
		}
	}
	return false
}

func buildStoresNode(stores map[string]StoreEntry) (*yaml.Node, error) {
	out := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if len(stores) == 0 {
		return out, nil
	}

	names := make([]string, 0, len(stores))
	for n := range stores {
		names = append(names, n)
	}
	sort.Strings(names)

	if hasPrefixCollision(names) {
		// Fall back to flat slash-paths so every entry remains addressable.
		for _, name := range names {
			v := &yaml.Node{}
			if err := v.Encode(stores[name]); err != nil {
				return nil, fmt.Errorf("encode %q: %w", name, err)
			}
			out.Content = append(out.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: name},
				v,
			)
		}
		return out, nil
	}

	type tnode struct {
		isStore  bool
		entry    StoreEntry
		children map[string]*tnode
	}
	root := &tnode{children: map[string]*tnode{}}
	for _, name := range names {
		parts := strings.Split(name, "/")
		cur := root
		for i, p := range parts {
			child, ok := cur.children[p]
			if !ok {
				child = &tnode{children: map[string]*tnode{}}
				cur.children[p] = child
			}
			if i == len(parts)-1 {
				child.isStore = true
				child.entry = stores[name]
			}
			cur = child
		}
	}

	var emit func(parent *yaml.Node, t *tnode) error
	emit = func(parent *yaml.Node, t *tnode) error {
		keys := make([]string, 0, len(t.children))
		for k := range t.children {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := t.children[k]
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
			if child.isStore {
				v := &yaml.Node{}
				if err := v.Encode(child.entry); err != nil {
					return err
				}
				parent.Content = append(parent.Content, keyNode, v)
				continue
			}
			v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			if err := emit(v, child); err != nil {
				return err
			}
			parent.Content = append(parent.Content, keyNode, v)
		}
		return nil
	}
	if err := emit(out, root); err != nil {
		return nil, err
	}
	return out, nil
}

// hasPrefixCollision reports whether any name is also a slash-prefix of
// another. When that happens the nested form would have to make a single
// key both a leaf entry and a parent group, so we emit flat instead.
func hasPrefixCollision(sortedNames []string) bool {
	for i, n := range sortedNames {
		for j := i + 1; j < len(sortedNames); j++ {
			m := sortedNames[j]
			if !strings.HasPrefix(m, n) {
				break
			}
			if len(m) > len(n) && m[len(n)] == '/' {
				return true
			}
		}
	}
	return false
}

func yamlKindName(k yaml.Kind) string {
	switch k {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.AliasNode:
		return "alias"
	case yaml.DocumentNode:
		return "document"
	}
	return "unknown"
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
