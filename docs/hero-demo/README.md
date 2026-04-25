# hero-demo

The fixtures behind `docs/hero.png`. Everything here is scaffolding for the
README screenshot — not part of the `store` binary.

## What it is

A small dotfiles repo laid out to exercise every feature the hero shot
needs in a single TUI frame:

- **Nested groups** organized as `editors/`, `terminals/`, `tools/`, so
  the ledger has a real tree to render. The hero frame catches one open
  group (`terminals`) alongside two collapsed groups (`editors`, `tools`)
  so both the `+` and `−` glyphs are visible.
- **Mixed ledger states** across the leaves: 4 linked, 1 partial, 1
  missing, 1 conflict. Group rows aggregate worst-child state, so
  `terminals` reports `partial` (one linked, one missing) while
  `editors` and `tools` show `linked`.
- **Multi-target store** (`shells`) sitting at the top level so its
  detail pane is what you see when the cursor lands on it. Per-target
  link counts come out as `2/2 linked`, `1/2 partial`, `0/1 missing`.
- **Hooks block** on the selected store (`pre` + `post`).
- **Platform filter** (`when: os: linux`) so the `filter matches` line
  appears in the detail pane.
- **Templates** — `.zshrc` references one secret and two user vars,
  driving the `templates  1 secret · 2 vars` line.
- **Conflict** — `work` has `when: hostname: work-laptop` and a real
  directory at the target path so it renders as `✕ conflict`.

## Layout

```text
hero-demo/
  .store/config.yaml             # nested config, committed to the repo
  editors/
    nvim/                        # linked (whole-directory)
  shells/                        # multi-target; partial
    .zshrc .bashrc               # targeted at ~
    config.fish                  # targeted at ~/.config/fish
    aliases.fish                 #   (intentionally not linked)
    config.nu                    # targeted at ~/.config/nushell (not linked)
  terminals/
    ghostty/                     # linked (whole-directory)
    tmux/                        # missing (no symlinks created)
  tools/
    git/                         # linked (whole-directory)
    lazygit/                     # linked (whole-directory)
  work/                          # conflict (real dir at target path)
  capture.sh                     # launch the live TUI in the demo state
  snapshot.py                    # regenerate docs/hero.png
```

## See the demo live

```sh
$ make build && mv store ~/.local/bin/     # or wherever's on your PATH
$ ./capture.sh                             # opens store tui in an ephemeral HOME
```

`capture.sh` builds `$TMPDIR/store-hero-demo/home` with the exact mix of
symlinks, missing targets, and a conflicting directory that the hero
image shows, then exec's `store tui` inside it. Your real home is never
touched. Press `q` to quit.

Inside the TUI, press `j j l k`. The cursor walks down to the
`terminals` group, expands it (`l`), and steps back up to `shells`. That
matches the frame in `docs/hero.png`.

## Regenerate the PNG

```sh
$ make build
$ python3 -m venv .venv && .venv/bin/pip install pyte pillow
$ .venv/bin/python snapshot.py --store ./store --out ../hero.png
```

The script spawns `store tui` in a pty, drives `j j l k` after the initial
render to expand `terminals` and land the cursor on `shells`, captures
the final frame with [pyte](https://github.com/selectel/pyte), and
rasterises it with Pillow using a palette pulled from
`internal/tui/theme.go`.

Flags:

| Flag          | Default                            | Notes                                       |
| ------------- | ---------------------------------- | ------------------------------------------- |
| `--store`     | `$(which store)` or `$TMPDIR/store`| Path to the `store` binary                  |
| `--out`       | `docs/hero.png`                    | Output PNG                                  |
| `--cols`      | `96`                               | Terminal width in cells                     |
| `--rows`      | `36`                               | Terminal height in cells                    |
| `--font-size` | `22`                               | Pixel size passed to Pillow                 |

## Why the initial wait

`snapshot.py` idles for ~6.5 seconds before pressing any keys. That's
because `termenv` probes the terminal for truecolor support on startup
and waits up to 5 seconds for a response over stdin. We let it time out
rather than forging a reply — answering the probe races with bubbletea's
input reader and produced garbled frames in testing.
