package tui

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PaletteCmd is a single entry in the command palette.
type PaletteCmd struct {
	Name    string   // canonical name, e.g., "target add" or "modify"
	Aliases []string // optional nicknames used for matching
	Summary string   // one-line description
	Prompt  string   // "<name>" or "<path>" etc., shown after selection
	Build   func(arg string) Intent
}

// Intent is a pending action the main model should dispatch. The palette
// surfaces it via the closing message; the App decides what to do.
type Intent struct {
	Kind IntentKind
	Arg  string
	Name string // command name, for logging
}

// IntentKind enumerates every palette-dispatchable action. Keep the list
// flat — the payload is always a string.
type IntentKind int

const (
	IntentNone IntentKind = iota
	IntentApplyAll
	IntentInit
	IntentImport
	IntentAdopt
	IntentAdd
	IntentModify
	IntentRemove
	IntentRemoveAll
	IntentList
	IntentPath
	IntentRename
	IntentEdit
	IntentStatus
	IntentDiff
	IntentDoctor
	IntentVersion
	IntentTargetAdd
	IntentTargetRemove
	IntentTargetModify
	IntentTargetWhen
	IntentSecretSet
	IntentSecretGet
	IntentSecretRemove
	IntentSecretList
)

// PaletteCommands returns the full command list. Kept here because it is
// the authoritative registry; everywhere else indexes into it.
func PaletteCommands() []PaletteCmd {
	return []PaletteCmd{
		{Name: "apply", Summary: "reconcile every store in the config",
			Build: func(_ string) Intent { return Intent{Kind: IntentApplyAll, Name: "apply"} }},
		{Name: "init", Summary: "create .store/config.yaml in this repo",
			Build: func(_ string) Intent { return Intent{Kind: IntentInit, Name: "init"} }},
		{Name: "import", Summary: "scan for existing symlinks and import them",
			Build: func(_ string) Intent { return Intent{Kind: IntentImport, Name: "import"} }},
		{Name: "adopt", Summary: "move a file or directory into the store and symlink it back", Prompt: "<path>",
			Build: func(s string) Intent { return Intent{Kind: IntentAdopt, Arg: s, Name: "adopt"} }},
		{Name: "add", Summary: "create a new store entry", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentAdd, Arg: s, Name: "add"} }},
		{Name: "modify", Summary: "replace files or target on an existing store", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentModify, Arg: s, Name: "modify"} }},
		{Name: "remove", Summary: "unlink a store and delete its entry", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentRemove, Arg: s, Name: "remove"} }},
		{Name: "remove --all", Aliases: []string{"removeall"}, Summary: "remove every configured store (confirmed)",
			Build: func(_ string) Intent { return Intent{Kind: IntentRemoveAll, Name: "remove --all"} }},
		{Name: "list", Summary: "print a one-line summary of every store",
			Build: func(_ string) Intent { return Intent{Kind: IntentList, Name: "list"} }},
		{Name: "path", Summary: "copy the on-disk path of a store directory", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentPath, Arg: s, Name: "path"} }},
		{Name: "rename", Summary: "rename a store (updates config and moves the directory)", Prompt: "<old> <new>",
			Build: func(s string) Intent { return Intent{Kind: IntentRename, Arg: s, Name: "rename"} }},
		{Name: "edit", Summary: "open .store/config.yaml in $EDITOR",
			Build: func(_ string) Intent { return Intent{Kind: IntentEdit, Name: "edit"} }},
		{Name: "status", Summary: "show symlink state for every store",
			Build: func(_ string) Intent { return Intent{Kind: IntentStatus, Name: "status"} }},
		{Name: "diff", Summary: "preview what apply would change",
			Build: func(_ string) Intent { return Intent{Kind: IntentDiff, Name: "diff"} }},
		{Name: "doctor", Summary: "run health diagnostics",
			Build: func(_ string) Intent { return Intent{Kind: IntentDoctor, Name: "doctor"} }},
		{Name: "version", Summary: "print the store version",
			Build: func(_ string) Intent { return Intent{Kind: IntentVersion, Name: "version"} }},
		{Name: "target add", Summary: "add a target to a store", Prompt: "<name> <path>",
			Build: func(s string) Intent { return Intent{Kind: IntentTargetAdd, Arg: s, Name: "target add"} }},
		{Name: "target remove", Summary: "remove a target from a store", Prompt: "<name> <path>",
			Build: func(s string) Intent { return Intent{Kind: IntentTargetRemove, Arg: s, Name: "target remove"} }},
		{Name: "target modify", Summary: "replace files or patterns on a target", Prompt: "<name> <path>",
			Build: func(s string) Intent { return Intent{Kind: IntentTargetModify, Arg: s, Name: "target modify"} }},
		{Name: "target when", Summary: "set or clear a target's platform filter", Prompt: "<name> <path>",
			Build: func(s string) Intent { return Intent{Kind: IntentTargetWhen, Arg: s, Name: "target when"} }},
		{Name: "secret set", Summary: "create or update an encrypted secret", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentSecretSet, Arg: s, Name: "secret set"} }},
		{Name: "secret get", Summary: "print the value of one decrypted secret", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentSecretGet, Arg: s, Name: "secret get"} }},
		{Name: "secret remove", Aliases: []string{"secret rm"}, Summary: "delete a secret", Prompt: "<name>",
			Build: func(s string) Intent { return Intent{Kind: IntentSecretRemove, Arg: s, Name: "secret remove"} }},
		{Name: "secret list", Summary: "list stored secret names",
			Build: func(_ string) Intent { return Intent{Kind: IntentSecretList, Name: "secret list"} }},
	}
}

