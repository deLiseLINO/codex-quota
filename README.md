# CQ (Codex Quota Monitor)

A TUI for switching between Codex accounts and monitoring quota usage, written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Demo](demo.gif)

## Features

- Fast account switching across many accounts
- Multi-target apply: set active account for Codex and/or OpenCode in one flow
- Accounts from local app storage, OpenCode auth, and Codex auth
- OAuth authentication via browser
- - Two view modes: compact for many accounts, tabs for focused viewing when you have just a few.
## Installation

```bash
go install github.com/deLiseLINO/codex-quota/cmd/cq@latest
```

**Note:** Make sure your Go bin directory is available in `PATH`.

Build from source:

```bash
git clone https://github.com/deLiseLINO/codex-quota.git
cd codex-quota
go install ./cmd/cq
```

## Usage

Run the app:

```bash
cq
```

Typical flow:

1. Press `n` to add/import account via OAuth.
2. Move between accounts with arrows (or `h`/`j`/`k`/`l`).
3. Press `Enter` (or `o`) to apply account to Codex/OpenCode.
4. Use `r`/`R` to refresh quota and `i` for details.

## Controls

- `r` — refresh active account
- `R` — refresh all accounts
- `i` — toggle additional info
- `n` — add account (OAuth)
- `Enter` / `o` — apply active account to codex or opencode
- `x` — delete active account (with source selection)
- `v` — switch view mode (tabs/compact)
- `↑` `↓` `←` `→` (or `h` `j` `k` `l`) — switch active account
- `q` / `Ctrl+C` — quit
- `Esc` — close modal/info/error/notice (or quit if nothing is open)
