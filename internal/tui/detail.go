package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/platform"
	"github.com/cushycush/store/internal/render"
	storeops "github.com/cushycush/store/internal/store"
)

// Detail holds per-store detail state: which targets are expanded, and
// which target the user is "within" when navigating via j/k.
type Detail struct {
	expanded  map[string]map[string]bool // storeName -> targetPath -> expanded
	scanCache map[string]templateScan    // storeName -> cached template-ref counts
}

// templateScan caches the result of scanTemplateRefs for one store.
type templateScan struct {
	secrets int
	vars    int
}

// NewDetail returns a detail tracker. All targets default to expanded
// until the user collapses one.
func NewDetail() *Detail {
	return &Detail{
		expanded:  make(map[string]map[string]bool),
		scanCache: make(map[string]templateScan),
	}
}

// InvalidateScans clears the cached template-ref counts. Called from the
// app's refresh() so edits to store contents are reflected after `r`.
func (d *Detail) InvalidateScans() {
	d.scanCache = make(map[string]templateScan)
}

// IsExpanded reports whether the given target of a store is currently
// shown in full. New entries default to expanded.
func (d *Detail) IsExpanded(storeName, target string) bool {
	m, ok := d.expanded[storeName]
	if !ok {
		return true
	}
	v, ok := m[target]
	if !ok {
		return true
	}
	return v
}

// Toggle flips the expansion state of a target.
func (d *Detail) Toggle(storeName, target string) {
	m, ok := d.expanded[storeName]
	if !ok {
		m = make(map[string]bool)
		d.expanded[storeName] = m
	}
	cur := true
	if v, ok := m[target]; ok {
		cur = v
	}
	m[target] = !cur
}

