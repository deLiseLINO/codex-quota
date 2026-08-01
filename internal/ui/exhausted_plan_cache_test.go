package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestPlanTypeCachePersistsAcrossActiveRefresh(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.LoadingMap = map[string]bool{"acc-1": true}
	m.UsageData = map[string]api.UsageData{}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			PlanType: "pro",
			Windows:  []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40}},
		},
	})
	got := updated.(Model)
	if !got.isPaidByKnownPlan("acc-1") {
		t.Fatalf("expected known paid plan after DataMsg")
	}

	refreshed, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	afterRefresh := refreshed.(Model)
	if _, ok := afterRefresh.UsageData["acc-1"]; ok {
		t.Fatalf("expected usage data to be cleared for active refresh")
	}
	if !afterRefresh.isPaidByKnownPlan("acc-1") {
		t.Fatalf("expected known plan cache to survive active refresh")
	}
}

func TestPlanTypeCachePreFilledFromUIState(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	uiState := config.UIState{
		PlanTypes: map[string]string{"acc-1": "pro"},
	}
	m := InitialModelWithUIState([]*config.Account{account}, map[string][]string{}, map[string][]string{}, uiState)

	if !m.isPaidByKnownPlan("acc-1") {
		t.Fatalf("expected plan type pre-filled from uiState at init")
	}
	if got := m.PlanTypeByAccount["acc-1"]; got != "pro" {
		t.Fatalf("expected plan type 'pro', got %q", got)
	}
}

func TestPlanTypeCachePreFilledPrunesStaleEntries(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	uiState := config.UIState{
		PlanTypes: map[string]string{
			"acc-1":     "pro",
			"stale-acc": "plus",
		},
	}
	m := InitialModelWithUIState([]*config.Account{account}, map[string][]string{}, map[string][]string{}, uiState)

	if _, ok := m.PlanTypeByAccount["stale-acc"]; ok {
		t.Fatalf("expected stale plan type entry to be pruned at init")
	}
	if got := m.PlanTypeByAccount["acc-1"]; got != "pro" {
		t.Fatalf("expected valid plan type 'pro', got %q", got)
	}
}

func TestSetKnownPlanTypeReturnsChanged(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)

	if !m.setKnownPlanType("acc-1", "pro") {
		t.Fatalf("expected changed=true for first set")
	}
	if m.setKnownPlanType("acc-1", "pro") {
		t.Fatalf("expected changed=false for same value")
	}
	if !m.setKnownPlanType("acc-1", "plus") {
		t.Fatalf("expected changed=true for changed value")
	}
}

func TestUIStateSnapshotIncludesPlanTypes(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.setKnownPlanType("acc-1", "pro")

	snapshot := m.uiStateSnapshot()
	if snapshot.PlanTypes == nil {
		t.Fatalf("expected plan types in snapshot, got nil")
	}
	if got := snapshot.PlanTypes["acc-1"]; got != "pro" {
		t.Fatalf("expected plan type 'pro' for acc-1, got %q", got)
	}

	m.PlanTypeByAccount["acc-1"] = "free"
	if got := snapshot.PlanTypes["acc-1"]; got != "pro" {
		t.Fatalf("expected snapshot to own plan type map, got %q after model mutation", got)
	}
}

func TestUIStateSnapshotCopiesPlanTypes(t *testing.T) {
	m := InitialModel(nil, map[string][]string{}, map[string][]string{}, false)
	m.PlanTypeByAccount = map[string]string{"acc-1": "pro"}

	snapshot := m.uiStateSnapshot()
	snapshot.PlanTypes["acc-1"] = "free"

	if got := m.PlanTypeByAccount["acc-1"]; got != "pro" {
		t.Fatalf("expected snapshot mutation not to affect model, got %q", got)
	}
}

func TestKnownPlanTypePrunedForRemovedAccounts(t *testing.T) {
	accounts := []*config.Account{
		{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged},
		{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	m.PlanTypeByAccount["acc-1"] = "pro"
	m.PlanTypeByAccount["acc-2"] = "free"

	nextAccounts := []*config.Account{{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged}}
	updated, _ := m.Update(AccountsMsg{Accounts: nextAccounts})
	got := updated.(Model)

	if _, ok := got.PlanTypeByAccount["acc-1"]; ok {
		t.Fatalf("expected removed account plan cache to be pruned")
	}
	if _, ok := got.PlanTypeByAccount["acc-2"]; !ok {
		t.Fatalf("expected existing account plan cache to remain")
	}
}
