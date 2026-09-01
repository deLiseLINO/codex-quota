package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) activeAccount() *config.Account {
	if len(m.Accounts) == 0 {
		return nil
	}
	if m.ActiveAccountIx < 0 || m.ActiveAccountIx >= len(m.Accounts) {
		return nil
	}
	return m.Accounts[m.ActiveAccountIx]
}

func (m Model) activeAccountKey() string {
	account := m.activeAccount()
	if account == nil {
		return ""
	}
	return account.Key
}

func (m Model) compactVisualOrderIndices() []int {
	if len(m.Accounts) == 0 {
		return nil
	}

	normal := make([]int, 0, len(m.Accounts))
	exhausted := make([]int, 0, len(m.Accounts))
	for i, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			exhausted = append(exhausted, i)
		} else {
			normal = append(normal, i)
		}
	}
	return append(normal, exhausted...)
}

func (m *Model) moveActiveAccountCompact(delta int) {
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return
	}

	pos := -1
	for i, idx := range order {
		if idx == m.ActiveAccountIx {
			pos = i
			break
		}
	}
	if pos == -1 {
		m.ActiveAccountIx = order[0]
		return
	}

	next := (pos + delta) % len(order)
	if next < 0 {
		next += len(order)
	}
	m.ActiveAccountIx = order[next]
}

func (m *Model) syncActiveAccount() {
	m.Loading = true
	m.Err = nil
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""
	m.clearTabWindowAnimations()

	if acc := m.activeAccount(); acc != nil {
		if data, ok := m.UsageData[acc.Key]; ok {
			m.Data = data
			m.Loading = false
			m.Err = m.ErrorsMap[acc.Key]
			if !m.CompactMode {
				m.startTabWindowAnimationsFromZero(acc.Key, data, tabSwitchAnimationDuration)
			}
			return
		}
	}
	m.Data = api.UsageData{}
}

func (m *Model) normalizeActiveAccountForView(activeKey string) {
	activeKey = strings.TrimSpace(activeKey)
	if len(m.Accounts) == 0 {
		m.ActiveAccountIx = 0
		return
	}

	if activeKey != "" {
		for i, account := range m.Accounts {
			if account != nil && account.Key == activeKey {
				m.ActiveAccountIx = i
				return
			}
		}

		// Legacy fallback: older ui_state.json stored the bare workspace UUID as
		// the active key, before per-user composite keys existed. Resolve it to an
		// account whose AccountID matches — but only if exactly one does, so we
		// never arbitrarily pick between two users in the same workspace.
		if idx, ok := uniqueAccountIndexByLegacyKey(m.Accounts, activeKey); ok {
			m.ActiveAccountIx = idx
			return
		}
	}

	if m.CompactMode {
		if order := m.compactVisualOrderIndices(); len(order) > 0 {
			m.ActiveAccountIx = order[0]
			return
		}
	}

	m.ActiveAccountIx = 0
}

// uniqueAccountIndexByLegacyKey resolves a legacy bare-workspace-UUID key to a
// single account. It returns ok=false when no account matches, or when more than
// one account shares that AccountID (two users in one workspace) — the caller
// must not guess between them.
func uniqueAccountIndexByLegacyKey(accounts []*config.Account, legacyKey string) (int, bool) {
	legacyKey = strings.TrimSpace(legacyKey)
	if legacyKey == "" {
		return 0, false
	}
	matchIdx := -1
	for i, account := range accounts {
		if account == nil {
			continue
		}
		if strings.TrimSpace(account.AccountID) == legacyKey {
			if matchIdx != -1 {
				return 0, false
			}
			matchIdx = i
		}
	}
	if matchIdx == -1 {
		return 0, false
	}
	return matchIdx, true
}

func (m *Model) syncAndFetchActiveAccount() tea.Cmd {
	m.syncActiveAccount()
	return tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
}
