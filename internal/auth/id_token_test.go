package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestAccountFromTokenResponseCapturesIDToken(t *testing.T) {
	tok := makeAuthTestJWT(t, map[string]any{
		"email": "user@example.com",
		"sub":   "acc-1",
		"exp":   int64(time.Now().Add(time.Hour).Unix()),
	})
	resp := &tokenExchangeResponse{
		AccessToken:  tok,
		RefreshToken: "refresh",
		IDToken:      "idt-1",
	}

	account, err := accountFromTokenResponse(resp)
	if err != nil {
		t.Fatalf("account from token response: %v", err)
	}
	if account.IDToken != "idt-1" {
		t.Fatalf("expected id_token captured, got %q", account.IDToken)
	}
}

func TestRefreshTokenCapturesIDToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-tok","refresh_token":"new-refresh","id_token":"idt-new","expires_in":3600}`))
	}))
	defer server.Close()
	t.Setenv("CQ_TOKEN_URL", server.URL+"/oauth/token")
	setupAuthTokenEnv(t)

	account := &config.Account{
		AccountID:    "acc-1",
		AccessToken:  "old-tok",
		RefreshToken: "refresh-old",
		ClientID:     "client-1",
		ExpiresAt:    time.Now(),
		Source:       config.SourceManaged,
		Writable:     true,
	}
	if err := RefreshToken(account); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if account.IDToken != "idt-new" {
		t.Fatalf("expected id_token captured on refresh, got %q", account.IDToken)
	}
	if account.AccessToken != "new-tok" {
		t.Fatalf("expected access token updated, got %q", account.AccessToken)
	}
}

func TestCopyResolvedAccountCopiesIDToken(t *testing.T) {
	target := &config.Account{AccountID: "acc-1", AccessToken: "old-access", RefreshToken: "old-refresh"}
	fresh := &config.Account{AccountID: "acc-1", AccessToken: "fresh-access", RefreshToken: "fresh-refresh", IDToken: "fresh-idt"}

	copyResolvedAccount(target, fresh)

	if target.IDToken != "fresh-idt" {
		t.Fatalf("expected id_token copied, got %q", target.IDToken)
	}
}

func setupAuthTokenEnv(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")
	t.Setenv("CODEX_HOME", tmp+"/codex")
	t.Setenv("OPENCODE_AUTH_PATH", tmp+"/opencode/auth.json")
	t.Setenv("OPENCODE_DATA_DIR", tmp+"/opencode-data")
	t.Setenv("HOME", tmp+"/home")
}

func makeAuthTestJWT(t *testing.T, claims map[string]any) string {
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