// Palette is the `:` command palette overlay.
type Palette struct {
	query    textinput.Model
	arg      textinput.Model
	commands []PaletteCmd
	filtered []match
	cursor   int
	chosen   *PaletteCmd
	intent   Intent
	done     bool

	reveal float64 // 0..1 slide-down progress
}

type match struct {
	idx   int
	score int
}

// NewPalette constructs an empty palette in query phase.
func NewPalette() *Palette {
	q := textinput.New()
	q.Prompt = ""
	q.Placeholder = "type a command…"
	q.CharLimit = 64
	q.Focus()
	p := &Palette{query: q, commands: PaletteCommands()}
	p.rebuildMatches()
	return p
}

// Tick advances the slide-in reveal animation.
func (p *Palette) Tick(dt float64) { p.reveal = min1(p.reveal + dt*8) }

// Revealed reports the current 0..1 reveal progress.
func (p *Palette) Revealed() float64 { return p.reveal }

// Done reports whether the palette should close.
func (p *Palette) Done() bool { return p.done }

// Intent returns the chosen intent (zero if cancelled).
func (p *Palette) Intent() Intent { return p.intent }

// Update processes a key message.
func (p *Palette) Update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch k.String() {
	case "esc":
		p.done = true
		return nil
	case "enter":
		if p.chosen == nil {
			if len(p.filtered) == 0 {
				return nil
			}
			sel := &p.commands[p.filtered[p.cursor].idx]
			if sel.Prompt == "" {
				p.intent = sel.Build("")
				p.done = true
				return nil
			}
			p.chosen = sel
			p.arg = newArgInput(sel.Prompt)
			return nil
		}
		p.intent = p.chosen.Build(strings.TrimSpace(p.arg.Value()))
		p.done = true
		return nil
	case "down", "ctrl+n":
		if p.chosen == nil && p.cursor < len(p.filtered)-1 {
			p.cursor++
		}
		return nil
	case "up", "ctrl+p":
		if p.chosen == nil && p.cursor > 0 {
			p.cursor--
		}
		return nil
	}
	if p.chosen != nil {
		var cmd tea.Cmd
		p.arg, cmd = p.arg.Update(msg)
		return cmd
	}
	var cmd tea.Cmd
	p.query, cmd = p.query.Update(msg)
	p.rebuildMatches()
	return cmd
}

