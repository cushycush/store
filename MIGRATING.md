# Upgrading from 1.x to 2.0

## TL;DR

- **Bare `store` is no longer destructive.** Use `store apply` to reconcile symlinks.
- Everything else in 2.0 is additive. Your `.store/config.yaml` from 1.x works as-is.

## Breaking change: bare `store` prints help

In 1.x, running `store` with no arguments reconciled all symlinks. In 2.0, it
prints help. Reconciliation now has an explicit verb: `store apply`.

```sh
# 1.x
$ store                 # reconciled symlinks

# 2.0
$ store                 # prints help
$ store apply           # reconciles symlinks
```

**Action required:** if you have scripts, Makefiles, cron jobs, shell aliases,
or `chezmoi`-style post-deploy hooks that run bare `store`, update them to
`store apply` before upgrading. A bare `store` invocation in a cron job that
used to reconcile will now silently do nothing.

The reason for this change is the same reason other package managers (git,
npm, docker, cargo) show help on bare invocation: unintended reconciliation
from typing `store` by muscle memory has caused real damage. An explicit verb
makes the intent unambiguous.

## Deprecation: `removeall` → `remove --all`

`store removeall` still works but is hidden from `--help` and prints a
deprecation warning.

```sh
# Old
$ store removeall

# New
$ store remove --all          # prompts for confirmation
$ store remove --all --yes    # skip the confirmation prompt
```

## What's new in 2.0

### `store tui` — interactive dashboard

A keyboard-driven dashboard reachable via `store tui`. Every CLI verb works
from inside it via a `:` command palette with fuzzy matching. See the
[Interactive TUI section of the README](README.md#interactive-tui) for the
full keymap.

### Four new top-level verbs

| Verb           | Purpose                                                         |
| -------------- | --------------------------------------------------------------- |
| `store list`   | One-line summary of every configured store, no filesystem access |
| `store path`   | Print the repo path of a store, e.g. `cd $(store path nvim)`    |
| `store rename` | Rename a store (moves the directory and updates config)         |
| `store edit`   | Open `.store/config.yaml` in `$EDITOR`                          |

### Positional target arguments

```sh
# Old
$ store add nvim -t ~/.config/nvim
$ store target add shells -t ~/.config/fish

# New (both forms still work)
$ store add nvim ~/.config/nvim
$ store target add shells ~/.config/fish
```

Paths under `$HOME` are rewritten to `~`-prefix on save, so config stays
portable across machines even when the shell expands `~` before store sees
the path.

### `--dry-run` everywhere

Every mutating command now accepts `--dry-run`, including `store apply`. If
you reach for it out of habit from other tools, it's there.

### Additive `modify`

```sh
$ store modify shells --add-file .zprofile
$ store modify shells --remove-file .bashrc
$ store modify shells --add-pattern "*.vim"
```

Flags compose in order: `--clear-*` → `--files`/`--patterns` (full replace) →
`--add-*` → `--remove-*`. The original replace flags still work unchanged.

## What hasn't changed

- Config file format (`.store/config.yaml`)
- Existing commands (`init`, `import`, `adopt`, `add`, `modify`, `remove <name>`,
  `status`, `diff`, `doctor`, `version`, `secret *`, `target *`, `completion`)
- Hooks, templates, secrets, platform conditionals, ignore patterns
- Encryption formats, secret storage on disk
- Shell completion scripts

If you're upgrading an unattended setup and just want the symlinks to keep
being reconciled, the single-line fix is:

```sh
# Wherever you had this:
store
# Change to:
store apply
```

That's it. Your config and every other workflow carries over.
