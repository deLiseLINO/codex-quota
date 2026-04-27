package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
	"github.com/deLiseLINO/codex-quota/internal/update"
)

func (m Model) confirmActionMenu() (tea.Model, tea.Cmd) {
	items := m.actionMenuItems()
	if len(items) == 0 {
		m.resetActionMenuState()
		return m, nil
	}
	if m.ActionMenuCursor < 0 || m.ActionMenuCursor >= len(items) {
		m.ActionMenuCursor = 0
	}

	selected := items[m.ActionMenuCursor]
	m.resetActionMenuState()

	switch selected.ID {
	case actionMenuApply:
		return m.beginApplyFlow()
	case actionMenuRefresh:
		return m.beginRefreshActive()
	case actionMenuRefreshAll:
		return m.beginRefreshAll()
	case actionMenuInfo:
		m.ShowInfo = true
		m.Notice = ""
		m.Err = nil
		return m, nil
	case actionMenuAdd:
		return m.beginAddAccount()
	case actionMenuView:
		return m.toggleViewMode()
	case actionMenuDelete:
		return m.beginDeleteFlow()
	case actionMenuUpdate:
		if !m.openUpdatePrompt() {
			return m, nil
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) openHelpOverlay() {
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	m.HelpVisible = true
}

func (m *Model) resetHelpState() {
	m.HelpVisible = false
}

func (m *Model) openActionMenu() {
	m.resetHelpState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	m.ActionMenuVisible = true
	m.ActionMenuCursor = 0
}

func (m *Model) resetActionMenuState() {
	m.ActionMenuVisible = false
	m.ActionMenuCursor = 0
}

func (m *Model) openUpdatePrompt() bool {
	if m.UpdatePromptVersion == "" || !update.SupportsAutoUpdate(m.UpdatePromptMethod) {
		return false
	}
	m.UpdatePromptVisible = true
	m.UpdatePromptCursor = 0
	m.resetHelpState()
	m.resetActionMenuState()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	m.resetDeleteState()
	m.resetApplyState()
	return true
}

func (m Model) beginDeleteFlow() (tea.Model, tea.Cmd) {
	if len(m.Accounts) == 0 {
		return m, nil
	}
	account := m.activeAccount()
	if account == nil {
		return m, nil
	}
	if !account.Writable {
		m.resetActionMenuState()
		m.resetHelpState()
		m.resetDeleteState()
		m.resetApplyState()
		m.ShowInfo = false
		m.Err = nil
		m.Notice = "cannot delete this account (read-only)"
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)
	}

	sources := m.deletableSourcesForAccount(account)
	if len(sources) == 0 {
		m.resetActionMenuState()
		m.resetHelpState()
		m.resetDeleteState()
		m.resetApplyState()
		m.ShowInfo = false
		m.Err = nil
		m.Notice = "cannot delete this account (no writable source found)"
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)
	}

	m.resetActionMenuState()
	m.resetHelpState()
	m.startDeleteFlow(sources)
	m.ShowInfo = false
	m.Err = nil
	m.Notice = ""
	return m, nil
}

func (m Model) beginRefreshActive() (tea.Model, tea.Cmd) {
	if m.activeAccount() == nil {
		return m, nil
	}
	m.Loading = true
	m.Err = nil
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""

	if m.LoadingMap == nil {
		m.LoadingMap = make(map[string]bool)
	}
	delete(m.UsageData, m.activeAccountKey())
	delete(m.ErrorsMap, m.activeAccountKey())
	delete(m.compactBarAnimations, m.activeAccountKey())
	m.clearTabWindowAnimations()
	return m, m.fetchNextCmd()
}

func (m Model) beginRefreshAll() (tea.Model, tea.Cmd) {
	m.Loading = true
	m.Err = nil
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""

	m.UsageData = make(map[string]api.UsageData)
	m.ErrorsMap = make(map[string]error)
	m.LoadingMap = make(map[string]bool)
	m.compactBarAnimations = make(map[string]compactBarAnimation)
	m.tabWindowAnimations = make(map[string]tabWindowAnimation)
	m.animationTicking = false

	return m, m.fetchNextCmd()
}

func (m Model) toggleViewMode() (tea.Model, tea.Cmd) {
	m.CompactMode = !m.CompactMode
	if m.CompactMode {
		m.clearTabWindowAnimations()
	} else {
		m.clearCompactBarAnimations()
	}
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""
	return m, tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd(), SaveUIStateSnapshotCmd(m.uiStateSnapshot()))
}

func (m Model) beginAddAccount() (tea.Model, tea.Cmd) {
	if m.AddAccountLoginVisible {
		return m, nil
	}
	m.Loading = false
	m.Err = nil
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Notice = ""
	return m, StartAddAccountLoginCmd()
}

func (m Model) beginApplyFlow() (tea.Model, tea.Cmd) {
	if m.activeAccount() == nil {
		return m, nil
	}
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.startApplyFlow()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	return m, nil
}

