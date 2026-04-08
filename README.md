![store logo](https://res.cloudinary.com/cush/image/upload/v1775189496/screenshot-2026-04-02_22-10-09_y9sbug.png)

`store` manages dotfile symlinks from a single repository without requiring a mirrored target directory layout.

## Table of Contents

- [Overview](#overview)
- [How It Differs from GNU Stow](#how-it-differs-from-gnu-stow)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Config Format](#config-format)
- [Hooks](#hooks)
- [Secrets](#secrets)
- [Platform Conditionals](#platform-conditionals)
- [Ignoring Files](#ignoring-files)
- [Shell Completion](#shell-completion)
- [Status Indicators](#status-indicators)
- [How It Works](#how-it-works)
- [Troubleshooting](#troubleshooting)

## Overview

Each store is a top-level directory in your repo. The config in `.store/config.yaml` tells `store` where that directory, or selected files inside it, should appear on disk.

Example repo layout:

```text
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

Example result after running `store`:

```text
~/.config/nvim                -> ~/dotfiles/nvim
~/.zshrc                      -> ~/dotfiles/shells/.zshrc
~/.bashrc                     -> ~/dotfiles/shells/.bashrc
~/.config/fish/config.fish    -> ~/dotfiles/shells/config.fish
~/.config/nushell/config.nu   -> ~/dotfiles/shells/config.nu
~/.config/git                 -> ~/dotfiles/git
```

`store` supports whole-directory links, file-level links, multiple targets per store, encrypted template rendering for secrets, hooks, platform filters, ignore patterns, diff previews, health checks, and importing an existing symlink layout back into config.

## How It Differs from GNU Stow

| Differentiator               | GNU Stow                                                | store                                                              |
| ---------------------------- | ------------------------------------------------------- | ------------------------------------------------------------------ |
| **Directory structure**      | Must mirror the target filesystem hierarchy             | Uses top-level store directories plus explicit config              |
| **Target paths**             | Inferred from package layout and invocation directory   | Declared explicitly per store or target in YAML                    |
| **Configuration**            | Convention-based                                        | Single `.store/config.yaml` file                                   |
| **Granularity**              | Primarily package/file symlinks based on layout         | Whole-directory mode or file-mode with `files`/`patterns`          |
| **Multiple targets**         | Usually split across separate packages                  | One store can deploy to multiple target paths                      |
| **Conflict handling**        | Stops and requires manual cleanup                       | Detects conflicts and can move existing files into the store       |
| **Setup on new machine**     | Run `stow` from the right place with the right packages | Run `store` anywhere inside the repo                               |
| **Secret management**        | No built-in encrypted secret store                      | Encrypted secrets plus `{{ secret "name" }}` templates             |
| **Platform conditionals**    | No built-in platform filters                            | `when:` supports OS, arch, distro, hostname, shell, and WSL        |
| **Ignore patterns**          | Manual exclusions or separate packages                  | Built-in global ignores plus per-store/per-target `ignore:`        |
| **Shell completion**         | Shell-specific setup outside the tool                   | `store completion` generates scripts for major shells              |
| **Health checks (doctor)**   | No built-in diagnostics command                         | `store doctor` checks config, targets, secrets, and platform skips |
| **Import existing symlinks** | No import command                                       | `store import` scans existing symlinks and writes config           |
| **Dry run preview**          | No built-in preview command                             | `store diff` shows create/replace/conflict actions before changes  |

## Installation

### go install

```sh
$ go install github.com/cush/store/cmd/store@latest
```

### Build from source

```sh
$ git clone https://github.com/cush/store.git
$ cd store
$ make build VERSION=1.0.0
$ mv store /usr/local/bin/
```

## Quick Start

1. Initialize a dotfiles repo:

```sh
$ mkdir ~/dotfiles
$ cd ~/dotfiles
$ git init
$ store init
```

2. Add a whole-directory store:

```sh
$ store add nvim -t ~/.config/nvim
```

3. Add a file-level store for files that live in your home directory:

```sh
$ store add shells -t ~ -f .zshrc -f .bashrc
```

4. Add more targets to the same store:

```sh
$ store target add shells -t ~/.config/fish -f config.fish
$ store target add shells -t ~/.config/nushell -f config.nu
```

5. Commit the repo:

```sh
$ git add -A
$ git commit -m "add dotfiles"
$ git push
```

6. Restore everything on a new machine:

```sh
$ git clone https://github.com/you/dotfiles.git ~/dotfiles
$ cd ~/dotfiles
$ store
```

If target files already exist, `store` detects the conflict, offers to move those files into the repo, and then creates the symlinks.

## Commands

Run `store --help` for the full CLI tree. The command reference below documents every subcommand defined by the CLI.

### `store`

Applies all configured stores. This is the default command you run after cloning a dotfiles repo on a new machine.

| Flag      | Short | Description                                                                                   |
| --------- | ----- | --------------------------------------------------------------------------------------------- |
| `--only`  |       | Apply only the named stores; repeatable                                                       |
| `--force` |       | Create `.bak` backups without prompting when conflicts require overwriting files in the store |

```sh
$ store --only nvim --only git
$ store --force
```

```text
$ store
Storing all stores:
  nvim -> ~/.config/nvim
  shells -> ~ (files)
  shells -> ~/.config/fish (files)
  shells -> ~/.config/nushell (files)
```

How it works:

- Filters out stores whose `when:` clause does not match the current platform.
- Runs global `pre-store` and `post-store` hooks if present.
- Detects conflicts before linking and prompts once for all selected stores.
- Prompts for a secrets passphrase only if one of the selected stores needs template rendering.

### `store init`

Creates `.store/config.yaml` in the current directory.

```sh
$ store init
```

```text
Initialized store config at .store/config.yaml
```

Use this once at the root of a new dotfiles repo.

### `store import`

Scans for existing symlinks that already point into the repo and imports them into `.store/config.yaml`.

| Flag         | Short | Description                                                                                                    |
| ------------ | ----- | -------------------------------------------------------------------------------------------------------------- |
| `--scan-dir` |       | Directory to scan for symlinks; repeatable; defaults to `~`, `~/.config`, `~/.local/share`, and `~/.local/bin` |
| `--dry-run`  |       | Print discovered imports without writing config                                                                |

```sh
$ store import --dry-run
$ store import --scan-dir ~/.config --scan-dir ~/.local/bin
```

```text
$ store import --dry-run
Scanning for symlinks pointing into /home/user/dotfiles...

Found:
  nvim   ~/.config/nvim -> nvim/               (whole directory)
  shells ~/.zshrc -> shells/.zshrc             (file)
  shells ~/.bashrc -> shells/.bashrc           (file)

Dry run: would import 3 symlinks as 2 stores
```

How it works:

- Scans the immediate contents of each scan directory for symlinks.
- Keeps only links whose targets resolve inside the repo and point at a top-level store directory.
- Groups discovered links into single-target or multi-target store entries.
- Skips stores that already exist in config.

Relevant details:

- Without `--dry-run`, `store import` prints the discovered mapping and asks for confirmation before writing config.
- The persistent `--force` flag skips that confirmation prompt.

### `store add <name>`

Creates a store directory if needed, adds the config entry, and immediately links it if a target is provided.

| Flag         | Short | Description                                             |
| ------------ | ----- | ------------------------------------------------------- |
| `--target`   | `-t`  | Target path for the store                               |
| `--files`    | `-f`  | Explicit files to link individually; repeatable         |
| `--patterns` | `-p`  | Glob patterns to match files; repeatable; supports `**` |

```sh
$ store add nvim -t ~/.config/nvim
$ store add shells -t ~ -f .zshrc -f .bashrc
$ store add configs -t ~/.config -p "**/*.conf"
$ store add git
```

How it works:

- Without `--target`, the store is saved but not linked yet.
- Without `--files` or `--patterns`, the entire store directory is linked.
- Relative targets are resolved to absolute paths before being saved.

### `store modify <name>`

Updates an existing single-target store. Each provided flag replaces the entire field.

| Flag               | Short | Description                          |
| ------------------ | ----- | ------------------------------------ |
| `--target`         | `-t`  | Replace the target path              |
| `--files`          | `-f`  | Replace the file list; repeatable    |
| `--patterns`       | `-p`  | Replace the pattern list; repeatable |
| `--clear-files`    |       | Remove all files from the entry      |
| `--clear-patterns` |       | Remove all patterns from the entry   |

```sh
$ store modify nvim -t ~/.config/nvim-custom
$ store modify shells --clear-files -p ".zsh*" -p ".bash*"
```

How it works:

- Removes old symlinks before applying the updated config.
- Re-links the store after saving the new entry.
- Refuses to run on multi-target stores; use `store target modify` instead.

### `store target add <name>`

Adds another target to an existing store.

| Flag         | Short | Description                                             |
| ------------ | ----- | ------------------------------------------------------- |
| `--target`   | `-t`  | Target path to add; required                            |
| `--files`    | `-f`  | Explicit files to link individually; repeatable         |
| `--patterns` | `-p`  | Glob patterns to match files; repeatable; supports `**` |

```sh
$ store target add shells -t ~/.config/fish -f config.fish
$ store target add shells -t ~/.config/nushell -p "*.nu"
```

How it works:

- Automatically migrates a single-target store to the `targets:` format.
- Rejects duplicate target paths after normalizing `~` and absolute paths.

### `store target remove <name>`

Removes one target from a store and unlinks that target's symlinks.

| Flag       | Short | Description                     |
| ---------- | ----- | ------------------------------- |
| `--target` | `-t`  | Target path to remove; required |

```sh
$ store target remove shells -t ~/.config/fish
```

```text
removed target ~/.config/fish from shells
```

How it works:

- Finds the target by normalized path.
- Unlinks that target.
- Migrates the store back to single-target format automatically if one target remains.

### `store target modify <name>`

Updates the `files` or `patterns` for one target inside a multi-target store.

| Flag               | Short | Description                          |
| ------------------ | ----- | ------------------------------------ |
| `--target`         | `-t`  | Target path to modify; required      |
| `--files`          | `-f`  | Replace the file list; repeatable    |
| `--patterns`       | `-p`  | Replace the pattern list; repeatable |
| `--clear-files`    |       | Remove all files from the target     |
| `--clear-patterns` |       | Remove all patterns from the target  |

```sh
$ store target modify shells -t ~ -f .zshrc -f .bashrc -f .zprofile
$ store target modify shells -t ~/.config/nushell --clear-files -p "*.nu"
```

How it works:

- Removes old symlinks for the selected target first.
- Re-links only the updated target after saving config.

### `store remove <name>`

Removes the store's symlinked targets and deletes the store entry from config.

```sh
$ store remove nvim
```

```text
Removed store nvim (~/.config/nvim)
```

The directory inside your repo is left untouched.

### `store removeall`

Removes all configured store symlinks and deletes their config entries.

```sh
$ store removeall
```

```text
Removing all stores:
  removed nvim (~/.config/nvim)
  removed git (~/.config/git)
```

How it works:

- Runs global `pre-remove` and `post-remove` hooks if present.
- Continues removing other stores even if one store fails, then reports aggregated errors.

### `store status [name]`

Shows the current symlink state for one store or all stores.

```sh
$ store status
$ store status nvim
```

```text
$ store status
  nvim                 [linked]   ~/.config/nvim
  shells               .zshrc               [linked]   ~/.zshrc
  shells               config.fish          [missing]  ~/.config/fish/config.fish
  git                  [conflict] ~/.config/git
```

How it works:

- In file mode, each linked file is reported separately.
- When run without a name, stores filtered out by `when:` are omitted.

### `store diff`

Previews what `store` would do without changing anything.

| Flag     | Short | Description                               |
| -------- | ----- | ----------------------------------------- |
| `--only` |       | Preview only the named stores; repeatable |

```sh
$ store diff
$ store diff --only nvim --only git
```

```text
$ store diff
  nvim   ~/.config/nvim                     [ok]
  shells .zshrc → ~/.zshrc                 [ok]
  shells config.fish → ~/.config/fish/config.fish [create]
  git    ~/.config/git                     [conflict]

Summary: 2 ok, 1 to create, 1 conflict, 0 to replace
```

How it works:

- Uses the same platform filtering as `store`.
- Reports current targets as `ok`, `create`, `conflict`, `replace`, or `error`.
- `replace` means a broken symlink would be removed and recreated.

### `store doctor`

Runs diagnostics against the current repo and reports errors, warnings, and informational issues.

```sh
$ store doctor
```

```text
Checking store health...

  [ok] 4 stores configured
  [ok] all symlinks healthy

  [info] directory "scratch" exists but is not configured as a store
  [warn] secrets file not found but templates reference secrets

2 issues found (1 warning, 1 info)
```

What it checks:

- Config entries whose store directory is missing.
- Top-level directories in the repo that are not configured as stores.
- Broken symlinks.
- Multiple stores claiming the same target path.
- Missing secrets file or missing referenced secrets when `STORE_PASSPHRASE` is available.
- Empty store directories.
- Stores skipped on the current platform because of `when:`.

Relevant details:

- `store doctor` exits with an error only when it finds doctor-level errors.
- Warnings and info are still printed but do not make the command fail.

### `store version`

Prints the current version.

```sh
$ store version
$ store --version
```

```text
store version 1.0.0
```

If built without an injected version, the binary reports `dev`.

### `store secret set <name> [value]`

Creates or updates one encrypted secret.

```sh
$ store secret set github_token
$ store secret set api_key "super-secret"
```

How it works:

- If `value` is omitted, `store` prompts for it with hidden input.
- Reads the passphrase from `STORE_PASSPHRASE` or prompts interactively.

### `store secret get <name>`

Prints one decrypted secret value.

```sh
$ store secret get github_token
```

### `store secret rm <name>`

Deletes one secret from `.store/secrets.enc`.

```sh
$ store secret rm github_token
```

### `store secret list`

Lists secret names in sorted order.

```sh
$ store secret list
```

Relevant details for all secret commands:

- Secrets are stored in `.store/secrets.enc`.
- Secret names are listed; secret values are never listed.
- See [Secrets](#secrets) for template rendering details.

### `store completion [bash|zsh|fish|powershell]`

Generates a shell completion script for the selected shell.

```sh
$ store completion bash
$ store completion zsh
$ store completion fish
```

Use it with the install instructions in [Shell Completion](#shell-completion).

## Config Format

Configuration lives in `.store/config.yaml`.

Single-target format:

```yaml
stores:
  nvim:
    target: "~/.config/nvim"
  shells:
    target: "~"
    files:
      - .zshrc
      - .bashrc
    ignore:
      - "*.bak"
    hooks:
      post: "printf 'shells linked\n'"
    when:
      os: linux
```

Multi-target format:

```yaml
stores:
  shells:
    targets:
      - target: "~"
        files:
          - .zshrc
          - .bashrc
      - target: "~/.config/fish"
        files:
          - config.fish
      - target: "~/.config/nushell"
        patterns:
          - "*.nu"
        ignore:
          - "*.bak"
```

Rules:

- A store may use either `target` or `targets`, not both.
- In multi-target mode, `files`, `patterns`, and `ignore` belong inside each `targets[]` entry.
- If a store entry has no target yet, it is valid config but nothing is linked.

### Top-level store fields

| Field      | Required | Description                                      |
| ---------- | -------- | ------------------------------------------------ |
| `target`   | No       | Target path for a single-target store            |
| `files`    | No       | Explicit list of files to link individually      |
| `patterns` | No       | Glob patterns to match files; supports `**`      |
| `ignore`   | No       | Glob patterns to exclude from linking            |
| `hooks`    | No       | Per-store `pre` and `post` shell commands        |
| `when`     | No       | Platform filter for this store                   |
| `targets`  | No       | List of per-target entries for multi-target mode |

### Nested `targets[]` fields

| Field                | Required | Description                                         |
| -------------------- | -------- | --------------------------------------------------- |
| `targets[].target`   | Yes      | Target path for this target entry                   |
| `targets[].files`    | No       | Explicit files to link individually for this target |
| `targets[].patterns` | No       | Glob patterns to match files for this target        |
| `targets[].ignore`   | No       | Glob patterns to exclude for this target            |

### `hooks` fields

| Field        | Required | Description                                          |
| ------------ | -------- | ---------------------------------------------------- |
| `hooks.pre`  | No       | Command run before link or unlink work for the store |
| `hooks.post` | No       | Command run after link or unlink work for the store  |

### `when` fields

| Field                 | Required | Description                                                         |
| --------------------- | -------- | ------------------------------------------------------------------- |
| `when.os`             | No       | OS name such as `linux`, `darwin`, or `windows`                     |
| `when.arch`           | No       | CPU architecture such as `amd64` or `arm64`                         |
| `when.distro`         | No       | Distro or OS family such as `ubuntu`, `arch`, `macos`, or `windows` |
| `when.distro_version` | No       | Distro version string                                               |
| `when.hostname`       | No       | Hostname to match                                                   |
| `when.shell`          | No       | Current shell name such as `zsh`, `bash`, `fish`, or `nu`           |
| `when.wsl`            | No       | Whether the machine is running under WSL                            |

### Matching and mode behavior

- No `files` and no `patterns`: whole-directory mode.
- Any `files` or `patterns`: file mode.
- `files` and `patterns` can be used together; matches are combined and deduplicated.
- `ignore` applies in both explicit file mode and pattern mode.
- If whole-directory mode would include ignored content, `store` automatically promotes that target to file mode.

### Pattern syntax

| Pattern          | Matches                                          |
| ---------------- | ------------------------------------------------ |
| `.zsh*`          | `.zshrc`, `.zshenv`, and similar top-level files |
| `*.conf`         | Top-level `.conf` files                          |
| `**/*.conf`      | `.conf` files at any depth                       |
| `config/*.lua`   | `.lua` files directly inside `config/`           |
| `**/*.{lua,vim}` | `.lua` and `.vim` files recursively              |

### Target path rules

- `~` and `~/...` are supported on the CLI and in config.
- In YAML, every `~` path must be quoted, for example `target: "~"` or `target: "~/.config/nvim"`.
- Relative CLI target paths are resolved to absolute paths before being saved.

## Hooks

Hooks let you run shell commands around store operations. Use them for post-link reloads, cache rebuilds, or validation steps tied to a specific store or to the whole repo.

Per-store hook example:

```yaml
stores:
  tmux:
    target: "~/.config/tmux"
    hooks:
      post: "tmux source-file ~/.config/tmux/tmux.conf 2>/dev/null || true"
  nvim:
    target: "~/.config/nvim"
    hooks:
      pre: "nvim --headless -c 'checkhealth' -c 'qa' 2>/dev/null"
```

Global hook layout:

```text
.store/hooks/
  pre-store
  post-store
  pre-remove
  post-remove
```

How it works:

- Per-store hooks are configured in YAML and executed with `sh -c`.
- Global hooks are executable files in `.store/hooks/` and are run directly.
- Global hook scripts must have the executable bit set; non-executable files are ignored.

Relevant details:

- `pre` hooks abort that store on failure.
- `post` hooks only warn on failure.
- `pre-store` and `pre-remove` abort the whole command on failure.
- Hooks receive `STORE_ROOT`, `STORE_ACTION`, and detected platform variables; per-store hooks also receive `STORE_NAME` and `STORE_TARGET`.

## Secrets

Secrets let you keep encrypted values in the repo while rendering those values into linked files locally at apply time.

Template example:

```yaml
stores:
  git:
    target: "~/.config/git"
    files:
      - .gitconfig
```

```text
[github]
    token = {{ secret "github_token" }}
```

Usage example:

```sh
$ store secret set github_token
$ store
```

How it works:

- Template placeholders use `{{ secret "name" }}`.
- `store` scans the selected store directories and only prompts for a passphrase if rendering is needed.
- Secrets are stored in `.store/secrets.enc`.
- Rendered files are written into a local staging directory under `$XDG_STATE_HOME/store/<repo-hash>/` or `~/.local/state/store/<repo-hash>/` if `XDG_STATE_HOME` is unset.
- Files without secret placeholders are symlinked directly; rendered files are symlinked from the staging directory.

Relevant details:

- Encryption uses Argon2id for key derivation and XChaCha20-Poly1305 for authenticated encryption.
- If a referenced secret is missing, the render step fails and the store operation stops.
- The passphrase comes from `STORE_PASSPHRASE` or hidden interactive input.
- `.store/secrets.enc` is designed to be committed; the plaintext secrets are not.

## Platform Conditionals

Platform conditionals let one repo serve multiple machines without separate branches or manual edits.

Example:

```yaml
stores:
  linux-shell:
    target: "~"
    files:
      - .bashrc
    when:
      os: linux

  work-tools:
    target: "~/work"
    when:
      hostname: work-laptop

  wsl-tools:
    target: "~/.local/bin"
    when:
      wsl: true
```

How it works:

- `store` compares each populated `when:` field against detected platform info.
- All specified fields must match.
- Stores without `when:` always apply.

Relevant details:

- Detected values include OS, arch, distro, distro version, hostname, shell, and WSL state.
- Commands that operate on the configured set, such as `store`, `store diff`, and `store status`, skip non-matching stores.
- `store doctor` reports skipped stores as informational issues so you can confirm the filter is doing what you expect.

## Ignoring Files

Ignore patterns exclude files or directories from linking. This is useful for editor backups, nested metadata, scratch files, or partial stores that should not be exposed at the target.

Example:

```yaml
stores:
  nvim:
    target: "~/.config/nvim"
    ignore:
      - "*.bak"
      - "scratch/"
      - "**/*.test.lua"
```

How it works:

- Global ignore patterns are always active: `.store`, `.store/**`, `.git`, `.git/**`, `.gitignore`, `.DS_Store`.
- User-defined `ignore:` patterns are added on top of the global ignore set.
- Directory ignore patterns such as `scratch/` exclude the whole directory tree.

Relevant details:

- Ignore matching uses the same glob engine as `patterns:`.
- If a target has `ignore:` or if the store contains globally ignored files, whole-directory mode is automatically promoted to file mode so ignored content never appears at the target.
- `ignore:` may be defined at the top level for single-target stores or inside each `targets[]` entry for multi-target stores.

## Shell Completion

`store completion` generates completion scripts for supported shells. This is the easiest way to make the command and store-name arguments discoverable from the terminal.

Install examples:

### Bash

```sh
$ mkdir -p ~/.local/share/bash-completion/completions
$ store completion bash > ~/.local/share/bash-completion/completions/store
```

### Zsh

```sh
$ store completion zsh > "${fpath[1]}/_store"
```

### Fish

```sh
$ mkdir -p ~/.config/fish/completions
$ store completion fish > ~/.config/fish/completions/store.fish
```

### PowerShell

```sh
$ store completion powershell | Out-String | Invoke-Expression
```

How it works:

- The CLI generates shell-native completion scripts through Cobra.
- Existing store names are offered as tab completions for commands that take a configured store name, such as `modify`, `remove`, `status`, and `target` subcommands.

Relevant details:

- If your shell requires an explicit completion system bootstrap, enable that in the shell separately.
- Regenerate the script after upgrading `store` if you want completion descriptions and command trees to stay current.

## Status Indicators

| Status       | Meaning                                                          |
| ------------ | ---------------------------------------------------------------- |
| `[linked]`   | A symlink exists and points to the expected source               |
| `[missing]`  | No symlink exists yet                                            |
| `[conflict]` | A non-store file or directory exists where `store` wants to link |
| `[broken]`   | A symlink exists but points to a missing source                  |

These statuses appear in `store status`, and the same underlying states drive `store diff`.

## How It Works

- **Root discovery:** commands can run from any subdirectory inside the repo; `store` walks upward until it finds `.store/`.
- **Portable target storage:** CLI target paths keep `~` prefixes when possible so config stays portable across machines.
- **Absolute symlinks:** link targets are resolved to absolute source paths so the working directory does not matter.
- **Conflict resolution:** before linking, `store` detects files or directories already occupying the target and offers to move them into the repo. If files already exist in the store where moved content would land, `store` lists the `.bak` backups it would create.
- **Whole-directory vs file mode:** no `files` or `patterns` means whole-directory mode unless ignored content forces promotion to file mode.
- **Template staging:** secret-backed templates are rendered into a machine-local staging directory and linked from there.
- **Performance:** explicit files use direct stat checks, non-recursive globs avoid full walks, and only `**` patterns trigger recursive traversal.

## Troubleshooting

### `target: ~` does not work in YAML

Bare `~` is YAML null, not a string. Quote every `~` path:

```yaml
stores:
  shell:
    target: "~"
```

The same rule applies to paths such as `"~/.config/nvim"`.

### `store` says there is no `.store` directory

Run the command inside a repo initialized with `store init`, or initialize the current directory first.

### Existing files already occupy the target path

`store` reports conflicts before linking and offers to move those files into the store.

```text
The following files conflict with store symlinks:
  ~/.config/nvim (directory -> will be moved to ~/dotfiles/nvim)
  ~/.zshrc (file -> will be moved to ~/dotfiles/shells/.zshrc)

Move these files into the store and create symlinks? [y/N]
```

If files already exist inside the store where moved content would land, `store` asks before creating `.bak` backups. Use `--force` to skip that backup confirmation.

### `store diff` shows `replace`

`replace` means the target is currently a broken symlink. Running `store` will remove it and recreate it correctly.

### `store doctor` warns about secrets

If templates reference secrets and `.store/secrets.enc` is missing, doctor warns immediately. If the secrets file exists, set `STORE_PASSPHRASE` before running `store doctor` if you want it to verify that referenced secret names are actually present.
