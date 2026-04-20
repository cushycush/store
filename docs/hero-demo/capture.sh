#!/usr/bin/env bash
#
# Sets up an ephemeral $HOME that reproduces the ledger state shown in
# the hero screenshot, then exec's `store tui` inside it. Your real home
# is untouched. Press `q` to quit.
#
# After the TUI opens, press `j` twice to move the cursor onto `shells` —
# that's the selection shown in docs/hero.png.
#
# Requires the `store` binary on your PATH (or pass one via `STORE=...`).

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STAGE="${TMPDIR:-/tmp}/store-hero-demo"
FAKE_HOME="$STAGE/home"
FAKE_STATE="$STAGE/state"
STORE="${STORE:-store}"

rm -rf "$STAGE"
mkdir -p "$FAKE_HOME/.config/fish" "$FAKE_HOME/.config/nushell" "$FAKE_STATE"

# Surface the repo as ~/dotfiles so the header shows a portable label.
ln -s "$HERE" "$FAKE_HOME/dotfiles"
DOTS="$FAKE_HOME/dotfiles"

# Link through ~/dotfiles so the TUI reports them as linked (the symlink
# target has to match the source path the TUI computes from its repo root).
ln -s "$DOTS/git"              "$FAKE_HOME/.config/git"
ln -s "$DOTS/nvim"             "$FAKE_HOME/.config/nvim"
ln -s "$DOTS/shells/.zshrc"    "$FAKE_HOME/.zshrc"
ln -s "$DOTS/shells/.bashrc"   "$FAKE_HOME/.bashrc"
ln -s "$DOTS/shells/config.fish" "$FAKE_HOME/.config/fish/config.fish"
# aliases.fish and ~/.config/nushell/config.nu intentionally missing.

# tmux: nothing at ~/.config/tmux.

# work: a real directory at the target path produces a conflict.
mkdir -p "$FAKE_HOME/work/notes"
printf 'meeting notes\n' > "$FAKE_HOME/work/notes/2026-04-20.md"

cd "$DOTS"
exec env HOME="$FAKE_HOME" XDG_STATE_HOME="$FAKE_STATE" PWD="$DOTS" "$STORE" tui
