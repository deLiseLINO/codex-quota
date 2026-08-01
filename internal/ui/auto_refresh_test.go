package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestAutoRefreshTickRefreshesActiveOnActiveInterval(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.Settings.BackgroundIntervalSec = 300
	m = markLoaded(m, "managed:1", "managed:2")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	// Arm the active account at t0.
	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	// 29s later: nothing yet.
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(29 * time.Second)})
	got2 := updated2.(Model)
	if got2.LoadingMap["managed:1"] {
		t.Fatalf("did not expect active refresh at 29s")
	}

	// 30s later: active account due, background not.
	updated3, _ := got2.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got3 := updated3.(Model)
	if !got3.LoadingMap["managed:1"] {
		t.Fatalf("expected active account refresh at 30s")
	}
	if got3.LoadingMap["managed:2"] {
		t.Fatalf("did not expect background refresh at 30s")
	}
}

func TestAutoRefreshTickRefreshesBackgroundOnBackgroundInterval(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.Settings.BackgroundIntervalSec = 300
	m = markLoaded(m, "managed:1", "managed:2")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	// Background due at 300s.
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(300 * time.Second)})
	got2 := updated2.(Model)
	if !got2.LoadingMap["managed:2"] {
		t.Fatalf("expected background account refresh at 300s")
	}
}

func TestAutoRefreshDisabledStopsRefreshing(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Settings.AutoRefreshEnabled = false
	m.Settings.ActiveIntervalSec = 30
	m.Settings.BackgroundIntervalSec = 300
	m = markLoaded(m, "managed:1", "managed:2")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got2 := updated2.(Model)
	if got2.LoadingMap["managed:1"] {
		t.Fatalf("did not expect refresh when auto-refresh disabled")
	}
}

func TestAutoRefreshTickArmsWithoutImmediateRefresh(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)
	if got.LoadingMap["managed:1"] {
		t.Fatalf("did not expect immediate refresh on first tick")
	}
}

func TestAutoRefreshTickContinuesAfterARefresh(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	// Arm at t0.
	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	// First auto-refresh at t0+30.
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got2 := updated2.(Model)
	if !got2.LoadingMap["managed:1"] {
		t.Fatalf("expected first auto-refresh at t0+30")
	}

	// Data arrives; loading clears. Timer continues from t0+30.
	updated3, _ := got2.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       api.UsageData{Allowed: true, Windows: []api.QuotaWindow{}},
	})
	got3 := updated3.(Model)

	// Second auto-refresh at t0+60 (not earlier).
	updated4, _ := got3.Update(AutoRefreshTickMsg{Now: t0.Add(60 * time.Second)})
	got4 := updated4.(Model)
	if !got4.LoadingMap["managed:1"] {
		t.Fatalf("expected second auto-refresh at t0+60")
	}
}

func TestManualRefreshResetsAutoRefreshTimer(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	// Arm at t0, then manual refresh at t0+10.
	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)
	refreshed, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got2 := refreshed.(Model)
	// Simulate the manual refresh's data arriving through the real message path.
	arrived, _ := got2.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       api.UsageData{Allowed: true},
	})
	got2 = arrived.(Model)

	// Next tick after manual refresh re-arms (no refresh).
	updated2, _ := got2.Update(AutoRefreshTickMsg{Now: t0.Add(35 * time.Second)})
	got3 := updated2.(Model)
	if got3.LoadingMap["managed:1"] {
		t.Fatalf("did not expect auto-refresh immediately after manual refresh")
	}

	// 30s after re-arm the timer fires.
	updated3, _ := got3.Update(AutoRefreshTickMsg{Now: t0.Add(65 * time.Second)})
	got4 := updated3.(Model)
	if !got4.LoadingMap["managed:1"] {
		t.Fatalf("expected auto-refresh 30s after manual refresh")
	}
}

