package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestRenderWindowsViewSingleWindowHasGroupHeader(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})

	out := ansi.Strip(model.renderWindowsView())
	hasWeeklyHeader := false
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "Weekly" {
			hasWeeklyHeader = true
			break
		}
	}
	if !hasWeeklyHeader {
		t.Fatalf("expected standalone group header for single window output:\n%s", out)
	}
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected window label in output, got:\n%s", out)
	}
}

func TestRenderWindowsViewMultipleWindowsKeepsGroupHeaders(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "5 hour usage limit",
			WindowSec:   18000,
			LeftPercent: 10.0,
			ResetAt:     time.Now().Add(1 * time.Hour),
		},
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 60.0,
			ResetAt:     time.Now().Add(24 * time.Hour),
		},
	})

	out := ansi.Strip(model.renderWindowsView())
	hasFiveHourHeader := false
	hasWeeklyHeader := false
	for _, line := range strings.Split(out, "\n") {
		switch strings.TrimSpace(line) {
		case "5 hour":
			hasFiveHourHeader = true
		case "Weekly":
			hasWeeklyHeader = true
		}
	}
	if !hasFiveHourHeader {
		t.Fatalf("expected 5 hour group header, got:\n%s", out)
	}
	if !hasWeeklyHeader {
		t.Fatalf("expected Weekly group header, got:\n%s", out)
	}
	if !strings.Contains(out, "5 hour usage limit") || !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected both window labels in output, got:\n%s", out)
	}
}

func TestRenderWindowsView_CentersWindowHeaderOverBar(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 60.0,
			ResetAt:     time.Now().Add(24 * time.Hour),
		},
	})
	model.Width = 180

	out := ansi.Strip(model.renderWindowsView())
	lines := strings.Split(out, "\n")
	headerIx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "Weekly" {
			headerIx = i
			break
		}
	}
	if headerIx < 0 || headerIx+1 >= len(lines) {
		t.Fatalf("expected Weekly header followed by row, got:\n%s", out)
	}
	headerLine := lines[headerIx]
	rowLine := lines[headerIx+1]

	headerStart := strings.Index(headerLine, "Weekly")

	nameWidth, barWidth, _, _ := model.windowRowLayout(604800)
	barStart := model.windowLeadOffset(604800) + ansi.StringWidth(windowRowIndent) + nameWidth + 1
	headerCenter := headerStart + (len("Weekly") / 2)
	barCenter := barStart + (barWidth / 2)
	if delta := headerCenter - barCenter; delta < -1 || delta > 1 {
		t.Fatalf("header not centered over bar: headerCenter=%d barCenter=%d row=%q", headerCenter, barCenter, rowLine)
	}
}

func TestViewCentersContentInLargeViewport(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.Width = 180
	model.Height = 40

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")

	titleLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Codex Quota") {
			titleLine = line
			break
		}
	}
	if titleLine == "" {
		t.Fatalf("title line not found in rendered output")
	}
	if !strings.HasPrefix(titleLine, "  ") {
		t.Fatalf("expected centered line with left padding, got: %q", titleLine)
	}
}

func TestViewHeaderHintKeepsVisibleGapFromTitle(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.Width = 180
	model.Height = 40
	model.UpdateAvailableHint = "Update available • press u"

	out := ansi.Strip(model.View())
	for _, line := range strings.Split(out, "\n") {
		titlePos := strings.Index(line, "Codex Quota")
		hintPos := strings.Index(line, "Update available")
		if titlePos >= 0 && hintPos >= 0 {
			titleEnd := titlePos + len("Codex Quota")
			if hintPos-titleEnd < headerUpdateHintGap {
				t.Fatalf("header hint gap = %d, want >= %d in line %q", hintPos-titleEnd, headerUpdateHintGap, line)
			}
			return
		}
	}
	t.Fatalf("expected title and hint on the same line in rendered output:\n%s", out)
}

