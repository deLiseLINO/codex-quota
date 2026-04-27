package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderMessageModalWrapsLongErrorWithoutTruncation(t *testing.T) {
	message := `token refresh failed: refresh failed with status 403: {"error":{"code":"unsupported_country","message":"this request cannot be completed"}}`

	out := ansi.Strip(renderMessageModal("Error", message, ErrorStyle, 120))
	if strings.Contains(out, "...") {
		t.Fatalf("expected full wrapped message without truncation:\n%s", out)
	}
	if !strings.Contains(out, "unsupported_country") {
		t.Fatalf("expected error details to remain visible:\n%s", out)
	}
	if !strings.Contains(out, "status 403") {
		t.Fatalf("expected status code in modal:\n%s", out)
	}
}

func TestRenderMessageModalStaysWithinViewport(t *testing.T) {
	message := `token refresh failed: refresh failed with status 403: {"error":{"code":"unsupported_country","message":"this request cannot be completed"}}`

	out := ansi.Strip(renderMessageModal("Error", message, ErrorStyle, 80))
	if width := maxOverlayLineWidth(out); width > 80-messageModalInset {
		t.Fatalf("modal width = %d, want <= %d\n%s", width, 80-messageModalInset, out)
	}
}

func TestRenderMessageModalKeepsShortNoticeCompact(t *testing.T) {
	out := ansi.Strip(renderMessageModal("Notice", "saved", NoticeStyle, 120))
	if !strings.Contains(out, "saved") {
		t.Fatalf("expected notice text in modal:\n%s", out)
	}
	if width := maxOverlayLineWidth(out); width < messageModalMinWidth {
		t.Fatalf("modal width = %d, want >= %d", width, messageModalMinWidth)
	}
}

