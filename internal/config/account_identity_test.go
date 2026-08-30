package config

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseAccessToken_UsesNestedAuthObjectChatGPTAccountID(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	exp := time.Now().Add(2 * time.Hour).Unix()

	token := makeTestJWT(t, map[string]any{
		"cid":   "client-123",
		"email": "user@example.com",
		"sub":   "auth0|sub-id",
		"exp":   exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
	})

	claims := ParseAccessToken(token)
	if claims.AccountID != uuid {
		t.Fatalf("expected account id %q, got %q", uuid, claims.AccountID)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("expected email %q, got %q", "user@example.com", claims.Email)
	}
	if claims.ClientID != "client-123" {
		t.Fatalf("expected client id %q, got %q", "client-123", claims.ClientID)
	}
	if claims.ExpiresAt.IsZero() {
		t.Fatalf("expected non-zero expiration")
	}
}

func TestParseAccessToken_UsesClientIDClaim(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	exp := time.Now().Add(2 * time.Hour).Unix()

	token := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"email":     "user@example.com",
		"sub":       "auth0|sub-id",
		"exp":       exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
	})

	claims := ParseAccessToken(token)
	if claims.ClientID != "app_EMoamEEZ73f0CkXaXp7hrann" {
		t.Fatalf("expected client id %q, got %q", "app_EMoamEEZ73f0CkXaXp7hrann", claims.ClientID)
	}
}

func TestDedupeAccounts_MergesByAccountIDWhenEmailMissingOnOneSide(t *testing.T) {
	accountID := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	now := time.Now().UTC()

	codex := &Account{
		AccountID:   accountID,
		Email:       "",
		AccessToken: "older-token",
		ExpiresAt:   now,
		Source:      SourceCodex,
		Writable:    true,
		FilePath:    "/tmp/codex-auth.json",
	}
	managed := &Account{
		AccountID:    accountID,
		Email:        "user@example.com",
		Label:        "user@example.com",
		AccessToken:  "newer-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    now.Add(1 * time.Hour),
		Source:       SourceManaged,
		Writable:     true,
		FilePath:     "/tmp/accounts.json",
	}

	deduped := dedupeAccounts([]*Account{codex, managed})
	if len(deduped) != 1 {
		t.Fatalf("expected 1 account after dedupe, got %d", len(deduped))
	}

	got := deduped[0]
	if got.AccountID != accountID {
		t.Fatalf("expected account id %q, got %q", accountID, got.AccountID)
	}
	if got.Email != "user@example.com" {
		t.Fatalf("expected merged email, got %q", got.Email)
	}
	if got.Label != "user@example.com" {
		t.Fatalf("expected merged label, got %q", got.Label)
	}
	if got.Source != SourceManaged {
		t.Fatalf("expected managed source to win priority, got %q", got.Source)
	}
}

func TestAccountPriorityIncludesEveryExternalSource(t *testing.T) {
	sources := []Source{SourceManaged, SourceOpenCode, SourceCodex, SourcePi, SourceOMP}
	for i := 1; i < len(sources); i++ {
		if accountPriority(&Account{Source: sources[i-1]}) <= accountPriority(&Account{Source: sources[i]}) {
			t.Fatalf("%s should outrank %s", sources[i-1], sources[i])
		}
	}
}

func TestFinalizeAccount_ReplacesTechnicalLabelWithEmail(t *testing.T) {
	accountID := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"

	tests := []struct {
		name  string
		label string
	}{
		{name: "auth0 prefix", label: "auth0|SZltqUcP3OMO8k3f18EOszJr"},
		{name: "short account id", label: shortAccountID(accountID)},
		{name: "n/a", label: "n/a"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{
				Label:     tc.label,
				Email:     "user@example.com",
				AccountID: accountID,
				Source:    SourceManaged,
			}

			finalizeAccount(account)
			if account.Label != "user@example.com" {
				t.Fatalf("expected label replaced with email, got %q", account.Label)
			}
		})
	}
}

func TestMigrateManagedAccounts_CanonicalizesIDAndNormalizesLabel(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	token := makeTestJWT(t, map[string]any{
		"sub": "auth0|SZltqUcP3OMO8k3f18EOszJr",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
		"email": "new@example.com",
		"cid":   "client-xyz",
		"exp":   time.Now().Add(30 * time.Minute).Unix(),
	})

	input := []managedAccount{
		{
			Label:        "auth0|SZltqUcP3OMO8k3f18EOszJr",
			Email:        "",
			AccountID:    "auth0|SZltqUcP3OMO8k3f18EOszJr",
			AccessToken:  token,
			RefreshToken: "refresh-token",
		},
	}

	output, changed := migrateManagedAccounts(input)
	if !changed {
		t.Fatalf("expected changed=true from migration")
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output account, got %d", len(output))
	}

	got := output[0]
	if got.AccountID != uuid {
		t.Fatalf("expected canonical uuid account id %q, got %q", uuid, got.AccountID)
	}
	if got.Email != "new@example.com" {
		t.Fatalf("expected email from token, got %q", got.Email)
	}
	if got.Label != "new@example.com" {
		t.Fatalf("expected label normalized to email, got %q", got.Label)
	}
	if got.ClientID != "client-xyz" {
		t.Fatalf("expected client id from token, got %q", got.ClientID)
	}
	if got.ExpiresAt == 0 {
		t.Fatalf("expected expires_at to be backfilled")
	}
}

func makeTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]any{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + ".sig"
}
