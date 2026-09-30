package config

import (
	"testing"
	"time"
)

// makeUserJWT builds a token for a specific workspace + user, matching the real
// ChatGPT token shape (chatgpt_account_id is the shared workspace UUID,
// chatgpt_user_id distinguishes users within it).
func makeUserJWT(t *testing.T, workspaceID, userID, email string, exp int64) string {
	t.Helper()
	return makeTestJWT(t, map[string]any{
		"email": email,
		"exp":   exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": workspaceID,
			"chatgpt_user_id":    userID,
		},
	})
}

func TestParseAccessToken_ExtractsUserIDButKeepsWorkspaceAccountID(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	user := "user-F2mPxZoCMmsTSNPoLRk7QGNM"
	token := makeUserJWT(t, workspace, user, "member@example.com", time.Now().Add(time.Hour).Unix())

	claims := ParseAccessToken(token)
	if claims.AccountID != workspace {
		t.Fatalf("AccountID must stay the workspace UUID for the API header; got %q", claims.AccountID)
	}
	if claims.UserID != user {
		t.Fatalf("expected UserID %q, got %q", user, claims.UserID)
	}
}

func TestParseAccessToken_FallsBackToSubForUserID(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	token := makeTestJWT(t, map[string]any{
		"sub": "auth0|639ba3ad69b8ae8e1d804587",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": workspace,
		},
	})

	claims := ParseAccessToken(token)
	if claims.AccountID != workspace {
		t.Fatalf("expected workspace AccountID %q, got %q", workspace, claims.AccountID)
	}
	if claims.UserID != "auth0|639ba3ad69b8ae8e1d804587" {
		t.Fatalf("expected UserID to fall back to sub, got %q", claims.UserID)
	}
}

func TestDedupeAccounts_KeepsTwoUsersInSameWorkspace(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	member := &Account{
		AccountID:    workspace,
		UserID:       "user-member",
		Email:        "member@example.com",
		AccessToken:  "member-access",
		RefreshToken: "member-refresh",
		ExpiresAt:    time.Unix(1000, 0),
		Source:       SourceManaged,
		Writable:     true,
	}
	owner := &Account{
		AccountID:    workspace,
		UserID:       "user-owner",
		Email:        "owner@example.com",
		AccessToken:  "owner-access",
		RefreshToken: "owner-refresh",
		ExpiresAt:    time.Unix(2000, 0),
		Source:       SourceManaged,
		Writable:     true,
	}

	deduped := dedupeAccounts([]*Account{member, owner})
	if len(deduped) != 2 {
		t.Fatalf("expected 2 distinct accounts for two users in one workspace, got %d: %#v", len(deduped), deduped)
	}

	// The shared workspace UUID must be preserved on both (it is the API header).
	for _, a := range deduped {
		if a.AccountID != workspace {
			t.Fatalf("workspace UUID must be preserved as AccountID, got %q", a.AccountID)
		}
	}
}

func TestSameIdentity_DistinguishesUsersInSameWorkspace(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	member := &Account{AccountID: workspace, UserID: "user-member", Email: "member@example.com"}
	owner := &Account{AccountID: workspace, UserID: "user-owner", Email: "owner@example.com"}

	if sameIdentity(member, owner) {
		t.Fatal("two users in the same workspace must NOT be the same identity")
	}
	if !sameIdentity(member, &Account{AccountID: workspace, UserID: "user-member"}) {
		t.Fatal("same workspace + same user must be the same identity")
	}
}

func TestCanonicalAccountID_UnchangedByUserID(t *testing.T) {
	// CanonicalAccountID feeds the ChatGPT-Account-Id header and external writes;
	// it must always return the workspace UUID regardless of per-user data.
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	if got := CanonicalAccountID(workspace, "auth0|somesub"); got != workspace {
		t.Fatalf("expected workspace UUID, got %q", got)
	}
}

// The safety-critical regression: an owner refreshing their token must not
// overwrite the member's tokens in a local store just because they share a
// workspace UUID.
func TestSyncAccountEverywhere_DoesNotOverwriteDifferentUserSameWorkspace(t *testing.T) {
	setupSyncTestEnv(t)

	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	memberToken := makeUserJWT(t, workspace, "user-member", "member@example.com", 1000)
	member := &Account{
		AccountID:    workspace,
		UserID:       "user-member",
		Email:        "member@example.com",
		AccessToken:  memberToken,
		RefreshToken: "member-refresh",
		ExpiresAt:    time.Unix(1000, 0),
		Source:       SourceCodex,
		Writable:     true,
	}
	// Seed Codex with the member's credentials.
	if _, err := ApplyAccountToCodex(member); err != nil {
		t.Fatalf("seed codex with member: %v", err)
	}

	// Now the owner (same workspace, different user) syncs a fresher bundle.
	ownerToken := makeUserJWT(t, workspace, "user-owner", "owner@example.com", 2000)
	owner := &Account{
		AccountID:    workspace,
		UserID:       "user-owner",
		Email:        "owner@example.com",
		AccessToken:  ownerToken,
		RefreshToken: "owner-refresh",
		ExpiresAt:    time.Unix(2000, 0),
		Source:       SourceManaged,
		Writable:     true,
	}
	if err := SyncAccountEverywhere(owner); err != nil {
		t.Fatalf("sync owner: %v", err)
	}

	// Codex must still hold the member's tokens — the owner is a different user.
	codexAcc, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load codex: %v", err)
	}
	if codexAcc.RefreshToken != "member-refresh" {
		t.Fatalf("member's Codex credentials were overwritten by a co-workspace user: %#v", codexAcc)
	}

	// Across all sources, both users must surface as distinct accounts (member
	// from Codex, owner from the managed store) rather than collapsing into one.
	all, err := LoadAllAccounts()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	users := map[string]bool{}
	for _, a := range all {
		users[a.UserID] = true
	}
	if !users["user-member"] || !users["user-owner"] {
		t.Fatalf("expected both users present as distinct accounts, got %#v", all)
	}
}

func TestMigrateManagedAccounts_DoesNotCollapseTwoUsersInWorkspace(t *testing.T) {
	workspace := "9cb5304a-39d0-4c09-8456-6b2691689666"
	memberToken := makeUserJWT(t, workspace, "user-member", "member@example.com", time.Now().Add(time.Hour).Unix())
	ownerToken := makeUserJWT(t, workspace, "user-owner", "owner@example.com", time.Now().Add(time.Hour).Unix())

	input := []managedAccount{
		{AccountID: workspace, AccessToken: memberToken, RefreshToken: "member-refresh"},
		{AccountID: workspace, AccessToken: ownerToken, RefreshToken: "owner-refresh"},
	}

	output, _ := migrateManagedAccounts(input)
	if len(output) != 2 {
		t.Fatalf("migration collapsed two users into %d record(s): %#v", len(output), output)
	}
	for _, item := range output {
		if item.UserID == "" {
			t.Fatalf("migration did not backfill UserID: %#v", item)
		}
		if item.AccountID != workspace {
			t.Fatalf("migration changed workspace UUID: %#v", item)
		}
	}
}