func TestViewTabModeHintAddsBlankLineBeforeTabs(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.Width = 180
	model.Height = 40
	model.CompactMode = false
	model.UpdateAvailableHint = "Update available • press u"

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")

	titleIdx := -1
	tabsIdx := -1
	for i, line := range lines {
		if titleIdx == -1 && strings.Contains(line, "Codex Quota") {
			titleIdx = i
		}
		if tabsIdx == -1 && strings.Contains(line, "user@example.com") {
			tabsIdx = i
		}
	}

	if titleIdx == -1 || tabsIdx == -1 {
		t.Fatalf("expected both title and tabs in output:\n%s", out)
	}
	if tabsIdx-titleIdx < 2 {
		t.Fatalf("expected at least one blank line between header and tabs, got titleIdx=%d tabsIdx=%d\n%s", titleIdx, tabsIdx, out)
	}
}

func TestViewTabModeAddsBlankLineBeforeTabsWithoutHint(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.Width = 180
	model.Height = 40
	model.CompactMode = false

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")

	titleIdx := -1
	tabsIdx := -1
	for i, line := range lines {
		if titleIdx == -1 && strings.Contains(line, "Codex Quota") {
			titleIdx = i
		}
		if tabsIdx == -1 && strings.Contains(line, "user@example.com") {
			tabsIdx = i
		}
	}

	if titleIdx == -1 || tabsIdx == -1 {
		t.Fatalf("expected both title and tabs in output:\n%s", out)
	}
	if tabsIdx-titleIdx < 2 {
		t.Fatalf("expected at least one blank line between header and tabs without hint, got titleIdx=%d tabsIdx=%d\n%s", titleIdx, tabsIdx, out)
	}
}

func TestViewTabModeLoadingRendersWeeklyOnlyForUnknownOrFreePlan(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = false
	model.Loading = true

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected weekly loading skeleton row in tab mode output:\n%s", out)
	}
	if strings.Contains(out, "5 hour usage limit") {
		t.Fatalf("did not expect 5 hour loading skeleton row for unknown/free plan:\n%s", out)
	}
	if !strings.Contains(out, "Loading...") {
		t.Fatalf("expected loading status in tab mode skeleton output:\n%s", out)
	}
}

func TestViewTabModeLoadingRendersWeeklyOnlyForKnownFreePlan(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = false
	model.Loading = true
	model.PlanTypeByAccount = map[string]string{
		"account-1": "free",
	}

	out := ansi.Strip(model.View())
	if strings.Contains(out, "5 hour usage limit") {
		t.Fatalf("did not expect 5 hour loading skeleton for known free plan:\n%s", out)
	}
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected weekly loading skeleton row in tab mode output:\n%s", out)
	}
}

func TestViewTabModeLoadingRenders5HourForKnownSubscription(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = false
	model.Loading = true
	model.PlanTypeByAccount = map[string]string{
		"account-1": "pro",
	}

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "5 hour usage limit") {
		t.Fatalf("expected 5 hour loading skeleton for known subscription:\n%s", out)
	}
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected weekly loading skeleton row in tab mode output:\n%s", out)
	}
}

func TestViewUsesCompactFooterOnMainScreen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "←→ Move") || !strings.Contains(out, "v View") || !strings.Contains(out, "Enter Menu") || !strings.Contains(out, "? Help") {
		t.Fatalf("expected compact footer with primary actions:\n%s", out)
	}
	if strings.Contains(out, "r Refresh") || strings.Contains(out, "R All") {
		t.Fatalf("did not expect refresh hints in footer:\n%s", out)
	}
	if strings.Contains(out, "o Apply") {
		t.Fatalf("did not expect apply shortcut in footer:\n%s", out)
	}
	if strings.Contains(out, "[enter/o] apply") {
		t.Fatalf("did not expect legacy verbose footer:\n%s", out)
	}
}

func TestViewUsesCompactModeSpecificFooter(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "↑↓ Move") || strings.Contains(out, "←→ Move") {
		t.Fatalf("expected compact-mode footer navigation hint:\n%s", out)
	}
	if !strings.Contains(out, "v View") || !strings.Contains(out, "Enter Menu") {
		t.Fatalf("expected compact-mode footer actions:\n%s", out)
	}
	if strings.Contains(out, "r Refresh") || strings.Contains(out, "R All") {
		t.Fatalf("did not expect refresh hints in compact footer:\n%s", out)
	}
}