func (m *Model) startDeleteFlow(sources []config.Source) {
	m.resetDeleteState()
	m.resetApplyState()

	m.DeleteSourceOptions = dedupeSources(sources)
	m.DeleteSources = make(map[config.Source]bool, len(m.DeleteSourceOptions))
	for _, source := range m.DeleteSourceOptions {
		m.DeleteSources[source] = true
	}

	m.DeleteSourceCursor = 0
	m.DeleteSourceSelect = len(m.DeleteSourceOptions) > 1
	m.DeleteConfirm = !m.DeleteSourceSelect
}

func (m *Model) resetDeleteState() {
	m.DeleteSourceSelect = false
	m.DeleteSourceOptions = nil
	m.DeleteSources = nil
	m.DeleteSourceCursor = 0
	m.DeleteConfirm = false
}

func (m *Model) resetApplyState() {
	m.ApplyTargetSelect = false
	m.ApplyConfirm = false
	m.ApplyTargets = nil
	m.ApplyTargetCursor = 0
}

func (m *Model) startApplyFlow() {
	m.resetApplyState()
	m.ApplyTargetSelect = true
	m.ApplyTargets = map[config.Source]bool{
		config.SourceCodex:    true,
		config.SourceOpenCode: true,
		config.SourcePi:       true,
	}
	m.ApplyTargetCursor = 0
}

func (m *Model) toggleApplyTargetSelection(source config.Source) {
	if source != config.SourceCodex && source != config.SourceOpenCode && source != config.SourcePi {
		return
	}
	if m.ApplyTargets == nil {
		m.ApplyTargets = map[config.Source]bool{}
	}
	if m.ApplyTargets[source] && m.selectedApplyTargetCount() <= 1 {
		return
	}
	m.ApplyTargets[source] = !m.ApplyTargets[source]
}

func (m *Model) toggleCurrentApplyTargetSelection() {
	targets := applyTargetsOrdered()
	if len(targets) == 0 {
		return
	}
	if m.ApplyTargetCursor < 0 || m.ApplyTargetCursor >= len(targets) {
		m.ApplyTargetCursor = 0
	}
	m.toggleApplyTargetSelection(targets[m.ApplyTargetCursor])
}

func (m *Model) moveApplyTargetCursor(delta int) {
	targets := applyTargetsOrdered()
	if len(targets) == 0 {
		m.ApplyTargetCursor = 0
		return
	}
	m.ApplyTargetCursor = (m.ApplyTargetCursor + delta + len(targets)) % len(targets)
}

func (m *Model) setApplyTargetsAll(selected bool) {
	if m.ApplyTargets == nil {
		m.ApplyTargets = map[config.Source]bool{}
	}
	for _, source := range applyTargetsOrdered() {
		m.ApplyTargets[source] = selected
	}
}

func (m Model) selectedApplyTargets() []config.Source {
	targets := make([]config.Source, 0, 3)
	for _, source := range applyTargetsOrdered() {
		if m.ApplyTargets != nil && m.ApplyTargets[source] {
			targets = append(targets, source)
		}
	}
	return targets
}

func (m Model) selectedApplyTargetCount() int {
	count := 0
	for _, source := range applyTargetsOrdered() {
		if m.ApplyTargets != nil && m.ApplyTargets[source] {
			count++
		}
	}
	return count
}

func applyTargetsOrdered() []config.Source {
	return []config.Source{config.SourceCodex, config.SourceOpenCode, config.SourcePi}
}

func dedupeApplyTargets(targets []config.Source) []config.Source {
	seen := map[config.Source]bool{}
	for _, target := range targets {
		if target != config.SourceCodex && target != config.SourceOpenCode && target != config.SourcePi {
			continue
		}
		seen[target] = true
	}

	output := make([]config.Source, 0, len(seen))
	for _, source := range applyTargetsOrdered() {
		if seen[source] {
			output = append(output, source)
		}
	}
	return output
}

func mapKeysSortedBySource(values map[config.Source]string) []config.Source {
	keys := make([]config.Source, 0, len(values))
	for source := range values {
		keys = append(keys, source)
	}
	return dedupeApplyTargets(keys)
}

func formatTargetErrors(errorsByTarget map[config.Source]error) string {
	if len(errorsByTarget) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errorsByTarget))
	for _, source := range applyTargetsOrdered() {
		err, ok := errorsByTarget[source]
		if !ok || err == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %v", sourceDisplayName(source), err))
	}
	return strings.Join(parts, "; ")
}

func (m Model) deletableSourcesForAccount(account *config.Account) []config.Source {
	if account == nil {
		return nil
	}

	seen := m.collectKnownSourcesForAccount(account)

	if len(seen) == 0 {
		if account.Source == config.SourceManaged || account.Source == config.SourceOpenCode || account.Source == config.SourceCodex || account.Source == config.SourcePi {
			seen[account.Source] = true
		}
	}

	if strings.TrimSpace(account.AccountID) == "" && strings.TrimSpace(account.Email) == "" {
		delete(seen, config.SourceManaged)
	}

	return orderedSources(seen)
}