// RenderDetail builds the detail section body for a single store. The
// caller supplies the already-rendered rule header separately.
func RenderDetail(root, name string, cfg *config.Config, pi platform.Info, d *Detail, width int) string {
	if name == "" || cfg == nil {
		return StyleDim.Render("  (no store selected)")
	}
	entry, ok := cfg.Stores[name]
	if !ok {
		return StyleDim.Render("  store not found: " + name)
	}
	var b strings.Builder

	// Basic properties block (target/targets, mode, platform filter).
	targets := entry.ResolvedTargets()
	switch len(targets) {
	case 0:
		b.WriteString(line("  target", StyleDim.Render("(none)")))
	case 1:
		t := targets[0]
		b.WriteString(line("  target", t.Target))
		b.WriteString(line("  mode", modeLabel(t)))
	default:
		b.WriteString(line("  targets", fmt.Sprintf("%d", len(targets))))
	}
	b.WriteString(line("  platform", fmt.Sprintf("%s · %s · %s", pi.OS, pi.Arch, pi.Distro)))
	if entry.When != nil {
		label := lipgloss.NewStyle().Foreground(ColorLinked).Render("matches")
		if !entry.When.Matches(pi) {
			label = lipgloss.NewStyle().Foreground(ColorDim).Render("skipped")
		}
		b.WriteString(line("  filter", label))
	}
	if secrets, vars := d.scanTemplates(filepath.Join(root, name), name); secrets > 0 || vars > 0 {
		b.WriteString(line("  templates", templateSummary(secrets, vars)))
	}
	if entry.Hooks != nil && (entry.Hooks.Pre != "" || entry.Hooks.Post != "") {
		b.WriteString("\n")
		b.WriteString("  " + StyleMuted.Render("hooks") + "\n")
		if entry.Hooks.Pre != "" {
			b.WriteString("    " + StyleDim.Render("pre  ") + StyleFg.Render(entry.Hooks.Pre) + "\n")
		}
		if entry.Hooks.Post != "" {
			b.WriteString("    " + StyleDim.Render("post ") + StyleFg.Render(entry.Hooks.Post) + "\n")
		}
	}
	b.WriteString("\n")

	// File/target bodies.
	results := storeops.GetStatus(root, name, entry)
	byTarget := groupByTarget(results)

	switch {
	case len(targets) == 0:
		// no-op
	case len(targets) == 1:
		t := targets[0]
		if t.HasFileMode() {
			b.WriteString(renderFilesBlock(byTarget[t.Target]))
		} else {
			// Whole-directory single-target: one-line status with the glyph.
			b.WriteString("  " + renderWholeDir(byTarget[t.Target]))
			b.WriteString("\n")
		}
	default:
		for _, t := range targets {
			expanded := d.IsExpanded(name, t.Target)
			agg := aggregate(byTarget[t.Target])
			b.WriteString(renderTargetRule(t.Target, agg, linkedCount(byTarget[t.Target]), totalCount(byTarget[t.Target]), expanded, width))
			b.WriteString("\n")
			if expanded {
				if t.HasFileMode() {
					b.WriteString(renderFilesBlock(byTarget[t.Target]))
				} else {
					b.WriteString("    " + renderWholeDir(byTarget[t.Target]))
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// line renders a two-column "label  value" row with aligned columns.
func line(label, value string) string {
	lab := lipgloss.NewStyle().Foreground(ColorMuted).Width(12).Render(label)
	return lab + "   " + StyleFg.Render(value) + "\n"
}

func modeLabel(t config.TargetEntry) string {
	switch {
	case t.HasFileMode() && len(t.Files) > 0 && len(t.Patterns) > 0:
		return "file mode · files + patterns"
	case t.HasFileMode() && len(t.Files) > 0:
		return "file mode · " + plural(len(t.Files), "file", "files")
	case t.HasFileMode() && len(t.Patterns) > 0:
		return "file mode · " + plural(len(t.Patterns), "pattern", "patterns")
	default:
		return "whole directory"
	}
}

func groupByTarget(results []storeops.StatusInfo) map[string][]storeops.StatusInfo {
	out := make(map[string][]storeops.StatusInfo)
	for _, r := range results {
		key := r.Target
		if r.File != "" {
			// For file-mode entries, Target is a per-file path. Collapse back to
			// the enclosing target by stripping the file suffix.
			if idx := strings.LastIndex(r.Target, r.File); idx > 0 {
				key = strings.TrimRight(r.Target[:idx], "/")
			}
		}
		out[key] = append(out[key], r)
	}
	return out
}

func linkedCount(infos []storeops.StatusInfo) int {
	n := 0
	for _, r := range infos {
		if r.Error == nil && r.Status == linker.StatusLinked {
			n++
		}
	}
	return n
}

func totalCount(infos []storeops.StatusInfo) int { return len(infos) }

func renderWholeDir(infos []storeops.StatusInfo) string {
	if len(infos) == 0 {
		return StyleDim.Render("(no info)")
	}
	info := infos[0]
	st := fileState(info)
	glyph := lipgloss.NewStyle().Foreground(st.Color()).Render(st.Glyph())
	return glyph + "  " + StyleFg.Render(info.Target)
}

func renderFilesBlock(infos []storeops.StatusInfo) string {
	if len(infos) == 0 {
		return "    " + StyleDim.Render("(no matching files)") + "\n"
	}
	var b strings.Builder
	b.WriteString("    " + StyleMuted.Render("files") + "\n")
	for _, info := range infos {
		st := fileState(info)
		glyph := lipgloss.NewStyle().Foreground(st.Color()).Render(st.Glyph())
		name := info.File
		if name == "" {
			name = info.Target
		}
		suffix := ""
		if info.Error != nil {
			suffix = "  " + StyleDim.Render("("+info.Error.Error()+")")
		} else if info.Status == linker.StatusBroken || info.Status == linker.StatusDrift {
			suffix = "  " + StyleDim.Render("("+info.Status.String()+")")
		}
		b.WriteString("      " + glyph + "  " + StyleFg.Render(name) + suffix + "\n")
	}
	return b.String()
}

func renderTargetRule(target string, st State, linked, total int, expanded bool, width int) string {
	arrow := "▾"
	if !expanded {
		arrow = "▸"
	}
	title := StyleDim.Render(arrow) + " " + StyleFg.Render(target)
	right := fmt.Sprintf("%d/%d %s", linked, total, st.Label())
	// Render a compact indented rule: 2-space indent, title, padding, right label.
	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}
	titleW := lipgloss.Width(title)
	rightStyled := lipgloss.NewStyle().Foreground(st.Color()).Render(right)
	rightW := lipgloss.Width(rightStyled)
	fill := innerWidth - titleW - rightW - 1
	if fill < 1 {
		fill = 1
	}
	return "  " + title + " " + strings.Repeat(" ", fill) + rightStyled
}

// scanTemplates returns cached template-ref counts for one store, computing
// and memoising on first call. The cache is invalidated by InvalidateScans,
// which the App calls on every refresh() so edits are picked up after `r`.
func (d *Detail) scanTemplates(dir, storeName string) (secrets, vars int) {
	if r, ok := d.scanCache[storeName]; ok {
		return r.secrets, r.vars
	}
	r := scanTemplateRefs(dir)
	d.scanCache[storeName] = r
	return r.secrets, r.vars
}

// scanTemplateRefs walks a store's source directory and returns the count
// of unique secret names and unique var names referenced across all text
// files. Missing directories, binary files, and read errors are skipped.
func scanTemplateRefs(dir string) templateScan {
	secretSet := map[string]struct{}{}
	varSet := map[string]struct{}{}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil || render.IsBinary(content) {
			return nil
		}
		if !render.HasTemplates(content) {
			return nil
		}
		for _, s := range render.SecretNames(content) {
			secretSet[s] = struct{}{}
		}
		for _, v := range render.VarNames(content) {
			varSet[v] = struct{}{}
		}
		return nil
	})
	return templateScan{secrets: len(secretSet), vars: len(varSet)}
}

func templateSummary(secrets, vars int) string {
	parts := make([]string, 0, 2)
	if secrets > 0 {
		parts = append(parts, plural(secrets, "secret", "secrets"))
	}
	if vars > 0 {
		parts = append(parts, plural(vars, "var", "vars"))
	}
	return strings.Join(parts, " · ")
}

func fileState(info storeops.StatusInfo) State {
	if info.Error != nil {
		return StateConflict
	}
	switch info.Status {
	case linker.StatusLinked:
		return StateLinked
	case linker.StatusMissing:
		return StateMissing
	case linker.StatusConflict:
		return StateConflict
	case linker.StatusBroken:
		return StateBroken
	case linker.StatusDrift:
		return StateDrift
	}
	return StateMissing
}