func TestCurrentOverlayModal_ErrorIncludesDismissHint(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Err = assertErr("token refresh failed")

	out := ansi.Strip(m.currentOverlayModal())
	if !strings.Contains(out, "token refresh failed") {
		t.Fatalf("expected original error text in modal:\n%s", out)
	}
	if !strings.Contains(out, "[enter/esc] Close") {
		t.Fatalf("expected close hint in error modal:\n%s", out)
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }

func assertErr(v string) error { return testErr(v) }

func TestRenderHelpModalShowsGroupedSections(t *testing.T) {
	m := testModelForHotkeys(1)

	out := ansi.Strip(m.renderHelpModal())
	if !strings.Contains(out, "Keyboard help") {
		t.Fatalf("expected help title in modal:\n%s", out)
	}
	if !strings.Contains(out, "Primary") {
		t.Fatalf("expected primary section in help modal:\n%s", out)
	}
	if strings.Contains(out, "Actions menu") || strings.Contains(out, "Aliases") || strings.Contains(out, "Modal controls") {
		t.Fatalf("did not expect removed sections in help modal:\n%s", out)
	}
	if !strings.Contains(out, "Open account menu") {
		t.Fatalf("expected menu guidance in help modal:\n%s", out)
	}
	if !strings.Contains(out, "Refresh all accounts") {
		t.Fatalf("expected refresh all guidance in help modal:\n%s", out)
	}
	if !strings.Contains(out, "o          Apply to Codex/OpenCode/Pi") {
		t.Fatalf("expected apply action in primary help:\n%s", out)
	}
	if strings.Contains(out, "Use the actions menu for secondary tasks") {
		t.Fatalf("did not expect explanatory note in help modal:\n%s", out)
	}
}

func TestRenderAddAccountLoginModalShowsAuthInstructions(t *testing.T) {
	m := testModelForHotkeys(1)
	m.AddAccountLoginVisible = true
	m.AddAccountLoginURL = "https://auth.openai.com/oauth/authorize?client_id=test"
	m.AddAccountBrowserFailed = true
	m.Width = 120

	out := ansi.Strip(m.renderAddAccountLoginModal())
	if !strings.Contains(out, "Connect ChatGPT account") {
		t.Fatalf("expected login modal title:\n%s", out)
	}
	if !strings.Contains(out, "Complete authorization in your browser.") {
		t.Fatalf("expected browser instructions:\n%s", out)
	}
	if !strings.Contains(out, "If your browser did not open, open this URL manually:") {
		t.Fatalf("expected manual open instructions:\n%s", out)
	}
	if !strings.Contains(out, "Waiting for authorization...") {
		t.Fatalf("expected waiting status:\n%s", out)
	}
	if !strings.Contains(out, "[c] Copy   [esc] Cancel") {
		t.Fatalf("expected modal hotkeys:\n%s", out)
	}
	if !strings.Contains(out, "https:") || !strings.Contains(out, "auth.openai.com") || !strings.Contains(out, "client_id") {
		t.Fatalf("expected auth URL in login modal:\n%s", out)
	}
}

func TestRenderActionMenuModalListsPrimaryActions(t *testing.T) {
	m := testModelForHotkeys(1)
	m.ActionMenuVisible = true

	out := ansi.Strip(m.renderActionMenuModal())
	if !strings.Contains(out, "Account actions") {
		t.Fatalf("expected action menu title:\n%s", out)
	}
	if !strings.Contains(out, "Current account") || !strings.Contains(out, "Global actions") {
		t.Fatalf("expected grouped action menu sections:\n%s", out)
	}
	if !strings.Contains(out, "Apply to Codex/OpenCode/Pi") || !strings.Contains(out, "Delete account") {
		t.Fatalf("expected account actions in menu:\n%s", out)
	}
	if !strings.Contains(out, "Refresh all") || !strings.Contains(out, "Switch view") || !strings.Contains(out, "Add account") {
		t.Fatalf("expected global actions in menu:\n%s", out)
	}
	if !strings.Contains(out, "1. Apply to Codex/OpenCode/Pi") || !strings.Contains(out, "5. Refresh all") {
		t.Fatalf("expected sequential numbering across sections:\n%s", out)
	}
}

func TestRenderActionMenuModalShowsInstallUpdateInGlobalActions(t *testing.T) {
	m := testModelForHotkeys(1)
	m.ActionMenuVisible = true
	m.UpdatePromptVersion = "1.2.3"
	m.UpdatePromptMethod = "brew"

	out := ansi.Strip(m.renderActionMenuModal())
	if !strings.Contains(out, "Install update") {
		t.Fatalf("expected install update action in menu:\n%s", out)
	}
	if !strings.Contains(out, "Global actions") {
		t.Fatalf("expected global actions section in menu:\n%s", out)
	}
}

func TestRenderActionMenuModalKeepsShortcutColumnAlignedForLongLabels(t *testing.T) {
	m := testModelForHotkeys(1)
	m.ActionMenuVisible = true

	out := ansi.Strip(m.renderActionMenuModal())
	lines := strings.Split(out, "\n")

	applyLine := ""
	switchViewLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Apply to Codex/OpenCode/Pi") {
			applyLine = line
		}
		if strings.Contains(line, "Switch view") {
			switchViewLine = line
		}
	}

	if applyLine == "" || switchViewLine == "" {
		t.Fatalf("expected both action lines in menu:\n%s", out)
	}

	applyShortcutPos := strings.LastIndex(applyLine, " o")
	switchShortcutPos := strings.LastIndex(switchViewLine, " v")
	if applyShortcutPos < 0 || switchShortcutPos < 0 {
		t.Fatalf("expected aligned shortcut suffixes in menu:\n%s", out)
	}
	if applyShortcutPos != switchShortcutPos {
		t.Fatalf("expected shortcut column alignment, got %d vs %d\n%s", applyShortcutPos, switchShortcutPos, out)
	}
}

func maxOverlayLineWidth(s string) int {
	maxWidth := 0
	for _, line := range strings.Split(s, "\n") {
		if width := ansi.StringWidth(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
