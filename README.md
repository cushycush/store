# store

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
  zsh/
    .zshrc
    .zshenv
  git/
    .gitconfig
```

`store` creates symlinks so that each directory appears at its configured target:

```
~/.config/nvim  ->  ~/dotfiles/nvim
~/.zsh          ->  ~/dotfiles/zsh
~/.config/git   ->  ~/dotfiles/git
```

All configuration lives in a single `.store/config.yaml` file that you commit alongside your dotfiles.

## How It Differs from GNU Stow

| | GNU Stow | store |
|---|---|---|
| **Directory structure** | Must mirror the target filesystem hierarchy | Flat -- each store is a top-level directory |
| **Target paths** | Inferred from directory structure and stow directory location | Explicitly declared per store in YAML config |
| **Configuration** | Convention-based (directory layout is the config) | Single `config.yaml` file |
| **Granularity** | Symlinks individual files within directories | Symlinks whole directories |
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
go build -o store ./cmd/store
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

2. Add a store for your Neovim config:

```sh
store add nvim
# When prompted, enter the target path: ~/.config/nvim
```

This creates the `nvim/` directory, saves the target to `.store/config.yaml`, and creates the symlink.

3. Move your existing config files into the store directory:

```sh
mv ~/.config/nvim/init.lua ~/dotfiles/nvim/
```

Since the symlink points `~/.config/nvim` to `~/dotfiles/nvim`, your editor picks up the files from the repo directory.

4. Commit and push:

```sh
git add -A && git commit -m "add nvim config"
git push
```

5. On a new machine, clone and restore all symlinks:

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
  zsh -> ~/.zsh
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

Adds a new store. Creates the directory if it doesn't exist, prompts for the target path, saves the entry to config, and creates the symlink.

```sh
$ store add nvim
Created directory /home/user/dotfiles/nvim
Where should "nvim" be symlinked to?
Target path: ~/.config/nvim
  nvim -> ~/.config/nvim
```

Target paths can use `~` for the home directory or absolute paths. Relative paths are automatically resolved to absolute paths before saving.

### `store remove <name>`

Removes the symlink for the named store and deletes its config entry.

```sh
$ store remove nvim
Removed store nvim (~/.config/nvim)
```

The store directory and its contents in your repo are not deleted -- only the symlink and config entry are removed.

### `store removeall`

Removes symlinks and config entries for all stores. If a particular store fails to remove (e.g., due to a conflict), its config entry is preserved and the remaining stores are still processed.

```sh
$ store removeall
Removing all stores:
  removed nvim (~/.config/nvim)
  removed zsh (~/.zsh)
  removed git (~/.config/git)
```

### `store status [name]`

Shows the symlink status for one or all stores.

```sh
$ store status
  nvim                 [linked]   ~/.config/nvim
  zsh                  [missing]  ~/.zsh
  git                  [conflict] ~/.config/git
```

```sh
$ store status nvim
  nvim                 [linked]   ~/.config/nvim
```

## Config Format

Configuration is stored in `.store/config.yaml` at the root of your repository:

```yaml
stores:
    nvim:
        target: ~/.config/nvim
    zsh:
        target: ~/.zsh
    git:
        target: ~/.config/git
```

Each entry maps a store name (a directory in your repo) to a target path (where the symlink is created).

**Supported target path formats:**

- `~` prefix -- expanded to the user's home directory (e.g., `~/.config/nvim`). Portable across machines.
- Absolute paths -- used as-is (e.g., `/home/user/.config/nvim`).

Relative paths entered during `store add` are automatically converted to absolute paths.

## Status Indicators

| Status | Meaning |
|---|---|
| `[linked]` | Symlink exists and points to the correct store directory. |
| `[missing]` | No symlink exists at the target path. Run `store` to create it. |
| `[conflict]` | Something exists at the target path but it is not a symlink managed by store. Resolve manually. |
| `[broken]` | A symlink exists but its destination no longer exists. Running `store` will replace it. |

## How It Works

- **Root discovery:** Commands can be run from any subdirectory. `store` walks up the directory tree to find the nearest `.store/` directory, similar to how `git` finds `.git/`.
- **Symlinks are absolute:** When creating symlinks, source paths are resolved to absolute paths. This means symlinks work regardless of your working directory.
- **Conflict detection:** Before creating or removing a symlink, `store` checks the target path. It refuses to overwrite files or directories that aren't managed by store, preventing accidental data loss.
- **Broken symlink recovery:** If a symlink exists but points to a nonexistent path, `store` removes it and creates a fresh one pointing to the correct source.

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
