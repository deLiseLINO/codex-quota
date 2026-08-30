package config

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestApplyAccountToTarget_AllSources(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi", "auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	acct := &Account{
		AccessToken:  "tok-target-test",
		RefreshToken: "ref-target-test",
		AccountID:    "acc-targets",
		Email:        "targets@example.com",
		ExpiresAt:    time.UnixMilli(1880000000000),
		Source:       SourceManaged,
		Writable:     true,
	}

	// 1. Apply to Codex
	codexPath, err := ApplyAccountToTarget(acct, SourceCodex)
	if err != nil || codexPath != filepath.Join(tmp, "codex", "auth.json") {
		t.Errorf("ApplyAccountToTarget(SourceCodex) = %q, err = %v", codexPath, err)
	}

	// 2. Apply to OpenCode
	openCodePath, err := ApplyAccountToTarget(acct, SourceOpenCode)
	if err != nil || openCodePath != filepath.Join(tmp, "opencode", "auth.json") {
		t.Errorf("ApplyAccountToTarget(SourceOpenCode) = %q, err = %v", openCodePath, err)
	}

	// 3. Apply to Pi
	piPath, err := ApplyAccountToTarget(acct, SourcePi)
	if err != nil || piPath != filepath.Join(tmp, "pi", "auth.json") {
		t.Errorf("ApplyAccountToTarget(SourcePi) = %q, err = %v", piPath, err)
	}

	// 4. Apply to OMP
	ompPath, err := ApplyAccountToTarget(acct, SourceOMP)
	if err != nil || ompPath != filepath.Join(tmp, "omp", "agent.db") {
		t.Errorf("ApplyAccountToTarget(SourceOMP) = %q, err = %v", ompPath, err)
	}

	// 5. Unsupported target
	_, err = ApplyAccountToTarget(acct, "unsupported")
	if err == nil {
		t.Errorf("expected error on unsupported apply target, got nil")
	}

	// 6. Nil account
	_, err = ApplyAccountToTarget(nil, SourceCodex)
	if err == nil {
		t.Errorf("expected error on nil account apply, got nil")
	}
}

func TestApplyAccountToTargets_BatchExecutionAndDedupe(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi", "auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	acct := &Account{
		AccessToken:  "tok-batch",
		RefreshToken: "ref-batch",
		AccountID:    "acc-batch",
		Email:        "batch@example.com",
		ExpiresAt:    time.UnixMilli(1880000000000),
		Source:       SourceManaged,
		Writable:     true,
	}

	// Includes duplicates and unsupported sources
	targets := []Source{SourceCodex, SourceOpenCode, SourcePi, SourceOMP, SourceCodex, "invalid-source"}
	paths, errs := ApplyAccountToTargets(acct, targets)
	if len(errs) != 0 {
		t.Errorf("unexpected apply errors: %v", errs)
	}
	if len(paths) != 4 {
		t.Errorf("expected 4 paths applied, got %d: %v", len(paths), paths)
	}

	// Nil account
	_, nilErrs := ApplyAccountToTargets(nil, targets)
	if len(nilErrs) == 0 {
		t.Errorf("expected error for nil account apply to targets")
	}
}

