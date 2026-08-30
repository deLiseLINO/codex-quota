package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPiPath_PrecedenceAndResolution(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Case 1: Default resolution
	expectedDefault := filepath.Join(tmp, ".pi", "agent", "auth.json")
	if got := piAuthPath(); got != expectedDefault {
		t.Errorf("piAuthPath() default = %q, want %q", got, expectedDefault)
	}
	if paths := piAuthPaths(); len(paths) != 1 || paths[0] != expectedDefault {
		t.Errorf("piAuthPaths() default = %v, want [%s]", paths, expectedDefault)
	}

	// Case 2: PI_CODING_AGENT_DIR override
	agentDir := filepath.Join(tmp, "custom-pi-agent")
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	expectedAgentDir := filepath.Join(agentDir, "auth.json")
	if got := piAuthPath(); got != expectedAgentDir {
		t.Errorf("piAuthPath() with PI_CODING_AGENT_DIR = %q, want %q", got, expectedAgentDir)
	}

	// Case 3: CQ_PI_AUTH_PATH explicit test override wins over PI_CODING_AGENT_DIR
	customAuth := filepath.Join(tmp, "override", "auth.json")
	t.Setenv("CQ_PI_AUTH_PATH", customAuth)
	if got := piAuthPath(); got != customAuth {
		t.Errorf("piAuthPath() with CQ_PI_AUTH_PATH = %q, want %q", got, customAuth)
	}

	// Case 4: HasExistingPiAuth
	if HasExistingPiAuth() {
		t.Errorf("HasExistingPiAuth() should be false before file creation")
	}
	_ = os.MkdirAll(filepath.Dir(customAuth), 0o700)
	_ = os.WriteFile(customAuth, []byte(`{}`), 0o600)
	if !HasExistingPiAuth() {
		t.Errorf("HasExistingPiAuth() should be true after file creation")
	}
	// Case 5: Empty path returns nil
	t.Setenv("CQ_PI_AUTH_PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "")
	if got := piAuthPath(); got != "" {
		t.Errorf("piAuthPath() with empty HOME = %q, want empty", got)
	}
	if paths := piAuthPaths(); paths != nil {
		t.Errorf("piAuthPaths() with empty HOME = %v, want nil", paths)
	}
	if HasExistingPiAuth() {
		t.Errorf("HasExistingPiAuth() with empty path should be false")
	}
}

func TestLoadPiAccountFile_StrictOAuthOnly(t *testing.T) {
	tests := []struct {
		name        string
		jsonContent string
		wantAccount bool
		wantAccess  string
		wantEmail   string
		wantAccID   string
		wantExpMs   int64
	}{
		{
			name: "valid openai-codex oauth",
			jsonContent: `{
				"openai-codex": {
					"type": "oauth",
					"access": "tok-pi-access",
					"refresh": "tok-pi-refresh",
					"accountId": "acc-123",
					"email": "pi@example.com",
					"expires": 1800000000000
				}
			}`,
			wantAccount: true,
			wantAccess:  "tok-pi-access",
			wantEmail:   "pi@example.com",
			wantAccID:   "acc-123",
			wantExpMs:   1800000000000,
		},
		{
			name: "untyped openai-codex with access token",
			jsonContent: `{
				"openai-codex": {
					"access": "tok-untyped",
					"accountId": "acc-456"
				}
			}`,
			wantAccount: true,
			wantAccess:  "tok-untyped",
			wantAccID:   "acc-456",
		},
		{
			name: "openai-codex with api_key type ignored",
			jsonContent: `{
				"openai-codex": {
					"type": "api_key",
					"key": "sk-static-key"
				}
			}`,
			wantAccount: false,
		},
		{
			name: "standard openai api_key ignored to avoid conflation",
			jsonContent: `{
				"openai": {
					"type": "api_key",
					"key": "sk-openai-key"
				}
			}`,
			wantAccount: false,
		},
		{
			name:        "empty JSON root",
			jsonContent: `{}`,
			wantAccount: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			authFile := filepath.Join(tmp, "auth.json")
			if err := os.WriteFile(authFile, []byte(tt.jsonContent), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}

			acct, err := loadPiAccountFile(authFile, SourcePi, true)
			if err != nil {
				t.Fatalf("loadPiAccountFile error: %v", err)
			}

			if !tt.wantAccount {
				if acct != nil {
					t.Errorf("expected nil account, got %+v", acct)
				}
				return
			}

			if acct == nil {
				t.Fatalf("expected account, got nil")
			}
			if acct.AccessToken != tt.wantAccess {
				t.Errorf("AccessToken = %q, want %q", acct.AccessToken, tt.wantAccess)
			}
			if tt.wantEmail != "" && acct.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", acct.Email, tt.wantEmail)
			}
			if tt.wantAccID != "" && acct.AccountID != tt.wantAccID {
				t.Errorf("AccountID = %q, want %q", acct.AccountID, tt.wantAccID)
			}
			if tt.wantExpMs > 0 && acct.ExpiresAt.UnixMilli() != tt.wantExpMs {
				t.Errorf("ExpiresAt ms = %d, want %d", acct.ExpiresAt.UnixMilli(), tt.wantExpMs)
			}
			if acct.Source != SourcePi {
				t.Errorf("Source = %q, want %q", acct.Source, SourcePi)
			}
		})
	}
}

