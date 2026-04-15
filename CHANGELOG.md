# Changelog

All notable changes to `store` are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
