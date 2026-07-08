package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/update"
	"github.com/deLiseLINO/codex-quota/internal/version"
)

const (
	updatePromptChoiceNow = iota
	updatePromptChoiceSkip
	updatePromptChoiceDismiss
	updatePromptChoiceCount

	updatePromptModalMinWidth = 72
	updatePromptModalMaxWidth = 104
	updatePromptViewportInset = 6
	updatePromptBorderWidth   = 2
)

func (m Model) handleUpdatePrompt(msg tea.KeyMsg, keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.UpdatePromptCursor = (m.UpdatePromptCursor - 1 + updatePromptChoiceCount) % updatePromptChoiceCount
		return m, nil
	case "down", "j":
		m.UpdatePromptCursor = (m.UpdatePromptCursor + 1) % updatePromptChoiceCount
		return m, nil
	case "1":
		m.UpdatePromptCursor = updatePromptChoiceNow
		return m.confirmUpdatePrompt()
	case "2":
		m.UpdatePromptCursor = updatePromptChoiceSkip
		return m.confirmUpdatePrompt()
	case "3":
		m.UpdatePromptCursor = updatePromptChoiceDismiss
		return m.confirmUpdatePrompt()
	case "esc":
		m.UpdatePromptCursor = updatePromptChoiceSkip
		return m.confirmUpdatePrompt()
	case "enter":
		return m.confirmUpdatePrompt()
	}

	return m, nil
}

func (m Model) confirmUpdatePrompt() (tea.Model, tea.Cmd) {
	switch m.UpdatePromptCursor {
	case updatePromptChoiceNow:
		m.UpdatePromptVisible = false
		m.UpdateAvailableHint = ""
		m.pendingUpdateMethod = m.UpdatePromptMethod
		m.hasPendingUpdateMethod = true
		return m, tea.Quit
	case updatePromptChoiceDismiss:
		m.UpdatePromptVisible = false
		m.UpdateAvailableHint = ""
		return m, DismissUpdateVersionCmd(m.UpdatePromptVersion)
	default:
		m.UpdatePromptVisible = false
		if strings.TrimSpace(m.UpdatePromptVersion) != "" && update.SupportsAutoUpdate(m.UpdatePromptMethod) {
			m.UpdateAvailableHint = "Update available • press u"
		}
		return m, nil
	}
}

func (m Model) renderUpdatePromptModal() string {
	command := update.CommandString(m.UpdatePromptMethod, m.UpdatePromptVersion)
	lines := []string{
		UpdateHintStyle.Render("Update available"),
		InfoValueStyle.Render(fmt.Sprintf("%s -> %s", version.Current(), m.UpdatePromptVersion)),
		"",
		InfoValueStyle.Render("Release notes: " + update.ReleaseNotesURL(m.UpdatePromptVersion)),
		"",
		renderUpdatePromptOption(1, "Update now", command, m.UpdatePromptCursor == updatePromptChoiceNow),
		renderUpdatePromptOption(2, "Skip", "", m.UpdatePromptCursor == updatePromptChoiceSkip),
		renderUpdatePromptOption(3, "Skip until next version", "", m.UpdatePromptCursor == updatePromptChoiceDismiss),
		"",
		InfoValueStyle.Render("[↑/↓] Move   [enter] Select   [esc] Skip"),
	}
	return InfoBoxStyle.Copy().Width(m.updatePromptModalWidth(lines)).Render(strings.Join(lines, "\n"))
}

func renderUpdatePromptOption(index int, label, command string, selected bool) string {
	cursor := " "
	if selected {
		cursor = ">"
	}
	if strings.TrimSpace(command) != "" {
		label = fmt.Sprintf("%s (runs `%s`)", label, command)
	}
	return InfoValueStyle.Render(fmt.Sprintf("%s %d. %s", cursor, index, label))
}

func (m Model) updatePromptModalWidth(lines []string) int {
	target := updatePromptModalMinWidth
	for _, line := range lines {
		width := ansi.StringWidth(line) + 2
		if width > target {
			target = width
		}
	}
	if target > updatePromptModalMaxWidth {
		target = updatePromptModalMaxWidth
	}

	if m.Width <= 0 {
		return target
	}

	maxAllowed := m.Width - updatePromptViewportInset - updatePromptBorderWidth
	if maxAllowed <= 0 {
		return updatePromptModalMinWidth
	}
	if maxAllowed < updatePromptModalMinWidth {
		return maxAllowed
	}
	if target > maxAllowed {
		return maxAllowed
	}
	return target
}