// View renders the palette body.
func (p *Palette) View(width int) string {
	prompt := StyleEmber.Render(":") + " " + p.query.View()
	var lines []string
	lines = append(lines, prompt)
	if p.chosen != nil {
		argLine := StyleDim.Render(">> ") + StyleFg.Render(p.chosen.Name) + " " +
			StyleDim.Render(p.chosen.Prompt) + "\n" +
			StyleDim.Render("   ") + p.arg.View()
		lines = append(lines, argLine)
		lines = append(lines, StyleDim.Render("  enter run · esc cancel"))
		return strings.Join(lines, "\n")
	}
	if len(p.filtered) == 0 {
		lines = append(lines, StyleDim.Render("  no matches"))
	} else {
		for i, m := range p.filtered {
			if i >= 10 {
				break
			}
			c := &p.commands[m.idx]
			marker := "  "
			nameStyle := StyleFg
			summaryStyle := StyleDim
			if i == p.cursor {
				marker = StyleEmber.Render(GlyphCursor) + " "
				nameStyle = StyleSelected
				summaryStyle = StyleMuted
			}
			name := nameStyle.Render(c.Name)
			arg := ""
			if c.Prompt != "" {
				arg = " " + StyleDim.Render(c.Prompt)
			}
			pad := 20 - lipgloss.Width(c.Name)
			if pad < 2 {
				pad = 2
			}
			line := marker + name + arg + strings.Repeat(" ", max0(pad, 2)) +
				summaryStyle.Render(Clip(c.Summary, max0(width-36, 10)))
			lines = append(lines, line)
		}
	}
	lines = append(lines, StyleDim.Render("  enter select · esc close · ↑/↓ move"))
	return strings.Join(lines, "\n")
}

// Footer returns the key hints line for the overlay frame.
func (p *Palette) Footer() string {
	return StyleDim.Render("command palette · all store commands")
}

func (p *Palette) rebuildMatches() {
	q := strings.TrimSpace(strings.ToLower(p.query.Value()))
	p.filtered = p.filtered[:0]
	for i, c := range p.commands {
		score := fuzzyScore(q, c.Name, c.Aliases)
		if score < 0 {
			continue
		}
		p.filtered = append(p.filtered, match{idx: i, score: score})
	}
	sort.SliceStable(p.filtered, func(i, j int) bool {
		if p.filtered[i].score != p.filtered[j].score {
			return p.filtered[i].score > p.filtered[j].score
		}
		return p.commands[p.filtered[i].idx].Name < p.commands[p.filtered[j].idx].Name
	})
	if p.cursor >= len(p.filtered) {
		p.cursor = 0
	}
}

// fuzzyScore returns a positive score for a match, or -1 if no match.
// Simple scheme: exact-prefix > word-prefix > subsequence > no match.
func fuzzyScore(query, name string, aliases []string) int {
	if query == "" {
		return 1
	}
	candidates := append([]string{name}, aliases...)
	best := -1
	for _, c := range candidates {
		lc := strings.ToLower(c)
		switch {
		case lc == query:
			if best < 100 {
				best = 100
			}
		case strings.HasPrefix(lc, query):
			if best < 80 {
				best = 80
			}
		case wordPrefix(lc, query):
			if best < 60 {
				best = 60
			}
		case strings.Contains(lc, query):
			if best < 40 {
				best = 40
			}
		case subseq(lc, query):
			if best < 20 {
				best = 20
			}
		}
	}
	return best
}

func wordPrefix(s, q string) bool {
	for _, word := range strings.Fields(s) {
		if strings.HasPrefix(word, q) {
			return true
		}
	}
	return false
}

func subseq(s, q string) bool {
	if q == "" {
		return true
	}
	qi := 0
	qr := []rune(q)
	for _, r := range s {
		if qi < len(qr) && unicode.ToLower(r) == unicode.ToLower(qr[qi]) {
			qi++
		}
	}
	return qi == len(qr)
}

func newArgInput(prompt string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = prompt
	ti.CharLimit = 512
	ti.Focus()
	return ti
}

func min1(x float64) float64 {
	if x > 1 {
		return 1
	}
	return x
}
