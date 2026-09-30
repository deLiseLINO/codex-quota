package ui

import (
	"testing"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestUniqueAccountIndexByLegacyKey_ResolvesWhenSingleMatch(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	accounts := []*config.Account{
		{Key: workspace + " user-owner", AccountID: workspace, UserID: "user-owner"},
	}

	idx, ok := uniqueAccountIndexByLegacyKey(accounts, workspace)
	if !ok || idx != 0 {
		t.Fatalf("expected legacy bare-UUID key to resolve to the single account; ok=%v idx=%d", ok, idx)
	}
}

func TestUniqueAccountIndexByLegacyKey_AmbiguousWhenTwoUsersShareWorkspace(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	accounts := []*config.Account{
		{Key: workspace + " user-member", AccountID: workspace, UserID: "user-member"},
		{Key: workspace + " user-owner", AccountID: workspace, UserID: "user-owner"},
	}

	if _, ok := uniqueAccountIndexByLegacyKey(accounts, workspace); ok {
		t.Fatal("legacy key must not resolve when two users share the workspace UUID")
	}
}

func TestUniqueAccountIndexByLegacyKey_NoMatch(t *testing.T) {
	accounts := []*config.Account{
		{Key: "other-workspace user-x", AccountID: "other-workspace", UserID: "user-x"},
	}
	if _, ok := uniqueAccountIndexByLegacyKey(accounts, "9cb5304a-39d0-4c09-8456-6b2691689666"); ok {
		t.Fatal("expected no match for unknown legacy key")
	}
}
