package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestApplyFlow_CursorMovementAndClamping(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()

	// Initial cursor is 0
	if m.ApplyTargetCursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.ApplyTargetCursor)
	}

	// Move down
	m.moveApplyTargetCursor(1)
	if m.ApplyTargetCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.ApplyTargetCursor)
	}

	m.moveApplyTargetCursor(1)
	if m.ApplyTargetCursor != 2 {
		t.Fatalf("expected cursor 2, got %d", m.ApplyTargetCursor)
	}

	m.moveApplyTargetCursor(1)
	if m.ApplyTargetCursor != 3 {
		t.Fatalf("expected cursor 3, got %d", m.ApplyTargetCursor)
	}

	// Move down past end (should wrap to 0)
	m.moveApplyTargetCursor(1)
	if m.ApplyTargetCursor != 0 {
		t.Fatalf("expected cursor wrap to 0, got %d", m.ApplyTargetCursor)
	}

	// Move up from 0 (should wrap to last index 3)
	m.moveApplyTargetCursor(-1)
	if m.ApplyTargetCursor != 3 {
		t.Fatalf("expected cursor wrap to 3, got %d", m.ApplyTargetCursor)
	}

	// Move with large delta (modulo behavior)
	m.moveApplyTargetCursor(10) // 3 + 10 = 13 % 4 = 1
	if m.ApplyTargetCursor != 1 {
		t.Fatalf("expected cursor 1 after delta 10, got %d", m.ApplyTargetCursor)
	}
	m.moveApplyTargetCursor(-9) // 1 - 9 = -8 % 4 = 0
	if m.ApplyTargetCursor != 0 {
		t.Fatalf("expected cursor 0 after delta -9, got %d", m.ApplyTargetCursor)
	}
}

func TestApplyFlow_ToggleAndMinOneInvariant(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()

	// 1. Unsupported source is a safe no-op
	m.toggleApplyTargetSelection("unsupported")

	// 2. Nil map initializes
	m.ApplyTargets = nil
	m.toggleApplyTargetSelection(config.SourcePi)
	if !m.ApplyTargets[config.SourcePi] {
		t.Errorf("expected Pi initialized to true from nil map")
	}

	// Set initial state
	m.ApplyTargets = map[config.Source]bool{
		config.SourceCodex:    true,
		config.SourceOpenCode: true,
		config.SourcePi:       false,
		config.SourceOMP:      false,
	}

	// Toggle Pi on (cursor at index 2)
	m.ApplyTargetCursor = 2
	m.toggleCurrentApplyTargetSelection()
	if !m.ApplyTargets[config.SourcePi] {
		t.Errorf("expected Pi toggled to true")
	}

	// Toggle Codex off (cursor at index 0)
	m.ApplyTargetCursor = 0
	m.toggleCurrentApplyTargetSelection()
	if m.ApplyTargets[config.SourceCodex] {
		t.Errorf("expected Codex toggled to false")
	}

	// Toggle OpenCode off (cursor at index 1) -> now only Pi is true (count = 1)
	m.ApplyTargetCursor = 1
	m.toggleCurrentApplyTargetSelection()
	if m.ApplyTargets[config.SourceOpenCode] {
		t.Errorf("expected OpenCode toggled to false")
	}

	// Min-one selection invariant: trying to toggle the last remaining target (Pi) off must be blocked!
	m.ApplyTargetCursor = 2
	m.toggleCurrentApplyTargetSelection()
	if !m.ApplyTargets[config.SourcePi] {
		t.Errorf("min-one invariant violated: last remaining target was toggled off!")
	}

	// Out-of-bounds cursor clamps to 0
	m.ApplyTargetCursor = -5
	m.toggleCurrentApplyTargetSelection()
	if m.ApplyTargetCursor != 0 {
		t.Errorf("expected cursor clamped to 0 from -5, got %d", m.ApplyTargetCursor)
	}

	m.ApplyTargetCursor = 100
	m.toggleCurrentApplyTargetSelection()
	if m.ApplyTargetCursor != 0 {
		t.Errorf("expected cursor clamped to 0 from 100, got %d", m.ApplyTargetCursor)
	}

	// setApplyTargetsAll from nil
	m.ApplyTargets = nil
	m.setApplyTargetsAll(true)
	if len(m.selectedApplyTargets()) != 4 {
		t.Errorf("expected all 4 targets selected after setApplyTargetsAll(true)")
	}
	m.setApplyTargetsAll(false)
	if len(m.selectedApplyTargets()) != 0 {
		t.Errorf("expected 0 targets selected after setApplyTargetsAll(false)")
	}
}

