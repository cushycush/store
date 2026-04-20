package tui

import (
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cushycush/store/internal/config"
	"github.com/cushycush/store/internal/linker"
	"github.com/cushycush/store/internal/platform"
	storeops "github.com/cushycush/store/internal/store"
)

// OverlayKind identifies the active overlay, if any.
type OverlayKind int

const (
	OverlayNone OverlayKind = iota
	OverlayPalette
	OverlayActions
	OverlayConfirm
	OverlayInput
	OverlaySecrets
	OverlayDoctor
	OverlayHelp
)

// App is the top-level Bubble Tea model.
type App struct {
	// immutable references
	root    string
	version string

	// config + derived state
	cfg      *config.Config
	plat     platform.Info
	uninit   bool
	stores   *Stores
	detail   *Detail
	activity *Activity

	keys Keymap

	// window + readiness
	width, height int
	ready         bool

	// main-view focus
	fullscreenLog bool

	// filter
	filterMode  bool
	filterInput textinput.Model

	// overlays
	overlay      OverlayKind
	palette      *Palette
	actions      *Actions
	confirm      *Confirm
	input        *Input
	secrets      *Secrets
	doctor       *Doctor
	help         *Help
	inputAction  string // routes input result to the right handler
	inputContext string // carries names/paths across multi-step prompts

	// motion
	startedAt     time.Time
	detailFlashAt time.Time
	freshMarks    map[string]time.Time

	// pending confirm action
	confirmAction string
	confirmCtx    string
}

// New constructs the model. cfg may be nil if the repo isn't initialized.
func New(root, version string, cfg *config.Config) *App {
	if cfg == nil {
		cfg = &config.Config{Stores: map[string]config.StoreEntry{}}
	}
	a := &App{
		root:       root,
		version:    version,
		cfg:        cfg,
		plat:       platform.Detect(),
		keys:       DefaultKeymap(),
		stores:     NewStores(),
		detail:     NewDetail(),
		activity:   NewActivity(200),
		uninit:     !config.Exists(root),
		freshMarks: map[string]time.Time{},
		startedAt:  time.Now(),
	}
	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 64
	a.filterInput = ti
	a.refresh()
	return a
}

// Run starts the program.
func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init implements tea.Model. We kick off the continuous animation tick.
func (a *App) Init() tea.Cmd {
	return Tick()
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.ready = true
		return a, nil

	case FrameMsg:
		// Decay fresh-change sparks. A tick is cheap; re-render every frame
		// while any animation is in flight or the heartbeat is visible.
		return a, Tick()

	case OpResult:
		a.absorbOpResult(m)
		if m.Reload {
			return a, CmdReloadConfig(a.root)
		}
		return a, nil

	case ConfigReloadedMsg:
		if m.Err == nil && m.Cfg != nil {
			a.cfg = m.Cfg
			a.uninit = false
		}
		a.refresh()
		return a, nil
	}

	// Overlay takes keystrokes first.
	if a.overlay != OverlayNone {
		return a.updateOverlay(msg)
	}
	if a.filterMode {
		return a.updateFilter(msg)
	}
	if k, ok := msg.(tea.KeyMsg); ok {
		return a.handleKey(k)
	}
	return a, nil
}

func (a *App) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.fullscreenLog {
		switch k.String() {
		case "esc", "\\":
			a.fullscreenLog = false
		case "j", "down":
			a.activity.ScrollDown()
		case "k", "up":
			a.activity.ScrollUp()
		case "q", "ctrl+c":
			return a, tea.Quit
		}
		return a, nil
	}

	switch {
	case key.Matches(k, a.keys.Quit):
		return a, tea.Quit
	case key.Matches(k, a.keys.Help):
		a.overlay = OverlayHelp
		a.help = NewHelp(a.keys)
		return a, nil
	case key.Matches(k, a.keys.Palette):
		a.overlay = OverlayPalette
		a.palette = NewPalette()
		return a, nil
	case key.Matches(k, a.keys.Refresh):
		a.refresh()
		a.activity.Ok("refreshed")
		return a, nil
	case key.Matches(k, a.keys.Activity):
		a.fullscreenLog = !a.fullscreenLog
		return a, nil
	case key.Matches(k, a.keys.Filter):
		a.filterMode = true
		a.filterInput.SetValue("")
		a.filterInput.Focus()
		return a, textinput.Blink
	}

	if a.uninit {
		if k.String() == "i" {
			return a, CmdInit(a.root)
		}
		if key.Matches(k, a.keys.Palette) {
			a.overlay = OverlayPalette
			a.palette = NewPalette()
		}
		return a, nil
	}

	return a.handleListKey(k)
}

