package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/config"
	"github.com/deLiseLINO/codex-quota/internal/update"
)

func TestInitialModelWithStartupUpdateShowsPrompt(t *testing.T) {
	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, &StartupUpdatePrompt{
		Version: "0.1.5",
		Method:  update.MethodGo,
	})

	if !m.UpdatePromptVisible {
		t.Fatalf("UpdatePromptVisible = false, want true")
	}
	if m.UpdatePromptVersion != "0.1.5" {
		t.Fatalf("UpdatePromptVersion = %q, want %q", m.UpdatePromptVersion, "0.1.5")
	}
	if m.UpdateAvailableHint != "" {
		t.Fatalf("UpdateAvailableHint = %q, want empty", m.UpdateAvailableHint)
	}
}

func TestUpdatePromptEnterQueuesUpdateAndQuit(t *testing.T) {
	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, &StartupUpdatePrompt{
		Version: "0.1.5",
		Method:  update.MethodGo,
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.UpdatePromptVisible {
		t.Fatalf("UpdatePromptVisible = true, want false")
	}

	method, ok := got.PendingUpdate()
	if !ok || method != update.MethodGo {
		t.Fatalf("PendingUpdate() = (%s, %v), want (%s, true)", method, ok, update.MethodGo)
	}

	if msg := cmd(); msg == nil {
		t.Fatalf("cmd() = nil, want quit message")
	}
}

func TestUpdatePromptDismissPersistsVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, &StartupUpdatePrompt{
		Version: "0.1.5",
		Method:  update.MethodBrew,
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.UpdatePromptVisible {
		t.Fatalf("UpdatePromptVisible = true, want false")
	}
	if got.UpdateAvailableHint != "" {
		t.Fatalf("UpdateAvailableHint = %q, want empty", got.UpdateAvailableHint)
	}
	if cmd != nil {
		_ = cmd()
	}

	state, err := config.LoadUpdateState()
	if err != nil {
		t.Fatalf("LoadUpdateState() error = %v", err)
	}
	if state.DismissedVersion != "0.1.5" {
		t.Fatalf("DismissedVersion = %q, want %q", state.DismissedVersion, "0.1.5")
	}
}

func TestUpdatePromptSkipKeepsHint(t *testing.T) {
	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, &StartupUpdatePrompt{
		Version: "0.1.5",
		Method:  update.MethodGo,
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.UpdatePromptVisible {
		t.Fatalf("UpdatePromptVisible = true, want false")
	}
	if got.UpdateAvailableHint != "Update available • press u" {
		t.Fatalf("UpdateAvailableHint = %q, want expected hint", got.UpdateAvailableHint)
	}
	if cmd != nil {
		t.Fatalf("cmd != nil, want nil")
	}
}

func TestUpdateAvailableMsgShowsHint(t *testing.T) {
	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, nil)

	updated, _ := m.Update(UpdateAvailableMsg{Version: "0.1.5", Method: update.MethodGo})
	got := updated.(Model)
	if got.UpdatePromptVersion != "0.1.5" {
		t.Fatalf("UpdatePromptVersion = %q, want %q", got.UpdatePromptVersion, "0.1.5")
	}
	if got.UpdatePromptMethod != update.MethodGo {
		t.Fatalf("UpdatePromptMethod = %q, want %q", got.UpdatePromptMethod, update.MethodGo)
	}
	if got.UpdateAvailableHint != "Update available • press u" {
		t.Fatalf("UpdateAvailableHint = %q, want expected hint", got.UpdateAvailableHint)
	}
}

func TestRenderUpdatePromptModalExpandsInWideViewport(t *testing.T) {
	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, &StartupUpdatePrompt{
		Version: "0.1.4",
		Method:  update.MethodGo,
	})
	m.Width = 160

	out := ansi.Strip(m.renderUpdatePromptModal())
	if !strings.Contains(out, "Release notes: https://github.com/deLiseLINO/codex-quota/releases/tag/v0.1.4") {
		t.Fatalf("expected full release notes line in modal:\n%s", out)
	}
	if !strings.Contains(out, "1. Update now (runs `go install github.com/deLiseLINO/codex-quota/cmd/cq@v0.1.4`)") {
		t.Fatalf("expected full update command line in modal:\n%s", out)
	}
	if width := maxLineWidth(out); width > updatePromptModalMaxWidth {
		t.Fatalf("modal width = %d, want <= %d", width, updatePromptModalMaxWidth)
	}
}

func TestRenderUpdatePromptModalShrinksToViewport(t *testing.T) {
	m := InitialModelWithStartupUpdate(nil, nil, nil, config.UIState{}, &StartupUpdatePrompt{
		Version: "0.1.4",
		Method:  update.MethodGo,
	})
	m.Width = 80

	out := ansi.Strip(m.renderUpdatePromptModal())
	if width := maxLineWidth(out); width > m.Width-updatePromptViewportInset {
		t.Fatalf("modal width = %d, want <= %d\n%s", width, m.Width-updatePromptViewportInset, out)
	}
}

func maxLineWidth(s string) int {
	maxWidth := 0
	for _, line := range strings.Split(s, "\n") {
		if width := ansi.StringWidth(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
