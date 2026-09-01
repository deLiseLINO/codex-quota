package ui

import (
	"sort"
	"strings"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m *Model) setKnownPlanType(accountKey string, planType string) bool {
	if accountKey == "" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(planType))
	if normalized == "" {
		return false
	}
	if m.PlanTypeByAccount == nil {
		m.PlanTypeByAccount = make(map[string]string)
	}
	if existing := m.PlanTypeByAccount[accountKey]; existing == normalized {
		return false
	}
	m.PlanTypeByAccount[accountKey] = normalized
	return true
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

func pruneKeysForMissingAccounts[V any](mp map[string]V, accounts []*config.Account) bool {
	if len(mp) == 0 {
		return false
	}
	valid := make(map[string]struct{}, len(accounts))
	for _, acc := range accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		valid[acc.Key] = struct{}{}
	}
	changed := false
	for key := range mp {
		if _, ok := valid[key]; ok {
			continue
		}
		delete(mp, key)
		changed = true
	}
	return changed
}

func (m *Model) pruneKnownPlanTypes() {
	pruneKeysForMissingAccounts(m.PlanTypeByAccount, m.Accounts)
}

func (m *Model) pruneExhaustedSticky() bool {
	return pruneKeysForMissingAccounts(m.ExhaustedSticky, m.Accounts)
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
	planTypes := make(map[string]string, len(m.PlanTypeByAccount))
	for key, planType := range m.PlanTypeByAccount {
		planTypes[key] = planType
	}
	return config.UIState{
		CompactMode:          m.CompactMode,
		ExhaustedAccountKeys: m.exhaustedStickyKeys(),
		ActiveAccountKey:     activeKey,
		PlanTypes:            planTypes,
		LastApplyTargets:     m.applyTargetStrings(),
	}
}