func TestLoadPiAccountFile_ErrorsAndMissing(t *testing.T) {
	tmp := t.TempDir()

	// Case 1: Missing file returns nil, nil
	acct, err := loadPiAccountFile(filepath.Join(tmp, "nonexistent.json"), SourcePi, true)
	if err != nil || acct != nil {
		t.Errorf("missing file: got acct=%v, err=%v, want nil, nil", acct, err)
	}

	// Case 2: Corrupted JSON returns descriptive error
	corruptFile := filepath.Join(tmp, "corrupt.json")
	_ = os.WriteFile(corruptFile, []byte("invalid-json{"), 0o600)
	acct, err = loadPiAccountFile(corruptFile, SourcePi, true)
	if err == nil {
		t.Errorf("expected error on corrupt JSON, got nil")
	}
}

func TestApplyAndSavePiAccount_PermissionsAndProviderPreservation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	piAuth := filepath.Join(tmp, "auth.json")
	t.Setenv("CQ_PI_AUTH_PATH", piAuth)

	// Seed existing auth.json with unrelated providers and extra metadata
	initial := `{
		"anthropic": {"type": "api_key", "key": "sk-ant-123"},
		"google": {"type": "api_key", "key": "sk-goog-456"},
		"openai": {"type": "api_key", "key": "sk-openai-api-key"},
		"custom_meta": {"version": 1}
	}`
	_ = os.WriteFile(piAuth, []byte(initial), 0o600)

	acct := &Account{
		AccessToken:  "tok-applied-pi",
		RefreshToken: "ref-applied-pi",
		AccountID:    "acc-pi-999",
		Email:        "user@pi.dev",
		ExpiresAt:    time.UnixMilli(1890000000000),
		Source:       SourcePi,
		FilePath:     piAuth,
		Writable:     true,
	}

	appliedPath, err := ApplyAccountToPi(acct)
	if err != nil {
		t.Fatalf("ApplyAccountToPi error: %v", err)
	}
	if appliedPath != piAuth {
		t.Errorf("appliedPath = %q, want %q", appliedPath, piAuth)
	}

	// Verify file mode
	info, err := os.Stat(piAuth)
	if err != nil {
		t.Fatalf("stat auth.json: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected 0600 permissions on auth.json, got %#o", perm)
	}

	root, err := readJSONMap(piAuth)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}

	// Verify other providers and custom fields are preserved
	if anthropic := asMap(root["anthropic"]); anthropic == nil || anthropic["key"] != "sk-ant-123" {
		t.Errorf("anthropic key corrupted: %v", root["anthropic"])
	}
	if google := asMap(root["google"]); google == nil || google["key"] != "sk-goog-456" {
		t.Errorf("google key corrupted: %v", root["google"])
	}
	if openai := asMap(root["openai"]); openai == nil || openai["key"] != "sk-openai-api-key" {
		t.Errorf("openai api key corrupted: %v", root["openai"])
	}

	// Verify openai-codex has correct oauth credentials
	codexObj := asMap(root["openai-codex"])
	if codexObj == nil {
		t.Fatalf("expected openai-codex in root")
	}
	if codexObj["type"] != "oauth" {
		t.Errorf("expected type oauth, got %v", codexObj["type"])
	}
	if codexObj["access"] != "tok-applied-pi" {
		t.Errorf("expected access tok-applied-pi, got %v", codexObj["access"])
	}
	if codexObj["refresh"] != "ref-applied-pi" {
		t.Errorf("expected refresh ref-applied-pi, got %v", codexObj["refresh"])
	}
	if codexObj["accountId"] != "acc-pi-999" {
		t.Errorf("expected accountId acc-pi-999, got %v", codexObj["accountId"])
	}
	if codexObj["email"] != "user@pi.dev" {
		t.Errorf("expected email user@pi.dev, got %v", codexObj["email"])
	}

	// Test SaveAccount via source dispatch
	acct.AccessToken = "tok-pi-saved"
	if err := SaveAccount(acct); err != nil {
		t.Fatalf("SaveAccount error: %v", err)
	}
	rootSaved, _ := readJSONMap(piAuth)
	if codexSaved := asMap(rootSaved["openai-codex"]); codexSaved == nil || codexSaved["access"] != "tok-pi-saved" {
		t.Errorf("SaveAccount failed to update access token: %v", rootSaved["openai-codex"])
	}
}