func TestApplyFlow_NumberKeyShortcuts(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()

	// Initial targets: set all false except Codex
	m.ApplyTargets = map[config.Source]bool{
		config.SourceCodex:    true,
		config.SourceOpenCode: false,
		config.SourcePi:       false,
		config.SourceOMP:      false,
	}

	// Press '1' to toggle Codex off/on when multiple selected
	m.ApplyTargets[config.SourceOpenCode] = true
	updated, _ := m.handleApplyTargetSelection("1")
	m = updated.(Model)
	if m.ApplyTargets[config.SourceCodex] {
		t.Errorf("pressing '1' should toggle Codex to false")
	}

	// Press '2' to toggle OpenCode
	updated, _ = m.handleApplyTargetSelection("2")
	m = updated.(Model)
	// Min-one prevents toggling off last remaining
	if !m.ApplyTargets[config.SourceOpenCode] {
		t.Errorf("min-one invariant should keep OpenCode true")
	}

	// Press '3' to toggle Pi
	updated, _ = m.handleApplyTargetSelection("3")
	m = updated.(Model)
	if !m.ApplyTargets[config.SourcePi] {
		t.Errorf("pressing '3' should toggle Pi to true")
	}

	// Press '4' to toggle OMP
	updated, _ = m.handleApplyTargetSelection("4")
	m = updated.(Model)
	if !m.ApplyTargets[config.SourceOMP] {
		t.Errorf("pressing '4' should toggle OMP to true")
	}

	// Press invalid numeric keys (e.g. '5', '9')
	updated, _ = m.handleApplyTargetSelection("5")
	m = updated.(Model)
	updated, _ = m.handleApplyTargetSelection("9")
	m = updated.(Model)
	updated, _ = m.handleApplyTargetSelection("unrecognized")
	m = updated.(Model)
}

func TestApplyFlow_ModalRenderingAndConfirmation(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()
	m.Width = 120

	// 1. Target Selection Modal rendering
	outSelect := ansi.Strip(m.renderApplyTargetModal())
	if !strings.Contains(outSelect, "Codex app/cli") {
		t.Errorf("expected Codex in apply modal:\n%s", outSelect)
	}
	if !strings.Contains(outSelect, "OpenCode") {
		t.Errorf("expected OpenCode in apply modal:\n%s", outSelect)
	}
	if !strings.Contains(outSelect, "Pi agent") {
		t.Errorf("expected Pi agent in apply modal:\n%s", outSelect)
	}
	if !strings.Contains(outSelect, "Oh My Pi (active account)") {
		t.Errorf("expected active Oh My Pi account in apply modal:\n%s", outSelect)
	}

	// 2. Press enter to go to Confirm modal
	updated, _ := m.handleApplyTargetSelection("enter")
	m = updated.(Model)
	if m.ApplyTargetSelect || !m.ApplyConfirm {
		t.Fatalf("expected transition to ApplyConfirm modal")
	}

	// 3. Confirm Modal rendering with explicit selections
	outConfirm := ansi.Strip(m.renderApplyConfirmModal())
	if !strings.Contains(outConfirm, "Apply this account to:") {
		t.Errorf("expected confirm prompt in modal:\n%s", outConfirm)
	}
	if !strings.Contains(outConfirm, "[enter] Confirm") {
		t.Errorf("expected enter confirmation in modal:\n%s", outConfirm)
	}

	// 4. Confirm Modal rendering with empty selections (fallback)
	mEmpty := m
	mEmpty.ApplyTargets = map[config.Source]bool{}
	outFallback := ansi.Strip(mEmpty.renderApplyConfirmModal())
	if !strings.Contains(outFallback, "codex, opencode, pi, omp") {
		t.Errorf("expected fallback labels in confirm modal with empty selections:\n%s", outFallback)
	}

	// 5. Press enter to confirm -> executes ApplyToTargetsCmd and closes modal
	updated, cmd := m.handleApplyConfirm("enter")
	m = updated.(Model)
	if m.ApplyConfirm || m.ApplyTargetSelect {
		t.Errorf("modal should be closed after confirmation")
	}
	if cmd == nil {
		t.Errorf("expected ApplyToTargetsCmd command returned on confirm")
	}

	// 6. Confirm when ApplyTargets is empty falls back to all targets
	mFallbackConfirm := testModelForHotkeys(1)
	mFallbackConfirm.startApplyFlow()
	mFallbackConfirm.ApplyTargets = nil
	mFallbackConfirm.ApplyConfirm = true
	updated, cmd = mFallbackConfirm.handleApplyConfirm("enter")
	if cmd == nil {
		t.Errorf("expected ApplyToTargetsCmd command returned with fallback targets")
	}
}