func TestViewKeepsFooterWhenHelpOverlayIsOpen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.HelpVisible = true

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Enter Menu") || !strings.Contains(out, "? Help") {
		t.Fatalf("expected footer to remain visible behind help overlay:\n%s", out)
	}
	if !strings.Contains(out, "Keyboard help") {
		t.Fatalf("expected help overlay content:\n%s", out)
	}
}

func TestViewKeepsFooterWhenActionMenuIsOpen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.ActionMenuVisible = true

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Enter Menu") || !strings.Contains(out, "? Help") {
		t.Fatalf("expected footer to remain visible behind action menu:\n%s", out)
	}
	if !strings.Contains(out, "Account actions") {
		t.Fatalf("expected action menu overlay content:\n%s", out)
	}
}

func TestViewKeepsFooterWhenApplyModalIsOpen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.startApplyFlow()

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Enter Menu") || !strings.Contains(out, "? Help") {
		t.Fatalf("expected footer to remain visible behind apply modal:\n%s", out)
	}
	if !strings.Contains(out, "Select targets to apply") {
		t.Fatalf("expected apply modal content:\n%s", out)
	}
}

func TestViewKeepsFooterWhenDeleteModalIsOpen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.startDeleteFlow([]config.Source{config.SourceManaged, config.SourceCodex})

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Enter Menu") || !strings.Contains(out, "? Help") {
		t.Fatalf("expected footer to remain visible behind delete modal:\n%s", out)
	}
	if !strings.Contains(out, "Delete account") {
		t.Fatalf("expected delete modal content:\n%s", out)
	}
}

func TestViewKeepsFooterWhenNoticeIsOpen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.Notice = "saved"

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Enter Menu") || !strings.Contains(out, "? Help") {
		t.Fatalf("expected footer to remain visible behind notice modal:\n%s", out)
	}
	if !strings.Contains(out, "saved") {
		t.Fatalf("expected notice modal content:\n%s", out)
	}
}

func TestRenderCompactView_MixedRowsStayAligned(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 150

	model.Accounts = []*config.Account{
		{Key: "a1", Label: "first@example.com", Email: "first@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "second@example.com", Email: "second@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "third@example.com", Email: "third@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 0
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}
	model.LoadingMap = map[string]bool{"a2": true}
	model.ErrorsMap = map[string]error{}

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected non-empty rendered view")
	}

	if !strings.Contains(out, "Loading...") {
		t.Fatalf("expected loading status in compact view")
	}
	if !strings.Contains(out, "Queued...") {
		t.Fatalf("expected queued status in compact view")
	}
	if !strings.Contains(out, "30%") {
		t.Fatalf("expected percentage metadata in compact view")
	}
}

func TestRenderCompactView_NarrowWidthRendersWithoutBreakage(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 72

	out := ansi.Strip(model.renderCompactView())
	if !strings.Contains(out, "Loading...") {
		t.Fatalf("expected loading state in narrow mode output:\n%s", out)
	}
	if !strings.Contains(out, "user@example.com") {
		t.Fatalf("expected account label in narrow mode output:\n%s", out)
	}
}

func TestRenderCompactView_NarrowWidthDoesNotOverflowLineWidth(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 92
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "clawsharedbot.hastily044@site.test", Email: "x", AccountID: "1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "delise.nl.test@gmail.com", Email: "y", AccountID: "2", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 100.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 79.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	limit := model.preferredContentWidth()
	for i, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > limit {
			t.Fatalf("line %d width=%d exceeds limit=%d: %q", i, w, limit, line)
		}
	}
}

func TestRenderCompactView_VeryNarrowWidthDoesNotSplitPercentLines(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 60
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "clawsharedbot.hastily044@site.test", Email: "x", AccountID: "1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "delise.nl.test@gmail.com", Email: "y", AccountID: "2", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 100.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 78.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	if strings.Contains(out, "\n.0%") {
		t.Fatalf("expected percent text to stay on same line without split, got:\n%s", out)
	}
}

