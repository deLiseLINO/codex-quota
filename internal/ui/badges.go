package ui

import (
	"strings"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) activeSourceBadgesForAccount(account *config.Account) string {
	if account == nil || len(m.ActiveSourcesByIdentity) == 0 {
		return ""
	}

	hasCodex := false
	hasOpenCode := false
	hasPi := false
	appendLabels := func(labels []string) {
		for _, label := range labels {
			source, ok := sourceFromLabel(label)
			if !ok {
				continue
			}
			if source == config.SourceCodex {
				hasCodex = true
			}
			if source == config.SourceOpenCode {
				hasOpenCode = true
			}
			if source == config.SourcePi {
				hasPi = true
			}
		}
	}

	for _, key := range config.ActiveIdentityKeys(account) {
		appendLabels(m.ActiveSourcesByIdentity[key])
	}

	if !hasCodex && !hasOpenCode && !hasPi {
		return ""
	}

	parts := make([]string, 0, 3)
	if hasCodex {
		parts = append(parts, "C")
	}
	if hasOpenCode {
		parts = append(parts, "O")
	}
	if hasPi {
		parts = append(parts, "P")
	}
	return strings.Join(parts, "•")
}

func (m Model) hasSubscription(account *config.Account) bool {
	if account == nil || account.Key == "" {
		return false
	}
	return m.isPaidByKnownPlan(account.Key)
}

func (m Model) renderActiveSourceBadges(account *config.Account, isRowActive bool) string {
	raw := m.activeSourceBadgesForAccount(account)
	if raw == "" {
		return ""
	}

	cStyle := SourceCodexBadgeMutedStyle
	oStyle := SourceOpenCodeBadgeMutedStyle
	pStyle := SourcePiBadgeMutedStyle
	if isRowActive {
		cStyle = SourceCodexBadgeActiveStyle
		oStyle = SourceOpenCodeBadgeActiveStyle
		pStyle = SourcePiBadgeActiveStyle
	}

	var b strings.Builder
	b.WriteString(SourceBadgeBracketStyle.Render("["))
	for _, r := range raw {
		switch r {
		case 'C':
			b.WriteString(cStyle.Render("C"))
		case 'O':
			b.WriteString(oStyle.Render("O"))
		case 'P':
			b.WriteString(pStyle.Render("P"))
		case '•':
			b.WriteString(SourceBadgeSeparatorStyle.Render("•"))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(SourceBadgeBracketStyle.Render("]"))
	return b.String()
}

func (m Model) activeSourceBadgesDisplayWidth(account *config.Account) int {
	raw := m.activeSourceBadgesForAccount(account)
	if raw == "" {
		return 0
	}
	return len([]rune(raw)) + 2
}
