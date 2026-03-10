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

func maxOverlayLineWidth(s string) int {
	maxWidth := 0
	for _, line := range strings.Split(s, "\n") {
		if width := ansi.StringWidth(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