func TestApplyFlow_DefaultSelectionRespectsExistingFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi_auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp_agent.db"))

	// Case 1: When neither exists, Pi and OMP are false
	m1 := testModelForHotkeys(1)
	m1.startApplyFlow()
	if m1.ApplyTargets[config.SourcePi] {
		t.Errorf("expected Pi false when file missing")
	}
	if m1.ApplyTargets[config.SourceOMP] {
		t.Errorf("expected OMP false when file missing")
	}

	// Case 2: When Pi exists, Pi is true
	_ = os.WriteFile(filepath.Join(tmp, "pi_auth.json"), []byte(`{}`), 0o600)
	m2 := testModelForHotkeys(1)
	m2.startApplyFlow()
	if !m2.ApplyTargets[config.SourcePi] {
		t.Errorf("expected Pi true when file exists")
	}
	if m2.ApplyTargets[config.SourceOMP] {
		t.Errorf("expected OMP false when file missing")
	}

	// Case 3: When OMP exists, OMP is true
	_ = os.WriteFile(filepath.Join(tmp, "omp_agent.db"), []byte(``), 0o600)
	m3 := testModelForHotkeys(1)
	m3.startApplyFlow()
	if !m3.ApplyTargets[config.SourceOMP] {
		t.Errorf("expected OMP true when file exists")
	}
}

func TestApplyFlow_KeyHandlingAndCancel(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()

	// 1. Up and Down keys move cursor
	updated, _ := m.handleApplyTargetSelection("up")
	m = updated.(Model)
	if m.ApplyTargetCursor != 3 {
		t.Errorf("up arrow cursor = %d, want 3", m.ApplyTargetCursor)
	}

	updated, _ = m.handleApplyTargetSelection("down")
	m = updated.(Model)
	if m.ApplyTargetCursor != 0 {
		t.Errorf("down arrow cursor = %d, want 0", m.ApplyTargetCursor)
	}

	updated, _ = m.handleApplyTargetSelection("k")
	m = updated.(Model)
	if m.ApplyTargetCursor != 3 {
		t.Errorf("k key cursor = %d, want 3", m.ApplyTargetCursor)
	}

	updated, _ = m.handleApplyTargetSelection("j")
	m = updated.(Model)
	if m.ApplyTargetCursor != 0 {
		t.Errorf("j key cursor = %d, want 0", m.ApplyTargetCursor)
	}

	// 2. Space key toggles current cursor selection
	wasSelected := m.ApplyTargets[config.SourceCodex]
	updated, _ = m.handleApplyTargetSelection(" ")
	m = updated.(Model)
	if m.ApplyTargets[config.SourceCodex] == wasSelected {
		t.Errorf("space should toggle selection")
	}

	// 3. 'a' toggles all targets
	updated, _ = m.handleApplyTargetSelection("a")
	m = updated.(Model)
	if len(m.selectedApplyTargets()) != 4 {
		t.Errorf("expected all 4 selected on 'a'")
	}

	// 4. Enter with 0 selected selects all and advances to confirm
	m.ApplyTargets = map[config.Source]bool{}
	updated, _ = m.handleApplyTargetSelection("enter")
	m = updated.(Model)
	if !m.ApplyConfirm || len(m.selectedApplyTargets()) != 4 {
		t.Errorf("expected all selected and advanced to confirm")
	}

	// 5. Esc cancels apply flow
	m.startApplyFlow()
	updated, _ = m.handleApplyTargetSelection("esc")
	m = updated.(Model)
	if m.ApplyTargetSelect || m.ApplyConfirm {
		t.Errorf("esc should close apply modals")
	}

	// 6. 'q' and 'ctrl+c' quit from selection modal
	m.startApplyFlow()
	_, cmdQ := m.handleApplyTargetSelection("q")
	if cmdQ == nil {
		t.Errorf("expected quit command from 'q'")
	}
	_, cmdCtrlC := m.handleApplyTargetSelection("ctrl+c")
	if cmdCtrlC == nil {
		t.Errorf("expected quit command from 'ctrl+c'")
	}

	// 7. Confirm modal esc cancels
	m.startApplyFlow()
	updated, _ = m.handleApplyTargetSelection("enter")
	m = updated.(Model)
	if !m.ApplyConfirm {
		t.Fatalf("expected ApplyConfirm true")
	}
	updated, _ = m.handleApplyConfirm("esc")
	m = updated.(Model)
	if m.ApplyConfirm {
		t.Errorf("esc on confirm modal should close it")
	}

	// 8. Confirm modal 'q' and 'ctrl+c' quit
	m.startApplyFlow()
	m.ApplyConfirm = true
	_, cmdConfirmQ := m.handleApplyConfirm("q")
	if cmdConfirmQ == nil {
		t.Errorf("expected quit command from confirm 'q'")
	}
	_, cmdConfirmCtrlC := m.handleApplyConfirm("ctrl+c")
	if cmdConfirmCtrlC == nil {
		t.Errorf("expected quit command from confirm 'ctrl+c'")
	}

	// 9. Confirm with nil account
	mEmpty := testModelForHotkeys(0)
	mEmpty.startApplyFlow()
	mEmpty.ApplyConfirm = true
	updated, cmd := mEmpty.handleApplyConfirm("enter")
	mEmpty = updated.(Model)
	if mEmpty.ApplyConfirm || cmd != nil {
		t.Errorf("confirm with nil account should reset without command")
	}

	// 10. beginApplyFlow with nil active account
	mNil := testModelForHotkeys(0)
	updated, cmd = mNil.beginApplyFlow()
	if cmd != nil {
		t.Errorf("beginApplyFlow on empty accounts should return nil command")
	}
}

