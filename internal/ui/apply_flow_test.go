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
}

func TestApplyFlow_ToggleAndMinOneInvariant(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()

	// Initially Codex and OpenCode are selected
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

	// Select all via 'a'
	m.setApplyTargetsAll(true)
	if len(m.selectedApplyTargets()) != 4 {
		t.Errorf("expected all 4 targets selected after setApplyTargetsAll(true)")
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

	// Press '2' to toggle OpenCode
	updated, _ := m.handleApplyTargetSelection("2")
	m = updated.(Model)
	if !m.ApplyTargets[config.SourceOpenCode] {
		t.Errorf("pressing '2' should toggle OpenCode to true")
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
	if !strings.Contains(outSelect, "Oh My Pi (pool/profile)") {
		t.Errorf("expected Oh My Pi (pool/profile) in apply modal:\n%s", outSelect)
	}

	// 2. Press enter to go to Confirm modal
	updated, _ := m.handleApplyTargetSelection("enter")
	m = updated.(Model)
	if m.ApplyTargetSelect || !m.ApplyConfirm {
		t.Fatalf("expected transition to ApplyConfirm modal")
	}

	// 3. Confirm Modal rendering
	outConfirm := ansi.Strip(m.renderApplyConfirmModal())
	if !strings.Contains(outConfirm, "Apply this account to:") {
		t.Errorf("expected confirm prompt in modal:\n%s", outConfirm)
	}
	if !strings.Contains(outConfirm, "[enter] Confirm") {
		t.Errorf("expected enter confirmation in modal:\n%s", outConfirm)
	}

	// 4. Press enter to confirm -> executes ApplyToTargetsCmd and closes modal
	updated, cmd := m.handleApplyConfirm("enter")
	m = updated.(Model)
	if m.ApplyConfirm || m.ApplyTargetSelect {
		t.Errorf("modal should be closed after confirmation")
	}
	if cmd == nil {
		t.Errorf("expected ApplyToTargetsCmd command returned on confirm")
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
