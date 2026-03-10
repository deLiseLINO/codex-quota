package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestArrowNavigationWorksInBothModes(t *testing.T) {
	tests := []struct {
		name       string
		compact    bool
		keyType    tea.KeyType
		wantActive int
	}{
		{name: "normal down", compact: false, keyType: tea.KeyDown, wantActive: 1},
		{name: "normal right", compact: false, keyType: tea.KeyRight, wantActive: 1},
		{name: "normal up", compact: false, keyType: tea.KeyUp, wantActive: 2},
		{name: "normal left", compact: false, keyType: tea.KeyLeft, wantActive: 2},
		{name: "compact down", compact: true, keyType: tea.KeyDown, wantActive: 1},
		{name: "compact right", compact: true, keyType: tea.KeyRight, wantActive: 1},
		{name: "compact up", compact: true, keyType: tea.KeyUp, wantActive: 2},
		{name: "compact left", compact: true, keyType: tea.KeyLeft, wantActive: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testModelForHotkeys(3)
			m.CompactMode = tc.compact
			m.ActiveAccountIx = 0

			updated, _ := m.Update(tea.KeyMsg{Type: tc.keyType})
			got := updated.(Model).ActiveAccountIx
			if got != tc.wantActive {
				t.Fatalf("expected active index %d, got %d", tc.wantActive, got)
			}
		})
	}
}

func TestInitialModelUsesPersistedCompactMode(t *testing.T) {
	m := InitialModel([]*config.Account{}, map[string][]string{}, map[string][]string{}, true)
	if !m.CompactMode {
		t.Fatalf("expected compact mode to be initialized from persisted state")
	}
}

func TestEnterOpensActionMenuOnMainScreen(t *testing.T) {
	m := testModelForHotkeys(2)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.ActionMenuVisible {
		t.Fatalf("expected action menu to open on enter")
	}
	if got.ApplyTargetSelect || got.ApplyConfirm {
		t.Fatalf("did not expect apply flow to open directly on enter")
	}
}

func TestEnterInApplySelectionKeepsModalSemantics(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()
	m.ApplyTargets = map[config.Source]bool{
		config.SourceCodex:    true,
		config.SourceOpenCode: false,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if got.ApplyTargetSelect {
		t.Fatalf("expected apply target selection to close on enter")
	}
	if !got.ApplyConfirm {
		t.Fatalf("expected apply confirm step to open on enter")
	}
}

func TestQuestionMarkOpensHelpOverlay(t *testing.T) {
	m := testModelForHotkeys(1)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(Model)

	if !got.HelpVisible {
		t.Fatalf("expected help overlay to open")
	}
	if got.ActionMenuVisible {
		t.Fatalf("did not expect action menu to open")
	}
}

func TestEscCancelsAddAccountLoginModal(t *testing.T) {
	m := testModelForHotkeys(1)
	m.AddAccountLoginVisible = true
	m.AddAccountLoginURL = "https://auth.openai.com/example"
	m.AddAccountBrowserFailed = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)

	if got.AddAccountLoginVisible {
		t.Fatalf("expected add account login modal to close on esc")
	}
	if got.AddAccountLoginURL != "" {
		t.Fatalf("expected login URL to clear on esc, got %q", got.AddAccountLoginURL)
	}
	if got.AddAccountBrowserFailed {
		t.Fatalf("expected browser failure flag to clear on esc")
	}
	if cmd == nil {
		t.Fatalf("expected cancel command on esc")
	}
}

func TestQQuitsFromAddAccountLoginModal(t *testing.T) {
	m := testModelForHotkeys(1)
	m.AddAccountLoginVisible = true
	m.AddAccountLoginURL = "https://auth.openai.com/example"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(Model)

	if got.AddAccountLoginVisible {
		t.Fatalf("expected add account login modal to close on q")
	}
	if cmd == nil {
		t.Fatalf("expected quit command from add account login modal")
	}
}

