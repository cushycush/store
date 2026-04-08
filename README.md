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

|                           | GNU Stow                                                      | store                                                                         |
| ------------------------- | ------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| **Directory structure**   | Must mirror the target filesystem hierarchy                   | Flat -- each store is a top-level directory                                   |
| **Target paths**          | Inferred from directory structure and stow directory location | Explicitly declared per store in YAML config                                  |
| **Configuration**         | Convention-based (directory layout is the config)             | Single `config.yaml` file                                                     |
| **Granularity**           | Symlinks individual files within directories                  | Symlinks whole directories or individual files via patterns                   |
| **Multiple targets**      | Requires separate packages per target location                | One store can deploy to multiple target paths                                 |
| **Conflict handling**     | Refuses to proceed; user must manually move files             | Detects conflicts, offers to move existing files into the store automatically |
| **Setup on new machine**  | Run `stow` per package from the correct parent directory      | Run `store` from anywhere in the repo                                         |
| **Secret management**     | No built-in support                                           | Encrypted secrets with template rendering                                     |
| **Platform conditionals** | No built-in support                                           | `when:` clause for OS/arch/distro/hostname filtering                          |
| **Ignore patterns**       | Requires manual exclusion or separate packages                | Built-in ignore with global defaults                                          |

## Installation

### go install

```sh
go install github.com/cush/store/cmd/store@latest
```

### Build from source

```sh
git clone https://github.com/cush/store.git
cd store
make build VERSION=0.8.0
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

5. If files already existed at the target paths when you ran `store add`, store detected them and offered to move them into the store directories for you:

```
The following files conflict with store symlinks:
  ~/.config/nvim (directory -> will be moved to ~/dotfiles/nvim)
  ~/.zshrc (file -> will be moved to ~/dotfiles/shells/.zshrc)
  ~/.bashrc (file -> will be moved to ~/dotfiles/shells/.bashrc)

Move these files into the store and create symlinks? [y/N] y
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

**Flags:**

| Flag      | Description                                      |
| --------- | ------------------------------------------------ |
| `--only`  | Only store specific entries by name (repeatable) |
| `--force` | Create .bak backups without prompting            |

```sh
# Store only specific entries
$ store --only nvim --only git

# Automatically accept backup prompts
$ store --force
```

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

| Flag         | Short | Description                                              |
| ------------ | ----- | -------------------------------------------------------- |
| `--target`   | `-t`  | Target path for the symlink                              |
| `--files`    | `-f`  | Explicit files to symlink (repeatable)                   |
| `--patterns` | `-p`  | Glob patterns to match files (repeatable, supports `**`) |

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

| Flag               | Short | Description                        |
| ------------------ | ----- | ---------------------------------- |
| `--target`         | `-t`  | New target path                    |
| `--files`          | `-f`  | Replace file list (repeatable)     |
| `--patterns`       | `-p`  | Replace pattern list (repeatable)  |
| `--clear-files`    |       | Remove all files from the entry    |
| `--clear-patterns` |       | Remove all patterns from the entry |

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

| Flag         | Short | Description                                              |
| ------------ | ----- | -------------------------------------------------------- |
| `--target`   | `-t`  | Target path for the symlink (required)                   |
| `--files`    | `-f`  | Explicit files to symlink (repeatable)                   |
| `--patterns` | `-p`  | Glob patterns to match files (repeatable, supports `**`) |

### `store target remove <name>`

Removes a specific target from a store by its path. Symlinks for the removed target are unlinked. If only one target remains, the store is automatically migrated back to the single-target format.

```sh
$ store target remove shells -t ~/.config/fish
  removed target ~/.config/fish from shells
```

**Flags:**

| Flag       | Short | Description                      |
| ---------- | ----- | -------------------------------- |
| `--target` | `-t`  | Target path to remove (required) |

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