func (a *App) handleListKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(k, a.keys.Up):
		a.stores.Up()
		a.flashDetail()
	case key.Matches(k, a.keys.Down):
		a.stores.Down()
		a.flashDetail()
	case key.Matches(k, a.keys.Top):
		a.stores.Top()
		a.flashDetail()
	case key.Matches(k, a.keys.Bottom):
		a.stores.Bottom()
		a.flashDetail()
	case key.Matches(k, a.keys.Enter):
		if name := a.stores.Selected(); name != "" {
			a.overlay = OverlayActions
			a.actions = NewActions(name)
		}
	case key.Matches(k, a.keys.Space):
		return a, a.quickToggle()
	case key.Matches(k, a.keys.Diff):
		if name := a.stores.Selected(); name != "" {
			return a, CmdDiff(a.root, name, a.cfg.Stores[name])
		}
	case key.Matches(k, a.keys.ApplyAll):
		return a, CmdApplyAll(a.root, a.cfg)
	case key.Matches(k, a.keys.Remove):
		if name := a.stores.Selected(); name != "" {
			a.openRemoveConfirm(name)
		}
	case key.Matches(k, a.keys.Back):
		// esc clears filter if active
		if a.stores.FilterQuery() != "" {
			a.stores.Filter("")
		}
	case k.String() == " ":
		// Handled by key.Matches above via keys.Space
	default:
		// For multi-target stores, allow space-on-target-row via detail
		// pane keys — TODO (not wired into this list handler yet).
	}
	return a, nil
}

func (a *App) quickToggle() tea.Cmd {
	name := a.stores.Selected()
	if name == "" {
		return nil
	}
	entry := a.cfg.Stores[name]
	results := storeops.GetStatus(a.root, name, entry)
	allLinked := len(results) > 0
	for _, r := range results {
		if r.Error != nil || r.Status != linker.StatusLinked {
			allLinked = false
			break
		}
	}
	if allLinked {
		return CmdUnlinkOne(a.root, name, entry)
	}
	return CmdApplyOne(a.root, name, entry)
}

func (a *App) updateFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}
	switch k.String() {
	case "esc":
		a.filterMode = false
		a.filterInput.Blur()
		a.stores.Filter("")
		return a, nil
	case "enter":
		a.filterMode = false
		a.filterInput.Blur()
		return a, nil
	}
	var cmd tea.Cmd
	a.filterInput, cmd = a.filterInput.Update(msg)
	a.stores.Filter(a.filterInput.Value())
	return a, cmd
}

func (a *App) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch a.overlay {
	case OverlayPalette:
		if a.palette == nil {
			a.overlay = OverlayNone
			return a, nil
		}
		cmd := a.palette.Update(msg)
		// advance reveal each frame
		if _, ok := msg.(FrameMsg); ok {
			a.palette.Tick(1.0 / float64(AnimationFPS))
		}
		if a.palette.Done() {
			intent := a.palette.Intent()
			a.overlay = OverlayNone
			a.palette = nil
			return a, a.dispatchIntent(intent)
		}
		return a, cmd
	case OverlayActions:
		if a.actions == nil {
			a.overlay = OverlayNone
			return a, nil
		}
		if a.actions.Update(msg) {
			chosen := a.actions.Chosen()
			name := a.actions.StoreName
			a.overlay = OverlayNone
			a.actions = nil
			return a, a.dispatchAction(chosen, name)
		}
		return a, nil
	case OverlayConfirm:
		cmd := a.confirm.Update(msg)
		if a.confirm.Done() {
			ok := a.confirm.Ok()
			action := a.confirmAction
			ctx := a.confirmCtx
			a.overlay = OverlayNone
			a.confirm = nil
			a.confirmAction = ""
			a.confirmCtx = ""
			if ok {
				return a, a.runConfirmed(action, ctx)
			}
			a.activity.Warn("cancelled " + action)
			return a, nil
		}
		return a, cmd
	case OverlayInput:
		cmd := a.input.Update(msg)
		if a.input.Done() {
			if a.input.Cancelled() {
				a.overlay = OverlayNone
				a.input = nil
				a.inputAction = ""
				a.inputContext = ""
				return a, nil
			}
			value := a.input.Value()
			action := a.inputAction
			ctx := a.inputContext
			a.overlay = OverlayNone
			a.input = nil
			a.inputAction = ""
			a.inputContext = ""
			return a, a.runInput(action, ctx, value)
		}
		return a, cmd
	case OverlaySecrets:
		cmd := a.secrets.Update(msg)
		if a.secrets.Done() {
			a.overlay = OverlayNone
			a.secrets = nil
		}
		return a, cmd
	case OverlayDoctor:
		cmd := a.doctor.Update(msg)
		if a.doctor.Done() {
			a.overlay = OverlayNone
			a.doctor = nil
		}
		return a, cmd
	case OverlayHelp:
		a.help.Update(msg)
		if a.help.Done() {
			a.overlay = OverlayNone
			a.help = nil
		}
		return a, nil
	}
	return a, nil
}

