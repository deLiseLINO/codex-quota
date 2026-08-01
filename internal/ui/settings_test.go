package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestSOpensAndClosesSettingsScreen(t *testing.T) {
	m := testModelForHotkeys(1)

	updated, _ := m.Update(keyRune('s'))
	got := updated.(Model)
	if !got.SettingsVisible {
		t.Fatalf("expected settings screen to open on s")
	}
	if got.HelpVisible || got.ActionMenuVisible {
		t.Fatalf("did not expect other overlays to open")
	}

	closed, _ := got.Update(keyRune('s'))
	gotClosed := closed.(Model)
	if gotClosed.SettingsVisible {
		t.Fatalf("expected settings screen to close on s")
	}
}

func TestEscClosesSettingsScreen(t *testing.T) {
	m := testModelForHotkeys(1)
	m.openSettingsOverlay()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.SettingsVisible {
		t.Fatalf("expected settings screen to close on esc")
	}
}

func TestQuestionMarkIgnoredWhileSettingsOpen(t *testing.T) {
	m := testModelForHotkeys(1)
	m.openSettingsOverlay()

	updated, _ := m.Update(keyRune('?'))
	got := updated.(Model)
	if !got.SettingsVisible {
		t.Fatalf("expected settings screen to stay open")
	}
	if got.HelpVisible {
		t.Fatalf("did not expect help to open while settings open")
	}
}

func TestSettingsSwallowsOtherHotkeys(t *testing.T) {
	m := testModelForHotkeys(2)
	m.openSettingsOverlay()

	updated, _ := m.Update(keyRune('x'))
	got := updated.(Model)
	if got.DeleteSourceSelect || got.DeleteConfirm {
		t.Fatalf("expected delete flow not to open while settings open")
	}
	if !got.SettingsVisible {
		t.Fatalf("expected settings screen to remain open")
	}
}

func TestSettingsToggleRowsFlipOnEnterAndSpace(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:      true,
		ActiveIntervalSec:       30,
		BackgroundIntervalSec:   300,
		CheckForUpdateOnStartup: true,
	}
	m.openSettingsOverlay()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(Model)
	if got.Settings.AutoRefreshEnabled {
		t.Fatalf("expected auto-refresh to flip to false on space")
	}

	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got2 := updated2.(Model)
	if !got2.Settings.AutoRefreshEnabled {
		t.Fatalf("expected auto-refresh to flip back to true on enter")
	}

	for i := 0; i < 3; i++ {
		moved, _ := got2.Update(tea.KeyMsg{Type: tea.KeyDown})
		got2 = moved.(Model)
	}
	if got2.settingsCursor != 3 {
		t.Fatalf("expected cursor on update-check row, got %d", got2.settingsCursor)
	}

	toggled, _ := got2.Update(tea.KeyMsg{Type: tea.KeySpace})
	got3 := toggled.(Model)
	if got3.Settings.CheckForUpdateOnStartup {
		t.Fatalf("expected update-check to flip to false on space")
	}
}

func TestSettingsArrowStepsInterval(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:      true,
		ActiveIntervalSec:       30,
		BackgroundIntervalSec:   300,
		CheckForUpdateOnStartup: true,
	}
	m.openSettingsOverlay()

	moved, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := moved.(Model)
	if got.settingsCursor != 1 {
		t.Fatalf("expected cursor on active interval row, got %d", got.settingsCursor)
	}

	right, _ := got.Update(tea.KeyMsg{Type: tea.KeyRight})
	got2 := right.(Model)
	if got2.Settings.ActiveIntervalSec != 35 {
		t.Fatalf("expected active interval 35 after right step, got %d", got2.Settings.ActiveIntervalSec)
	}

	left, _ := got2.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got3 := left.(Model)
	if got3.Settings.ActiveIntervalSec != 30 {
		t.Fatalf("expected active interval 30 after left step, got %d", got3.Settings.ActiveIntervalSec)
	}

	moved2, _ := got3.Update(tea.KeyMsg{Type: tea.KeyDown})
	got4 := moved2.(Model)
	right2, _ := got4.Update(tea.KeyMsg{Type: tea.KeyRight})
	got5 := right2.(Model)
	if got5.Settings.BackgroundIntervalSec != 360 {
		t.Fatalf("expected background interval 360 after right step, got %d", got5.Settings.BackgroundIntervalSec)
	}
}

func TestSettingsArrowStepsClampAtBounds(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:    true,
		ActiveIntervalSec:     config.ActiveIntervalMinSec,
		BackgroundIntervalSec: config.BackgroundIntervalMinSec,
	}
	m.openSettingsOverlay()

	moved, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := moved.(Model)
	left, _ := got.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got2 := left.(Model)
	if got2.Settings.ActiveIntervalSec != config.ActiveIntervalMinSec {
		t.Fatalf("expected active interval clamped at min, got %d", got2.Settings.ActiveIntervalSec)
	}
}

