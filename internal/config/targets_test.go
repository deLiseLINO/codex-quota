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
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
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
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
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
	ompDB, err := sql.Open("sqlite", filepath.Join(tmp, "omp", "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ompDB.Exec(`INSERT INTO auth_credentials (provider, credential_type, data) VALUES ('anthropic', 'api-key', '{"key":"keep"}')`); err != nil {
		t.Fatal(err)
	}
	ompDB.Close()

	// 1. Delete from Managed
	if err := DeleteAccountFromSource(acct, SourceManaged); err != nil {
		t.Errorf("delete managed: %v", err)
	}
	if managed, err := LoadManagedAccounts(); err != nil || len(managed) != 0 {
		t.Fatalf("managed removal: accounts=%#v err=%v", managed, err)
	}

	// 2. Delete from Codex
	if err := DeleteAccountFromSource(acct, SourceCodex); err != nil {
		t.Errorf("delete codex: %v", err)
	}
	if codex, err := loadCodexAccountFile(codexAuthPath()); err != nil || codex != nil {
		t.Fatalf("codex removal: account=%#v err=%v", codex, err)
	}

	// 3. Delete from OpenCode
	if err := DeleteAccountFromSource(acct, SourceOpenCode); err != nil {
		t.Errorf("delete opencode: %v", err)
	}
	if openCode, err := loadOpenCodeAccountFile(opencodeAuthPath(), SourceOpenCode, true); err != nil || openCode != nil {
		t.Fatalf("opencode removal: account=%#v err=%v", openCode, err)
	}

	// 4. Delete from Pi
	if err := DeleteAccountFromSource(acct, SourcePi); err != nil {
		t.Errorf("delete pi: %v", err)
	}
	if pi, err := loadPiAccountFile(piAuthPath(), SourcePi, true); err != nil || pi != nil {
		t.Fatalf("pi removal: account=%#v err=%v", pi, err)
	}

	// 5. Delete from OMP
	if err := DeleteAccountFromSource(acct, SourceOMP); err != nil {
		t.Errorf("delete omp: %v", err)
	}
	if omp, err := loadOMPAccounts(ompAgentDbPath()); err != nil || len(omp) != 0 {
		t.Fatalf("OMP removal: accounts=%#v err=%v", omp, err)
	}
	ompDB, err = sql.Open("sqlite", ompAgentDbPath())
	if err != nil {
		t.Fatal(err)
	}
	defer ompDB.Close()
	var providers int
	if err := ompDB.QueryRow(`SELECT COUNT(*) FROM auth_credentials WHERE provider = 'anthropic' AND json_extract(data, '$.key') = 'keep'`).Scan(&providers); err != nil || providers != 1 {
		t.Fatalf("unrelated provider changed: count=%d err=%v", providers, err)
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

	// 4. Seed a legacy OMP database with two Codex rows; only the first is active.
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

	if gotUnique := result.ActiveSourcesByIdentity["account:acc-unique-omp"]; !reflect.DeepEqual(gotUnique, []string{"omp"}) {
		t.Errorf("OMP pool account should receive an active badge: %v", gotUnique)
	}
}
