# store

A CLI dotfile manager written in Go. One repo, one YAML config, one command per machine. See `README.md` for the full feature set and command reference.

## Design Context

### Users

`store` is a CLI dotfile manager written in Go. The audience is:

- **Power CLI users** who keep their config in git, type fast, and rarely leave the terminal.
- **Vim users / tmux-dwellers / dotfile tinkerers** — people who have strong muscle memory (`h/j/k/l`, `/` to filter, `:q` to quit) and who *enjoy* tuning their setup.
- **Hobbyist maintainers** rather than enterprise operators. They're setting up personal machines and the occasional VPS, not administering fleets.

Viewing context: a terminal emulator, almost always on a dark background. Sessions are short and transactional — open the TUI, inspect link state, link or unlink one or two stores, close it. Not a dashboard people leave open.

Primary job-to-be-done: *know which stores are linked and which aren't, fix the ones that aren't, and get back to what I was doing* — with a safety net for destructive actions.

### Brand Personality

Three words: **austere · personal · hobbyist**.

The tool should feel:

- Honest about what your filesystem looks like. No marketing tone, no ceremony, no "welcome!" chrome.
- Hand-tended. A small utility a friend might share. Allows one small flourish that makes it recognisably *store*, not charm-tui #17.
- Quietly confident. The user is a peer, not a beginner to be walked through.

Emotional goals: *trust* (safety gating on destructive actions, predictable state), *recognition* (a single visual tell that says "this is store"), *calm* (no urgent reds unless something is actually broken).

### Aesthetic Direction

Target: **austere / mono** — minimal chrome, no outer box, whitespace and a single accent carry the composition.

- **No outer frame.** The header strip, the rule separators, and the footer hint row define the canvas. The terminal is the canvas.
- **Near-monochrome palette.** Neutrals do 95% of the work. One signature accent does the remaining 5% — reserved for cursor, active selection, and one recurring flourish. Severity colors (green/red/amber) stay in the palette but desaturated and dark; they appear only on state changes.
- **One signature flourish** that feels personal, not templated. A deliberate glyph, an empty-state line with a wink, a small asymmetry. Not more than one — the flourish loses force if it's everywhere.
- **Section labels in small caps, no bold-muted.** Sections are recognised by whitespace and micro-typography, not by bordered boxes.
- **Borders only on modals.** Overlays earn their frame because they interrupt the main flow. The main view does not.
- **Default renders on a plain font.** No octant glyphs, no Nerd Font dependency as a default. Any "fancy" rendering is opt-in and the fallback is the visual baseline.

**Anti-references** (explicitly not this):

- Charm / Glamour / Bubble Tea defaults — pastel teal accent, muted pastel palette, nested rounded-corner boxes, `lipgloss.Place`-centred overlays over a bordered parent. This is the reflex look and it reads as AI-made.
- Plain ncurses / `top` / `htop` default-palette look — unstyled system colors, no typographic intent, nothing that says a human cared. Uncared-for is not austere; it's just neglected.

Neutral on reference, acceptable in the space: `fzf`, `delta`, `gitui`'s quieter moments, the default `git log --oneline` alignment, `ripgrep --pretty` output — terminal interfaces where the *content* is the design.

### Design Principles

1. **Whitespace over chrome.** If a border or rule can be replaced with a blank line or an indent, replace it. The main view has no outer frame. Modals do, and the modal's frame is the only frame on screen.

2. **One accent, one flourish.** Pick a single signature color and use it sparingly for the cursor, active item, and key hints. Reserve one small delightful detail per screen that makes the interface recognisable — nowhere else.

3. **Typography over decoration.** Hierarchy comes from case, weight, and whitespace — not from boxes, glyphs, or color. Section labels are small-caps neutral. Store names are the largest type on screen in normal weight. Status glyphs are single characters, never padded.

4. **Respect vim muscle memory.** `h/j/k/l` belong to movement. Single letters that collide with navigation (`h`, `l`) cannot be bound to unrelated actions like overlays or mutations. When in doubt, use capitals for broad-scope or destructive actions and surface that convention in help.

5. **Default to the conservative rendering.** If a visual choice relies on a non-default font, a non-default terminal, or a non-default palette, that choice is opt-in. The default view is the lowest-common-denominator view, and it should still look deliberately designed — not a degraded version of the "real" UI.

6. **Earn destructive power.** `link`/`unlink` on a single store is safe enough for a single keystroke; anything broader (link-all, unlink-all, remove-store) requires a typed confirmation. The safety floor rises with blast radius.