func TestDeletePiAuthAccount_IsolationAndSafety(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	piAuth := filepath.Join(tmp, "auth.json")
	t.Setenv("CQ_PI_AUTH_PATH", piAuth)

	// Seed auth.json with openai-codex, static openai, and anthropic
	initial := `{
		"anthropic": {"type": "api_key", "key": "sk-ant"},
		"openai": {"type": "api_key", "key": "sk-openai"},
		"openai-codex": {"type": "oauth", "access": "tok-delete-me"}
	}`
	_ = os.WriteFile(piAuth, []byte(initial), 0o600)

	// Case 1: Delete with a SourceManaged account (whose FilePath is accounts.json)
	// Must target canonical piAuthPath() without touching accounts.json
	cqAccountsPath := filepath.Join(tmp, "accounts.json")
	accountsContent := `{"accounts":[{"account_id":"acc-1"}]}`
	_ = os.WriteFile(cqAccountsPath, []byte(accountsContent), 0o600)

	managedAcct := &Account{
		AccountID: "acc-1",
		Source:    SourceManaged,
		FilePath:  cqAccountsPath,
		Writable:  true,
	}

	if err := DeletePiAuthAccount(managedAcct); err != nil {
		t.Fatalf("DeletePiAuthAccount error: %v", err)
	}

	// Verify openai-codex deleted from Pi auth.json
	rootAfter, err := readJSONMap(piAuth)
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if _, ok := rootAfter["openai-codex"]; ok {
		t.Errorf("expected openai-codex deleted from pi auth.json")
	}
	// Verify static openai and anthropic are preserved
	if _, ok := rootAfter["openai"]; !ok {
		t.Errorf("openai static key should remain after codex deletion")
	}
	if _, ok := rootAfter["anthropic"]; !ok {
		t.Errorf("anthropic key should remain after codex deletion")
	}

	// Verify CQ accounts.json was NOT touched
	rawAccounts, _ := os.ReadFile(cqAccountsPath)
	if string(rawAccounts) != accountsContent {
		t.Errorf("accounts.json was corrupted by Pi deletion!")
	}

	// Case 2: Deleting again when openai-codex is already gone is a safe no-op
	if err := DeletePiAuthAccount(managedAcct); err != nil {
		t.Errorf("second delete should be safe no-op, got: %v", err)
	}

	// Case 3: Delete with nil account is safe no-op
	if err := DeletePiAuthAccount(nil); err != nil {
		t.Errorf("delete nil should be safe no-op, got: %v", err)
	}

	// Case 4: Delete when auth.json is missing on disk -> safe no-op
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "missing_auth.json"))
	if err := DeletePiAuthAccount(managedAcct); err != nil {
		t.Errorf("delete missing file should be safe no-op, got: %v", err)
	}

	// Case 5: Delete when auth.json is corrupted JSON -> returns error
	corruptAuth := filepath.Join(tmp, "corrupt_auth.json")
	_ = os.WriteFile(corruptAuth, []byte("bad-json{"), 0o600)
	t.Setenv("CQ_PI_AUTH_PATH", corruptAuth)
	if err := DeletePiAuthAccount(managedAcct); err == nil {
		t.Errorf("expected error on deleting from corrupt JSON, got nil")
	}

	// Case 6: Delete when piAuthPath() is empty -> safe no-op
	t.Setenv("CQ_PI_AUTH_PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "")
	if err := DeletePiAuthAccount(managedAcct); err != nil {
		t.Errorf("delete with empty path should be safe no-op, got: %v", err)
	}
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_PI_AUTH_PATH", piAuth)
}

func TestApplyAccountToPi_ErrorsAndEdgeCases(t *testing.T) {
	tmp := t.TempDir()

	// 1. Nil account returns error
	if _, err := ApplyAccountToPi(nil); err == nil {
		t.Errorf("expected error applying nil account to Pi")
	}

	// 2. Empty path returns error
	t.Setenv("CQ_PI_AUTH_PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "")
	acct := &Account{AccessToken: "tok", Source: SourcePi}
	if _, err := ApplyAccountToPi(acct); err == nil {
		t.Errorf("expected error when Pi path is empty")
	}
	t.Setenv("HOME", tmp)

	// 3. Corrupt JSON in existing auth.json returns error
	corruptAuth := filepath.Join(tmp, "corrupt.json")
	_ = os.WriteFile(corruptAuth, []byte("bad-json{"), 0o600)
	t.Setenv("CQ_PI_AUTH_PATH", corruptAuth)
	if _, err := ApplyAccountToPi(acct); err == nil {
		t.Errorf("expected error when existing Pi auth.json is corrupt")
	}
}