func TestView_CompactNarrowDoesNotWrapPercentFragmentsToOwnLines(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 60
	model.Height = 36
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "clawsharedbot.hastily044@site.test", Email: "x", AccountID: "1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "delise.nl.test@gmail.com", Email: "y", AccountID: "2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.usa10@gmail.com", Email: "z", AccountID: "3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delise.usa30@gmail.com", Email: "w", AccountID: "4", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 100.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 100.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
		"a3": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 100.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
		"a4": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 100.0, ResetAt: time.Now().Add(6*24*time.Hour + 23*time.Hour)}}},
	}

	out := ansi.Strip(model.View())
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > model.Width {
			t.Fatalf("expected compact view line width <= %d, got %d in %q", model.Width, w, line)
		}
	}
	if strings.Contains(out, "\n.0%") || strings.Contains(out, "\n  .0%") {
		t.Fatalf("expected no wrapped percent fragments, got:\n%s", out)
	}
}

func TestRenderCompactView_LoadingAndQueuedShareRowGeometry(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 140
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "first@example.com", Email: "first@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "second@example.com", Email: "second@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "third@example.com", Email: "third@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.LoadingMap = map[string]bool{"a2": true}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}
	model.ErrorsMap = map[string]error{}

	out := ansi.Strip(model.renderCompactView())
	rawLines := strings.Split(out, "\n")
	lines := make([]string, 0, 3)
	for _, line := range rawLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 non-empty lines, got %d", len(lines))
	}

	loadingWidth := ansi.StringWidth(strings.TrimRight(lines[1], " "))
	queuedWidth := ansi.StringWidth(strings.TrimRight(lines[2], " "))
	diff := loadingWidth - queuedWidth
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Fatalf("expected loading and queued rows to have near-equal width, got %d vs %d", loadingWidth, queuedWidth)
	}
}

func TestView_DoesNotOverflowViewportWidthOnNarrowScreen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Width = 120
	model.Height = 30

	out := model.View()
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w > model.Width {
			t.Fatalf("line width = %d, want <= %d\n%s", w, model.Width, ansi.Strip(line))
		}
	}
}

func TestRenderAccountTabs_DoesNotOverflowOnNarrowScreen(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 75.0,
			ResetAt:     time.Now().Add(5 * time.Hour),
		},
	})
	model.Width = 96
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "clawsharedbot.hastily044@site.test", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "delise.nl.test@gmail.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delise.usa10@gmail.com", Email: "delise.usa10@gmail.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 1

	tabs := model.renderAccountTabs()
	if w := ansi.StringWidth(tabs); w > model.Width-8 {
		t.Fatalf("tabs width = %d, want <= %d\n%s", w, model.Width-8, ansi.Strip(tabs))
	}
}

func TestRenderAccountTabs_ShowsNoMoreThanThreeAccounts(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 75.0,
			ResetAt:     time.Now().Add(5 * time.Hour),
		},
	})
	model.Width = 220
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "acc1@example.com", Email: "acc1@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "acc2@example.com", Email: "acc2@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "acc3@example.com", Email: "acc3@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "acc4@example.com", Email: "acc4@example.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
		{Key: "a5", Label: "acc5@example.com", Email: "acc5@example.com", AccountID: "id-5", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 2

	tabs := ansi.Strip(model.renderAccountTabs())
	visible := 0
	for _, account := range model.Accounts {
		if strings.Contains(tabs, account.Label) {
			visible++
		}
	}
	if visible > 3 {
		t.Fatalf("visible accounts = %d, want <= 3; tabs=%q", visible, tabs)
	}
}

func TestView_DoesNotOverflowAcrossTypicalViewportRange(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "clawsharedbot.hastily044@site.test", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "delise.nl.test@gmail.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delise.usa10@gmail.com", Email: "delise.usa10@gmail.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 0
	model.Height = 28

	for width := 88; width <= 240; width++ {
		model.Width = width
		out := model.View()
		for _, line := range strings.Split(out, "\n") {
			if got := ansi.StringWidth(line); got > model.Width {
				t.Fatalf("width=%d line width=%d exceeds viewport\n%s", width, got, ansi.Strip(line))
			}
		}
	}
}

func TestView_MediumViewportKeepsHorizontalInset(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "clawsharedbot.hastily044@site.test", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "delise.nl.test@gmail.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delise.usa10@gmail.com", Email: "delise.usa10@gmail.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 0
	model.Width = 120
	model.Height = 28

	out := ansi.Strip(model.View())
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Codex Quota") {
			if !strings.HasPrefix(line, "  ") {
				t.Fatalf("expected at least two leading spaces before title on medium viewport, got %q", line)
			}
			return
		}
	}
	t.Fatalf("title line not found in output:\n%s", out)
}

