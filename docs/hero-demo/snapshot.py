#!/usr/bin/env python3
"""
Capture the store TUI as a PNG for the README hero image.

Spawns `store tui` inside an ephemeral HOME where the ledger lands in a
known mixed state (linked/partial/missing/conflict + a multi-target store
with templates and hooks, organized into nested groups), drives a short
key sequence so the frame shows one expanded group alongside collapsed
ones with the cursor on `shells`, reads the final frame through a pty,
parses it with pyte, and renders it to a PNG with Pillow.

Usage:
  docs/hero-demo/snapshot.py [--store PATH] [--out PATH] [--cols N] [--rows N]

Defaults write docs/hero.png at 96x36 cells.
"""
from __future__ import annotations

import argparse
import fcntl
import os
import pty
import select
import shutil
import signal
import struct
import subprocess
import sys
import termios
import time
from pathlib import Path

import pyte
from PIL import Image, ImageDraw, ImageFont


HERE = Path(__file__).resolve().parent
REPO = HERE.parent.parent


# ----- palette (pulled from internal/tui/theme.go) -----------------------

BG = "#111111"
COLORS = {
    "default":  "#EDE6DC",  # ColorFg
    "muted":    "#A69B8A",
    "dim":      "#6B6558",
    "faint":    "#3F3B35",
    "ember":    "#E89A3A",
    "emberlow": "#7A5324",
    "linked":   "#8AA27A",
    "partial":  "#D9A55E",
    "missing":  "#847C6E",
    "error":    "#C27B6B",
}


def setup_stage(stage: Path) -> tuple[Path, Path, Path]:
    """Create the ephemeral HOME with the mixed symlink state. Returns
    (home, state, dotfiles) where dotfiles is the repo root inside the
    fake home so the header shows `~/dotfiles`. Symlinks point at the
    dotfiles path (not the real HERE) so the TUI's symlink-target check
    reports them as linked — when the repo is reached via the dotfiles
    symlink, source paths are computed under `~/dotfiles/...`."""
    if stage.exists():
        shutil.rmtree(stage)
    home = stage / "home"
    state = stage / "state"
    (home / ".config/fish").mkdir(parents=True)
    (home / ".config/nushell").mkdir(parents=True)
    state.mkdir(parents=True)

    dotfiles = home / "dotfiles"
    dotfiles.symlink_to(HERE)

    (home / ".config/nvim").symlink_to(dotfiles / "editors/nvim")
    (home / ".config/ghostty").symlink_to(dotfiles / "terminals/ghostty")
    (home / ".config/git").symlink_to(dotfiles / "tools/git")
    (home / ".config/lazygit").symlink_to(dotfiles / "tools/lazygit")
    (home / ".zshrc").symlink_to(dotfiles / "shells/.zshrc")
    (home / ".bashrc").symlink_to(dotfiles / "shells/.bashrc")
    (home / ".config/fish/config.fish").symlink_to(dotfiles / "shells/config.fish")
    # aliases.fish, nushell/config.nu, and terminals/tmux intentionally missing.

    # work: a real directory at the target path produces a conflict.
    notes = home / "work/notes"
    notes.mkdir(parents=True)
    (notes / "2026-04-20.md").write_text("meeting notes\n")

    return home, state, dotfiles


def spawn_tui(store_bin: str, home: Path, state: Path, cwd: Path, cols: int, rows: int) -> tuple[int, int]:
    pid, fd = pty.fork()
    if pid == 0:
        env = os.environ.copy()
        env["HOME"] = str(home)
        env["XDG_STATE_HOME"] = str(state)
        env["TERM"] = "xterm-256color"
        env["COLORTERM"] = "truecolor"
        # PWD tells Go's os.Getwd to keep the symlink path; without it the
        # tui would display the resolved absolute path in the header.
        env["PWD"] = str(cwd)
        os.chdir(cwd)
        os.execvpe(store_bin, [store_bin, "tui"], env)
    # Parent: set the pty window size so bubbletea lays out with known dimensions.
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
    return pid, fd


def drain(fd: int, screen_stream: pyte.Stream, timeout: float) -> None:
    """Read from fd and feed pyte for `timeout` seconds."""
    end = time.monotonic() + timeout
    while True:
        remaining = end - time.monotonic()
        if remaining <= 0:
            return
        r, _, _ = select.select([fd], [], [], remaining)
        if not r:
            return
        try:
            data = os.read(fd, 65536)
        except OSError:
            return
        if not data:
            return
        screen_stream.feed(data.decode("utf-8", errors="replace"))