func TestCInAddAccountLoginModalTriggersCopy(t *testing.T) {
	m := testModelForHotkeys(1)
	m.AddAccountLoginVisible = true
	m.AddAccountLoginURL = "https://auth.openai.com/example"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := updated.(Model)

	if !got.AddAccountLoginVisible {
		t.Fatalf("expected add account login modal to remain open")
	}
	if cmd == nil {
		t.Fatalf("expected copy command from add account login modal")
	}
}

func TestMouseClickOnAddAccountLoginURLTriggersOpen(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Width = 140
	m.Height = 40
	m.AddAccountLoginVisible = true
	m.AddAccountLoginURL = "https://auth.openai.com/example"

	layout := m.addAccountLoginModalLayout()
	modalHeight := lipgloss.Height(InfoBoxStyle.Copy().Width(layout.Width).Render(strings.Join(layout.Lines, "\n")))
	footerHeight := lipgloss.Height(HelpStyle.Render("\n" + m.renderFooter()))
	bodyHeight := m.Height - footerHeight
	startX := (m.Width - layout.Width) / 2
	startY := (bodyHeight - modalHeight) / 2

	clickX := startX + 3
	clickY := startY + 1 + layout.URLStartLine
	updated, cmd := m.Update(tea.MouseMsg{
		X:      clickX,
		Y:      clickY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	got := updated.(Model)

	if !got.AddAccountLoginVisible {
		t.Fatalf("expected add account login modal to remain open")
	}
	if cmd == nil {
		t.Fatalf("expected open-browser command from URL click")
	}
}

func TestApplyHotkeyOpensApplyFlow(t *testing.T) {
	m := testModelForHotkeys(1)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	got := updated.(Model)

	if !got.ApplyTargetSelect {
		t.Fatalf("expected apply flow to open on o")
	}
	if got.ActionMenuVisible {
		t.Fatalf("did not expect action menu to open")
	}
}

func TestActionMenuApplyOpensApplyFlow(t *testing.T) {
	m := testModelForHotkeys(1)
	m.ActionMenuVisible = true
	m.ActionMenuCursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.ApplyTargetSelect {
		t.Fatalf("expected apply flow to open from action menu")
	}
	if got.ActionMenuVisible {
		t.Fatalf("expected action menu to close after selection")
	}
}

func TestActionMenuRefreshAllTriggersBulkRefresh(t *testing.T) {
	m := testModelForHotkeys(3)
	m.ActionMenuVisible = true
	m.ActionMenuCursor = 4 // first item in Global actions
	m.Notice = "old notice"
	m.UsageData["managed:1"] = api.UsageData{Allowed: true}
	m.ErrorsMap["managed:1"] = nil
	m.LoadingMap["managed:1"] = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if got.ActionMenuVisible {
		t.Fatalf("expected action menu to close after refresh all")
	}
	if got.Notice != "" {
		t.Fatalf("expected no notice for refresh-all, got %q", got.Notice)
	}
	if len(got.UsageData) != 0 {
		t.Fatalf("expected usage cache reset, got %d entries", len(got.UsageData))
	}
	if len(got.LoadingMap) != 3 {
		t.Fatalf("expected three loading accounts scheduled, got %d entries", len(got.LoadingMap))
	}
}

func TestHelpAliasesOpenHelpOverlay(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "question mark", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}},
		{name: "dot alias", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}}},
		{name: "comma alias", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}}},
		{name: "russian yu", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'ю'}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testModelForHotkeys(1)

			updated, _ := m.Update(tc.msg)
			got := updated.(Model)

			if !got.HelpVisible {
				t.Fatalf("expected help overlay to open for %s", tc.name)
			}
		})
	}
}

func TestQQuitsFromHelpOverlay(t *testing.T) {
	m := testModelForHotkeys(1)
	m.HelpVisible = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if _, ok := updated.(Model); !ok {
		t.Fatalf("expected model to remain valid")
	}
	if cmd == nil {
		t.Fatalf("expected quit command from help overlay")
	}
}

