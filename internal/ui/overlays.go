package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) currentOverlayModal() string {
	if m.UpdatePromptVisible {
		return m.renderUpdatePromptModal()
	}

	if m.ShowInfo {
		return m.renderInfoModal()
	}

	if m.DeleteSourceSelect {
		return m.renderDeleteSourceModal()
	}

	if m.DeleteConfirm {
		return m.renderDeleteConfirmModal()
	}

	if m.ApplyTargetSelect {
		return m.renderApplyTargetModal()
	}

	if m.ApplyConfirm {
		return m.renderApplyConfirmModal()
	}

	if m.Err != nil {
		return renderMessageModal("Error", m.Err.Error(), ErrorStyle, m.Width)
	}

	if m.Notice != "" {
		return renderMessageModal("Notice", m.Notice, NoticeStyle, m.Width)
	}

	if m.activeAccount() == nil {
		return renderMessageModal("No accounts", "No accounts loaded.\nPress n to add account.", WarningStyle, m.Width)
	}

	return ""
}

const (
	messageModalMinWidth = 64
	messageModalMaxWidth = 104
	messageModalInset    = 6
)

func renderMessageModal(title, message string, titleStyle lipgloss.Style, viewportWidth int) string {
	message = strings.TrimSpace(message)
	width := messageModalWidth(title, message, viewportWidth)
	bodyWidth := width - 2 // account for InfoBoxStyle horizontal padding
	if bodyWidth < 1 {
		bodyWidth = 1
	}
	wrappedMessage := lipgloss.NewStyle().Width(bodyWidth).Render(message)
	content := strings.Join([]string{
		titleStyle.Render(title),
		InfoValueStyle.Render(wrappedMessage),
	}, "\n\n")
	return InfoBoxStyle.Copy().Width(width).Render(content)
}

func messageModalWidth(title, message string, viewportWidth int) int {
	target := messageModalMinWidth
	for _, line := range strings.Split(strings.TrimSpace(message), "\n") {
		width := ansi.StringWidth(line) + 2
		if width > target {
			target = width
		}
	}
	if titleWidth := ansi.StringWidth(title) + 2; titleWidth > target {
		target = titleWidth
	}
	if target > messageModalMaxWidth {
		target = messageModalMaxWidth
	}
	if viewportWidth <= 0 {
		return target
	}

	maxAllowed := viewportWidth - messageModalInset - 2 // border width
	if maxAllowed <= 0 {
		return messageModalMinWidth
	}
	if maxAllowed < messageModalMinWidth {
		return maxAllowed
	}
	if target > maxAllowed {
		return maxAllowed
	}
	return target
}

func (m Model) renderDeleteSourceModal() string {
	lines := []string{
		WarningStyle.Render("Delete account"),
		InfoValueStyle.Render("Select sources to delete:"),
	}

	for i, source := range m.DeleteSourceOptions {
		cursor := " "
		if i == m.DeleteSourceCursor {
			cursor = ">"
		}
		mark := " "
		if m.isDeleteSourceSelected(source) {
			mark = "x"
		}
		lines = append(lines, InfoValueStyle.Render(fmt.Sprintf("%s [%d] [%s] %s", cursor, i+1, mark, sourceDisplayName(source))))
	}

	lines = append(lines, "")
	lines = append(lines, InfoValueStyle.Render("[↑/↓] Move   [space] Toggle   [enter] Next   [esc] Cancel"))

	content := strings.Join(lines, "\n")
	return InfoBoxStyle.Copy().Width(68).Render(content)
}

func (m Model) renderDeleteConfirmModal() string {
	lines := []string{
		WarningStyle.Render("Delete account"),
		InfoValueStyle.Render(fmt.Sprintf("Sources: %s", sourceListText(m.selectedDeleteSources()))),
	}
	lines = append(lines, InfoValueStyle.Render("[enter] Confirm   [esc] Cancel"))

	content := strings.Join(lines, "\n")
	return InfoBoxStyle.Copy().Width(68).Render(content)
}

func (m Model) renderApplyTargetModal() string {
	targets := applyTargetsOrdered()

	lines := []string{
		WarningStyle.Render("Apply account"),
		InfoValueStyle.Render("Select targets to apply:"),
	}

	for i, target := range targets {
		cursor := " "
		if i == m.ApplyTargetCursor {
			cursor = ">"
		}
		mark := " "
		if m.ApplyTargets != nil && m.ApplyTargets[target] {
			mark = "x"
		}
		label := "Codex app/cli"
		if target == config.SourceOpenCode {
			label = "OpenCode"
		}
		lines = append(lines, InfoValueStyle.Render(fmt.Sprintf("%s [%d] [%s] %s", cursor, i+1, mark, label)))
	}

	lines = append(lines, "")
	lines = append(lines, InfoValueStyle.Render("[↑/↓] Move   [space] Toggle   [enter] Next   [esc] Cancel"))

	content := strings.Join(lines, "\n")
	return InfoBoxStyle.Copy().Width(68).Render(content)
}