| Flag               | Short | Description                         |
| ------------------ | ----- | ----------------------------------- |
| `--target`         | `-t`  | Target path to modify (required)    |
| `--files`          | `-f`  | Replace file list (repeatable)      |
| `--patterns`       | `-p`  | Replace pattern list (repeatable)   |
| `--clear-files`    |       | Remove all files from the target    |
| `--clear-patterns` |       | Remove all patterns from the target |

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
store version 0.8.0
```

The `--version` flag also works:

```sh
$ store --version
store version 0.8.0
```

When built without a version (e.g., `go build ./cmd/store`), the version defaults to `dev`. Use the Makefile to build with a specific version:

```sh
make build VERSION=0.8.0
```

### `store status [name]`

Shows the symlink status for one or all stores. For file-mode stores, each file is shown individually.

````sh
$ store status
  nvim                 [linked]   ~/.config/nvim
  shells               .zshrc               [linked]   ~/.zshrc
  shells               .bashrc              [linked]   ~/.bashrc
  shells               config.fish          [linked]   ~/.config/fish/config.fish
  shells               config.nu            [linked]   ~/.config/nushell/config.nu
  git                  [conflict] ~/.config/git

### `store secret set <name> [value]`

Sets an encrypted secret. If value is omitted, prompts for it with hidden input.

```sh
# Prompt for value
$ store secret set github_token
Enter passphrase: ****
Enter secret value: ****

# Provide value as argument
$ store secret set api_key "your-secret-value"
Enter passphrase: ****
````

### `store secret get <name>`

Prints the decrypted value of a secret.

```sh
$ store secret get github_token
Enter passphrase: ****
your-secret-value
```

### `store secret rm <name>`

Removes a secret from the encrypted store.

```sh
$ store secret rm github_token
Enter passphrase: ****
Removed secret github_token
```

### `store secret list`

Lists all secret names (not values), sorted alphabetically.

```sh
$ store secret list
Enter passphrase: ****
api_key
github_token
```

All secret commands prompt for a passphrase or read it from the `STORE_PASSPHRASE` environment variable.

### `store status [name]`

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
  tmux:
    target: ~/.config/tmux
    hooks:
      post: "tmux source-file ~/.config/tmux/tmux.conf 2>/dev/null || true"
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

Files containing `{{ secret "name" }}` placeholders are automatically detected and rendered during store operations. No configuration changes are needed.

Each entry maps a store name (a directory in your repo) to one or more target paths (where symlinks are created). The `ignore:` field can be used on both top-level store entries and individual targets in multi-target mode to exclude specific files from being symlinked.

### Entry Fields

#### Single-target fields