// dispatchAction maps ActionID (row menu) to a tea.Cmd or follow-up overlay.
func (a *App) dispatchAction(id ActionID, name string) tea.Cmd {
	if name == "" {
		return nil
	}
	entry := a.cfg.Stores[name]
	switch id {
	case ActionApply:
		return CmdApplyOne(a.root, name, entry)
	case ActionUnlink:
		return CmdUnlinkOne(a.root, name, entry)
	case ActionDiff:
		return CmdDiff(a.root, name, entry)
	case ActionModify:
		a.openInput("modify_target", name,
			"new target for "+name,
			"leaves files/patterns untouched",
			currentTarget(entry))
		return nil
	case ActionRename:
		a.openInput("rename", name, "rename "+name+" to…",
			"moves the store directory and updates config", "")
		return nil
	case ActionTargetOps:
		// Open the palette preloaded with "target" so the user sees the
		// three sub-commands immediately. We push the query by hand.
		a.overlay = OverlayPalette
		p := NewPalette()
		p.query.SetValue("target ")
		p.rebuildMatches()
		a.palette = p
		return nil
	case ActionPath:
		return CmdPath(a.root, name, a.cfg)
	case ActionRemove:
		a.openRemoveConfirm(name)
		return nil
	}
	return nil
}

// dispatchIntent maps a palette intent to a tea.Cmd or follow-up overlay.
func (a *App) dispatchIntent(i Intent) tea.Cmd {
	switch i.Kind {
	case IntentNone:
		return nil
	case IntentApplyAll:
		return CmdApplyAll(a.root, a.cfg)
	case IntentInit:
		return CmdInit(a.root)
	case IntentImport:
		return CmdImport(a.root, a.cfg)
	case IntentAdopt:
		if i.Arg == "" {
			a.openInput("adopt", "", "path to adopt", "e.g. ~/.config/nvim or ~/.zshrc", "")
			return nil
		}
		return CmdAdopt(a.root, a.cfg, i.Arg)
	case IntentAdd:
		if i.Arg == "" {
			a.openInput("add_name", "", "new store name", "", "")
			return nil
		}
		a.openInput("add_target", i.Arg,
			"target for "+i.Arg,
			"leave blank to save entry without linking", "")
		return nil
	case IntentModify:
		if i.Arg == "" {
			a.openInput("modify_name", "", "store name to modify", "", "")
			return nil
		}
		entry := a.cfg.Stores[i.Arg]
		a.openInput("modify_target", i.Arg,
			"new target for "+i.Arg,
			"leaves files/patterns untouched",
			currentTarget(entry))
		return nil
	case IntentRemove:
		if i.Arg == "" {
			a.openInput("remove_name", "", "store name to remove", "", "")
			return nil
		}
		a.openRemoveConfirm(i.Arg)
		return nil
	case IntentRemoveAll:
		a.confirmAction = "remove_all"
		a.confirm = NewConfirm(
			"remove all stores?",
			[]string{
				"this will:",
				"  · unlink every symlink",
				"  · delete every config entry",
				"  · leave store directories in the repo untouched",
			},
			"remove all",
		)
		a.overlay = OverlayConfirm
		return nil
	case IntentList:
		return CmdList(a.root, a.cfg)
	case IntentPath:
		if i.Arg == "" {
			a.openInput("path_name", "", "store name", "", "")
			return nil
		}
		return CmdPath(a.root, i.Arg, a.cfg)
	case IntentRename:
		if i.Arg == "" {
			a.openInput("rename_old", "", "store to rename", "", "")
			return nil
		}
		parts := strings.Fields(i.Arg)
		if len(parts) >= 2 {
			return CmdRename(a.root, a.cfg, parts[0], parts[1])
		}
		a.openInput("rename_new", parts[0], "new name for "+parts[0], "", "")
		return nil
	case IntentEdit:
		// Release the terminal so $EDITOR can take over, then resume.
		return tea.ExecProcess(nil, func(err error) tea.Msg {
			return OpResult{Label: "edit", Kind: ActivityOK, Msg: "edited config", Reload: true}
		})
	case IntentStatus:
		return CmdStatus(a.root, a.cfg)
	case IntentDiff:
		// Diff every store.
		return func() tea.Msg {
			var lines []string
			for name, entry := range a.cfg.Stores {
				results := storeops.GetStatus(a.root, name, entry)
				for _, r := range results {
					lines = append(lines, "diff "+name+": ["+fileState(r).Label()+"] "+r.Target)
				}
			}
			return OpResult{Label: "diff", Kind: ActivityOK, Msg: strings.Join(lines, "\n")}
		}
	case IntentDoctor:
		a.overlay = OverlayDoctor
		a.doctor = NewDoctor(a.root)
		return nil
	case IntentVersion:
		a.activity.Ok("store version " + a.version)
		return nil
	case IntentSecretSet, IntentSecretGet, IntentSecretRemove, IntentSecretList:
		a.overlay = OverlaySecrets
		a.secrets = NewSecrets(a.root)
		// Secret-* intents all land in the same overlay; it handles
		// name/value/delete flows itself.
		return nil
	case IntentTargetAdd, IntentTargetRemove, IntentTargetModify:
		// These want a name + path. Punt to the palette's argument prompt:
		// the palette already captured the args into Arg. Split them.
		parts := strings.Fields(i.Arg)
		if len(parts) < 2 {
			a.activity.Warn(i.Name + ": need `<name> <target>`")
			return nil
		}
		// Minimal in-TUI handling: guide user to the CLI for complex cases.
		a.activity.Ok(i.Name + " · " + strings.Join(parts, " ") + " — run from the CLI for flag options")
		return nil
	}
	return nil
}

