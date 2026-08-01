package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestDeletableSourcesForAccount_UsesEmailFallbackWhenAccountIDDiffers(t *testing.T) {
	account := &config.Account{
		Key:       "managed:1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "uuid-in-list",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{
		"email:user@example.com": []string{"app", "codex"},
	}, map[string][]string{}, false)

	got := m.deletableSourcesForAccount(account)
	want := []config.Source{config.SourceManaged, config.SourceCodex}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deletable sources mismatch: got %v, want %v", got, want)
	}
}

func TestDeletableSourcesForAccount_UsesActiveIdentityKeysFallback(t *testing.T) {
	account := &config.Account{
		Key:         "managed:1",
		Label:       "n/a",
		AccessToken: "same-access-token",
		Source:      config.SourceManaged,
		Writable:    true,
	}

	activeIdentity := &config.Account{AccessToken: "same-access-token"}
	activeMap := map[string][]string{}
	for _, key := range config.ActiveIdentityKeys(activeIdentity) {
		activeMap[key] = []string{"opencode", "codex"}
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, activeMap, false)

	got := m.deletableSourcesForAccount(account)
	want := []config.Source{config.SourceOpenCode, config.SourceCodex}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deletable sources mismatch: got %v, want %v", got, want)
	}
}

func TestDeletableSourcesForAccount_AllowsManagedWhenOnlyEmailPresent(t *testing.T) {
	account := &config.Account{
		Key:      "managed:1",
		Label:    "user@example.com",
		Email:    "user@example.com",
		Source:   config.SourceManaged,
		Writable: true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{
		"email:user@example.com": []string{"app"},
	}, map[string][]string{}, false)

	got := m.deletableSourcesForAccount(account)
	want := []config.Source{config.SourceManaged}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deletable sources mismatch: got %v, want %v", got, want)
	}
}

func TestInitSchedulesTwoFetchesWhenMultipleAccounts(t *testing.T) {
	m := testModelForHotkeys(3)

	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("expected non-nil init cmd")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from init cmd, got %T", msg)
	}
	if len(batch) != 4 {
		t.Fatalf("expected 4 startup commands (title + auto-refresh tick + 2 fetches), got %d", len(batch))
	}
}
