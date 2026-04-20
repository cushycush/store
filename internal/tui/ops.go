package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/doctor"
	"github.com/cushycush/store/internal/importer"
	"github.com/cushycush/store/internal/linker"
	storeops "github.com/cushycush/store/internal/store"
)

// OpResult is the message posted when a background op finishes.
type OpResult struct {
	Label string
	Kind  ActivityKind
	Msg   string
	Err   error
	// Reload signals the model to reload config + re-stat every store.
	Reload bool
	// CopyPath is emitted by the path op so the model can append the path to the log.
	CopyPath string
}

// CmdApplyOne reconciles one store.
func CmdApplyOne(root, name string, entry config.StoreEntry) tea.Cmd {
	return func() tea.Msg {
		if err := storeops.Store(root, name, entry); err != nil {
			return OpResult{Label: "apply " + name, Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "apply " + name, Kind: ActivityOK, Msg: "applied " + name, Reload: true}
	}
}

// CmdApplyAll reconciles every store.
func CmdApplyAll(root string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if err := storeops.StoreAll(root, cfg); err != nil {
			return OpResult{Label: "apply", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "apply", Kind: ActivityOK, Msg: "applied " + itoa(len(cfg.Stores)) + " stores", Reload: true}
	}
}

// CmdUnlinkOne removes symlinks for one store but keeps its config entry.
func CmdUnlinkOne(root, name string, entry config.StoreEntry) tea.Cmd {
	return func() tea.Msg {
		if err := storeops.StoreRemove(root, name, entry); err != nil {
			return OpResult{Label: "unlink " + name, Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "unlink " + name, Kind: ActivityOK, Msg: "unlinked " + name, Reload: true}
	}
}

// CmdUnlinkAll removes every store's symlinks.
func CmdUnlinkAll(root string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if err := storeops.StoreRemoveAll(root, cfg); err != nil {
			return OpResult{Label: "unlink all", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "unlink all", Kind: ActivityOK, Msg: "unlinked all stores", Reload: true}
	}
}

// CmdRemoveOne unlinks and deletes the config entry.
func CmdRemoveOne(root string, cfg *config.Config, name string) tea.Cmd {
	return func() tea.Msg {
		entry, ok := cfg.Stores[name]
		if !ok {
			return OpResult{Label: "remove " + name, Kind: ActivityErr, Msg: "no such store: " + name, Err: fmt.Errorf("no such store"), Reload: true}
		}
		if err := storeops.StoreRemove(root, name, entry); err != nil {
			return OpResult{Label: "remove " + name, Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		delete(cfg.Stores, name)
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "remove " + name, Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "remove " + name, Kind: ActivityOK, Msg: "removed " + name, Reload: true}
	}
}

// CmdRemoveAll unlinks and deletes every store entry.
func CmdRemoveAll(root string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if err := storeops.StoreRemoveAll(root, cfg); err != nil {
			return OpResult{Label: "remove --all", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		cfg.Stores = map[string]config.StoreEntry{}
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "remove --all", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "remove --all", Kind: ActivityOK, Msg: "removed every store", Reload: true}
	}
}

// CmdDiff writes a one-line-per-file diff report into the activity log.
// Not a real op — it uses GetStatus and synthesizes messages.
func CmdDiff(root, name string, entry config.StoreEntry) tea.Cmd {
	return func() tea.Msg {
		results := storeops.GetStatus(root, name, entry)
		if len(results) == 0 {
			return OpResult{Label: "diff " + name, Kind: ActivityWarn, Msg: "no files to diff"}
		}
		var lines []string
		for _, info := range results {
			state := "?"
			switch {
			case info.Error != nil:
				state = "error"
			case info.Status == linker.StatusLinked:
				state = "ok"
			case info.Status == linker.StatusMissing:
				state = "create"
			case info.Status == linker.StatusConflict:
				state = "conflict"
			case info.Status == linker.StatusBroken:
				state = "replace"
			case info.Status == linker.StatusDrift:
				state = "drift"
			}
			target := info.Target
			if info.File != "" {
				target = info.File + " → " + info.Target
			}
			lines = append(lines, "diff "+name+": ["+state+"] "+target)
		}
		return OpResult{Label: "diff " + name, Kind: ActivityOK, Msg: strings.Join(lines, "\n")}
	}
}

// CmdInit creates the .store/config.yaml file.
func CmdInit(root string) tea.Cmd {
	return func() tea.Msg {
		if config.Exists(root) {
			return OpResult{Label: "init", Kind: ActivityWarn, Msg: "already initialized"}
		}
		if err := os.MkdirAll(filepath.Join(root, config.ConfigDir), 0o755); err != nil {
			return OpResult{Label: "init", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		cfg := &config.Config{Stores: map[string]config.StoreEntry{}}
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "init", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		return OpResult{Label: "init", Kind: ActivityOK, Msg: "initialized .store/config.yaml", Reload: true}
	}
}

// CmdImport scans default dirs for symlinks into the repo and imports them.
func CmdImport(root string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		dirs := defaultImportDirs()
		links, err := importer.Scan(root, dirs)
		if err != nil {
			return OpResult{Label: "import", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		if len(links) == 0 {
			return OpResult{Label: "import", Kind: ActivityOK, Msg: "no importable symlinks found"}
		}
		added := importer.ToConfig(links, root)
		if cfg.Stores == nil {
			cfg.Stores = map[string]config.StoreEntry{}
		}
		count := 0
		for name, entry := range added {
			if _, exists := cfg.Stores[name]; exists {
				continue
			}
			cfg.Stores[name] = entry
			count++
		}
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "import", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		return OpResult{Label: "import", Kind: ActivityOK, Msg: "imported " + itoa(count) + " stores", Reload: true}
	}
}

// CmdAdopt moves a path into the repo and creates a store entry.
func CmdAdopt(root string, cfg *config.Config, path string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			return OpResult{Label: "adopt", Kind: ActivityWarn, Msg: "path required"}
		}
		expanded, err := config.ExpandHome(path)
		if err != nil {
			return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		info, err := os.Lstat(abs)
		if err != nil {
			return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return OpResult{Label: "adopt", Kind: ActivityErr, Msg: "already a symlink — use import"}
		}
		// Derive store name from basename, strip leading dots.
		name := strings.TrimLeft(filepath.Base(abs), ".")
		if name == "" {
			name = "adopted"
		}
		if _, exists := cfg.Stores[name]; exists {
			return OpResult{Label: "adopt", Kind: ActivityErr, Msg: "store already exists: " + name}
		}
		dest := filepath.Join(root, name)
		if err := os.Rename(abs, dest); err != nil {
			return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		target := abs
		if info.IsDir() {
			entry := config.StoreEntry{Target: portableHome(target)}
			cfg.Stores[name] = entry
			if err := config.Save(root, cfg); err != nil {
				return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err}
			}
			if err := storeops.Store(root, name, entry); err != nil {
				return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
			}
		} else {
			entry := config.StoreEntry{Target: portableHome(filepath.Dir(target)), Files: []string{filepath.Base(target)}}
			cfg.Stores[name] = entry
			if err := config.Save(root, cfg); err != nil {
				return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err}
			}
			if err := storeops.Store(root, name, entry); err != nil {
				return OpResult{Label: "adopt", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
			}
		}
		return OpResult{Label: "adopt", Kind: ActivityOK, Msg: "adopted " + name + " ← " + path, Reload: true}
	}
}

// CmdAddStore adds a new store entry (saved; applied on next `apply`).
func CmdAddStore(root string, cfg *config.Config, name, target string) tea.Cmd {
	return func() tea.Msg {
		if name == "" {
			return OpResult{Label: "add", Kind: ActivityWarn, Msg: "name required"}
		}
		if _, exists := cfg.Stores[name]; exists {
			return OpResult{Label: "add", Kind: ActivityErr, Msg: "already exists: " + name}
		}
		if cfg.Stores == nil {
			cfg.Stores = map[string]config.StoreEntry{}
		}
		entry := config.StoreEntry{}
		if target != "" {
			entry.Target = target
		}
		cfg.Stores[name] = entry
		// Create the store directory if missing.
		_ = os.MkdirAll(filepath.Join(root, name), 0o755)
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "add", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		msg := "added " + name
		if target != "" {
			msg += " → " + target
		}
		return OpResult{Label: "add", Kind: ActivityOK, Msg: msg, Reload: true}
	}
}

// CmdModifyTarget replaces the target path on a single-target store.
// The full matrix of modify flags is accessible via the CLI; the TUI
// exposes the common case.
func CmdModifyTarget(root string, cfg *config.Config, name, target string) tea.Cmd {
	return func() tea.Msg {
		entry, ok := cfg.Stores[name]
		if !ok {
			return OpResult{Label: "modify", Kind: ActivityErr, Msg: "no such store: " + name}
		}
		if entry.IsMultiTarget() {
			return OpResult{Label: "modify", Kind: ActivityErr, Msg: "multi-target store — use `target modify`"}
		}
		_ = storeops.StoreRemove(root, name, entry)
		entry.Target = target
		cfg.Stores[name] = entry
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "modify", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		if err := storeops.Store(root, name, entry); err != nil {
			return OpResult{Label: "modify", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "modify", Kind: ActivityOK, Msg: "modified " + name + " → " + target, Reload: true}
	}
}

// CmdRename moves the store directory and updates the config key.
func CmdRename(root string, cfg *config.Config, old, neu string) tea.Cmd {
	return func() tea.Msg {
		if old == "" || neu == "" {
			return OpResult{Label: "rename", Kind: ActivityWarn, Msg: "need both old and new names"}
		}
		entry, ok := cfg.Stores[old]
		if !ok {
			return OpResult{Label: "rename", Kind: ActivityErr, Msg: "no such store: " + old}
		}
		if _, exists := cfg.Stores[neu]; exists {
			return OpResult{Label: "rename", Kind: ActivityErr, Msg: "already exists: " + neu}
		}
		_ = storeops.StoreRemove(root, old, entry)
		if err := os.Rename(filepath.Join(root, old), filepath.Join(root, neu)); err != nil {
			return OpResult{Label: "rename", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		delete(cfg.Stores, old)
		cfg.Stores[neu] = entry
		if err := config.Save(root, cfg); err != nil {
			return OpResult{Label: "rename", Kind: ActivityErr, Msg: err.Error(), Err: err}
		}
		if err := storeops.Store(root, neu, entry); err != nil {
			return OpResult{Label: "rename", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "rename", Kind: ActivityOK, Msg: old + " → " + neu, Reload: true}
	}
}

// CmdEditConfig opens $EDITOR on .store/config.yaml synchronously. Bubble
// Tea's ReleaseTerminal/RestoreTerminal would be nicer but this is simple.
func CmdEditConfig(root string) tea.Cmd {
	return func() tea.Msg {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		cmd := exec.Command(editor, config.ConfigPath(root))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return OpResult{Label: "edit", Kind: ActivityErr, Msg: err.Error(), Err: err, Reload: true}
		}
		return OpResult{Label: "edit", Kind: ActivityOK, Msg: "edited config", Reload: true}
	}
}

// CmdStatus writes a status line per store into the activity log.
func CmdStatus(root string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		results := storeops.GetStatusAll(root, cfg)
		lines := make([]string, 0, len(results))
		for _, r := range results {
			name := r.Name
			target := r.Target
			if r.File != "" {
				target = r.File + " → " + r.Target
			}
			state := fileState(r).Label()
			lines = append(lines, "status "+name+": ["+state+"] "+target)
		}
		if len(lines) == 0 {
			return OpResult{Label: "status", Kind: ActivityWarn, Msg: "no stores configured"}
		}
		return OpResult{Label: "status", Kind: ActivityOK, Msg: strings.Join(lines, "\n")}
	}
}

// CmdList writes a one-line summary per store to the log.
func CmdList(root string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if cfg == nil || len(cfg.Stores) == 0 {
			return OpResult{Label: "list", Kind: ActivityWarn, Msg: "no stores configured"}
		}
		var lines []string
		for name, entry := range cfg.Stores {
			switch ts := entry.ResolvedTargets(); len(ts) {
			case 0:
				lines = append(lines, "list "+name+" · (no target)")
			case 1:
				lines = append(lines, "list "+name+" → "+ts[0].Target)
			default:
				lines = append(lines, "list "+name+" · "+itoa(len(ts))+" targets")
			}
		}
		return OpResult{Label: "list", Kind: ActivityOK, Msg: strings.Join(lines, "\n")}
	}
}

// CmdPath logs the on-disk path to the store directory.
func CmdPath(root, name string, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if _, ok := cfg.Stores[name]; !ok {
			return OpResult{Label: "path", Kind: ActivityErr, Msg: "no such store: " + name}
		}
		p := filepath.Join(root, name)
		return OpResult{Label: "path", Kind: ActivityOK, Msg: p, CopyPath: p}
	}
}

// CmdDoctor runs doctor and emits a summary line.
func CmdDoctor(root string) tea.Cmd {
	return func() tea.Msg {
		issues := doctor.Check(root)
		errs, warns, infos := 0, 0, 0
		for _, i := range issues {
			switch i.Level {
			case "error":
				errs++
			case "warn", "warning":
				warns++
			default:
				infos++
			}
		}
		msg := fmt.Sprintf("doctor · %d errors, %d warnings, %d info", errs, warns, infos)
		kind := ActivityOK
		if errs > 0 {
			kind = ActivityErr
		} else if warns > 0 {
			kind = ActivityWarn
		}
		return OpResult{Label: "doctor", Kind: kind, Msg: msg}
	}
}

// CmdReloadConfig reloads from disk. Emitted after an op with Reload=true.
type ConfigReloadedMsg struct {
	Cfg *config.Config
	Err error
}

// CmdReloadConfig is kicked by the App after any op that mutates state.
func CmdReloadConfig(root string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load(root)
		return ConfigReloadedMsg{Cfg: cfg, Err: err}
	}
}

func defaultImportDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		home,
		filepath.Join(home, ".config"),
		filepath.Join(home, ".local", "share"),
		filepath.Join(home, ".local", "bin"),
	}
}

// portableHome substitutes ~ for $HOME so stored paths are portable.
func portableHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}