// openInput configures an input overlay, routing its result back through
// runInput.
func (a *App) openInput(action, ctx, title, hint, defaultVal string) {
	a.inputAction = action
	a.inputContext = ctx
	in := NewInput(title, hint, "", false)
	if defaultVal != "" {
		in.input.SetValue(defaultVal)
	}
	a.input = in
	a.overlay = OverlayInput
}

// openRemoveConfirm opens the destructive-confirmation overlay for a
// single store.
func (a *App) openRemoveConfirm(name string) {
	a.confirmAction = "remove"
	a.confirmCtx = name
	entry := a.cfg.Stores[name]
	body := []string{"this will:"}
	for _, t := range entry.ResolvedTargets() {
		body = append(body, "  · unlink "+t.Target)
	}
	body = append(body, "  · delete the config entry",
		"  · leave "+a.root+"/"+name+" in the repo untouched")
	a.confirm = NewConfirm("remove "+name+"?", body, name)
	a.overlay = OverlayConfirm
}

// runInput processes the value returned by an input overlay.
func (a *App) runInput(action, ctx, value string) tea.Cmd {
	switch action {
	case "adopt":
		return CmdAdopt(a.root, a.cfg, value)
	case "add_name":
		a.openInput("add_target", value, "target for "+value,
			"leave blank to save entry without linking", "")
		return nil
	case "add_target":
		return CmdAddStore(a.root, a.cfg, ctx, value)
	case "modify_name":
		entry := a.cfg.Stores[value]
		a.openInput("modify_target", value,
			"new target for "+value,
			"leaves files/patterns untouched",
			currentTarget(entry))
		return nil
	case "modify_target":
		return CmdModifyTarget(a.root, a.cfg, ctx, value)
	case "remove_name":
		a.openRemoveConfirm(value)
		return nil
	case "path_name":
		return CmdPath(a.root, value, a.cfg)
	case "rename":
		return CmdRename(a.root, a.cfg, ctx, value)
	case "rename_old":
		a.openInput("rename_new", value, "new name for "+value, "", "")
		return nil
	case "rename_new":
		return CmdRename(a.root, a.cfg, ctx, value)
	}
	return nil
}

