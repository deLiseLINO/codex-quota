package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
)

func normalizeKey(key string) string {
	ruToEn := map[rune]rune{
		'й': 'q', 'ц': 'w', 'у': 'e', 'к': 'r', 'е': 't', 'н': 'y', 'г': 'u', 'ш': 'i', 'щ': 'o', 'з': 'p', 'х': '[', 'ъ': ']',
		'ф': 'a', 'ы': 's', 'в': 'd', 'а': 'f', 'п': 'g', 'р': 'h', 'о': 'j', 'л': 'k', 'д': 'l', 'ж': ';', 'э': '\'',
		'я': 'z', 'ч': 'x', 'с': 'c', 'м': 'v', 'и': 'b', 'т': 'n', 'ь': 'm', 'б': ',', 'ю': '.',
		'Й': 'Q', 'Ц': 'W', 'У': 'E', 'К': 'R', 'Е': 'T', 'Н': 'Y', 'Г': 'U', 'Ш': 'I', 'Щ': 'O', 'З': 'P', 'Х': '{', 'Ъ': '}',
		'Ф': 'A', 'Ы': 'S', 'В': 'D', 'А': 'F', 'П': 'G', 'Р': 'H', 'О': 'J', 'Л': 'K', 'Д': 'L', 'Ж': ':', 'Э': '"',
		'Я': 'Z', 'Ч': 'X', 'С': 'C', 'М': 'V', 'И': 'B', 'Т': 'N', 'Ь': 'M', 'Б': '<', 'Ю': '>',
	}

	if len(key) > 0 {
		runes := []rune(key)
		if len(runes) == 1 {
			if en, ok := ruToEn[runes[0]]; ok {
				return string(en)
			}
		}
	}
	return key
}

func normalizeHelpKey(rawKey, normalizedKey string) string {
	switch rawKey {
	case "?", "/", ".", ",":
		return "help"
	}
	switch normalizedKey {
	case "?", "/", ".", ",":
		return "help"
	}
	return normalizedKey
}

func (m Model) handleDeleteSourceSelection(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetDeleteState()
		return m, nil
	case "up", "k":
		m.moveDeleteSourceCursor(-1)
		return m, nil
	case "down", "j":
		m.moveDeleteSourceCursor(1)
		return m, nil
	case " ":
		m.toggleCurrentDeleteSource()
		return m, nil
	case "enter":
		if len(m.selectedDeleteSources()) == 0 {
			for _, source := range m.DeleteSourceOptions {
				m.setDeleteSourceSelected(source, true)
			}
		}
		m.DeleteSourceSelect = false
		m.DeleteConfirm = true
		return m, nil
	case "a":
		for _, source := range m.DeleteSourceOptions {
			m.setDeleteSourceSelected(source, true)
		}
		return m, nil
	}

	if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
		index := int(keyStr[0] - '1')
		if index >= 0 && index < len(m.DeleteSourceOptions) {
			m.DeleteSourceCursor = index
			m.toggleCurrentDeleteSource()
		}
	}

	return m, nil
}

func (m Model) handleHelpOverlay(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "help":
		m.resetHelpState()
		return m, nil
	}
	return m, nil
}

func (m Model) handleAddAccountLogin(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		m.AddAccountLoginVisible = false
		m.AddAccountLoginURL = ""
		m.AddAccountBrowserFailed = false
		m.AddAccountLoginStatus = ""
		return m, tea.Batch(CancelAddAccountLoginCmd(), tea.Quit)
	case "esc":
		m.AddAccountLoginVisible = false
		m.AddAccountLoginURL = ""
		m.AddAccountBrowserFailed = false
		m.AddAccountLoginStatus = ""
		return m, CancelAddAccountLoginCmd()
	case "c":
		if strings.TrimSpace(m.AddAccountLoginURL) == "" {
			return m, nil
		}
		return m, CopyToClipboardCmd(m.AddAccountLoginURL)
	}
	return m, nil
}

func (m Model) handleActionMenu(keyStr string) (tea.Model, tea.Cmd) {
	items := m.actionMenuItems()
	if len(items) == 0 {
		m.resetActionMenuState()
		return m, nil
	}

	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetActionMenuState()
		return m, nil
	case "up", "k":
		m.ActionMenuCursor = (m.ActionMenuCursor - 1 + len(items)) % len(items)
		return m, nil
	case "down", "j":
		m.ActionMenuCursor = (m.ActionMenuCursor + 1) % len(items)
		return m, nil
	case "enter":
		return m.confirmActionMenu()
	}

	if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
		index := int(keyStr[0] - '1')
		if index >= 0 && index < len(items) {
			m.ActionMenuCursor = index
			return m.confirmActionMenu()
		}
	}

	return m, nil
}

func (m Model) handleDeleteConfirm(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetDeleteState()
		return m, nil
	case "enter":
		account := m.activeAccount()
		if account == nil {
			m.resetDeleteState()
			return m, nil
		}

		sources := m.selectedDeleteSources()
		if len(sources) == 0 {
			sources = m.deletableSourcesForAccount(account)
		}
		if len(sources) == 0 {
			return m, nil
		}

		m.Loading = true
		m.Err = nil
		m.Notice = ""
		m.ShowInfo = false
		m.resetApplyState()
		m.resetDeleteState()
		m.Data = api.UsageData{}
		return m, DeleteAccountSourcesCmd(account, sources, account.Key)
	}

	return m, nil
}

func (m Model) handleApplyTargetSelection(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetApplyState()
		return m, nil
	case "up", "k":
		m.moveApplyTargetCursor(-1)
		return m, nil
	case "down", "j":
		m.moveApplyTargetCursor(1)
		return m, nil
	case " ":
		m.toggleCurrentApplyTargetSelection()
		return m, nil
	case "1":
		m.ApplyTargetCursor = 0
		m.toggleCurrentApplyTargetSelection()
		return m, nil
	case "2":
		m.ApplyTargetCursor = 1
		m.toggleCurrentApplyTargetSelection()
		return m, nil
	case "a":
		m.setApplyTargetsAll(true)
		return m, nil
	case "enter":
		if len(m.selectedApplyTargets()) == 0 {
			m.setApplyTargetsAll(true)
		}
		m.ApplyTargetSelect = false
		m.ApplyConfirm = true
		return m, nil
	}

	return m, nil
}

func (m Model) handleApplyConfirm(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetApplyState()
		return m, nil
	case "enter":
		account := m.activeAccount()
		if account == nil {
			m.resetApplyState()
			return m, nil
		}

		m.Loading = true
		m.Err = nil
		m.resetDeleteState()
		m.ShowInfo = false
		m.Notice = ""
		targets := m.selectedApplyTargets()
		if len(targets) == 0 {
			targets = applyTargetsOrdered()
		}
		m.resetApplyState()
		return m, ApplyToTargetsCmd(account, targets)
	}

	return m, nil
}
