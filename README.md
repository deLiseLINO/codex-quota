# CQ (Codex Quota)

A TUI for switching between Codex accounts and monitoring quota usage, written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Screenshot](demo-screenshot.png)

![Demo](demo.gif)

## Features

- Fast account switching across many accounts
- Multi-target apply: set the active credential for Codex, OpenCode, and Pi agent, plus the one active Oh My Pi (OMP) account
- Accounts loaded from local app storage, OpenCode auth, Codex auth, Pi agent auth (`~/.pi/agent/auth.json`), and OMP's active account (`~/.omp/agent/agent.db`)
- OAuth authentication via browser
- Configurable auto-refresh: the active account refreshes on a short interval, the rest on a longer background interval
- Two view modes: compact for many accounts, tabs for focused viewing when you have just a few.
## Installation

Homebrew:

```bash
brew install deLiseLINO/tap/codex-quota
```

Go install:

```bash
go install github.com/deLiseLINO/codex-quota/cmd/cq@latest
```

**Note:** Make sure your Go bin directory is available in `PATH`.

<details>
<summary>Build from source</summary>

```bash
git clone https://github.com/deLiseLINO/codex-quota.git
cd codex-quota
go install ./cmd/cq
```

</details>

## Usage

Run the app:

```bash
cq
```

Typical flow:

1. Press `n` to add/import account via OAuth.
2. Move between accounts with arrows.
3. Press `Enter` to open the actions menu for the active account and app-level actions.
4. Press `o` to apply the active account to selected apps (Codex/OpenCode/Pi/the active OMP account).
5. Use `r`/`R` to refresh quota and `?` for grouped keyboard help.

> **OMP ownership:** Selecting OMP gives CQ exclusive control of its Codex credential: it keeps one active OMP account and removes the other OMP Codex rows. Your managed account copies remain in CQ for later switching.

## Controls

- `↑` `↓` `←` `→` — both work for navigation; the UI highlights `↑/↓` in compact view and `←/→` in tabs view
- `Enter` — open actions menu for account and app-level actions
- `r` — refresh active account
- `R` — refresh all accounts
- `v` — switch view mode (also available via actions menu)
- `s` — open settings (auto-refresh intervals, update check)
- `?` — open grouped keyboard help
- `q` / `Ctrl+C` — quit

Additional shortcuts:

- `h` `j` `k` `l` — Vim-style navigation
- `o` — apply active account to targets (Codex, OpenCode, Pi, active OMP account)
- `i` — toggle additional info
- `n` — add account (OAuth)
- `x` — delete active account
- `u` — open update prompt when an update is available
- `Esc` — close modal/info/error/notice (or quit if nothing is open)
