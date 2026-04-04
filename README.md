![store logo](https://res.cloudinary.com/cush/image/upload/v1775189496/screenshot-2026-04-02_22-10-09_y9sbug.png)

A simpler alternative to [GNU Stow](https://www.gnu.org/software/stow/) for managing dotfile symlinks.

## Overview

`store` manages symlinks for your dotfiles without requiring mirrored directory structures. Each "store" is a directory in your repository that gets symlinked to a target location on your filesystem.

Your dotfiles repo might look like this:

```
~/dotfiles/
  .store/config.yaml
  nvim/
    init.lua
    lua/
  shells/
    .zshrc
    .bashrc
    config.fish
    config.nu
  git/
    .gitconfig
```

`store` creates symlinks so that each directory appears at its configured target:

```
~/.config/nvim       ->  ~/dotfiles/nvim
~/.zshrc             ->  ~/dotfiles/shells/.zshrc
~/.bashrc            ->  ~/dotfiles/shells/.bashrc
~/.config/fish/config.fish  ->  ~/dotfiles/shells/config.fish
~/.config/nushell/config.nu ->  ~/dotfiles/shells/config.nu
~/.config/git        ->  ~/dotfiles/git
```

A single store can have multiple targets, so files that belong together conceptually (like shell configs) can live in one directory even if they deploy to different locations on disk.

All configuration lives in a single `.store/config.yaml` file that you commit alongside your dotfiles.

## How It Differs from GNU Stow

| | GNU Stow | store |
|---|---|---|
| **Directory structure** | Must mirror the target filesystem hierarchy | Flat -- each store is a top-level directory |
| **Target paths** | Inferred from directory structure and stow directory location | Explicitly declared per store in YAML config |
| **Configuration** | Convention-based (directory layout is the config) | Single `config.yaml` file |
| **Granularity** | Symlinks individual files within directories | Symlinks whole directories or individual files via patterns |
| **Multiple targets** | Requires separate packages per target location | One store can deploy to multiple target paths |
| **Setup on new machine** | Run `stow` per package from the correct parent directory | Run `store` from anywhere in the repo |

## Installation

### go install

```sh
go install github.com/cush/store/cmd/store@latest
```

### Build from source

```sh
git clone https://github.com/cush/store.git
cd store
make build VERSION=0.3.0
# Move the binary somewhere in your PATH
mv store /usr/local/bin/
```

## Quick Start

1. Initialize a new dotfiles repo:

```sh
mkdir ~/dotfiles && cd ~/dotfiles
git init
store init
```

2. Add a store for your Neovim config (whole-directory symlink):

```sh
store add nvim -t ~/.config/nvim
```

This creates the `nvim/` directory, saves the target to `.store/config.yaml`, and creates the symlink.

3. Add a store with file-level symlinks for files that live in `~`:

```sh
store add shells -t ~ -f .zshrc -f .bashrc
```

This creates individual symlinks: `~/.zshrc -> ~/dotfiles/shells/.zshrc` and `~/.bashrc -> ~/dotfiles/shells/.bashrc`.

4. Add another target to the same store for files that live elsewhere:

```sh
store target add shells -t ~/.config/fish -f config.fish
store target add shells -t ~/.config/nushell -f config.nu
```

Now the `shells` store deploys to three different locations from a single directory.

5. Move your existing config files into the store directories:

```sh
mv ~/.config/nvim/init.lua ~/dotfiles/nvim/
mv ~/.zshrc ~/dotfiles/shells/
mv ~/.bashrc ~/dotfiles/shells/
mv ~/.config/fish/config.fish ~/dotfiles/shells/
mv ~/.config/nushell/config.nu ~/dotfiles/shells/
```

Since the symlinks point back to the repo directories, your tools pick up the files seamlessly.

6. Commit and push:

```sh
git add -A && git commit -m "add configs"
git push
```

7. On a new machine, clone and restore all symlinks:

```sh
git clone https://github.com/you/dotfiles.git ~/dotfiles
cd ~/dotfiles
store
```

Running `store` with no subcommand creates symlinks for all stores defined in the config.

## Commands

### `store`

Creates or restores symlinks for all stores in the config. This is the command you run after cloning your dotfiles repo on a new machine.

```sh
$ store
Storing all stores:
  nvim -> ~/.config/nvim
  shells -> ~ (files)
  shells -> ~/.config/fish (files)
  shells -> ~/.config/nushell (files)
  git -> ~/.config/git
```

If a symlink already exists and points to the correct source, it is left as-is. Broken symlinks are automatically replaced.

### `store init`

Initializes a new store config in the current directory. Creates `.store/config.yaml`.

```sh
$ store init
Initialized store config at .store/config.yaml
```

Run this once at the root of your dotfiles repo.

### `store add <name>`

Adds a new store. Creates the directory if it doesn't exist, saves the entry to config, and creates symlinks if a target is provided.

```sh
# Whole-directory symlink
$ store add nvim -t ~/.config/nvim
  nvim -> ~/.config/nvim

# File-level symlinks with explicit files
$ store add zsh -t ~ -f .zshrc -f .zshenv
  zsh -> ~ (files)

# File-level symlinks with glob patterns
$ store add shell -t ~ -p ".zsh*" -p ".bash*"
  shell -> ~ (files)

# Recursive glob patterns
$ store add configs -t ~/.config -p "**/*.conf"
  configs -> ~/.config (files)

# Add to config without a target (configure later with modify)
$ store add zsh
Added zsh to config (no target set)
```

**Flags:**

| Flag | Short | Description |
|---|---|---|
| `--target` | `-t` | Target path for the symlink |
| `--files` | `-f` | Explicit files to symlink (repeatable) |
| `--patterns` | `-p` | Glob patterns to match files (repeatable, supports `**`) |

Target paths can use `~` for the home directory or absolute paths. Relative paths are automatically resolved to absolute paths.

### `store modify <name>`

Updates fields on an existing store entry. Each flag replaces the entire field.

```sh
# Change target path
$ store modify nvim -t ~/.config/nvim-custom

# Replace the file list
$ store modify zsh -f .zshrc -f .zshenv -f .zprofile

# Switch from files to patterns
$ store modify zsh --clear-files -p ".zsh*"

# Remove patterns (reverts to whole-directory symlink)
$ store modify zsh --clear-patterns
```

Old symlinks are removed before the updated entry is applied.

**Note:** For stores with multiple targets, use `store target modify` instead. `store modify` only works on single-target stores.

**Flags:**

| Flag | Short | Description |
|---|---|---|
| `--target` | `-t` | New target path |
| `--files` | `-f` | Replace file list (repeatable) |
| `--patterns` | `-p` | Replace pattern list (repeatable) |
| `--clear-files` | | Remove all files from the entry |
| `--clear-patterns` | | Remove all patterns from the entry |

### `store target add <name>`

Adds a new target to an existing store. If the store currently uses the single-target format, it is automatically migrated to the multi-target format.

```sh
# Add a second target to an existing store
$ store target add shells -t ~/.config/fish -f config.fish
  shells -> ~/.config/fish (files)

# Add a target with patterns
$ store target add shells -t ~/.config/nushell -p "*.nu"
  shells -> ~/.config/nushell (files)
```

**Flags:**

| Flag | Short | Description |
|---|---|---|
| `--target` | `-t` | Target path for the symlink (required) |
| `--files` | `-f` | Explicit files to symlink (repeatable) |
| `--patterns` | `-p` | Glob patterns to match files (repeatable, supports `**`) |

### `store target remove <name>`

Removes a specific target from a store by its path. Symlinks for the removed target are unlinked. If only one target remains, the store is automatically migrated back to the single-target format.

```sh
$ store target remove shells -t ~/.config/fish
  removed target ~/.config/fish from shells
```

**Flags:**

| Flag | Short | Description |
|---|---|---|
| `--target` | `-t` | Target path to remove (required) |

### `store target modify <name>`

Modifies the files or patterns for a specific target within a store. The target is identified by its path. Each provided flag replaces the entire field.

```sh
# Add a file to an existing target
$ store target modify shells -t ~ -f .zshrc -f .bashrc -f .zprofile

# Switch a target from files to patterns
$ store target modify shells -t ~ --clear-files -p ".zsh*" -p ".bash*"
```

Old symlinks for the target are removed before the updated entry is applied.

**Flags:**

| Flag | Short | Description |
|---|---|---|
| `--target` | `-t` | Target path to modify (required) |
| `--files` | `-f` | Replace file list (repeatable) |
| `--patterns` | `-p` | Replace pattern list (repeatable) |
| `--clear-files` | | Remove all files from the target |
| `--clear-patterns` | | Remove all patterns from the target |

### `store remove <name>`

Removes the symlink(s) for the named store and deletes its config entry.

```sh
$ store remove nvim
Removed store nvim (~/.config/nvim)
```

The store directory and its contents in your repo are not deleted -- only the symlinks and config entry are removed.

### `store removeall`

Removes symlinks and config entries for all stores. If a particular store fails to remove (e.g., due to a conflict), its config entry is preserved and the remaining stores are still processed.

```sh
$ store removeall
Removing all stores:
  removed nvim (~/.config/nvim)
  removed shells (~)
  removed shells (~/.config/fish)
  removed shells (~/.config/nushell)
  removed git (~/.config/git)
```

### `store version`

Prints the current version.

```sh
$ store version
store version 0.3.0
```

The `--version` flag also works:

```sh
$ store --version
store version 0.3.0
```

When built without a version (e.g., `go build ./cmd/store`), the version defaults to `dev`. Use the Makefile to build with a specific version:

```sh
make build VERSION=0.3.0
```

### `store status [name]`

Shows the symlink status for one or all stores. For file-mode stores, each file is shown individually.

```sh
$ store status
  nvim                 [linked]   ~/.config/nvim
  shells               .zshrc               [linked]   ~/.zshrc
  shells               .bashrc              [linked]   ~/.bashrc
  shells               config.fish          [linked]   ~/.config/fish/config.fish
  shells               config.nu            [linked]   ~/.config/nushell/config.nu
  git                  [conflict] ~/.config/git
```

```sh
$ store status nvim
  nvim                 [linked]   ~/.config/nvim
```

## Config Format

Configuration is stored in `.store/config.yaml` at the root of your repository:

#### Single-target format

The simplest format maps a store to one target path:

```yaml
stores:
    nvim:
        target: ~/.config/nvim
    zsh:
        target: ~
        files:
            - .zshrc
            - .zshenv
    git:
        target: ~/.config/git
```

#### Multi-target format

A store can deploy to multiple target paths using the `targets` list. Each entry has its own `target`, `files`, and `patterns`:

```yaml
stores:
    nvim:
        target: ~/.config/nvim
    shells:
        targets:
            - target: ~
              files:
                  - .zshrc
                  - .bashrc
            - target: ~/.config/fish
              files:
                  - config.fish
            - target: ~/.config/nushell
              patterns:
                  - "*.nu"
    git:
        target: ~/.config/git
```

Using both `target` and `targets` on the same store entry is invalid.

Each entry maps a store name (a directory in your repo) to one or more target paths (where symlinks are created).

### Entry Fields

#### Single-target fields

| Field | Required | Description |
|---|---|---|
| `target` | No | Where symlinks are created. Without a target, the entry is saved but no symlinks are created. |
| `files` | No | Explicit list of files to symlink individually. |
| `patterns` | No | Glob patterns to match files. Supports `*`, `?`, `[...]`, and `**` for recursive matching. |

#### Multi-target fields

| Field | Required | Description |
|---|---|---|
| `targets` | No | List of target entries, each with its own `target`, `files`, and `patterns`. |
| `targets[].target` | Yes | Where symlinks are created for this target entry. |
| `targets[].files` | No | Explicit list of files to symlink individually for this target. |
| `targets[].patterns` | No | Glob patterns to match files for this target. |

**Behavior:**

- **No `files` or `patterns`:** The entire store directory is symlinked to the target (whole-directory mode).
- **`files` and/or `patterns` specified:** Only matched files are symlinked individually. Directory structure is preserved at the target.
- **Both `files` and `patterns`:** Results are combined and deduplicated.

These rules apply independently to each target in a multi-target store. One target can use whole-directory mode while another uses file-level symlinks.

### Pattern Syntax

Patterns use standard glob syntax with recursive matching support:

| Pattern | Matches |
|---|---|
| `.zsh*` | `.zshrc`, `.zshenv`, etc. at the top level |
| `*.conf` | All `.conf` files at the top level |
| `**/*.conf` | All `.conf` files at any depth |
| `config/*.lua` | `.lua` files inside `config/` |
| `**/*.{lua,vim}` | `.lua` and `.vim` files at any depth |

### Target Path Formats

- `~` prefix -- expanded to the user's home directory (e.g., `~/.config/nvim`). Portable across machines.
- Absolute paths -- used as-is (e.g., `/home/user/.config/nvim`).

Relative paths provided via `--target` are automatically converted to absolute paths.

## Status Indicators

| Status | Meaning |
|---|---|
| `[linked]` | Symlink exists and points to the correct store directory or file. |
| `[missing]` | No symlink exists at the target path. Run `store` to create it. |
| `[conflict]` | Something exists at the target path but it is not a symlink managed by store. Resolve manually. |
| `[broken]` | A symlink exists but its destination no longer exists. Running `store` will replace it. |

## How It Works

- **Root discovery:** Commands can be run from any subdirectory. `store` walks up the directory tree to find the nearest `.store/` directory, similar to how `git` finds `.git/`.
- **Symlinks are absolute:** When creating symlinks, source paths are resolved to absolute paths. This means symlinks work regardless of your working directory.
- **Conflict detection:** Before creating or removing a symlink, `store` checks the target path. It refuses to overwrite files or directories that aren't managed by store, preventing accidental data loss.
- **Broken symlink recovery:** If a symlink exists but points to a nonexistent path, `store` removes it and creates a fresh one pointing to the correct source.
- **File matching performance:** Explicit `files` entries are validated with a single stat call each (no directory walking). Simple glob patterns use `Glob` without recursive traversal. Only `**` patterns trigger a full directory walk, using the efficient `WalkDir` API.

## Troubleshooting

### "conflict: already exists and is not a symlink managed by store"

Something (a file or directory) already exists at the target path and wasn't created by `store`. Move or remove it manually, then run `store` again.

For example, if you're setting up on a new machine that has a default config at `~/.config/nvim`:

```sh
mv ~/.config/nvim ~/.config/nvim.bak
store
```

### "[broken]" status

The symlink exists but points to a directory that no longer exists. This can happen if the store directory was renamed or deleted. Running `store` will automatically replace broken symlinks.

### "no .store directory found"

You're not inside a repository that has been initialized with `store init`. Either `cd` into your dotfiles repo or run `store init` to set one up.