func TestApplyFlow_FormatErrorsAndHelpers(t *testing.T) {
	// 1. formatTargetErrors
	if got := formatTargetErrors(nil); got != "" {
		t.Errorf("formatTargetErrors(nil) = %q, want empty", got)
	}

	errMap := map[config.Source]error{
		config.SourceCodex:    os.ErrPermission,
		config.SourceOpenCode: nil,
		config.SourcePi:       os.ErrNotExist,
	}
	formatted := formatTargetErrors(errMap)
	if !strings.Contains(formatted, "codex:") || !strings.Contains(formatted, "pi:") {
		t.Errorf("formatTargetErrors output missing sources: %s", formatted)
	}

	// 2. mapKeysSortedBySource
	valMap := map[config.Source]string{
		config.SourceOMP:      "/path/omp",
		config.SourceCodex:    "/path/codex",
		config.SourcePi:       "/path/pi",
		config.SourceOpenCode: "/path/opencode",
		"invalid":             "/path/invalid",
	}
	sorted := mapKeysSortedBySource(valMap)
	expectedOrder := []config.Source{config.SourceCodex, config.SourceOpenCode, config.SourcePi, config.SourceOMP}
	for i, src := range expectedOrder {
		if i >= len(sorted) || sorted[i] != src {
			t.Errorf("sorted[%d] = %v, want %v", i, sorted[i], src)
		}
	}

	// 3. sourceFromLabel & sourceDisplayName & sourceListText
	labelTests := []struct {
		label    string
		wantSrc  config.Source
		wantOK   bool
		wantDisp string
	}{
		{"app", config.SourceManaged, true, "app"},
		{"managed", config.SourceManaged, true, "app"},
		{"codex", config.SourceCodex, true, "codex"},
		{"opencode", config.SourceOpenCode, true, "opencode"},
		{"pi", config.SourcePi, true, "pi"},
		{"omp", config.SourceOMP, true, "omp"},
		{"unknown", "", false, "unknown"},
	}
	for _, tt := range labelTests {
		src, ok := sourceFromLabel(tt.label)
		if ok != tt.wantOK || src != tt.wantSrc {
			t.Errorf("sourceFromLabel(%q) = (%v, %v), want (%v, %v)", tt.label, src, ok, tt.wantSrc, tt.wantOK)
		}
		disp := sourceDisplayName(config.Source(tt.label))
		if disp != tt.wantDisp {
			t.Errorf("sourceDisplayName(%q) = %q, want %q", tt.label, disp, tt.wantDisp)
		}
	}

	if got := sourceListText(nil); got != "n/a" {
		t.Errorf("sourceListText(nil) = %q, want n/a", got)
	}
	if got := sourceListText([]config.Source{config.SourceCodex, config.SourcePi}); got != "codex, pi" {
		t.Errorf("sourceListText = %q, want 'codex, pi'", got)
	}

	// 4. dedupeSources
	deduped := dedupeSources([]config.Source{config.SourceCodex, config.SourceCodex, "invalid", config.SourcePi})
	if len(deduped) != 2 {
		t.Errorf("dedupeSources len = %d, want 2", len(deduped))
	}
}