func TestView_FillsViewportCanvasWhenSizeIsKnown(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Width = 120
	model.Height = 24

	out := model.View()
	lines := strings.Split(out, "\n")
	if len(lines) != model.Height {
		t.Fatalf("line count = %d, want %d", len(lines), model.Height)
	}

	for i, line := range lines {
		if got := ansi.StringWidth(line); got != model.Width {
			t.Fatalf("line %d width = %d, want %d\n%s", i, got, model.Width, ansi.Strip(line))
		}
	}
}

func TestView_WideViewportStaysVisiblyCentered(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "clawsharedbot.hastily044@site.test", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "delise.nl.test@gmail.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delise.usa10@gmail.com", Email: "delise.usa10@gmail.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 0
	model.Width = 220
	model.Height = 30

	out := ansi.Strip(model.View())
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Codex Quota") {
			if !strings.HasPrefix(line, "                    ") { // at least ~20 spaces
				t.Fatalf("expected title to remain visually centered on wide viewport, got %q", line)
			}
			return
		}
	}

	t.Fatalf("title line not found in output:\n%s", out)
}

func TestView_TitleIsCenteredAcrossViewportWidths(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "clawsharedbot.hastily044@site.test", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "delise.nl.test@gmail.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delise.usa10@gmail.com", Email: "delise.usa10@gmail.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 0
	model.Height = 30

	for width := 176; width <= 320; width += 8 {
		model.Width = width
		out := ansi.Strip(model.View())
		lines := strings.Split(out, "\n")
		found := false
		for _, line := range lines {
			if !strings.Contains(line, "Codex Quota") {
				continue
			}
			found = true
			title := strings.TrimSpace(line)
			titleWidth := ansi.StringWidth(title)
			expected := (model.Width - titleWidth) / 2
			actual := len(line) - len(strings.TrimLeft(line, " "))
			if actual < expected-2 || actual > expected+2 {
				t.Fatalf("width=%d title left=%d expected≈%d line=%q", width, actual, expected, line)
			}
			break
		}
		if !found {
			t.Fatalf("title line not found for width=%d", width)
		}
	}
}

func TestView_WindowBarCenterAlignsWithTitleCenter(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "x", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "y", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 1
	model.Width = 200
	model.Height = 32

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")

	titleCenter := -1
	barCenter := -1
	for _, line := range lines {
		if titleCenter < 0 && strings.Contains(line, "Codex Quota") {
			start := strings.Index(line, "Codex Quota")
			titleCenter = start + (len("Codex Quota") / 2)
		}
		if barCenter < 0 && strings.Contains(line, "Weekly usage limit") {
			runes := []rune(line)
			startBar := -1
			endBar := -1
			for i, r := range runes {
				if r == '█' || r == '·' {
					if startBar < 0 {
						startBar = i
					}
					endBar = i
				} else if startBar >= 0 {
					break
				}
			}
			if startBar >= 0 && endBar >= startBar {
				barCenter = startBar + ((endBar - startBar) / 2)
			}
		}
	}

	if titleCenter < 0 || barCenter < 0 {
		t.Fatalf("failed to detect title/bar centers in output:\n%s", out)
	}

	delta := titleCenter - barCenter
	if delta < -2 || delta > 2 {
		t.Fatalf("expected title/bar centers aligned, got title=%d bar=%d delta=%d", titleCenter, barCenter, delta)
	}
}