func TestRefreshAllResetsAllAutoRefreshTimers(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.Settings.BackgroundIntervalSec = 300
	m = markLoaded(m, "managed:1", "managed:2")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	// Manual refresh-all at t0+10.
	refreshed, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got2 := refreshed.(Model)
	// Simulate refresh-all data arriving through the real message path.
	arrived1, _ := got2.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       api.UsageData{Allowed: true},
	})
	arrived2, _ := arrived1.(Model).Update(DataMsg{
		AccountKey: "managed:2",
		Data:       api.UsageData{Allowed: true},
	})
	got2 = arrived2.(Model)

	// Next tick after refresh-all re-arms (no background refresh at old due).
	updated2, _ := got2.Update(AutoRefreshTickMsg{Now: t0.Add(300 * time.Second)})
	got3 := updated2.(Model)
	if got3.LoadingMap["managed:2"] {
		t.Fatalf("did not expect background refresh immediately after refresh-all")
	}

	// 300s after re-arm the background timer fires.
	updated3, _ := got3.Update(AutoRefreshTickMsg{Now: t0.Add(600 * time.Second)})
	got4 := updated3.(Model)
	if !got4.LoadingMap["managed:2"] {
		t.Fatalf("expected background refresh 300s after refresh-all")
	}
}

func TestChangingActiveIntervalThroughSettingsResetsTimer(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	// Arm at t0.
	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	// Open Settings at t0+10, move to the active interval row, type "60".
	opened, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	settings := opened.(Model)
	moved, _ := settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	typed, _ := moved.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	typed2, _ := typed.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	got2 := typed2.(Model)
	if got2.Settings.ActiveIntervalSec != 60 {
		t.Fatalf("expected interval 60 after digit entry, got %d", got2.Settings.ActiveIntervalSec)
	}

	// Next tick after the change re-arms (no refresh at old 30s due).
	updated2, _ := got2.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got3 := updated2.(Model)
	if got3.LoadingMap["managed:1"] {
		t.Fatalf("did not expect refresh at old interval after settings change")
	}

	// 60s after re-arm the timer fires.
	updated3, _ := got3.Update(AutoRefreshTickMsg{Now: t0.Add(90 * time.Second)})
	got4 := updated3.(Model)
	if !got4.LoadingMap["managed:1"] {
		t.Fatalf("expected refresh at new interval after settings change")
	}
}

func TestInitStartsAutoRefreshTickEvenWhenDisabled(t *testing.T) {
	m := InitialModelWithSettingsAndStartupUpdate(nil, nil, nil, config.UIState{}, config.Settings{AutoRefreshEnabled: false}, nil)

	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected non-nil init cmd")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from init cmd, got %T", msg)
	}
	foundTick := false
	for _, c := range batch {
		if c == nil {
			continue
		}
		if sub := c(); sub != nil {
			if _, isTick := sub.(AutoRefreshTickMsg); isTick {
				foundTick = true
			}
		}
	}
	if !foundTick {
		t.Fatalf("expected auto-refresh tick to be running even when auto-refresh is disabled at startup")
	}
}

func TestAutoRefreshTickSkipsAccountsInFlight(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{"managed:1": t0}
	m.refreshScheduled = map[string]bool{}
	m.LoadingMap["managed:1"] = true
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0.Add(35 * time.Second)})
	got := updated.(Model)
	if got.refreshScheduled["managed:1"] {
		t.Fatalf("did not expect a refresh queued while the account is loading")
	}
}

func TestChangingBackgroundIntervalResetsBackgroundTimersOnly(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.Settings.BackgroundIntervalSec = 300
	m = markLoaded(m, "managed:1", "managed:2")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model) // arms both at t0

	// Active account refreshes on its interval at t0+30.
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got2 := updated2.(Model)
	if !got2.LoadingMap["managed:1"] {
		t.Fatalf("expected active refresh at t0+30")
	}
	got2.UsageData["managed:1"] = api.UsageData{Allowed: true}
	got2.LoadingMap["managed:1"] = false

	// Open Settings and step the background interval row.
	opened, _ := got2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	settings := opened.(Model)
	settings.settingsCursor = settingsRowBackgroundInterval
	stepped, _ := settings.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	got3 := stepped.(Model)

	// Active timer untouched: next refresh is still 30s after t0+30 → t0+60.
	updated3, _ := got3.Update(AutoRefreshTickMsg{Now: t0.Add(60 * time.Second)})
	got4 := updated3.(Model)
	if !got4.LoadingMap["managed:1"] {
		t.Fatalf("expected active refresh at t0+60 after background interval change")
	}

	// Background timer reset: no refresh at the old 300s due (t0+300).
	updated4, _ := got4.Update(AutoRefreshTickMsg{Now: t0.Add(300 * time.Second)})
	got5 := updated4.(Model)
	if got5.LoadingMap["managed:2"] {
		t.Fatalf("background refresh fired at old interval after background interval change")
	}
}