func TestQQuitsFromActionMenu(t *testing.T) {
	m := testModelForHotkeys(1)
	m.ActionMenuVisible = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if _, ok := updated.(Model); !ok {
		t.Fatalf("expected model to remain valid")
	}
	if cmd == nil {
		t.Fatalf("expected quit command from action menu")
	}
}

func TestQQuitsFromApplyModal(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if _, ok := updated.(Model); !ok {
		t.Fatalf("expected model to remain valid")
	}
	if cmd == nil {
		t.Fatalf("expected quit command from apply modal")
	}
}

func TestQQuitsFromUpdatePrompt(t *testing.T) {
	m := testModelForHotkeys(1)
	m.UpdatePromptVisible = true
	m.UpdatePromptVersion = "1.2.3"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if _, ok := updated.(Model); !ok {
		t.Fatalf("expected model to remain valid")
	}
	if cmd == nil {
		t.Fatalf("expected quit command from update prompt")
	}
}

func TestRefreshAllDoesNotSetNoticeModal(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Notice = "old notice"
	m.UsageData["managed:1"] = api.UsageData{Allowed: true}
	m.ErrorsMap["managed:1"] = nil
	m.LoadingMap["managed:1"] = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)

	if got.Notice != "" {
		t.Fatalf("expected no notice for refresh-all, got %q", got.Notice)
	}
	if len(got.UsageData) != 0 {
		t.Fatalf("expected usage cache reset, got %d entries", len(got.UsageData))
	}
	if len(got.ErrorsMap) != 0 {
		t.Fatalf("expected errors cache reset, got %d entries", len(got.ErrorsMap))
	}
	if len(got.LoadingMap) != 2 {
		t.Fatalf("expected two loading accounts scheduled, got %d entries", len(got.LoadingMap))
	}
	loadingCount := 0
	for _, isLoading := range got.LoadingMap {
		if isLoading {
			loadingCount++
		}
	}
	if loadingCount != 2 {
		t.Fatalf("expected exactly two loading markers, got %d", loadingCount)
	}
}

func TestRefreshAllLoadsInListOrderNotActivePriority(t *testing.T) {
	m := testModelForHotkeys(5)
	m.ActiveAccountIx = 4 // focus on the last account

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)

	if !got.LoadingMap["managed:1"] || !got.LoadingMap["managed:2"] || !got.LoadingMap["managed:3"] {
		t.Fatalf("expected first three accounts to be scheduled first, got loading map: %#v", got.LoadingMap)
	}
	if got.LoadingMap["managed:4"] || got.LoadingMap["managed:5"] {
		t.Fatalf("did not expect later accounts to be prioritized, got loading map: %#v", got.LoadingMap)
	}
}

func TestRefreshAllInCompactLoadsByVisualOrderWithExhaustedBlock(t *testing.T) {
	m := testModelForHotkeys(6)
	m.CompactMode = true
	// visual order in compact should become: 1,3,5,6,2,4
	m.ExhaustedSticky["managed:2"] = true
	m.ExhaustedSticky["managed:4"] = true
	m.ActiveAccountIx = 5 // focused account should not be prioritized

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)

	if !got.LoadingMap["managed:1"] || !got.LoadingMap["managed:3"] || !got.LoadingMap["managed:5"] {
		t.Fatalf("expected first three compact visual accounts (1,3,5) to be scheduled, got loading map: %#v", got.LoadingMap)
	}
	if got.LoadingMap["managed:6"] || got.LoadingMap["managed:2"] || got.LoadingMap["managed:4"] {
		t.Fatalf("did not expect later visual or exhausted block accounts to be scheduled first, got loading map: %#v", got.LoadingMap)
	}
}

