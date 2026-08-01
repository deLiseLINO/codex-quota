package config

import (
	"testing"
	"time"
)

func TestMergeAccountsFillsEmptyIDToken(t *testing.T) {
	managed := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", Source: SourceManaged, Writable: true}
	codex := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-1", Source: SourceCodex, Writable: true}

	got := mergeAccounts(managed, codex)
	if got.IDToken != "idt-1" {
		t.Fatalf("expected id_token filled from secondary, got %q", got.IDToken)
	}
}

func TestMergeAccountsKeepsPrimaryIDTokenWhenPresent(t *testing.T) {
	managed := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-managed", Source: SourceManaged, Writable: true}
	codex := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-codex", Source: SourceCodex, Writable: true}

	got := mergeAccounts(managed, codex)
	if got.IDToken != "idt-managed" {
		t.Fatalf("expected managed id_token kept, got %q", got.IDToken)
	}
}

func TestCloneAsManagedCarriesIDToken(t *testing.T) {
	codex := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-1", Source: SourceCodex}

	got := cloneAsManaged(codex)
	if got == nil || got.IDToken != "idt-1" {
		t.Fatalf("expected id_token carried into managed clone, got %#v", got)
	}
}

func TestNeedsManagedUpdateTracksIDToken(t *testing.T) {
	existing := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", Source: SourceManaged, Writable: true}
	incoming := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-new", ExpiresAt: time.Unix(2000, 0), Source: SourceCodex, Writable: true}

	if !needsManagedUpdate(existing, incoming) {
		t.Fatal("expected managed update needed when id_token arrives with fresher bundle")
	}
}

func TestResolveFreshAccountDetectsIDTokenChange(t *testing.T) {
	setupSyncTestEnv(t)

	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-new", Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("seed managed: %v", err)
	}

	account := &Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-old", Source: SourceManaged, Writable: true}
	fresh, changed, err := ResolveFreshAccount(account)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if fresh == nil || fresh.IDToken != "idt-new" {
		t.Fatalf("expected fresh id_token resolved, got %#v", fresh)
	}
	if !changed {
		t.Fatal("expected id_token change to mark account changed")
	}
}
