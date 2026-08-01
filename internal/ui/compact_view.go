package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) renderCompactView() string {
	if len(m.Accounts) == 0 {
		return "No accounts.\n"
	}

	accountWidth := m.compactAccountWidth()
	normalRows := make([]int, 0, len(m.Accounts))
	exhaustedRows := make([]int, 0, len(m.Accounts))

	for i, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			exhaustedRows = append(exhaustedRows, i)
			continue
		}
		normalRows = append(normalRows, i)
	}

	var s strings.Builder
	m.renderCompactRows(&s, normalRows, accountWidth)

	if len(exhaustedRows) > 0 {
		if len(normalRows) > 0 {
			s.WriteString("\n")
		}
		s.WriteString(CompactExhaustedHeaderStyle.Render("Exhausted accounts"))
		s.WriteString("\n")
		m.renderCompactRows(&s, exhaustedRows, accountWidth)
	}

	return s.String()
}

func (m Model) renderCompactRows(s *strings.Builder, rowIndexes []int, accountWidth int) {
	limit := m.preferredContentWidth()
	if limit <= 0 && m.Width > 0 {
		limit = m.Width
	}
	for _, i := range rowIndexes {
		if i < 0 || i >= len(m.Accounts) {
			continue
		}
		acc := m.Accounts[i]
		if acc == nil {
			continue
		}
		row := m.renderCompactAccountRow(i, acc, accountWidth)
		// Guard against style-induced line wraps on very narrow terminals.
		row = strings.ReplaceAll(row, "\n", " ")
		if limit > 0 && ansi.StringWidth(row) > limit {
			row = ansi.Cut(row, 0, limit)
		}
		s.WriteString(row)
		s.WriteString("\n")
	}
}

func (m Model) renderCompactAccountRow(index int, acc *config.Account, accountWidth int) string {
	var s strings.Builder
	isActive := index == m.ActiveAccountIx
	prefix := "  "
	if isActive {
		prefix = "> "
	}

	name := acc.Label
	if name == "" {
		name = acc.SourceLabel()
	}
	subscribed := m.hasSubscription(acc)
	badgeWidth := m.activeSourceBadgesDisplayWidth(acc)
	nameWidth := accountWidth
	if badgeWidth > 0 {
		nameWidth = accountWidth - badgeWidth - 1
		if nameWidth < 4 {
			nameWidth = 4
		}
	}
	name = truncateLabel(name, nameWidth-1)
	alignedName := fmt.Sprintf("%-*s", nameWidth, name)
	leftWidth := ansi.StringWidth(prefix) + nameWidth + 1
	if badgeWidth > 0 {
		leftWidth += badgeWidth + 1
	}
	barWidth, percentWidth, resetWidth := m.compactRowLayout(leftWidth)

	s.WriteString(prefix)
	if badgeWidth > 0 {
		s.WriteString(m.renderActiveSourceBadges(acc, isActive))
		s.WriteString(" ")
	}
	if subscribed && isActive {
		s.WriteString(SubscribedLabelActiveStyle.Render(alignedName))
	} else if subscribed {
		s.WriteString(SubscribedLabelMutedStyle.Render(alignedName))
	} else if isActive {
		s.WriteString(TabActiveStyle.Render(alignedName))
	} else {
		s.WriteString(LabelStyle.Render(alignedName))
	}
	s.WriteString(" ")

	if err := m.ErrorsMap[acc.Key]; err != nil {
		status := truncateLabel("Error: "+err.Error(), 24)
		s.WriteString(m.renderCompactStatusRow(status, subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}
	if m.LoadingMap[acc.Key] && !m.silentRefresh[acc.Key] {
		s.WriteString(m.renderCompactStatusRow("Loading...", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	data, ok := m.UsageData[acc.Key]
	if !ok {
		s.WriteString(m.renderCompactStatusRow("Queued...", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	window, ok := compactPrimaryWindow(data)
	if !ok {
		s.WriteString(m.renderCompactStatusRow("No quota data", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	ratio := m.compactBarRatio(acc.Key, clampRatio(window.LeftPercent/100))
	s.WriteString(renderSmoothBar(barWidth, ratio, defaultBarGradientStart, defaultBarGradientEnd))
	s.WriteString(" ")
	s.WriteString(m.renderCompactPercent(fmt.Sprintf("%.0f%%", window.LeftPercent), subscribed, percentWidth))
	reset := truncateLabelStrict(formatResetText(window.ResetAt), resetWidth)
	if resetWidth > 0 && strings.TrimSpace(reset) != "" {
		s.WriteString(ResetTimeStyle.Copy().Width(resetWidth).Render(reset))
	}
	return s.String()
}

func (m Model) isCompactAccountExhausted(accountKey string) bool {
	if accountKey == "" {
		return false
	}
	if m.ExhaustedSticky[accountKey] {
		return true
	}
	if m.LoadingMap[accountKey] && !m.silentRefresh[accountKey] {
		return false
	}
	if err := m.ErrorsMap[accountKey]; err != nil {
		return false
	}

	data, ok := m.UsageData[accountKey]
	if !ok {
		return false
	}
	return isConfirmedExhausted(data)
}

func (m Model) renderCompactStatusRow(status string, subscribed bool, barWidth, percentWidth, resetWidth int) string {
	row := renderSmoothBar(barWidth, 0, defaultBarGradientStart, defaultBarGradientEnd)
	row += " "
	row += m.renderCompactPercent("...", subscribed, percentWidth)
	if resetWidth > 0 {
		row += ResetTimeStyle.Copy().Width(resetWidth).Render(truncateLabelStrict(status, resetWidth))
	}
	return TabInactiveStyle.Render(row)
}

func (m Model) renderCompactPercent(value string, subscribed bool, width int) string {
	value = truncateLabelStrict(value, width)
	style := PercentStyle.Copy().Width(width)
	if !subscribed {
		return style.Render(value)
	}

	return style.Copy().Foreground(lipgloss.Color("177")).Render(value)
}

func (m Model) compactAccountWidth() int {
	width := m.Width
	if width <= 0 {
		width = m.preferredContentWidth()
	}
	switch {
	case width >= 140:
		return 30
	case width >= 120:
		return 24
	case width >= 100:
		return 20
	case width >= 84:
		return 16
	case width >= 72:
		return 18
	default:
		return 12
	}
}

func (m Model) compactRowLayout(leftWidth int) (barWidth, percentWidth, resetWidth int) {
	barWidth = m.defaultBarWidth()
	percentWidth = 5
	resetWidth = 26

	available := m.preferredContentWidth() - leftWidth
	if available <= 0 {
		return 6, 3, 0
	}

	const (
		minBarWidth     = 6
		minPercentWidth = 4
		minResetWidth   = 0
		gapWidth        = 1
		resetMarginLeft = 2
	)

	used := barWidth + gapWidth + percentWidth + resetMarginLeft + resetWidth
	shortage := used - available
	if shortage <= 0 {
		return
	}

	reduce := func(current, minimum int) int {
		if shortage <= 0 {
			return current
		}
		can := current - minimum
		if can <= 0 {
			return current
		}
		if can > shortage {
			can = shortage
		}
		shortage -= can
		return current - can
	}

	barWidth = reduce(barWidth, minBarWidth)
	resetWidth = reduce(resetWidth, minResetWidth)
	percentWidth = reduce(percentWidth, minPercentWidth)

	return
}
