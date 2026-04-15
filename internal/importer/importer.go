package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cushycush/store/internal/config"
)

// DiscoveredLink describes an existing symlink that points into the repo.
type DiscoveredLink struct {
	StoreName string // directory name in repo (e.g., "nvim")
	Source    string // path in repo (e.g., "/home/user/dotfiles/nvim")
	Target    string // symlink location (e.g., "/home/user/.config/nvim")
	File      string // empty for whole-dir, relative path for file-mode (e.g., ".zshrc")
}

// Scan checks the given directories for symlinks pointing into repoRoot.
func Scan(repoRoot string, scanDirs []string) ([]DiscoveredLink, error) {
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	realRepoRoot, err := filepath.EvalSymlinks(absRepoRoot)
	if err == nil {
		absRepoRoot = realRepoRoot
	}
	absRepoRoot = filepath.Clean(absRepoRoot)

	storeDirs, err := topLevelStoreDirs(absRepoRoot)
	if err != nil {
		return nil, err
	}

	var links []DiscoveredLink
	for _, scanDir := range scanDirs {
		expanded, err := config.ExpandHome(scanDir)
		if err != nil {
			return nil, fmt.Errorf("expand scan dir %q: %w", scanDir, err)
		}
		expanded, err = filepath.Abs(expanded)
		if err != nil {
			return nil, fmt.Errorf("resolve scan dir %q: %w", scanDir, err)
		}

		entries, err := os.ReadDir(expanded)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read scan dir %q: %w", expanded, err)
		}

		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink == 0 {
				continue
			}

			linkPath := filepath.Join(expanded, entry.Name())
			targetPath, err := os.Readlink(linkPath)
			if err != nil {
				return nil, fmt.Errorf("read symlink %q: %w", linkPath, err)
			}

			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(filepath.Dir(linkPath), targetPath)
			}
			targetPath, err = filepath.Abs(targetPath)
			if err != nil {
				return nil, fmt.Errorf("resolve symlink target %q: %w", linkPath, err)
			}
			if realTarget, err := filepath.EvalSymlinks(targetPath); err == nil {
				targetPath = realTarget
			}
			targetPath = filepath.Clean(targetPath)

			rel, ok := pathWithinRepo(absRepoRoot, targetPath)
			if !ok || rel == "." {
				continue
			}

			parts := strings.Split(rel, string(os.PathSeparator))
			storeName := parts[0]
			if !storeDirs[storeName] {
				continue
			}

			file := ""
			if len(parts) > 1 {
				file = filepath.Clean(filepath.Join(parts[1:]...))
			}

			links = append(links, DiscoveredLink{
				StoreName: storeName,
				Source:    targetPath,
				Target:    linkPath,
				File:      file,
			})
		}
	}

	sort.Slice(links, func(i, j int) bool {
		if links[i].StoreName != links[j].StoreName {
			return links[i].StoreName < links[j].StoreName
		}
		if links[i].Target != links[j].Target {
			return links[i].Target < links[j].Target
		}
		return links[i].File < links[j].File
	})

	return links, nil
}

// ToConfig converts discovered links into store config entries.
func ToConfig(links []DiscoveredLink, _ string) map[string]config.StoreEntry {

	type targetGroup struct {
		wholeDir bool
		files    map[string]struct{}
	}

	storeGroups := make(map[string]map[string]*targetGroup)
	for _, link := range links {
		target := portablePath(link.Target)
		if link.File != "" {
			target = portablePath(filepath.Dir(link.Target))
		}

		if storeGroups[link.StoreName] == nil {
			storeGroups[link.StoreName] = make(map[string]*targetGroup)
		}
		if storeGroups[link.StoreName][target] == nil {
			storeGroups[link.StoreName][target] = &targetGroup{files: make(map[string]struct{})}
		}

		group := storeGroups[link.StoreName][target]
		if link.File == "" {
			group.wholeDir = true
			continue
		}

		group.files[filepath.Clean(link.File)] = struct{}{}
	}

	entries := make(map[string]config.StoreEntry)
	storeNames := make([]string, 0, len(storeGroups))
	for name := range storeGroups {
		storeNames = append(storeNames, name)
	}
	sort.Strings(storeNames)

	for _, storeName := range storeNames {
		targetsByPath := storeGroups[storeName]
		targetPaths := make([]string, 0, len(targetsByPath))
		for target := range targetsByPath {
			targetPaths = append(targetPaths, target)
		}
		sort.Strings(targetPaths)

		targets := make([]config.TargetEntry, 0, len(targetPaths))
		for _, target := range targetPaths {
			group := targetsByPath[target]
			entry := config.TargetEntry{Target: target}
			if !group.wholeDir {
				entry.Files = sortedKeys(group.files)
			}
			targets = append(targets, entry)
		}

		if len(targets) == 1 {
			target := targets[0]
			entries[storeName] = config.StoreEntry{
				Target: target.Target,
				Files:  target.Files,
			}
			continue
		}

		entries[storeName] = config.StoreEntry{Targets: targets}
	}

	return entries
}

func topLevelStoreDirs(repoRoot string) (map[string]bool, error) {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("read repo root %q: %w", repoRoot, err)
	}

	storeDirs := make(map[string]bool)
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || name == config.ConfigDir || strings.HasPrefix(name, ".") {
			continue
		}
		storeDirs[name] = true
	}

	return storeDirs, nil
}

func pathWithinRepo(repoRoot, path string) (string, bool) {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return "", false
	}
	rel = filepath.Clean(rel)
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return rel, true
}

func portablePath(p string) string {
	cleaned := filepath.Clean(p)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.ToSlash(cleaned)
	}

	home = filepath.Clean(home)
	if cleaned == home {
		return "~"
	}

	prefix := home + string(os.PathSeparator)
	if suffix, ok := strings.CutPrefix(cleaned, prefix); ok {
		// Config files should use forward slashes so they're readable and
		// portable across machines; filepath.Join would produce backslashes
		// on Windows.
		return "~/" + filepath.ToSlash(suffix)
	}

	return filepath.ToSlash(cleaned)
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
