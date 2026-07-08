package ui

import (
	"sort"
	"strings"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m *Model) setKnownPlanType(accountKey string, planType string) {
	if accountKey == "" {
		return
	}
	normalized := strings.ToLower(strings.TrimSpace(planType))
	if normalized == "" {
		return
	}
	if m.PlanTypeByAccount == nil {
		m.PlanTypeByAccount = make(map[string]string)
	}
	m.PlanTypeByAccount[accountKey] = normalized
}

func (m Model) isPaidByKnownPlan(accountKey string) bool {
	if accountKey == "" || m.PlanTypeByAccount == nil {
		return false
	}
	planType := strings.ToLower(strings.TrimSpace(m.PlanTypeByAccount[accountKey]))
	if planType == "" {
		return false
	}
	return planType != "free"
}

func (m *Model) pruneKnownPlanTypes() {
	if len(m.PlanTypeByAccount) == 0 {
		return
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		valid[acc.Key] = struct{}{}
	}
	for key := range m.PlanTypeByAccount {
		if _, ok := valid[key]; !ok {
			delete(m.PlanTypeByAccount, key)
		}
	}
}

func (m *Model) setExhaustedStickyIfConfirmed(accountKey string, data api.UsageData) bool {
	if accountKey == "" {
		return false
	}
	if m.ExhaustedSticky == nil {
		m.ExhaustedSticky = make(map[string]bool)
	}

	if isConfirmedExhausted(data) {
		if m.ExhaustedSticky[accountKey] {
			return false
		}
		m.ExhaustedSticky[accountKey] = true
		return true
	}

	if isConfirmedNonExhausted(data) && m.ExhaustedSticky[accountKey] {
		delete(m.ExhaustedSticky, accountKey)
		return true
	}

	return false
}

func (m *Model) pruneExhaustedSticky() bool {
	if len(m.ExhaustedSticky) == 0 {
		return false
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		valid[acc.Key] = struct{}{}
	}

	changed := false
	for key := range m.ExhaustedSticky {
		if _, ok := valid[key]; ok {
			continue
		}
		delete(m.ExhaustedSticky, key)
		changed = true
	}
	return changed
}

func (m Model) exhaustedStickyKeys() []string {
	if len(m.ExhaustedSticky) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m.ExhaustedSticky))
	for key, exhausted := range m.ExhaustedSticky {
		if !exhausted || strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m Model) uiStateSnapshot() config.UIState {
	activeKey := ""
	if account := m.activeAccount(); account != nil {
		activeKey = account.Key
	}
	return config.UIState{
		CompactMode:          m.CompactMode,
		ExhaustedAccountKeys: m.exhaustedStickyKeys(),
		ActiveAccountKey:     activeKey,
	}
}