func TestSettingsDigitEntrySetsInterval(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:    true,
		ActiveIntervalSec:     30,
		BackgroundIntervalSec: 300,
	}
	m.openSettingsOverlay()

	moved, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := moved.(Model)

	updated, _ := got.Update(keyRune('1'))
	got2 := updated.(Model)
	if got2.Settings.ActiveIntervalSec != config.ActiveIntervalMinSec {
		t.Fatalf("expected active interval clamped at min after typing 1, got %d", got2.Settings.ActiveIntervalSec)
	}

	updated2, _ := got2.Update(keyRune('5'))
	got3 := updated2.(Model)
	if got3.Settings.ActiveIntervalSec != 15 {
		t.Fatalf("expected active interval 15 after typing 15, got %d", got3.Settings.ActiveIntervalSec)
	}

	updated3, _ := got3.Update(keyRune('5'))
	got4 := updated3.(Model)
	if got4.Settings.ActiveIntervalSec != 155 {
		t.Fatalf("expected active interval 155 after typing 155, got %d", got4.Settings.ActiveIntervalSec)
	}
}

func TestSettingsDigitEntryClampsToMax(t *testing.T) {
	m := testModelForHotkeys(1)
	m.openSettingsOverlay()

	moved, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := moved.(Model)
	for _, d := range []rune{'9', '9', '9'} {
		updated, _ := got.Update(keyRune(d))
		got = updated.(Model)
	}
	if got.Settings.ActiveIntervalSec != config.ActiveIntervalMaxSec {
		t.Fatalf("expected active interval clamped at max, got %d", got.Settings.ActiveIntervalSec)
	}
}

func TestSettingsNextDigitAfterArrowStepsRestartsDraft(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:    true,
		ActiveIntervalSec:     30,
		BackgroundIntervalSec: 300,
	}
	m.openSettingsOverlay()

	moved, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := moved.(Model)
	typed, _ := got.Update(keyRune('1'))
	got2 := typed.(Model)
	arrowed, _ := got2.Update(tea.KeyMsg{Type: tea.KeyRight})
	got3 := arrowed.(Model)

	typed2, _ := got3.Update(keyRune('2'))
	got4 := typed2.(Model)
	if got4.Settings.ActiveIntervalSec != config.ActiveIntervalMinSec {
		t.Fatalf("expected fresh draft starting at min after arrow step, got %d", got4.Settings.ActiveIntervalSec)
	}
}

func TestSettingsChangeAutoSaves(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:      true,
		ActiveIntervalSec:       30,
		BackgroundIntervalSec:   300,
		CheckForUpdateOnStartup: true,
	}
	m.openSettingsOverlay()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if updated.(Model).Settings.AutoRefreshEnabled {
		t.Fatalf("expected auto-refresh to flip to false")
	}
	if cmd == nil {
		t.Fatalf("expected save command on settings change")
	}
	cmd()

	loaded, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if loaded.AutoRefreshEnabled {
		t.Fatalf("expected persisted auto-refresh disabled, got enabled")
	}
}

func TestRenderSettingsModalShowsRows(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:      true,
		ActiveIntervalSec:       30,
		BackgroundIntervalSec:   300,
		CheckForUpdateOnStartup: true,
	}
	m.openSettingsOverlay()

	out := ansi.Strip(m.renderSettingsModal())
	if !strings.Contains(out, "Settings") {
		t.Fatalf("expected settings title in modal:\n%s", out)
	}
	if !strings.Contains(out, "Auto-refresh") {
		t.Fatalf("expected auto-refresh row in modal:\n%s", out)
	}
	if !strings.Contains(out, "Active refresh interval") {
		t.Fatalf("expected active interval row in modal:\n%s", out)
	}
	if !strings.Contains(out, "Background refresh interval") {
		t.Fatalf("expected background interval row in modal:\n%s", out)
	}
	if !strings.Contains(out, "Check for updates on startup") {
		t.Fatalf("expected update-check row in modal:\n%s", out)
	}
	if !strings.Contains(out, "30s") {
		t.Fatalf("expected human-readable active label in modal:\n%s", out)
	}
	if !strings.Contains(out, "5m") {
		t.Fatalf("expected human-readable background label in modal:\n%s", out)
	}
	if count := strings.Count(out, "[x]"); count != 2 {
		t.Fatalf("expected 2 checked toggles, got %d:\n%s", count, out)
	}
}

func TestRenderSettingsModalShowsOffToggleState(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.Settings{
		AutoRefreshEnabled:      false,
		ActiveIntervalSec:       30,
		BackgroundIntervalSec:   300,
		CheckForUpdateOnStartup: false,
	}
	m.openSettingsOverlay()

	out := ansi.Strip(m.renderSettingsModal())
	if count := strings.Count(out, "[ ]"); count != 2 {
		t.Fatalf("expected 2 unchecked toggles, got %d:\n%s", count, out)
	}
}

func TestFormatIntervalLabel(t *testing.T) {
	cases := []struct {
		secs int
		want string
	}{
		{30, "30s"},
		{300, "5m"},
		{90, "1m"},
		{3600, "1h"},
		{5400, "1h 30m"},
	}
	for _, tc := range cases {
		if got := formatIntervalLabel(tc.secs); got != tc.want {
			t.Fatalf("formatIntervalLabel(%d) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}
