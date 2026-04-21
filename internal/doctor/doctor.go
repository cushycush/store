package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/linker"
	"github.com/cushycush/store/v2/internal/platform"
	"github.com/cushycush/store/v2/internal/render"
	"github.com/cushycush/store/v2/internal/secrets"
	storeops "github.com/cushycush/store/v2/internal/store"
)

type Issue struct {
	Level   string // "error", "warning", "info"
	Message string
}

// Check runs all diagnostics and returns a list of issues.
func Check(root string) []Issue {
	cfg, err := config.Load(root)
	if err != nil {
		return []Issue{{Level: "error", Message: err.Error()}}
	}

	platformInfo := platform.Detect()
	var issues []Issue

	issues = append(issues, checkOrphanedConfigEntries(root, cfg)...)
	issues = append(issues, checkUnconfiguredDirectories(root, cfg)...)
	issues = append(issues, checkBrokenSymlinks(root, cfg)...)
	issues = append(issues, checkConflictingTargets(cfg)...)
	issues = append(issues, checkMissingSecrets(root, cfg)...)
	issues = append(issues, checkEmptyStores(root, cfg)...)
	issues = append(issues, checkPlatformSkippedStores(cfg, platformInfo)...)
	issues = append(issues, checkSymlinkCapability(root, platformInfo)...)

	return issues
}

// checkSymlinkCapability verifies that the current process can create
// symlinks. On Windows this requires Developer Mode or the
// SeCreateSymbolicLinkPrivilege; without one of those, os.Symlink returns a
// permission error at apply time. We probe once so the user sees a clear
// warning from `store doctor` instead of a cryptic failure later.
func checkSymlinkCapability(root string, info platform.Info) []Issue {
	if info.OS != "windows" {
		return nil
	}

	dir := filepath.Join(root, ".store")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}

	target, err := os.CreateTemp(dir, "store-symlink-probe-*")
	if err != nil {
		return nil
	}
	targetPath := target.Name()
	_ = target.Close()
	defer os.Remove(targetPath)

	linkPath := targetPath + ".link"
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return []Issue{{
			Level:   "warning",
			Message: "cannot create symlinks on this system — enable Windows Developer Mode or run as Administrator",
		}}
	}
	_ = os.Remove(linkPath)
	return nil
}

func checkOrphanedConfigEntries(root string, cfg *config.Config) []Issue {
	var issues []Issue
	for _, name := range sortedStoreNames(cfg) {
		storeDir := filepath.Join(root, name)
		info, err := os.Stat(storeDir)
		if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
			issues = append(issues, Issue{
				Level:   "warning",
				Message: fmt.Sprintf("store %q has no directory — did you delete it?", name),
			})
		}
	}
	return issues
}

func checkUnconfiguredDirectories(root string, cfg *config.Config) []Issue {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var issues []Issue
	for _, entry := range entries {
		if !entry.IsDir() || isHidden(entry.Name()) {
			continue
		}
		if _, ok := cfg.Stores[entry.Name()]; ok {
			continue
		}
		issues = append(issues, Issue{
			Level:   "info",
			Message: fmt.Sprintf("directory %q exists but is not configured as a store", entry.Name()),
		})
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Message < issues[j].Message
	})
	return issues
}

func checkBrokenSymlinks(root string, cfg *config.Config) []Issue {
	results := storeops.GetStatusAll(root, cfg)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		return results[i].Target < results[j].Target
	})

	var issues []Issue
	for _, result := range results {
		if result.Error != nil || result.Status != linker.StatusBroken {
			continue
		}
		issues = append(issues, Issue{
			Level:   "error",
			Message: fmt.Sprintf("store %q target %q has a broken symlink", result.Name, result.Target),
		})
	}
	return issues
}

func checkConflictingTargets(cfg *config.Config) []Issue {
	type claim struct {
		store  string
		target string
	}

	claimed := make(map[string]claim)
	var issues []Issue

	for _, name := range sortedStoreNames(cfg) {
		entry := cfg.Stores[name]
		for _, targetEntry := range entry.ResolvedTargets() {
			normalized := normalizeTarget(targetEntry.Target)
			if existing, ok := claimed[normalized]; ok && existing.store != name {
				issues = append(issues, Issue{
					Level:   "error",
					Message: fmt.Sprintf("target %q is claimed by both store %q and store %q", existing.target, existing.store, name),
				})
				continue
			}
			claimed[normalized] = claim{store: name, target: targetEntry.Target}
		}
	}

	return issues
}