def capture_frame(store_bin: str, cols: int, rows: int) -> pyte.Screen:
    stage = Path(os.environ.get("TMPDIR", "/tmp")) / "store-hero-demo"
    home, state, dotfiles = setup_stage(stage)
    pid, fd = spawn_tui(store_bin, home, state, dotfiles, cols, rows)

    screen = pyte.Screen(cols, rows)
    stream = pyte.Stream(screen)
    try:
        # termenv probes the terminal on startup (OSC 10/11/12 and a cursor
        # position report) and waits up to ~5 seconds for responses. We
        # could answer the probes, but feeding fake bytes back through the
        # pty races with bubbletea's input reader. Letting termenv time
        # out is slower but reliable — one extra 5s at snapshot time is
        # fine for a script no-one runs at interactive latency.
        drain(fd, stream, 6.5)
        # Default ledger order (collapsed): editors, shells, terminals,
        # tools, work. Step the cursor down to `terminals`, expand it with
        # `l` so the snapshot shows both a `−` (open) and `+` (collapsed)
        # group glyph in the same frame, then back up to `shells` for the
        # detail pane. Each press goes one at a time with a short gap so
        # bubbletea sees them as distinct key events and the row-fresh
        # animation advances between them.
        for key in (b"j", b"j", b"l", b"k"):
            os.write(fd, key)
            time.sleep(0.25)
            drain(fd, stream, 0.25)
        # Let the reveal + flash animations settle and the heartbeat tick.
        drain(fd, stream, 1.5)
    finally:
        try:
            os.write(fd, b"q")
        except OSError:
            pass
        time.sleep(0.2)
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        os.close(fd)
        os.waitpid(pid, 0)
    return screen


# ----- PNG renderer ------------------------------------------------------

def ansi_color(attr, fg: bool) -> str:
    """Translate a pyte cell color spec to a hex string."""
    name = attr.fg if fg else attr.bg
    if name == "default":
        return COLORS["default"] if fg else BG
    if isinstance(name, str) and len(name) == 6 and all(c in "0123456789abcdefABCDEF" for c in name):
        return "#" + name.upper()
    return COLORS.get(name, COLORS["default"] if fg else BG)


def pick_font(size: int) -> ImageFont.FreeTypeFont:
    # Prefer fonts with broad Unicode coverage. The TUI leans on geometric
    # shapes (▸ ▾ ● ◐ ○ ✕ ⚡ ◌) — a font that falls back to tofu for any of
    # these breaks the signature look.
    candidates = [
        "/home/cush/FontBase/IosevkaNerdFontMono-Regular.ttf",
        "/home/cush/FontBase/HackNerdFontMono-Regular.ttf",
        "/home/cush/FontBase/DejaVuSansMNerdFontMono-Regular.ttf",
        "/usr/share/fonts/TTF/DejaVuSansMono.ttf",
        "/usr/share/fonts/noto/NotoSansMono-Regular.ttf",
        "/usr/share/fonts/liberation/LiberationMono-Regular.ttf",
    ]
    for path in candidates:
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    raise RuntimeError("no monospace TTF found")


def render_png(screen: pyte.Screen, out: Path, font_size: int = 18) -> None:
    font = pick_font(font_size)
    bold_font = font  # keep it monospaced; bold pulls glyph widths on some families.

    # Measure cell size.
    bbox = font.getbbox("M")
    cell_w = bbox[2] - bbox[0]
    # Line height — tighter than the OS default looks more like a real terminal.
    ascent, descent = font.getmetrics()
    cell_h = ascent + descent + 2

    pad_x, pad_y = 24, 20
    width = pad_x * 2 + cell_w * screen.columns
    height = pad_y * 2 + cell_h * screen.lines

    img = Image.new("RGB", (width, height), BG)
    draw = ImageDraw.Draw(img)

    for y in range(screen.lines):
        row = screen.buffer[y]
        for x in range(screen.columns):
            cell = row[x]
            ch = cell.data
            if not ch:
                continue
            fg = ansi_color(cell, fg=True)
            bg = ansi_color(cell, fg=False)
            px = pad_x + x * cell_w
            py = pad_y + y * cell_h
            if bg != BG:
                draw.rectangle([px, py, px + cell_w, py + cell_h], fill=bg)
            use_font = bold_font if cell.bold else font
            if ch.strip() or bg != BG:
                draw.text((px, py), ch, font=use_font, fill=fg)

    img.save(out, "PNG", optimize=True)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--store", default=shutil.which("store") or f"{os.environ.get('TMPDIR', '/tmp')}/store")
    ap.add_argument("--out", default=str(REPO / "docs/hero.png"))
    ap.add_argument("--cols", type=int, default=96)
    ap.add_argument("--rows", type=int, default=36)
    ap.add_argument("--font-size", type=int, default=22)
    args = ap.parse_args()

    if not os.path.isfile(args.store) or not os.access(args.store, os.X_OK):
        print(f"store binary not found or not executable: {args.store}", file=sys.stderr)
        print("  build it with: make build && ./snapshot.py --store ./store", file=sys.stderr)
        return 1

    screen = capture_frame(args.store, args.cols, args.rows)
    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    render_png(screen, out, args.font_size)
    print(f"wrote {out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