func TestCompactArrowNavigationFollowsVisualOrderWithExhaustedBlock(t *testing.T) {
	m := testModelForHotkeys(4)
	m.CompactMode = true
	// visual order in compact should become: 1,3,2,4
	m.ExhaustedSticky["managed:2"] = true
	m.ExhaustedSticky["managed:4"] = true
	m.ActiveAccountIx = 0 // managed:1

	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	gotDown := down.(Model)
	if gotDown.ActiveAccountIx != 2 { // managed:3
		t.Fatalf("expected next compact item to be managed:3 (idx=2), got %d", gotDown.ActiveAccountIx)
	}

	down2, _ := gotDown.Update(tea.KeyMsg{Type: tea.KeyDown})
	gotDown2 := down2.(Model)
	if gotDown2.ActiveAccountIx != 1 { // managed:2 (first exhausted)
		t.Fatalf("expected next compact item to be managed:2 (idx=1), got %d", gotDown2.ActiveAccountIx)
	}

	up, _ := gotDown2.Update(tea.KeyMsg{Type: tea.KeyUp})
	gotUp := up.(Model)
	if gotUp.ActiveAccountIx != 2 { // back to managed:3
		t.Fatalf("expected previous compact item to be managed:3 (idx=2), got %d", gotUp.ActiveAccountIx)
	}
}

func TestInitialModelInCompactStartsFromFirstVisibleNonExhaustedAccount(t *testing.T) {
	accounts := []*config.Account{
		{Key: "managed:1", Label: "user1@example.com", Email: "user1@example.com", AccountID: "acc-1", Source: config.SourceManaged, Writable: true},
		{Key: "managed:2", Label: "user2@example.com", Email: "user2@example.com", AccountID: "acc-2", Source: config.SourceManaged, Writable: true},
		{Key: "managed:3", Label: "user3@example.com", Email: "user3@example.com", AccountID: "acc-3", Source: config.SourceManaged, Writable: true},
	}

	m := InitialModelWithUIState(
		accounts,
		map[string][]string{},
		map[string][]string{},
		config.UIState{
			CompactMode:          true,
			ExhaustedAccountKeys: []string{"managed:1"},
		},
	)

	if m.ActiveAccountIx != 1 {
		t.Fatalf("ActiveAccountIx = %d, want 1", m.ActiveAccountIx)
	}
	if !m.LoadingMap["managed:2"] {
		t.Fatalf("expected managed:2 to be queued for initial load, got loading map %#v", m.LoadingMap)
	}
	if m.LoadingMap["managed:1"] {
		t.Fatalf("did not expect exhausted managed:1 to be initial active load, got loading map %#v", m.LoadingMap)
	}
}

func TestAccountsMsgInCompactStartsFromFirstVisibleNonExhaustedAccount(t *testing.T) {
	m := testModelForHotkeys(3)
	m.CompactMode = true
	m.ExhaustedSticky["managed:1"] = true
	m.LoadingMap = map[string]bool{}

	updated, _ := m.Update(AccountsMsg{
		Accounts: []*config.Account{
			{Key: "managed:1", Label: "user1@example.com", Email: "user1@example.com", AccountID: "acc-1", Source: config.SourceManaged, Writable: true},
			{Key: "managed:2", Label: "user2@example.com", Email: "user2@example.com", AccountID: "acc-2", Source: config.SourceManaged, Writable: true},
			{Key: "managed:3", Label: "user3@example.com", Email: "user3@example.com", AccountID: "acc-3", Source: config.SourceManaged, Writable: true},
		},
	})
	got := updated.(Model)

	if got.ActiveAccountIx != 1 {
		t.Fatalf("ActiveAccountIx = %d, want 1", got.ActiveAccountIx)
	}
	if !got.LoadingMap["managed:2"] {
		t.Fatalf("expected managed:2 to be queued for active load, got loading map %#v", got.LoadingMap)
	}
}

func testModelForHotkeys(count int) Model {
	accounts := make([]*config.Account, 0, count)
	for i := 0; i < count; i++ {
		accounts = append(accounts, &config.Account{
			Key:       "managed:" + string(rune('1'+i)),
			Label:     "user" + string(rune('1'+i)) + "@example.com",
			Email:     "user" + string(rune('1'+i)) + "@example.com",
			AccountID: "acc-" + string(rune('1'+i)),
			Source:    config.SourceManaged,
			Writable:  true,
		})
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	m.Loading = false
	return m
}