func checkMissingSecrets(root string, cfg *config.Config) []Issue {
	references := collectSecretReferences(root, cfg)
	if len(references) == 0 {
		return nil
	}

	if !secrets.Exists(root) {
		return []Issue{{
			Level:   "warning",
			Message: "secrets file not found but templates reference secrets",
		}}
	}

	passphrase := os.Getenv("STORE_PASSPHRASE")
	if passphrase == "" {
		return nil
	}

	secretMap, err := secrets.Load(root, passphrase)
	if err != nil {
		return []Issue{{
			Level:   "warning",
			Message: fmt.Sprintf("failed to load secrets store: %v", err),
		}}
	}

	var issues []Issue
	for _, ref := range references {
		if _, ok := secretMap[ref.name]; ok {
			continue
		}
		issues = append(issues, Issue{
			Level:   "warning",
			Message: fmt.Sprintf("secret %q referenced in %q but not found in secrets store", ref.name, ref.file),
		})
	}

	return issues
}

func checkEmptyStores(root string, cfg *config.Config) []Issue {
	var issues []Issue
	for _, name := range sortedStoreNames(cfg) {
		storeDir := filepath.Join(root, name)
		entries, err := os.ReadDir(storeDir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			issues = append(issues, Issue{
				Level:   "info",
				Message: fmt.Sprintf("store %q directory is empty", name),
			})
		}
	}
	return issues
}

func checkPlatformSkippedStores(cfg *config.Config, info platform.Info) []Issue {
	var issues []Issue
	for _, name := range sortedStoreNames(cfg) {
		entry := cfg.Stores[name]
		if entry.When == nil || entry.When.Matches(info) {
			continue
		}
		issues = append(issues, Issue{
			Level:   "info",
			Message: platformSkipMessage(name, entry.When, info),
		})
	}
	return issues
}

type secretReference struct {
	name string
	file string
}

func collectSecretReferences(root string, cfg *config.Config) []secretReference {
	var refs []secretReference
	seen := make(map[string]struct{})

	for _, name := range sortedStoreNames(cfg) {
		storeDir := filepath.Join(root, name)
		if info, err := os.Stat(storeDir); err != nil || !info.IsDir() {
			continue
		}

		_ = filepath.WalkDir(storeDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".store" || d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || d.Name() == ".gitignore" || d.Name() == ".DS_Store" {
				return nil
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil || !render.HasSecrets(content) {
				return nil
			}

			relFile, relErr := filepath.Rel(root, path)
			if relErr != nil {
				relFile = path
			}
			fileKey := filepath.ToSlash(relFile)

			seenNames := make(map[string]struct{})
			for _, secretName := range render.SecretNames(content) {
				if _, ok := seenNames[secretName]; ok {
					continue
				}
				seenNames[secretName] = struct{}{}

				key := secretName + "\x00" + fileKey
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				refs = append(refs, secretReference{name: secretName, file: fileKey})
			}

			return nil
		})
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].file != refs[j].file {
			return refs[i].file < refs[j].file
		}
		return refs[i].name < refs[j].name
	})

	return refs
}

func platformSkipMessage(name string, when *config.WhenClause, info platform.Info) string {
	field, want, current := firstPlatformMismatch(when, info)
	if field == "" {
		return fmt.Sprintf("store %q will be skipped on this platform", name)
	}
	return fmt.Sprintf("store %q will be skipped on this platform (when: %s=%s, current: %s)", name, field, want, current)
}

func firstPlatformMismatch(when *config.WhenClause, info platform.Info) (field, want, current string) {
	if when == nil {
		return "", "", ""
	}
	if when.OS != "" && when.OS != info.OS {
		return "os", when.OS, info.OS
	}
	if when.Arch != "" && when.Arch != info.Arch {
		return "arch", when.Arch, info.Arch
	}
	if when.Distro != "" && when.Distro != info.Distro {
		return "distro", when.Distro, info.Distro
	}
	if when.DistroVersion != "" && when.DistroVersion != info.DistroVersion {
		return "distro_version", when.DistroVersion, info.DistroVersion
	}
	if when.Hostname != "" && when.Hostname != info.Hostname {
		return "hostname", when.Hostname, info.Hostname
	}
	if when.Shell != "" && when.Shell != info.Shell {
		return "shell", when.Shell, info.Shell
	}
	if when.WSL != nil && *when.WSL != info.WSL {
		return "wsl", fmt.Sprintf("%t", *when.WSL), fmt.Sprintf("%t", info.WSL)
	}
	return "", "", ""
}

func sortedStoreNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Stores))
	for name := range cfg.Stores {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeTarget(target string) string {
	expanded, err := config.ExpandHome(target)
	if err != nil {
		return filepath.Clean(target)
	}
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return filepath.Clean(expanded)
	}
	return filepath.Clean(abs)
}

func isHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
