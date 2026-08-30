# CQ (Codex Quota)

A TUI for switching between Codex accounts and monitoring quota usage, written in Go using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

![Screenshot](demo-screenshot.png)

![Demo](demo.gif)

## Features

- Fast account switching across many accounts
- Multi-target apply: set the active credential for installed Codex, OpenCode, Pi, and OMP harnesses
- Accounts loaded from CQ storage plus the external Codex, OpenCode, Pi (`~/.pi/agent/auth.json`), and active-profile OMP (`~/.omp/agent/agent.db`) stores
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
4. Press `o` to apply the active account to selected installed apps.
5. Use `r`/`R` to refresh quota and `?` for grouped keyboard help.

> **External OMP modes:** Applying an account to OMP switches its external credential store to **exclusive mode**: exactly that one OpenAI Codex credential remains. CQ's managed copies are not deleted.
> Choose **Restore all accounts to OMP pool** to switch the external OMP store back to **pool mode**: every eligible CQ-managed account is mirrored into OMP for native multi-account auto-balancing. Applying one account to OMP later returns the external store to exclusive mode.
> CQ remembers your last confirmed Apply target choice for the next launch; canceling the Apply modal does not change it.

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
- `o` — apply active account to installed targets
- `i` — toggle additional info
- `n` — add account (OAuth)
- `x` — delete active account
- `u` — open update prompt when an update is available
- `Esc` — close modal/info/error/notice (or quit if nothing is open)