func (m Model) collectKnownSourcesForAccount(account *config.Account) map[config.Source]bool {
	seen := map[config.Source]bool{}
	if account == nil {
		return seen
	}

	appendLabels := func(labels []string) {
		for _, label := range labels {
			source, ok := sourceFromLabel(label)
			if !ok {
				continue
			}
			seen[source] = true
		}
	}

	if m.SourcesByAccountID != nil {
		if accountID := strings.TrimSpace(account.AccountID); accountID != "" {
			appendLabels(m.SourcesByAccountID[accountID])
		}
		if email := strings.ToLower(strings.TrimSpace(account.Email)); email != "" {
			appendLabels(m.SourcesByAccountID["email:"+email])
		}
	}

	if m.ActiveSourcesByIdentity != nil {
		for _, key := range config.ActiveIdentityKeys(account) {
			appendLabels(m.ActiveSourcesByIdentity[key])
		}
	}

	return seen
}

func orderedSources(sourceMap map[config.Source]bool) []config.Source {
	if len(sourceMap) == 0 {
		return nil
	}

	ordered := []config.Source{config.SourceManaged, config.SourceOpenCode, config.SourceCodex, config.SourcePi}
	out := make([]config.Source, 0, len(sourceMap))
	for _, source := range ordered {
		if sourceMap[source] {
			out = append(out, source)
		}
	}
	return out
}

func sourceFromLabel(label string) (config.Source, bool) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "app", "managed":
		return config.SourceManaged, true
	case "opencode":
		return config.SourceOpenCode, true
	case "codex":
		return config.SourceCodex, true
	case "pi", "pi coding agent":
		return config.SourcePi, true
	default:
		return "", false
	}
}

func dedupeSources(sources []config.Source) []config.Source {
	seen := make(map[config.Source]bool, len(sources))
	for _, source := range sources {
		if source != config.SourceManaged && source != config.SourceOpenCode && source != config.SourceCodex && source != config.SourcePi {
			continue
		}
		seen[source] = true
	}
	return orderedSources(seen)
}

func sourceDisplayName(source config.Source) string {
	switch source {
	case config.SourceManaged:
		return "app"
	case config.SourceOpenCode:
		return "opencode"
	case config.SourceCodex:
		return "codex"
	case config.SourcePi:
		return "pi"
	default:
		return string(source)
	}
}

func sourceListText(sources []config.Source) string {
	if len(sources) == 0 {
		return "n/a"
	}
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, sourceDisplayName(source))
	}
	return strings.Join(parts, ", ")
}

func (m *Model) selectedDeleteSources() []config.Source {
	if len(m.DeleteSourceOptions) == 0 {
		return nil
	}
	selected := make([]config.Source, 0, len(m.DeleteSourceOptions))
	for _, source := range m.DeleteSourceOptions {
		if m.isDeleteSourceSelected(source) {
			selected = append(selected, source)
		}
	}
	return selected
}

func (m *Model) toggleDeleteSource(source config.Source) {
	if m.DeleteSources == nil {
		m.DeleteSources = map[config.Source]bool{}
	}
	if m.DeleteSources[source] && m.deleteSourceCount() <= 1 {
		return
	}
	m.DeleteSources[source] = !m.DeleteSources[source]
}

func (m *Model) toggleCurrentDeleteSource() {
	if len(m.DeleteSourceOptions) == 0 {
		return
	}
	if m.DeleteSourceCursor < 0 || m.DeleteSourceCursor >= len(m.DeleteSourceOptions) {
		m.DeleteSourceCursor = 0
	}
	m.toggleDeleteSource(m.DeleteSourceOptions[m.DeleteSourceCursor])
}

func (m *Model) moveDeleteSourceCursor(delta int) {
	if len(m.DeleteSourceOptions) == 0 {
		m.DeleteSourceCursor = 0
		return
	}
	m.DeleteSourceCursor = (m.DeleteSourceCursor + delta + len(m.DeleteSourceOptions)) % len(m.DeleteSourceOptions)
}

func (m *Model) setDeleteSourceSelected(source config.Source, selected bool) {
	if m.DeleteSources == nil {
		m.DeleteSources = map[config.Source]bool{}
	}
	m.DeleteSources[source] = selected
}

func (m Model) isDeleteSourceSelected(source config.Source) bool {
	if m.DeleteSources == nil {
		return false
	}
	return m.DeleteSources[source]
}

func (m Model) deleteSourceCount() int {
	count := 0
	for _, source := range m.DeleteSourceOptions {
		if m.isDeleteSourceSelected(source) {
			count++
		}
	}
	return count
}