// runConfirmed is called when a Confirm overlay succeeds.
func (a *App) runConfirmed(action, ctx string) tea.Cmd {
	switch action {
	case "remove":
		return CmdRemoveOne(a.root, a.cfg, ctx)
	case "remove_all":
		return CmdRemoveAll(a.root, a.cfg)
	}
	return nil
}

// absorbOpResult records an op's outcome in the activity log.
func (a *App) absorbOpResult(r OpResult) {
	if r.Msg == "" {
		return
	}
	// Multi-line messages (diff, status) each become their own entry.
	for _, line := range strings.Split(strings.TrimRight(r.Msg, "\n"), "\n") {
		if line == "" {
			continue
		}
		a.activity.Append(r.Kind, line)
	}
}

// refresh rebuilds the stores list and detail from the loaded config.
func (a *App) refresh() {
	a.stores.Refresh(a.root, a.cfg, a.freshMarks, time.Now())
}

// flashDetail restarts the detail rule's accent flash. Called when the
// cursor moves to a new store.
func (a *App) flashDetail() { a.detailFlashAt = time.Now() }

// View implements tea.Model.
func (a *App) View() string {
	if !a.ready {
		return ""
	}
	if a.fullscreenLog {
		return a.renderFullscreenLog()
	}
	main := a.renderMain()
	if a.overlay != OverlayNone {
		return a.renderOverlay(main)
	}
	// Pin the footer to the bottom. If the main body overflows the screen,
	// clip from the bottom so the footer always stays visible.
	footer := "  " + a.keys.FooterHints()
	budget := a.height - 2 // one blank line + footer line
	if budget < 1 {
		budget = 1
	}
	lines := strings.Split(main, "\n")
	if len(lines) > budget {
		lines = lines[:budget]
	}
	for len(lines) < budget {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n") + "\n\n" + footer
}

func currentTarget(entry config.StoreEntry) string {
	if entry.Target != "" {
		return entry.Target
	}
	if ts := entry.ResolvedTargets(); len(ts) > 0 {
		return ts[0].Target
	}
	return ""
}

// renderMain composes the whole main view.
func (a *App) renderMain() string {
	if a.uninit {
		return a.renderUninit()
	}
	width := a.width
	if width < 40 {
		width = 40
	}

	var b strings.Builder
	b.WriteString(a.renderHeader(width))
	b.WriteString("\n\n")

	// Stores section
	b.WriteString(Rule(width-4, a.stores.HeaderLine(), a.stores.Summary(), ""))
	b.WriteString("\n\n")

	now := time.Now()
	elapsed := now.Sub(a.startedAt)
	stagger := 25 * time.Millisecond
	dur := 180 * time.Millisecond

	rows := a.stores.Rows()
	if len(rows) == 0 && a.stores.FilterQuery() != "" {
		b.WriteString("  " + StyleDim.Render("no matches for \""+a.stores.FilterQuery()+"\""))
		b.WriteString("\n")
	} else if len(rows) == 0 {
		b.WriteString("  " + StyleDim.Render("no stores yet · press : then `adopt` or `add`"))
		b.WriteString("\n")
	} else {
		for i, r := range rows {
			rev := revealAt(i, len(rows), elapsed, stagger, dur)
			b.WriteString("  " + RenderRow(r, width-4, i == a.stores.Cursor(), rev))
			b.WriteString("\n")
		}
	}

	// Filter line (if active)
	if a.filterMode {
		b.WriteString("\n  " + StyleEmber.Render("/") + a.filterInput.View() + "\n")
	}

	b.WriteString("\n")

	// Detail section
	selName := a.stores.Selected()
	if selName != "" {
		entry, ok := a.cfg.Stores[selName]
		if ok {
			results := storeops.GetStatus(a.root, selName, entry)
			agg := aggregate(results)
			flash := decay(now.Sub(a.detailFlashAt), 200*time.Millisecond)
			ruleColor := agg.Color()
			if flash > 0 {
				ruleColor = Mix(agg.Color(), ColorEmber, flash)
			}
			b.WriteString(Rule(width-4, selName, agg.Label(), ruleColor))
			b.WriteString("\n\n")
			b.WriteString(RenderDetail(a.root, selName, a.cfg, a.plat, a.detail, width-4))
			b.WriteString("\n")
		}
	}

	// Activity section. Kept last so that when the terminal is short and
	// the body gets clipped, it's the log that disappears, not the store
	// list or the detail pane. The footer is pinned separately by View().
	if line := a.activity.RenderLine(width - 4); line != "" {
		b.WriteString(Rule(width-4, "recent", "", ColorDim))
		b.WriteString("\n\n  " + line)
	}
	return b.String()
}

func (a *App) renderHeader(width int) string {
	brand := StyleBold.Render("store")
	root := StyleDim.Render(shortHome(a.root))
	plat := StyleDim.Render(a.plat.OS + "/" + a.plat.Arch)
	heart := a.renderHeartbeat()

	left := "  " + brand
	right := root + StyleDim.Render("   ") + plat + StyleDim.Render("   ") + heart + " "
	used := lipgloss.Width(left) + lipgloss.Width(right)
	fill := width - used
	if fill < 1 {
		fill = 1
	}
	return left + strings.Repeat(" ", fill) + right
}

func (a *App) renderHeartbeat() string {
	// A slow 3-second pulse on the accent for the single header dot.
	p := pulse(time.Now(), 3*time.Second)
	c := Mix(ColorFaint, ColorEmber, 0.3+0.7*p)
	return lipgloss.NewStyle().Foreground(c).Render(GlyphHeart)
}

func (a *App) renderOverlay(_ string) string {
	// Modals earn their frame. We center the overlay on an empty canvas
	// rather than compositing over the main view — in a terminal, the
	// frame plus the centered layout is enough visual containment.
	width := a.width * 7 / 10
	if width < 48 {
		width = a.width - 8
	}
	if width < 36 {
		width = a.width - 2
	}

	var title, body, footer string
	switch a.overlay {
	case OverlayPalette:
		title = "command"
		body = a.palette.View(width)
		footer = a.palette.Footer()
	case OverlayActions:
		title = ""
		body = a.actions.View()
		footer = a.actions.Footer()
	case OverlayConfirm:
		title = ""
		body = a.confirm.View()
		footer = a.confirm.Footer()
	case OverlayInput:
		title = ""
		body = a.input.View()
		footer = a.input.Footer()
	case OverlaySecrets:
		title = "secrets"
		body = a.secrets.View()
		footer = a.secrets.Footer()
	case OverlayDoctor:
		title = "doctor"
		body = a.doctor.View()
		footer = a.doctor.Footer()
	case OverlayHelp:
		title = "help"
		body = a.help.View()
		footer = a.help.Footer()
	}

	frame := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(ColorDim).
		Padding(1, 2).
		Width(width).
		Render(composeOverlay(title, body, footer))

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, frame, lipgloss.WithWhitespaceChars(" "))
}

