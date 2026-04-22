# Changelog

All notable changes to `store` are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.4.0] - 2026-04-22

### Added

- **Git-style subcommand dispatch.** `store <sub>` now delegates to an
  external binary when `<sub>` isn't one of store's own commands. Two
  resolution paths, in order: `store-<sub>` on `$PATH` (strict git
  convention), then — for known companions like
  [`stock`](https://github.com/cushycush/stock) — the bare binary name.
  That means `store stock doctor`, `store stock install`, etc. all work
  out of the box once `stock` is on your `$PATH`, without a symlink shim.
  Unknown args still fall through to cobra's "unknown command" message.

## [2.3.0] - 2026-04-22

### Added

- **Nix flake** — `nix run github:cushycush/store`,
  `nix profile install github:cushycush/store`, and a pinned Go + gopls
  dev shell via `nix develop github:cushycush/store`. The flake installs
  bash/fish/zsh completions the same way the AUR package does.

## [2.2.0] - 2026-04-22

### Added

- The TUI header now shows a quiet `stock` signpost when the companion
  [`stock`](https://github.com/cushycush/stock) package-installer is on
  `$PATH`. It is rendered in the low-intensity ember tint so it reads as
  "available" without competing with the active store view. No coupling to
  stock's internal state — detection is a single `exec.LookPath` at app
  startup.

### Changed

- `internal/ui` now delegates its generic styling primitives (colors,
  bold/dim, doctor chips, prompts) to `store-core/ui` so stock can pick up
  the same CLI look. Store-specific chips (`StatusLinked`, `DiffCreate`,
  `StoreName`, …) stay in `store/internal/ui`. Import paths are unchanged
  for existing call sites.

## [2.1.0] - 2026-04-22

### Added

- `when:` filters now accept either a YAML scalar or a list in any string
  field. `os: linux` and `os: [linux, darwin]` are both valid; existing
  scalar configs are unchanged.

### Changed

- Platform detection, `when:` matching, `FindRoot`, `ExpandHome`, and the
  `STORE_*` hook env contract moved into a new shared module,
  [`github.com/cushycush/store-core`](https://github.com/cushycush/store-core).
  The `store` binary's behavior is unchanged; the split exists so the
  companion [`stock`](https://github.com/cushycush/stock) package-installer
  can consume the same pieces without vendoring them.

## [2.0.0] - 2026-04-20

### Breaking

- Running `store` with no arguments no longer reconciles symlinks; it prints
  help. Use `store apply` (new explicit verb) to reconcile. Scripts, cron
  jobs, or aliases that relied on bare `store` to reconcile must be updated
  before upgrading. See [MIGRATING.md](MIGRATING.md).
- `store removeall` is deprecated in favor of `store remove --all`. The old
  name still works as a hidden alias and prints a deprecation warning.

### Added

- **Interactive TUI (`store tui`)**: keyboard-driven dashboard with a store
  ledger, per-store detail pane, and recent activity log. Every CLI verb is
  reachable from inside it via a `:` command palette with fuzzy matching.
  Austere single-column layout; ember signature accent; vim-safe keymap;
  typed confirmation for destructive operations.
- **Command palette** in the TUI: fuzzy-matches `apply`, `init`, `import`,
  `adopt`, `add`, `modify`, `remove`, `list`, `path`, `rename`, `edit`,
  `status`, `diff`, `doctor`, `version`, `target {add,remove,modify}`,
  `secret {set,get,remove,list}`. Prompts inline for required arguments.
- **Target submenu** in the TUI: the row action menu's `target…` entry opens
  a submenu that lists each target with per-target actions (apply, unlink,
  modify files, remove target).
- **New CLI verbs**:
  - `store list` — one-line summary of every configured store; no filesystem
    access.
  - `store path <name>` — print the absolute repo path of a store; designed
    for shell substitution (`cd $(store path nvim)`).
  - `store rename <old> <new>` — rename a store (moves the directory and
    updates config).
  - `store edit` — open `.store/config.yaml` in `$EDITOR`.
- **`store apply --dry-run`** — preview changes without applying. Equivalent
  to `store diff`.
- **`store remove --all`** with `--yes` — remove every configured store at
  once. Replaces `store removeall`.
- **Positional target arguments** on `store add`, `store target add`,
  `store target remove`, and `store target modify`. Both forms work:
  `store add nvim ~/.config/nvim` and `store add nvim -t ~/.config/nvim`.
- **Additive modify flags** on `store modify` and `store target modify`:
  `--add-file`, `--remove-file`, `--add-pattern`, `--remove-pattern`, plus
  `--clear-files` / `--clear-patterns` alongside the existing replace flags.
  Compose in order: clear → replace → add → remove.
- **`--dry-run` coverage** across every mutating command: `import`, `adopt`,
  `modify`, `remove`, `target add/remove/modify`, and `apply`.

### Changed

- Positional target paths that resolve under `$HOME` are rewritten to a
  `~`-prefixed form before being stored, restoring the documented portability
  invariant after the shell expands unquoted `~` before the binary sees it.
- Empty-state messages across `status`, `list`, `diff`, and `apply` are
  consistent: *"No stores configured yet. Try `store adopt <path>` or
  `store add <name> <target>`."*
- `list`, `status`, and `diff` help text now cross-references the other two
  so new users can pick the right command without guessing.
- Error output no longer includes Cobra's usage dump after every error; the
  error stands on its own and usage remains available via `--help`.
- README restructured: new Interactive TUI section, chezmoi/dotbot/yadm
  comparison, expanded Troubleshooting, "How It Works" renamed to "Internals"
  to disambiguate from Concepts, and a Jump-to sub-TOC at the top of the
  Commands reference.

### Fixed

- Template rendering skips binary files that coincidentally contain `{{`
  byte patterns (also shipped as 1.3.1).

## [1.3.1] - 2026-04-19

### Fixed
- Skip template rendering for binary files (e.g. images) that coincidentally
  contain the bytes `{{`. Previously, `store add` would error with
  `parse template: unrecognized character in action: U+FFFD` when a module
  contained such a file.

## [1.3.0] - 2026-04-16

### Added
- `store adopt` command: move an existing file or directory into the repo, create
  a config entry, and symlink back. Supports `--name`, `--dry-run`, `--files`,
  and `--patterns` flags.
- Template engine rewrite using Go `text/template`. Templates now support
  `{{ env "VAR" }}`, `{{ .Hostname }}`, `{{ .OS }}`, `{{ .Arch }}`, `{{ .Distro }}`,
  `{{ .Shell }}`, and user-defined `{{ .Vars.key }}` variables in addition to
  `{{ secret "name" }}`.
- Top-level `vars` map in config for user-defined template variables.
- `Render` flag on store and target entries for explicit template rendering control.
- `[drift]` status indicator for rendered files whose target content has diverged.
- Status summary line in `store status` output showing counts per status.
- `FORCE_COLOR` environment variable to force colored output in non-TTY contexts.
- Verbose mode (`-v`/`--verbose`) for `test.sh` showing store commands and their
  colorized output.

### Changed
- `RenderContext` struct replaces raw secrets map throughout store and CLI layers,
  bundling secrets with platform data and user-defined variables.

## [1.2.2] - 2026-04-15

### Added
- `CHANGELOG.md` documenting all prior releases.

## [1.2.1] - 2026-04-15

### Changed
- Restructured the README around a newcomer-friendly narrative flow.

## [1.2.0] - 2026-04-15

### Added
- Cross-platform support for Windows, with CI running `go test` and builds on
  `ubuntu-latest`, `macos-latest`, and `windows-latest`.
- `store doctor` warns when the host cannot create symlinks on Windows.
- Per-platform hook dispatch, with hook tests split by build tag.

### Fixed
- Preserve hook command quoting under `cmd.exe` via `SysProcAttr.CmdLine`.
- Resolve paths on both sides of comparisons so Windows 8.3 short names match.
- Emit forward-slash paths from the importer and ignore matcher on Windows.
- Fall back to `PSModulePath` and `ComSpec` when detecting the shell on Windows.
- Make cross-platform tests portable across macOS, Windows, and Linux.

## [1.1.1] - 2026-04-14

### Changed
- Replaced GoReleaser with a matrix build workflow and bumped GitHub Actions
  to Node.js 24 compatible versions.

## [1.1.0] - 2026-04-08

### Added
- New color-scheme UI engine for command output.
- Full test suite covering the core workflows.

## [1.0.0] - 2026-04-08

### Added
- `store import` command and a symlink importer package for adopting existing
  symlink trees into a managed store.
- `store diff` command for dry-run previews of pending changes.
- `store doctor` command backed by a diagnostic package.
- Shell completion with tab completion for store names.

### Changed
- Rewrote the README with a table of contents and full command reference.
- Split `main.go` into focused modules and cleaned up internal package
  consistency.

## [0.9.0] - 2026-04-08

### Added
- `ignore` rules in config for excluding files from a store.

## [0.8.0] - 2026-04-08

### Added
- Conditional environment variables and config-level conditionals for skipping
  stores based on the current environment.

## [0.7.0] - 2026-04-08

### Added
- Secret management for values that should not be checked into the store.
- Initial testing infrastructure.

Note: v0.6.0 was a development-only release and is folded into v0.7.0.

## [0.5.1] - 2026-04-08

### Added
- `--only` flag for scoping operations to specific stores.

### Fixed
- Miscellaneous bug fixes and an added warning message.

## [0.5.0] - 2026-04-07

### Added
- Global and per-store pre/post hooks.

## [0.4.1] - 2026-04-06

### Added
- Conflict handling: list conflicting files and offer to move them aside, with
  a `--force` flag to bypass the prompt and back conflicts up as `.bak` files.

### Fixed
- `remove` and `removeall` no longer fail when `store` is not the owner of a
  symlink.
- Second redundant `symlink` call removed from the retry loop (it always
  failed; the retry loop already covered every case).
- Trailing-slash handling in paths.
- Race condition where the program could recreate a directory or file
  immediately after removal.

## [0.3.0] - 2026-04-04

### Added
- Multiple targets per store and nested subcommands.
- `expandTargetPath` helper so relative, tilde, and absolute paths all work
  consistently in config, commands, and subcommands.

## [0.2.0] - 2026-04-01

### Added
- Symlinked files and glob patterns within a target.
- `modify` command.

### Fixed
- File-mode symlink cleanup during `modify`, `remove`, and `removeall`.

## [0.1.0] - 2026-04-01

### Added
- First working release of `store`: add, remove, and removeall commands with
  symlink management.
- `version` command and initial README.
- `add` creates the target directory if it does not already exist, and
  `remove` commands clean up their config entries.

### Fixed
- Adding a store from a subdirectory of the project root using a relative path.

[1.2.2]: https://github.com/cushycush/store/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/cushycush/store/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/cushycush/store/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/cushycush/store/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/cushycush/store/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/cushycush/store/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/cushycush/store/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/cushycush/store/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/cushycush/store/compare/v0.5.1...v0.7.0
[0.5.1]: https://github.com/cushycush/store/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/cushycush/store/compare/v0.4.1...v0.5.0
[0.4.1]: https://github.com/cushycush/store/compare/v0.3.0...v0.4.1
[0.3.0]: https://github.com/cushycush/store/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cushycush/store/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/cushycush/store/releases/tag/v0.1.0
