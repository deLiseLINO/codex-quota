package config

import (
	"testing"
	"time"
)

func TestManagedStoreRoundTripsIDToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")

	account := &Account{
		Label:        "user@example.com",
		Email:        "user@example.com",
		AccountID:    "acc-1",
		AccessToken:  "tok",
		RefreshToken: "refresh",
		IDToken:      "idt-1",
		Source:       SourceManaged,
		Writable:     true,
	}
	if err := UpsertManagedAccount(account); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 account, got %d", len(got))
	}
	if got[0].IDToken != "idt-1" {
		t.Fatalf("expected id_token persisted, got %q", got[0].IDToken)
	}
}

func TestManagedStoreMergeFillsEmptyIDToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")

	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-1", Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("upsert with id_token: %v", err)
	}

	got, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].IDToken != "idt-1" {
		t.Fatalf("expected id_token filled on merge, got %#v", got)
	}
}

func TestManagedStoreMergeKeepsFresherIDToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")

	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-old", ExpiresAt: time.Unix(1000, 0), Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-new", ExpiresAt: time.Unix(2000, 0), Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("upsert fresh: %v", err)
	}

	got, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].IDToken != "idt-new" {
		t.Fatalf("expected fresher id_token kept, got %#v", got)
	}
}

func TestManagedStoreKeepsIDTokenWhenExpiryTies(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")

	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", IDToken: "idt-1", ExpiresAt: time.Unix(2000, 0), Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "tok", RefreshToken: "refresh", ExpiresAt: time.Unix(2000, 0), Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("upsert without id_token: %v", err)
	}

	got, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].IDToken != "idt-1" {
		t.Fatalf("expected id_token kept on equal expiry, got %#v", got)
	}
}