func TestView_WindowBarCenterStaysNearTitleCenterOnNarrowWidths(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 100.0,
			ResetAt:     time.Now().Add(6*24*time.Hour + 23*time.Hour),
		},
	})
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "11.gas910@8alias.com", Email: "11.gas910@8alias.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "clawsharedbot.hastily044@site.test", Email: "x", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "delise.nl.test@gmail.com", Email: "y", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 1
	model.Height = 32

	for _, width := range []int{96, 104, 112, 120} {
		model.Width = width
		out := ansi.Strip(model.View())
		lines := strings.Split(out, "\n")

		titleCenter := -1
		barCenter := -1
		for _, line := range lines {
			if titleCenter < 0 && strings.Contains(line, "Codex Quota") {
				start := strings.Index(line, "Codex Quota")
				titleCenter = start + (len("Codex Quota") / 2)
			}
			if barCenter < 0 && strings.Contains(line, "Weekly") {
				runes := []rune(line)
				startBar := -1
				endBar := -1
				for i, r := range runes {
					if r == '█' || r == '·' {
						if startBar < 0 {
							startBar = i
						}
						endBar = i
					} else if startBar >= 0 {
						break
					}
				}
				if startBar >= 0 && endBar >= startBar {
					barCenter = startBar + ((endBar - startBar) / 2)
				}
			}
		}

		if titleCenter < 0 || barCenter < 0 {
			t.Fatalf("width=%d failed to detect title/bar centers in output:\n%s", width, out)
		}

		delta := titleCenter - barCenter
		if delta < -4 || delta > 4 {
			t.Fatalf("width=%d expected near-center alignment, got title=%d bar=%d delta=%d", width, titleCenter, barCenter, delta)
		}
	}
}

func TestRenderCompactView_GroupsExhaustedAccountsAtBottom(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "normal-1@example.com", Email: "normal-1@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "exhausted@example.com", Email: "exhausted@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "normal-2@example.com", Email: "normal-2@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a3": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 80.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	headerIx := strings.Index(out, "Exhausted accounts")
	if headerIx < 0 {
		t.Fatalf("expected exhausted section header, got:\n%s", out)
	}

	normalIx := strings.Index(out, "normal-1@example.com")
	exhaustedIx := strings.Index(out, "exhausted@example.com")
	if normalIx < 0 || exhaustedIx < 0 {
		t.Fatalf("expected both normal and exhausted labels in output, got:\n%s", out)
	}
	if normalIx > headerIx {
		t.Fatalf("expected normal accounts before exhausted header, got:\n%s", out)
	}
	if exhaustedIx < headerIx {
		t.Fatalf("expected exhausted account below exhausted header, got:\n%s", out)
	}
}

func TestRenderCompactView_TreatsLimitReachedAsExhausted(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "normal@example.com", Email: "normal@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "limit-reached@example.com", Email: "limit-reached@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {LimitReached: true, Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 25.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	headerIx := strings.Index(out, "Exhausted accounts")
	limitReachedIx := strings.Index(out, "limit-reached@example.com")
	if headerIx < 0 || limitReachedIx < 0 {
		t.Fatalf("expected exhausted header and account in output, got:\n%s", out)
	}
	if limitReachedIx < headerIx {
		t.Fatalf("expected limit-reached account in exhausted section, got:\n%s", out)
	}
}

func TestRenderCompactView_LoadingAccountStaysInMainSection(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "loading@example.com", Email: "loading@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "exhausted@example.com", Email: "exhausted@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
	}
	model.LoadingMap = map[string]bool{"a1": true}
	model.UsageData = map[string]api.UsageData{
		"a1": {LimitReached: true, Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	headerIx := strings.Index(out, "Exhausted accounts")
	loadingIx := strings.Index(out, "loading@example.com")
	if headerIx < 0 || loadingIx < 0 {
		t.Fatalf("expected header and loading account in output, got:\n%s", out)
	}
	if loadingIx > headerIx {
		t.Fatalf("expected loading account to stay in main section, got:\n%s", out)
	}
}

func TestRenderCompactView_ActiveAccountHighlightWorksInExhaustedSection(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "normal@example.com", Email: "normal@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "exhausted@example.com", Email: "exhausted@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 1
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	if !strings.Contains(out, "> exhausted@example.com") {
		t.Fatalf("expected active marker in exhausted section, got:\n%s", out)
	}
}

func testModelWithWindows(windows []api.QuotaWindow) Model {
	accounts := []*config.Account{
		{
			Key:       "account-1",
			Label:     "user@example.com",
			Email:     "user@example.com",
			AccountID: "98609d8a-85fb-4ff8-aee2-9344e68fbe3f",
			Source:    config.SourceManaged,
			Writable:  true,
		},
	}

	model := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	model.Loading = false
	model.Data = api.UsageData{Windows: windows}
	return model
}