func (m Model) renderApplyConfirmModal() string {
	selected := m.selectedApplyTargets()
	targetLabel := sourceListText(selected)
	if len(selected) == 0 {
		targetLabel = "codex, opencode"
	}

	return renderMessageModal(
		"Apply account",
		fmt.Sprintf("Apply this account to: %s?\n[enter] Confirm   [esc] Cancel", targetLabel),
		WarningStyle,
		m.Width,
	)
}

func (m Model) renderInfoModal() string {
	account := m.activeAccount()

	email := "n/a"
	accountID := "n/a"
	source := "n/a"
	if account != nil {
		if account.Email != "" {
			email = account.Email
		}
		if account.AccountID != "" {
			accountID = account.AccountID
		}
		source = account.SourceLabel()
		if m.SourcesByAccountID != nil {
			if account.AccountID != "" {
				if sources := m.SourcesByAccountID[account.AccountID]; len(sources) > 0 {
					source = strings.Join(sources, ", ")
				}
			}
			if source == account.SourceLabel() && account.Email != "" {
				emailKey := "email:" + strings.ToLower(strings.TrimSpace(account.Email))
				if sources := m.SourcesByAccountID[emailKey]; len(sources) > 0 {
					source = strings.Join(sources, ", ")
				}
			}
		}
	}

	plan := m.Data.PlanType
	if plan == "" {
		plan = "n/a"
	}

	allowed := "n/a"
	limitReached := "n/a"
	if m.Data.PlanType != "" || len(m.Data.Windows) > 0 {
		allowed = boolText(m.Data.Allowed)
		limitReached = boolText(m.Data.LimitReached)
	}

	lines := []string{
		InfoTitleStyle.Render("Additional info"),
		fmt.Sprintf("%s %s", InfoKeyStyle.Render("email:"), InfoValueStyle.Render(email)),
		fmt.Sprintf("%s %s", InfoKeyStyle.Render("account_id:"), InfoValueStyle.Render(accountID)),
		fmt.Sprintf("%s %s", InfoKeyStyle.Render("source:"), InfoValueStyle.Render(source)),
		fmt.Sprintf("%s %s", InfoKeyStyle.Render("plan_type:"), InfoValueStyle.Render(plan)),
		fmt.Sprintf("%s %s", InfoKeyStyle.Render("allowed:"), InfoValueStyle.Render(allowed)),
		fmt.Sprintf("%s %s", InfoKeyStyle.Render("limit_reached:"), InfoValueStyle.Render(limitReached)),
	}

	content := strings.Join(lines, "\n")
	return InfoBoxStyle.Copy().Width(60).Render(content)
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func overlayCenter(base, modal string, width, height int) string {
	canvasWidth := width
	if canvasWidth < lipgloss.Width(base) {
		canvasWidth = lipgloss.Width(base)
	}
	if canvasWidth < lipgloss.Width(modal)+2 {
		canvasWidth = lipgloss.Width(modal) + 2
	}

	canvasHeight := height
	if canvasHeight < lipgloss.Height(base) {
		canvasHeight = lipgloss.Height(base)
	}
	if canvasHeight < lipgloss.Height(modal)+2 {
		canvasHeight = lipgloss.Height(modal) + 2
	}

	baseCanvas := lipgloss.Place(canvasWidth, canvasHeight, lipgloss.Left, lipgloss.Top, base)
	baseLines := strings.Split(baseCanvas, "\n")
	modalLines := strings.Split(modal, "\n")

	modalWidth := lipgloss.Width(modal)
	modalHeight := len(modalLines)
	startX := (canvasWidth - modalWidth) / 2
	if startX < 0 {
		startX = 0
	}
	startY := (canvasHeight - modalHeight) / 2
	if startY < 0 {
		startY = 0
	}

	for i, modalLine := range modalLines {
		y := startY + i
		if y < 0 || y >= len(baseLines) {
			continue
		}

		line := padANSI(baseLines[y], canvasWidth)
		modalLine = padANSI(modalLine, modalWidth)

		left := ansi.Cut(line, 0, startX)
		right := ansi.Cut(line, startX+modalWidth, canvasWidth)
		baseLines[y] = left + modalLine + right
	}

	return strings.Join(baseLines, "\n")
}

func padANSI(line string, targetWidth int) string {
	currentWidth := ansi.StringWidth(line)
	if currentWidth >= targetWidth {
		return line
	}
	return line + strings.Repeat(" ", targetWidth-currentWidth)
}
