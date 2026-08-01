# Codex Quota

Codex Quota (cq) is a terminal account switcher and quota monitor for Codex/OpenCode. It tracks usage limits across OpenAI accounts and lets the user switch between them.

## Language

**Active account**:
The account currently selected in the UI. Its data is visible and it refreshes on the shorter auto-refresh interval.
_Avoid_: current account, selected account, focused account

**Auto-refresh**:
Periodic, unattended reload of account usage data. The active account reloads on the active interval; all other accounts reload on the background interval.
_Avoid_: polling, auto-update, background refresh

**Plan type**:
An account's subscription tier (e.g. `free`, `plus`, `pro`). Persisted between runs so paid-account indicators show immediately at startup.
_Avoid_: plan, tier, subscription status