func TestDeleteAccountFromSource_AllSources(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi", "auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	acct := &Account{
		AccessToken:  "tok-del-test",
		RefreshToken: "ref-del-test",
		AccountID:    "acc-del",
		Email:        "del@example.com",
		Source:       SourceManaged,
		Writable:     true,
	}

	// Pre-apply to all targets
	_, _ = ApplyAccountToTargets(acct, []Source{SourceCodex, SourceOpenCode, SourcePi, SourceOMP})
	_ = UpsertManagedAccount(acct)

	// 1. Delete from Managed
	if err := DeleteAccountFromSource(acct, SourceManaged); err != nil {
		t.Errorf("delete managed: %v", err)
	}

	// 2. Delete from Codex
	if err := DeleteAccountFromSource(acct, SourceCodex); err != nil {
		t.Errorf("delete codex: %v", err)
	}

	// 3. Delete from OpenCode
	if err := DeleteAccountFromSource(acct, SourceOpenCode); err != nil {
		t.Errorf("delete opencode: %v", err)
	}

	// 4. Delete from Pi
	if err := DeleteAccountFromSource(acct, SourcePi); err != nil {
		t.Errorf("delete pi: %v", err)
	}

	// 5. Delete from OMP
	if err := DeleteAccountFromSource(acct, SourceOMP); err != nil {
		t.Errorf("delete omp: %v", err)
	}

	// 6. Unsupported source
	if err := DeleteAccountFromSource(acct, "unsupported"); err == nil {
		t.Errorf("expected error on unsupported delete source, got nil")
	}

	// 7. Nil account
	if err := DeleteAccountFromSource(nil, SourceCodex); err == nil {
		t.Errorf("expected error on nil account delete, got nil")
	}
}

func TestLoadAllAccountsWithSources_MultiSourceCombined(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi", "auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	// 1. Seed Codex
	_ = os.MkdirAll(filepath.Join(tmp, "codex"), 0o700)
	_ = os.WriteFile(filepath.Join(tmp, "codex", "auth.json"), []byte(`{"tokens":{"access_token":"tok-c","account_id":"acc-shared"}}`), 0o600)

	// 2. Seed OpenCode
	_ = os.MkdirAll(filepath.Join(tmp, "opencode"), 0o700)
	_ = os.WriteFile(filepath.Join(tmp, "opencode", "auth.json"), []byte(`{"openai":{"access":"tok-o","accountId":"acc-shared","email":"Shared@Example.com"}}`), 0o600)

	// 3. Seed Pi
	_ = os.MkdirAll(filepath.Join(tmp, "pi"), 0o700)
	_ = os.WriteFile(filepath.Join(tmp, "pi", "auth.json"), []byte(`{"openai-codex":{"type":"oauth","access":"tok-p","accountId":"acc-shared","email":"Shared@Example.com"}}`), 0o600)

	// 4. Seed OMP with multiple pooled accounts
	_ = os.MkdirAll(filepath.Join(tmp, "omp"), 0o700)
	ompDbPath := filepath.Join(tmp, "omp", "agent.db")
	ompDb, _ := sql.Open("sqlite", ompDbPath)
	_, _ = ompDb.Exec(`CREATE TABLE IF NOT EXISTS auth_credentials (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT, credential_type TEXT, data TEXT, disabled_cause TEXT, identity_key TEXT);`)
	p1, _ := json.Marshal(map[string]any{"type": "oauth", "access": "tok-omp-1", "accountId": "acc-shared", "email": "Shared@Example.com"})
	p2, _ := json.Marshal(map[string]any{"type": "oauth", "access": "tok-omp-2", "accountId": "acc-unique-omp", "email": "Unique@Example.com"})
	_, _ = ompDb.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', ?, 'account:acc-shared');`, string(p1))
	_, _ = ompDb.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', ?, 'account:acc-unique-omp');`, string(p2))
	ompDb.Close()

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		t.Fatalf("LoadAllAccountsWithSources error: %v", err)
	}

	// Verify all 4 sources indexed for shared account
	gotShared := result.ActiveSourcesByIdentity["account:acc-shared"]
	wantShared := []string{"codex", "opencode", "pi", "omp"}
	if !reflect.DeepEqual(gotShared, wantShared) {
		t.Errorf("active sources for acc-shared mismatch: got %v, want %v", gotShared, wantShared)
	}

	// Verify unique OMP account also has omp active source
	gotUnique := result.ActiveSourcesByIdentity["account:acc-unique-omp"]
	wantUnique := []string{"omp"}
	if !reflect.DeepEqual(gotUnique, wantUnique) {
		t.Errorf("active sources for acc-unique-omp mismatch: got %v, want %v", gotUnique, wantUnique)
	}
}
