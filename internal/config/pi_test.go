package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadPiCodexAccountFile_ReadsOpenAICodexOAuthCredential(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "auth.json")
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.WriteFile(path, []byte(`{"openai-codex":{"type":"oauth","access":"access-token","refresh":"refresh-token","accountId":"acc-123","expires":1893456000000}}`), 0o600); err != nil {
		t.Fatalf("write pi auth: %v", err)
	}

	account, err := loadPiCodexAccountFile(path)
	if err != nil {
		t.Fatalf("load pi auth: %v", err)
	}
	if account == nil {
		t.Fatalf("expected pi account")
	}
	if account.Source != SourcePi {
		t.Fatalf("Source = %q, want %q", account.Source, SourcePi)
	}
	if account.AccessToken != "access-token" || account.RefreshToken != "refresh-token" || account.AccountID != "acc-123" {
		t.Fatalf("unexpected account fields: %#v", account)
	}
	if !account.ExpiresAt.Equal(expires) {
		t.Fatalf("ExpiresAt = %v, want %v", account.ExpiresAt, expires)
	}
	if account.FilePath != path || !account.Writable {
		t.Fatalf("expected writable account from %q, got path %q writable %v", path, account.FilePath, account.Writable)
	}
}

func TestApplyAccountToPi_WritesOpenAICodexOAuthCredential(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(tmp, "pi", "agent"))

	path := filepath.Join(tmp, "pi", "agent", "auth.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir pi dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"anthropic":{"type":"api_key","key":"ANTHROPIC_API_KEY"}}`), 0o600); err != nil {
		t.Fatalf("write existing auth: %v", err)
	}

	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	gotPath, err := ApplyAccountToPi(&Account{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		AccountID:    "acc-123",
		ExpiresAt:    expires,
	})
	if err != nil {
		t.Fatalf("apply to pi: %v", err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pi auth: %v", err)
	}
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("decode pi auth: %v", err)
	}
	if root["anthropic"] == nil {
		t.Fatalf("expected existing provider credential to be preserved")
	}
	credential, ok := root["openai-codex"].(map[string]any)
	if !ok {
		t.Fatalf("expected openai-codex credential, got %#v", root["openai-codex"])
	}
	if credential["type"] != "oauth" || credential["access"] != "access-token" || credential["refresh"] != "refresh-token" || credential["accountId"] != "acc-123" {
		t.Fatalf("unexpected pi credential: %#v", credential)
	}
	if gotExpires, ok := credential["expires"].(float64); !ok || int64(gotExpires) != expires.UnixMilli() {
		t.Fatalf("expires = %#v, want %d", credential["expires"], expires.UnixMilli())
	}
}

func TestDeletePiAuthAccount_RemovesOpenAICodexCredentialOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(tmp, "pi", "agent"))

	path := filepath.Join(tmp, "pi", "agent", "auth.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir pi dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"openai-codex":{"type":"oauth","access":"access-token"},"anthropic":{"type":"api_key","key":"ANTHROPIC_API_KEY"}}`), 0o600); err != nil {
		t.Fatalf("write existing auth: %v", err)
	}

	if err := DeletePiAuthAccount(); err != nil {
		t.Fatalf("delete pi auth: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pi auth: %v", err)
	}
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("decode pi auth: %v", err)
	}
	if _, ok := root["openai-codex"]; ok {
		t.Fatalf("expected openai-codex credential to be removed: %#v", root)
	}
	if root["anthropic"] == nil {
		t.Fatalf("expected other provider credentials to be preserved")
	}
}