func composeOverlay(title, body, footer string) string {
	var b strings.Builder
	if title != "" {
		b.WriteString(StyleEmber.Render(":: ") + StyleBold.Render(title) + "\n\n")
	}
	b.WriteString(body)
	if footer != "" {
		b.WriteString("\n\n" + footer)
	}
	return b.String()
}

func (a *App) renderUninit() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString("  " + StyleBold.Render("store") + StyleDim.Render(" · no config here yet") + "\n\n")
	b.WriteString("  " + StyleFg.Render("press ") + StyleEmber.Render("i") + StyleFg.Render(" to create ")+StyleDim.Render(".store/config.yaml")+"\n")
	b.WriteString("  " + StyleFg.Render("press ") + StyleEmber.Render(":") + StyleFg.Render(" to open the command palette"))
	b.WriteString("\n\n\n  " + StyleDim.Render("q quit"))
	return b.String()
}

func (a *App) renderFullscreenLog() string {
	var b strings.Builder
	b.WriteString(Rule(a.width-2, "activity", "esc or \\ to close", ColorDim))
	b.WriteString("\n\n")
	b.WriteString(a.activity.RenderFull(a.width-4, a.height-4))
	return b.String()
}

func shortHome(root string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return root
	}
	if strings.HasPrefix(root, home) {
		return "~" + root[len(home):]
	}
	return root
}
