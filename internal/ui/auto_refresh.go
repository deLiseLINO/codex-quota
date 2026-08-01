package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const autoRefreshTickInterval = time.Second

func autoRefreshTickCmd() tea.Cmd {
	return tea.Tick(autoRefreshTickInterval, func(now time.Time) tea.Msg {
		return AutoRefreshTickMsg{Now: now}
	})
}

func (m Model) handleAutoRefreshTick(now time.Time) (tea.Model, tea.Cmd) {
	if m.lastRefresh == nil {
		m.lastRefresh = make(map[string]time.Time)
	}
	if m.refreshScheduled == nil {
		m.refreshScheduled = make(map[string]bool)
	}

	if !m.Settings.AutoRefreshEnabled {
		return m, autoRefreshTickCmd()
	}

	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		if m.refreshScheduled[acc.Key] {
			continue
		}
		if m.LoadingMap[acc.Key] {
			continue
		}
		last, ok := m.lastRefresh[acc.Key]
		if !ok {
			m.lastRefresh[acc.Key] = now
			continue
		}
		if now.Sub(last) < m.autoRefreshInterval(acc.Key) {
			continue
		}
		m.lastRefresh[acc.Key] = now
		m.refreshScheduled[acc.Key] = true
	}

	return m, tea.Batch(autoRefreshTickCmd(), m.fetchNextCmd())
}

func (m Model) autoRefreshInterval(accountKey string) time.Duration {
	if accountKey == m.activeAccountKey() {
		return time.Duration(m.Settings.ActiveIntervalSec) * time.Second
	}
	return time.Duration(m.Settings.BackgroundIntervalSec) * time.Second
}

func (m *Model) resetAutoRefreshTimers() {
	m.lastRefresh = make(map[string]time.Time)
}

func (m *Model) resetAutoRefreshTimer(accountKey string) {
	if accountKey == "" {
		return
	}
	if m.lastRefresh == nil {
		m.lastRefresh = make(map[string]time.Time)
	}
	delete(m.lastRefresh, accountKey)
}

func (m *Model) pruneAutoRefreshTimers() {
	if len(m.lastRefresh) == 0 {
		return
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, acc := range m.Accounts {
		if acc != nil && acc.Key != "" {
			valid[acc.Key] = struct{}{}
		}
	}
	for key := range m.lastRefresh {
		if _, ok := valid[key]; !ok {
			delete(m.lastRefresh, key)
		}
	}
}