func TestApplyToTargetsCmd_Execution(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi", "auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	acct := &config.Account{
		AccessToken:  "tok-apply-cmd",
		RefreshToken: "ref-apply-cmd",
		AccountID:    "acc-cmd",
		Email:        "cmd@example.com",
		Source:       config.SourceManaged,
		Writable:     true,
	}

	// 1. Apply to all targets via command
	cmd := ApplyToTargetsCmd(acct, []config.Source{config.SourceCodex, config.SourceOpenCode, config.SourcePi, config.SourceOMP})
	if cmd == nil {
		t.Fatalf("expected command from ApplyToTargetsCmd")
	}
	msg := cmd()
	if accountsMsg, ok := msg.(AccountsMsg); ok {
		if len(accountsMsg.Accounts) == 0 {
			t.Errorf("expected loaded accounts in AccountsMsg")
		}
	} else if errMsg, ok := msg.(ErrMsg); ok {
		t.Errorf("unexpected ErrMsg from ApplyToTargetsCmd: %v", errMsg.Err)
	}

	// 2. ApplyToTargetsCmd with nil account
	nilCmd := ApplyToTargetsCmd(nil, []config.Source{config.SourceCodex})
	if nilCmd != nil {
		t.Errorf("ApplyToTargetsCmd(nil) should return nil command")
	}

	// 3. ApplyToTargetsCmd with empty targets
	emptyCmd := ApplyToTargetsCmd(acct, []config.Source{})
	msgEmpty := emptyCmd()
	if errMsg, ok := msgEmpty.(ErrMsg); !ok {
		t.Errorf("expected ErrMsg for empty targets, got: %T", msgEmpty)
	} else if !strings.Contains(errMsg.Err.Error(), "no apply target selected") {
		t.Errorf("expected 'no apply target selected' error, got: %v", errMsg.Err)
	}
}

func TestBadges_EdgeCasesAndEmptySources(t *testing.T) {
	// 1. nil account returns empty
	m := testModelForHotkeys(1)
	if got := m.activeSourceBadgesForAccount(nil); got != "" {
		t.Errorf("expected empty badges for nil account, got %q", got)
	}

	// 2. Empty ActiveSourcesByIdentity returns empty
	m.ActiveSourcesByIdentity = nil
	acct := &config.Account{Key: "acc-1", AccountID: "acc-1"}
	if got := m.activeSourceBadgesForAccount(acct); got != "" {
		t.Errorf("expected empty badges when ActiveSourcesByIdentity is nil, got %q", got)
	}

	// 3. Account with no active sources returns empty
	m.ActiveSourcesByIdentity = map[string][]string{
		"account:acc-other": {"codex"},
	}
	if got := m.activeSourceBadgesForAccount(acct); got != "" {
		t.Errorf("expected empty badges when account not active in any source, got %q", got)
	}

	// 4. renderActiveSourceBadges with empty string returns empty
	if got := m.renderActiveSourceBadges(acct, true); got != "" {
		t.Errorf("expected empty rendered badges when no sources active, got %q", got)
	}

	// 5. renderActiveSourceBadges with inactive row style
	m.ActiveSourcesByIdentity = map[string][]string{
		"account:acc-1": {"codex", "opencode", "pi", "omp"},
	}
	outActive := m.renderActiveSourceBadges(acct, true)
	outMuted := m.renderActiveSourceBadges(acct, false)
	if outActive == "" || outMuted == "" {
		t.Errorf("expected rendered badges for all 4 sources")
	}
	if !strings.Contains(outActive, "C") || !strings.Contains(outActive, "O") || !strings.Contains(outActive, "P") || !strings.Contains(outActive, "M") {
		t.Errorf("expected all 4 letters in rendered badges, got: %s", outActive)
	}
	if !strings.Contains(outMuted, "C") || !strings.Contains(outMuted, "O") || !strings.Contains(outMuted, "P") || !strings.Contains(outMuted, "M") {
		t.Errorf("expected all 4 letters in muted rendered badges, got: %s", outMuted)
	}
}