| Field      | Required | Description                                                                                   |
| ---------- | -------- | --------------------------------------------------------------------------------------------- |
| `target`   | No       | Where symlinks are created. Without a target, the entry is saved but no symlinks are created. |
| `files`    | No       | Explicit list of files to symlink individually.                                               |
| `patterns` | No       | Glob patterns to match files. Supports `*`, `?`, `[...]`, and `**` for recursive matching.    |
| `ignore`   | No       | List of glob patterns to exclude from symlinking.                                             |
| `when`     | No       | Platform conditions for this store. See [Platform Conditionals](#platform-conditionals).      |
| `hooks`    | No       | Pre/post shell commands to run around store operations. See [Hooks](#hooks).                  |

#### Multi-target fields

| Field                | Required | Description                                                                  |
| -------------------- | -------- | ---------------------------------------------------------------------------- |
| `targets`            | No       | List of target entries, each with its own `target`, `files`, and `patterns`. |
| `targets[].target`   | Yes      | Where symlinks are created for this target entry.                            |
| `targets[].files`    | No       | Explicit list of files to symlink individually for this target.              |
| `targets[].patterns` | No       | Glob patterns to match files for this target.                                |
| `targets[].ignore`   | No       | List of glob patterns to exclude from symlinking for this target.            |

**Behavior:**

- **No `files` or `patterns`:** The entire store directory is symlinked to the target (whole-directory mode).
- **`files` and/or `patterns` specified:** Only matched files are symlinked individually. Directory structure is preserved at the target.
- **Both `files` and `patterns`:** Results are combined and deduplicated.

These rules apply independently to each target in a multi-target store. One target can use whole-directory mode while another uses file-level symlinks.

### Pattern Syntax

Patterns use standard glob syntax with recursive matching support:

| Pattern          | Matches                                    |
| ---------------- | ------------------------------------------ |
| `.zsh*`          | `.zshrc`, `.zshenv`, etc. at the top level |
| `*.conf`         | All `.conf` files at the top level         |
| `**/*.conf`      | All `.conf` files at any depth             |
| `config/*.lua`   | `.lua` files inside `config/`              |
| `**/*.{lua,vim}` | `.lua` and `.vim` files at any depth       |

### Target Path Formats

- `~` prefix -- expanded to the user's home directory (e.g., `~/.config/nvim`). **Must be quoted in YAML** (`target: "~"`) to avoid being interpreted as null by the YAML parser. Portable across machines.
- Absolute paths -- used as-is (e.g., `/home/user/.config/nvim`).

Relative paths provided via `--target` are automatically converted to absolute paths.

## Hooks

Hooks let you run shell commands before or after store operations. There are two levels: per-store hooks defined in `config.yaml`, and global hooks defined as scripts in `.store/hooks/`.

### Per-store hooks

Add a `hooks` field to any store entry with `pre` and/or `post` commands:

```yaml
stores:
  tmux:
    target: ~/.config/tmux
    hooks:
      post: "tmux source-file ~/.config/tmux/tmux.conf 2>/dev/null || true"
  nvim:
    target: ~/.config/nvim
    hooks:
      pre: "nvim --headless -c 'checkhealth' -c 'qa' 2>/dev/null"
      post: "echo 'NeoVim config linked'"
```

Per-store hooks run around that specific store's symlink operations. The `pre` hook runs before linking/unlinking, and `post` runs after. Commands are executed via `sh -c`.

| Hook   | On failure                          |
| ------ | ----------------------------------- |
| `pre`  | Aborts the operation for that store |
| `post` | Prints a warning, does not abort    |

### Global hooks

Place executable scripts in `.store/hooks/` to run before or after commands that operate on all stores:

```
.store/hooks/
  pre-store       # runs before store/storeall linking
  post-store      # runs after store/storeall linking
  pre-remove      # runs before removeall unlinking
  post-remove     # runs after removeall unlinking
```

Global hooks must be executable (`chmod +x`). Non-executable files are silently skipped.

| Hook          | On failure                       |
| ------------- | -------------------------------- |
| `pre-store`   | Aborts the entire command        |
| `pre-remove`  | Aborts the entire command        |
| `post-store`  | Prints a warning, does not abort |
| `post-remove` | Prints a warning, does not abort |

### Execution order

For a command like `store` (storeall):

1. `.store/hooks/pre-store` (global)
2. For each store entry:
   - `hooks.pre` (per-store -- skip this store on failure)
   - Create symlinks
   - `hooks.post` (per-store -- warn on failure)
3. `.store/hooks/post-store` (global)

### Environment variables

All hooks receive the following environment variables:

| Variable               | Description                                      |
| ---------------------- | ------------------------------------------------ |
| `STORE_ROOT`           | Absolute path to the repository root             |
| `STORE_ACTION`         | `link` or `unlink`                               |
| `STORE_NAME`           | Store entry name (per-store hooks only)          |
| `STORE_TARGET`         | Target path for the store (per-store hooks only) |
| `STORE_OS`             | Operating system (linux, darwin, windows)        |
| `STORE_ARCH`           | CPU architecture (amd64, arm64)                  |
| `STORE_DISTRO`         | Distribution (ubuntu, arch, macos, etc.)         |
| `STORE_DISTRO_VERSION` | Distribution version (24.04, rolling, etc.)      |
| `STORE_HOSTNAME`       | Machine hostname                                 |
| `STORE_WSL`            | Whether running in WSL (true/false)              |
| `STORE_SHELL`          | Current shell (zsh, bash, fish, nu)              |

## Secrets

Config files often contain secrets like API keys, tokens, or passwords. `store` encrypts these secrets so your dotfiles repository can remain public while keeping sensitive data secure.

### Template syntax

Use the `{{ secret "name" }}` placeholder in any file within a store. For example, a `.gitconfig` template:

```
[github]
	token = {{ secret "github_token" }}
```

### How it works

- Secrets are stored encrypted in `.store/secrets.enc` using argon2id for key derivation and XChaCha20-Poly1305 for authenticated encryption.
- When `store` creates symlinks, files containing `{{ secret "..." }}` are rendered with their decrypted values.
- Rendered files are stored in a local staging directory and symlinked from there.
- Non-template files are symlinked directly from the repository as usual.
- The passphrase is read from the `STORE_PASSPHRASE` environment variable or prompted interactively with hidden input.

### Quick start workflow

1. Add a secret to the encrypted store:

```sh
$ store secret set github_token
Enter passphrase: ****
Enter secret value: ****
```

2. Use the secret in a config file:

```sh
$ echo 'token = {{ secret "github_token" }}' > git/.gitconfig
```

3. Run `store` to render templates and create symlinks:

```sh
$ store
Enter passphrase: ****
Storing all stores:
  git -> ~/.config/git (files)
```

4. On a new machine, clone and restore:

```sh
$ git clone ~/dotfiles && cd ~/dotfiles
$ store
Enter passphrase: ****
```

### Environment variable

Set `STORE_PASSPHRASE` to avoid interactive prompts. This is useful for scripts or CI/CD environments.

```sh
export STORE_PASSPHRASE="your-secure-passphrase"
store
```

### Staging directory

Rendered files are stored at `$XDG_STATE_HOME/store/<hash>/`, which defaults to `~/.local/state/store/`. These files are local to the machine and should not be committed to your repository.

### Security note

The encrypted secrets file (`.store/secrets.enc`) is safe to commit to your repository. It uses industry-standard encryption to protect your data.

## Platform Conditionals

Different machines often need different configurations. The `when:` clause lets you target stores to specific platforms.

### Config syntax

Add the `when:` field to any store entry to define platform requirements:

```yaml
stores:
  # Linux-only store
  apt-configs:
    target: ~/.config/apt
    when:
      os: linux

  # macOS-only store
  brew-configs:
    target: ~/.config/brew
    when:
      os: darwin

  # Distro-specific store
  ubuntu-packages:
    target: ~/scripts
    files:
      - install-ubuntu.sh
    when:
      distro: ubuntu

  # Hostname-specific
  work-scripts:
    target: ~/work
    when:
      hostname: work-laptop

  # WSL-specific
  wsl-tools:
    target: ~/.local/bin
    when:
      wsl: true

  # Combined conditions (Linux + arm64)
  raspberry-pi:
    target: ~/pi-configs
    when:
      os: linux
      arch: arm64
```

### Available conditions

| Field            | Description                                    | Possible Values                                |
| ---------------- | ---------------------------------------------- | ---------------------------------------------- |
| `os`             | Operating system                               | `linux`, `darwin`, `windows`                   |
| `arch`           | CPU architecture                               | `amd64`, `arm64`, `arm`, etc.                  |
| `distro`         | Linux distribution or OS name                  | `ubuntu`, `fedora`, `arch`, `macos`, `windows` |
| `distro_version` | Version of the distribution                    | `24.04`, `rolling`, etc.                       |
| `hostname`       | Machine hostname                               | Any valid hostname                             |
| `shell`          | Current shell                                  | `zsh`, `bash`, `fish`, `nu`                    |
| `wsl`            | Whether running in Windows Subsystem for Linux | `true`, `false`                                |

### Matching behavior

- **AND logic**: All specified fields must match for the store to be applied.
- **Unset fields**: Any field not specified matches everything.
- **Default**: Stores without a `when:` clause always apply.
- **Filtering**: Non-matching stores are silently skipped during `store` and `store status` operations.

### Example multi-platform setup

```yaml
stores:
  # Common configs for all machines
  common:
    target: "~"
    files:
      - .gitconfig
      - .vimrc

  # Linux-specific shell configs
  linux-shell:
    target: "~"
    files:
      - .bashrc
    when:
      os: linux

  # macOS-specific shell configs
  macos-shell:
    target: "~"
    files:
      - .zshrc
    when:
      os: darwin

  # Work-only scripts
  work-tools:
    target: "~/work"
    when:
      hostname: work-laptop

## Ignoring Files

Exclude files and directories from being symlinked using the `ignore:` field. This is useful for nested stores, scratch files, or editor backups.

### Global Defaults

The following patterns are always excluded automatically. You don't need to manually ignore them:

- `.store/`
- `.git/`
- `.gitignore`
- `.DS_Store`

### Config Syntax

The `ignore:` field accepts a list of glob patterns. It can be used on both top-level store entries and individual targets in multi-target mode.

```yaml
stores:
  nvim:
    target: ~/.config/nvim
    ignore:
      - "*.bak"
      - "scratch/"
      - "**/*.test.lua"
```

### Auto-promotion

When a store is in whole-directory mode (no `files` or `patterns` specified), it normally creates a single symlink for the entire directory. If the directory contains ignored content (either via global defaults like `.git/` or explicit `ignore:` patterns), the store is automatically promoted to file mode.

In file mode, the target becomes a real directory, and `store` creates individual symlinks for every non-ignored file and directory inside the store. This ensures that ignored files never appear at the target location.

### Nested Stores

A common use case for ignore patterns is nested stores. If a store directory contains its own `.store/` directory for sub-stores, the inner `.store/` is automatically excluded by the global defaults. This allows you to organize complex configurations hierarchically without accidentally symlinking configuration metadata.

### Pattern Syntax

The `ignore:` field uses the same glob syntax as the `patterns:` field. It supports `*`, `**`, and `?` for flexible matching. See [Pattern Syntax](#pattern-syntax) for more details.

## Status Indicators

| Status       | Meaning                                                                                                                                       |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `[linked]`   | Symlink exists and points to the correct store directory or file.                                                                             |
| `[missing]`  | No symlink exists at the target path. Run `store` to create it.                                                                               |
| `[conflict]` | Something exists at the target path but it is not a symlink managed by store. Running `store` will offer to move it into the store directory. |
| `[broken]`   | A symlink exists but its destination no longer exists. Running `store` will replace it.                                                       |

## How It Works

- **Root discovery:** Commands can be run from any subdirectory. `store` walks up the directory tree to find the nearest `.store/` directory, similar to how `git` finds `.git/`.
- **Symlinks are absolute:** When creating symlinks, source paths are resolved to absolute paths. This means symlinks work regardless of your working directory.
- **Conflict resolution:** Before creating symlinks, `store` checks all target paths for conflicts. If files or directories already exist that aren't managed by store, it lists them and offers to move them into the store directory automatically. For directory conflicts, the contents are merged into the store directory. If any files already exist in the store directory, they are listed and a separate confirmation is required before creating `.bak` backups. Use `--force` to skip the backup confirmation.
- **Broken symlink recovery:** If a symlink exists but points to a nonexistent path, `store` removes it and creates a fresh one pointing to the correct source.
- **Hooks:** Per-store hooks (`hooks.pre`/`hooks.post` in config) run around individual store operations. Global hooks (executable scripts in `.store/hooks/`) run around commands that operate on all stores. Pre-hooks can abort the operation; post-hooks only warn on failure.
- **File matching performance:** Explicit `files` entries are validated with a single stat call each (no directory walking). Simple glob patterns use `Glob` without recursive traversal. Only `**` patterns trigger a full directory walk, using the efficient `WalkDir` API.

## Troubleshooting

### Target path is `~` but store says "no target configured"

In YAML, a bare `~` is a null value. If you write:

```yaml
stores:
  nvim:
    target: ~
```

The ~ is parsed as null (empty string in Go), and store treats it as configured. **Quote the tilde**:

```yaml
stores:
  nvim:
    target: "~"
```

This is a YAML semantic -- the same applies to any other tool using YAML config files.

### Conflicts: files already exist at the target path

When files or directories already exist where `store` wants to create symlinks, you'll see a prompt:

```
The following files conflict with store symlinks:
  ~/.config/nvim (directory -> will be moved to ~/dotfiles/nvim)
  ~/.zshrc (file -> will be moved to ~/dotfiles/shells/.zshrc)

Move these files into the store and create symlinks? [y/N]
```

Answering **y** moves the conflicting files into your store directories and creates the symlinks. This is useful when setting up on a machine that already has config files -- the existing files become the store-managed versions.

If files already exist in the store directory, they are listed and a separate prompt asks for confirmation before creating `.bak` backups:

```
The following files in the store will be backed up (.bak):
  ~/dotfiles/nvim/init.lua

Proceed with creating backups? [y/N]
```

Use `--force` to skip this prompt (backups are still listed so you can clean them up afterward).

Answering **N** (the default) aborts the operation with no changes made.

### "[broken]" status

The symlink exists but points to a directory that no longer exists. This can happen if the store directory was renamed or deleted. Running `store` will automatically replace broken symlinks.

### "no .store directory found"

You're not inside a repository that has been initialized with `store init`. Either `cd` into your dotfiles repo or run `store init` to set one up.