func TestAutoRefreshShowsDataNotLoadingInCompactView(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", Email: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.Width = 150
	m.UsageData = map[string]api.UsageData{
		"acc-1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got2 := updated2.(Model)

	if !got2.LoadingMap["acc-1"] || !got2.silentRefresh["acc-1"] {
		t.Fatalf("expected in-flight silent auto-refresh")
	}

	out := ansi.Strip(got2.renderCompactView())
	if strings.Contains(out, "Loading...") {
		t.Fatalf("did not expect Loading... during auto-refresh:\n%s", out)
	}
	if !strings.Contains(out, "40%") {
		t.Fatalf("expected data still visible during auto-refresh:\n%s", out)
	}
}

func TestAutoRefreshDataMsgSkipsAnimations(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.UsageData = map[string]api.UsageData{
		"acc-1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40}}},
	}
	m.LoadingMap = map[string]bool{}
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got2 := updated2.(Model)
	if !got2.silentRefresh["acc-1"] {
		t.Fatalf("expected auto-refresh marked silent")
	}

	updated3, _ := got2.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 60}},
		},
	})
	got3 := updated3.(Model)

	if got3.silentRefresh["acc-1"] {
		t.Fatalf("expected silentRefresh cleared after data arrives")
	}
	if len(got3.compactBarAnimations) != 0 {
		t.Fatalf("did not expect compact bar animation on auto-refresh")
	}
	if len(got3.tabWindowAnimations) != 0 {
		t.Fatalf("did not expect tab window animation on auto-refresh")
	}
	if got3.UsageData["acc-1"].Windows[0].LeftPercent != 60 {
		t.Fatalf("expected updated data stored")
	}
}

func TestManualRefreshIsNotSilent(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	refreshed, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := refreshed.(Model)
	if !got.LoadingMap["managed:1"] {
		t.Fatalf("expected account loading after manual refresh")
	}
	if got.silentRefresh["managed:1"] {
		t.Fatalf("did not expect manual refresh marked silent")
	}
}

func TestAutoRefreshTickRunsWhileOverlayOpen(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m = markLoaded(m, "managed:1")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0
	m.HelpVisible = true

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)
	if !got.HelpVisible {
		t.Fatalf("expected help overlay to remain open")
	}

	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(30 * time.Second)})
	got2 := updated2.(Model)
	if !got2.LoadingMap["managed:1"] {
		t.Fatalf("expected auto-refresh to continue while overlay open")
	}
}

func TestAutoRefreshErrorsSurfaceLikeManual(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Settings.AutoRefreshEnabled = true
	m.Settings.ActiveIntervalSec = 30
	m.Settings.BackgroundIntervalSec = 300
	m = markLoaded(m, "managed:1", "managed:2")
	m.lastRefresh = map[string]time.Time{}
	m.ActiveAccountIx = 0

	updated, _ := m.Update(AutoRefreshTickMsg{Now: t0})
	got := updated.(Model)

	// Background refresh at 300s surfaces its error in ErrorsMap.
	updated2, _ := got.Update(AutoRefreshTickMsg{Now: t0.Add(300 * time.Second)})
	got2 := updated2.(Model)
	updated3, _ := got2.Update(ErrMsg{AccountKey: "managed:2", Err: assertErr("boom")})
	got3 := updated3.(Model)
	if got3.ErrorsMap["managed:2"] == nil {
		t.Fatalf("expected background auto-refresh error to surface")
	}
}

func markLoaded(m Model, keys ...string) Model {
	for _, k := range keys {
		m.LoadingMap[k] = false
		m.UsageData[k] = api.UsageData{Allowed: true}
	}
	return m
}

var t0 = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
