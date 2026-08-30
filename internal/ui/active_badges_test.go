package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestActiveSourceBadgesForAccount(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"account:acc-1": []string{"codex", "opencode", "pi", "omp"},
	}, false)

	if got := m.activeSourceBadgesForAccount(account); got != "C•O•P•M" {
		t.Fatalf("badges mismatch: got %q, want %q", got, "C•O•P•M")
	}
}

func TestActiveSourceBadgesForAccount_PiAndOMP(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"account:acc-1": []string{"pi", "omp"},
	}, false)

	if got := m.activeSourceBadgesForAccount(account); got != "P•M" {
		t.Fatalf("badges mismatch: got %q, want %q", got, "P•M")
	}
}
func TestActiveSourceBadges_MultipleOMPAccountsBothRenderBadges(t *testing.T) {
	acct1 := &config.Account{
		Key:       "acc-1",
		Label:     "user1@omp.sh",
		Email:     "user1@omp.sh",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}
	acct2 := &config.Account{
		Key:       "acc-2",
		Label:     "user2@omp.sh",
		Email:     "user2@omp.sh",
		AccountID: "acc-2",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{acct1, acct2}, map[string][]string{}, map[string][]string{
		"account:acc-1": []string{"omp"},
		"account:acc-2": []string{"omp"},
	}, false)

	if got := m.activeSourceBadgesForAccount(acct1); got != "M" {
		t.Fatalf("acct1 badge mismatch: got %q, want %q", got, "M")
	}
	if got := m.activeSourceBadgesForAccount(acct2); got != "M" {
		t.Fatalf("acct2 badge mismatch: got %q, want %q", got, "M")
	}
}

func TestRenderAccountTabs_ShowsActiveBadges(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"account:acc-1": []string{"codex"},
	}, false)

	out := ansi.Strip(m.renderAccountTabs())
	if !strings.Contains(out, "[C]") {
		t.Fatalf("expected [C] badge in tabs output, got: %s", out)
	}
}

func TestRenderCompactView_ShowsActiveBadges(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"account:acc-1": []string{"opencode"},
	}, true)
	m.Loading = false
	m.Width = 120
	m.UsageData = map[string]api.UsageData{
		"acc-1": {
			Windows: []api.QuotaWindow{
				{
					Label:       "Weekly usage limit",
					WindowSec:   604800,
					LeftPercent: 10,
					ResetAt:     time.Now().Add(1 * time.Hour),
				},
			},
		},
	}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}

	out := ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "[O]") {
		t.Fatalf("expected [O] badge in compact output, got: %s", out)
	}
}

func TestActiveSourceBadgesForAccount_MatchesByTokenFallback(t *testing.T) {
	account := &config.Account{
		Key:         "acc-2",
		Label:       "n/a",
		AccessToken: "same-access-token",
		Source:      config.SourceManaged,
		Writable:    true,
	}

	activeAccount := &config.Account{
		AccessToken: "same-access-token",
	}
	keys := config.ActiveIdentityKeys(activeAccount)
	if len(keys) == 0 {
		t.Fatalf("expected non-empty active identity keys for token")
	}

	activeMap := map[string][]string{}
	for _, key := range keys {
		activeMap[key] = []string{"codex"}
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, activeMap, false)
	if got := m.activeSourceBadgesForAccount(account); got != "C" {
		t.Fatalf("badges mismatch with token fallback: got %q, want %q", got, "C")
	}
}

func TestActiveSourceBadgesDisplayWidth_IncludesBrackets(t *testing.T) {
	account := &config.Account{
		Key:       "acc-3",
		Label:     "user@example.com",
		AccountID: "acc-3",
		Source:    config.SourceManaged,
	}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"account:acc-3": []string{"codex", "opencode"},
	}, false)

	if got := m.activeSourceBadgesDisplayWidth(account); got != 5 {
		t.Fatalf("expected display width 5 for [C•O], got %d", got)
	}
}
