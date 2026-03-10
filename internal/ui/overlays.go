package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) currentOverlayModal() string {
	if m.UpdatePromptVisible {
		return m.renderUpdatePromptModal()
	}

	if m.HelpVisible {
		return m.renderHelpModal()
	}

	if m.AddAccountLoginVisible {
		return m.renderAddAccountLoginModal()
	}

	if m.ActionMenuVisible {
		return m.renderActionMenuModal()
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

func (m Model) renderHelpModal() string {
	primaryMove := "←/→"
	if m.CompactMode {
		primaryMove = "↑/↓"
	}

	lines := []string{
		InfoTitleStyle.Render("Keyboard help"),
		"",
		HelpSectionStyle.Render("Primary"),
		renderHelpLine(primaryMove, "Move between accounts"),
		renderHelpLine("Enter", "Open account menu"),
		renderHelpLine("o", "Apply to Codex/OpenCode"),
		renderHelpLine("r", "Refresh active account"),
		renderHelpLine("R", "Refresh all accounts"),
		renderHelpLine("?", "Open or close this help"),
		renderHelpLine("q", "Quit"),
		"",
	}
	return InfoBoxStyle.Copy().Width(56).Render(strings.Join(lines, "\n"))
}

func (m Model) renderAddAccountLoginModal() string {
	layout := m.addAccountLoginModalLayout()
	return InfoBoxStyle.Copy().Width(layout.Width).Render(strings.Join(layout.Lines, "\n"))
}

type addAccountLoginModalLayout struct {
	Lines        []string
	Width        int
	URLStartLine int
	URLEndLine   int
}

func (m Model) addAccountLoginModalLayout() addAccountLoginModalLayout {
	lines := []string{
		InfoTitleStyle.Render("Connect ChatGPT account"),
		"",
		InfoValueStyle.Render("Complete authorization in your browser. This window will close automatically after login."),
		"",
		InfoValueStyle.Render("If your browser did not open, open this URL manually:"),
	}

	bodyWidth := 78
	if m.Width > 0 && m.Width < 96 {
		bodyWidth = m.Width - 18
	}
	if bodyWidth < 24 {
		bodyWidth = 24
	}
	renderedURL := lipgloss.NewStyle().
		Width(bodyWidth).
		Foreground(lipgloss.Color("39")).
		Render(displayAuthURL(m.AddAccountLoginURL))
	urlStartLine := len(lines)
	lines = append(lines, renderedURL)
	urlEndLine := urlStartLine + lipgloss.Height(renderedURL) - 1
	lines = append(lines, "")
	if m.AddAccountBrowserFailed {
		lines = append(lines, NoticeStyle.Render("Browser did not open automatically."))
		lines = append(lines, "")
	}
	lines = append(lines, InfoValueStyle.Render("Waiting for authorization..."))
	if strings.TrimSpace(m.AddAccountLoginStatus) != "" {
		lines = append(lines, NoticeStyle.Render(m.AddAccountLoginStatus))
	}
	lines = append(lines, "")
	lines = append(lines, ActionMenuHintStyle.Render("[c] Copy   [esc] Cancel"))

	width := 84
	for _, line := range lines {
		if w := lipgloss.Width(line) + 2; w > width {
			width = w
		}
	}
	if width > 96 {
		width = 96
	}
	return addAccountLoginModalLayout{
		Lines:        lines,
		Width:        width,
		URLStartLine: urlStartLine,
		URLEndLine:   urlEndLine,
	}
}

func (m Model) addAccountLoginURLContainsPoint(x, y int) bool {
	if !m.AddAccountLoginVisible || strings.TrimSpace(m.AddAccountLoginURL) == "" || m.Width <= 0 || m.Height <= 0 {
		return false
	}

	layout := m.addAccountLoginModalLayout()
	modalHeight := lipgloss.Height(InfoBoxStyle.Copy().Width(layout.Width).Render(strings.Join(layout.Lines, "\n")))
	footerHeight := lipgloss.Height(HelpStyle.Render("\n" + m.renderFooter()))
	bodyHeight := m.Height - footerHeight
	if bodyHeight < modalHeight+2 {
		bodyHeight = modalHeight + 2
	}
	startX := (m.Width - layout.Width) / 2
	if startX < 0 {
		startX = 0
	}
	startY := (bodyHeight - modalHeight) / 2
	if startY < 0 {
		startY = 0
	}

	// InfoBoxStyle uses a border plus left padding of one cell, so URL text starts at +2.
	urlTextX := startX + 2
	urlTextY := startY + 1 + layout.URLStartLine
	urlTextWidth := layout.Width - 2
	urlTextHeight := layout.URLEndLine - layout.URLStartLine + 1

	return x >= urlTextX && x < urlTextX+urlTextWidth && y >= urlTextY && y < urlTextY+urlTextHeight
}

func renderHelpLine(key, description string) string {
	return fmt.Sprintf("%s %s", HelpKeyStyle.Render(fmt.Sprintf("%-10s", key)), InfoValueStyle.Render(description))
}

func (m Model) renderActionMenuModal() string {
	sections := m.actionMenuSections()
	lines := []string{
		ActionMenuTitleStyle.Render("Account actions"),
	}

	if account := m.activeAccount(); account != nil {
		label := account.Label
		if strings.TrimSpace(label) == "" {
			label = account.SourceLabel()
		}
		lines = append(lines, InfoValueStyle.Render(truncateLabel(label, 44)))
	}
	lines = append(lines, "")

	labelWidth := actionMenuLabelWidth(sections)
	index := 0
	for sectionIx, section := range sections {
		if strings.TrimSpace(section.Title) != "" {
			lines = append(lines, HelpSectionStyle.Render(section.Title))
		}
		for _, item := range section.Items {
			cursor := " "
			style := ActionMenuItemStyle
			if index == m.ActionMenuCursor {
				cursor = ">"
				style = ActionMenuSelectedStyle
			}
			line := fmt.Sprintf("%s %d. %-*s %s", cursor, index+1, labelWidth, item.Label, item.Shortcut)
			lines = append(lines, style.Render(line))
			index++
		}
		if sectionIx < len(sections)-1 {
			lines = append(lines, "")
		}
	}

	lines = append(lines, "")
	lines = append(lines, ActionMenuHintStyle.Render("[↑/↓] Move   [enter] Select   [esc] Close"))

	return InfoBoxStyle.Copy().Width(actionMenuModalWidth(lines)).Render(strings.Join(lines, "\n"))
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func displayAuthURL(value string) string {
	replacer := strings.NewReplacer(
		"://", ":\u200b//",
		"/", "/\u200b",
		"?", "?\u200b",
		"&", "&\u200b",
		"=", "=\u200b",
	)
	return replacer.Replace(strings.TrimSpace(value))
}
