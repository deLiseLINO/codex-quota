package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

const (
	settingsRowAutoRefresh = iota
	settingsRowActiveInterval
	settingsRowBackgroundInterval
	settingsRowUpdateCheck
	settingsRowCount
)

const (
	activeIntervalStepSec     = 5
	backgroundIntervalStepSec = 60
)

func (m *Model) openSettingsOverlay() {
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	m.SettingsVisible = true
	m.settingsCursor = 0
	m.settingsDraft = 0
	m.settingsDraftActive = false
}

func (m *Model) closeSettingsOverlay() {
	m.SettingsVisible = false
	m.settingsDraft = 0
	m.settingsDraftActive = false
}

func (m Model) handleSettingsOverlay(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "s":
		m.closeSettingsOverlay()
		return m, nil
	case "up", "k":
		m.moveSettingsCursor(-1)
		return m, nil
	case "down", "j":
		m.moveSettingsCursor(1)
		return m, nil
	case "enter", " ":
		if !m.settingsRowIsToggle() {
			return m, nil
		}
		m.flipSettingsToggle()
		if m.Settings.AutoRefreshEnabled {
			m.resetAutoRefreshTimers()
		}
		return m, SaveSettingsCmd(m.Settings)
	case "left", "h", "right", "l":
		if !m.settingsRowIsInterval() {
			return m, nil
		}
		step := m.settingsStepSize()
		if keyStr == "left" || keyStr == "h" {
			step = -step
		}
		m.stepSettingsInterval(step)
		m.resetSettingsIntervalTimers()
		return m, SaveSettingsCmd(m.Settings)
	}

	if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
		if m.settingsRowIsInterval() {
			m.typeSettingsDigit(int(keyStr[0] - '0'))
			m.resetSettingsIntervalTimers()
			return m, SaveSettingsCmd(m.Settings)
		}
	}

	return m, nil
}

func (m *Model) moveSettingsCursor(delta int) {
	m.settingsDraft = 0
	m.settingsDraftActive = false
	m.settingsCursor = (m.settingsCursor + delta + settingsRowCount) % settingsRowCount
}

func (m Model) settingsRowIsToggle() bool {
	return m.settingsCursor == settingsRowAutoRefresh || m.settingsCursor == settingsRowUpdateCheck
}

func (m Model) settingsRowIsInterval() bool {
	return m.settingsCursor == settingsRowActiveInterval || m.settingsCursor == settingsRowBackgroundInterval
}

func (m *Model) flipSettingsToggle() {
	switch m.settingsCursor {
	case settingsRowAutoRefresh:
		m.Settings.AutoRefreshEnabled = !m.Settings.AutoRefreshEnabled
	case settingsRowUpdateCheck:
		m.Settings.CheckForUpdateOnStartup = !m.Settings.CheckForUpdateOnStartup
	}
}

func (m *Model) resetSettingsIntervalTimers() {
	if m.settingsCursor != settingsRowBackgroundInterval {
		m.resetAutoRefreshTimer(m.activeAccountKey())
		return
	}
	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		if acc.Key != m.activeAccountKey() {
			m.resetAutoRefreshTimer(acc.Key)
		}
	}
}

func (m Model) settingsStepSize() int {
	if m.settingsCursor == settingsRowBackgroundInterval {
		return backgroundIntervalStepSec
	}
	return activeIntervalStepSec
}

func (m *Model) stepSettingsInterval(delta int) {
	m.settingsDraft = 0
	m.settingsDraftActive = false
	m.setSettingsIntervalValue(config.ClampInt(m.settingsIntervalValue()+delta, m.settingsIntervalMin(), m.settingsIntervalMax()))
}

func (m *Model) typeSettingsDigit(digit int) {
	if !m.settingsDraftActive {
		m.settingsDraft = digit
		m.settingsDraftActive = true
	} else {
		m.settingsDraft = m.settingsDraft*10 + digit
	}
	m.setSettingsIntervalValue(config.ClampInt(m.settingsDraft, m.settingsIntervalMin(), m.settingsIntervalMax()))
}

func (m Model) settingsIntervalValue() int {
	if m.settingsCursor == settingsRowBackgroundInterval {
		return m.Settings.BackgroundIntervalSec
	}
	return m.Settings.ActiveIntervalSec
}

func (m Model) settingsIntervalMin() int {
	if m.settingsCursor == settingsRowBackgroundInterval {
		return config.BackgroundIntervalMinSec
	}
	return config.ActiveIntervalMinSec
}

func (m Model) settingsIntervalMax() int {
	if m.settingsCursor == settingsRowBackgroundInterval {
		return config.BackgroundIntervalMaxSec
	}
	return config.ActiveIntervalMaxSec
}

func (m *Model) setSettingsIntervalValue(value int) {
	if m.settingsCursor == settingsRowBackgroundInterval {
		m.Settings.BackgroundIntervalSec = value
	} else {
		m.Settings.ActiveIntervalSec = value
	}
}

func (m Model) renderSettingsModal() string {
	lines := []string{
		InfoTitleStyle.Render("Settings"),
		"",
		m.renderSettingsToggleRow(settingsRowAutoRefresh, "Auto-refresh", m.Settings.AutoRefreshEnabled),
		m.renderSettingsIntervalRow(settingsRowActiveInterval, "Active refresh interval", m.Settings.ActiveIntervalSec),
		m.renderSettingsIntervalRow(settingsRowBackgroundInterval, "Background refresh interval", m.Settings.BackgroundIntervalSec),
		m.renderSettingsToggleRow(settingsRowUpdateCheck, "Check for updates on startup", m.Settings.CheckForUpdateOnStartup),
		"",
		ActionMenuHintStyle.Render("[↑/↓] Move   [enter/space] Toggle   [←/→] Step   [0-9] Type   [s/esc] Close"),
	}
	return InfoBoxStyle.Copy().Width(60).Render(strings.Join(lines, "\n"))
}

func (m Model) renderSettingsToggleRow(row int, label string, enabled bool) string {
	cursor := " "
	if m.settingsCursor == row {
		cursor = ">"
	}
	mark := " "
	if enabled {
		mark = "x"
	}
	return InfoValueStyle.Render(fmt.Sprintf("%s %-28s [%s]", cursor, label, mark))
}

func (m Model) renderSettingsIntervalRow(row int, label string, value int) string {
	cursor := " "
	if m.settingsCursor == row {
		cursor = ">"
	}
	return InfoValueStyle.Render(fmt.Sprintf("%s %-28s %d (%s)", cursor, label, value, formatIntervalLabel(value)))
}

func formatIntervalLabel(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	rem := minutes % 60
	if rem == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, rem)
}
