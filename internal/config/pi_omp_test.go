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

func TestLoadPiAccountFile_OpenAICodexOnly(t *testing.T) {
	tmp := t.TempDir()
	authPath := filepath.Join(tmp, ".pi", "agent", "auth.json")
	if err := os.MkdirAll(filepath.Dir(authPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Contains an OpenAI API key (which must NOT be loaded as Codex OAuth) and an openai-codex OAuth block
	payload := `{
		"anthropic": {"type": "api_key", "key": "sk-ant-test"},
		"openai": {"type": "api_key", "key": "sk-openai-api-key-do-not-conflate"},
		"openai-codex": {
			"type": "oauth",
			"access": "tok-pi-access",
			"refresh": "tok-pi-refresh",
			"accountId": "acc-pi-123",
			"email": "pi-user@example.com",
			"expires": 1800000000000
		}
	}`
	if err := os.WriteFile(authPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	acct, err := loadPiAccountFile(authPath, SourcePi, true)
	if err != nil {
		t.Fatalf("loadPiAccountFile error: %v", err)
	}
	if acct == nil {
		t.Fatalf("expected account loaded, got nil")
	}

	if acct.AccessToken != "tok-pi-access" {
		t.Errorf("got AccessToken %q, want %q", acct.AccessToken, "tok-pi-access")
	}
	if acct.RefreshToken != "tok-pi-refresh" {
		t.Errorf("got RefreshToken %q, want %q", acct.RefreshToken, "tok-pi-refresh")
	}
	if acct.AccountID != "acc-pi-123" {
		t.Errorf("got AccountID %q, want %q", acct.AccountID, "acc-pi-123")
	}
	if acct.Email != "pi-user@example.com" {
		t.Errorf("got Email %q, want %q", acct.Email, "pi-user@example.com")
	}
	if acct.Source != SourcePi {
		t.Errorf("got Source %q, want %q", acct.Source, SourcePi)
	}
	if !acct.Writable {
		t.Errorf("expected writable true")
	}
}

func TestLoadPiAccountFile_IgnoresOpenAIApiKeyWhenNoCodexOAuth(t *testing.T) {
	tmp := t.TempDir()
	authPath := filepath.Join(tmp, "auth.json")
	payload := `{
		"openai": {"type": "api_key", "key": "sk-openai-only"}
	}`
	if err := os.WriteFile(authPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("write auth.json: %v", err)
	}

	acct, err := loadPiAccountFile(authPath, SourcePi, true)
	if err != nil {
		t.Fatalf("loadPiAccountFile error: %v", err)
	}
	if acct != nil {
		t.Errorf("expected nil for apiKey-only openai block, got %+v", acct)
	}
}

func TestApplyAndSavePiAccount_PreservesOtherProviders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "auth.json"))

	initial := `{
		"anthropic": {"type": "api_key", "key": "sk-ant-keep-me"},
		"openai": {"type": "api_key", "key": "sk-openai-keep-me"}
	}`
	if err := os.WriteFile(filepath.Join(tmp, "auth.json"), []byte(initial), 0o600); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	acct := &Account{
		AccessToken:  "tok-applied-pi",
		RefreshToken: "ref-applied-pi",
		AccountID:    "acc-applied-pi",
		Email:        "applied@pi.dev",
		ExpiresAt:    time.UnixMilli(1900000000000),
		Source:       SourcePi,
		FilePath:     filepath.Join(tmp, "auth.json"),
		Writable:     true,
	}

	appliedPath, err := ApplyAccountToPi(acct)
	if err != nil {
		t.Fatalf("ApplyAccountToPi error: %v", err)
	}
	if appliedPath != filepath.Join(tmp, "auth.json") {
		t.Errorf("appliedPath = %q, want %q", appliedPath, filepath.Join(tmp, "auth.json"))
	}

	root, err := readJSONMap(filepath.Join(tmp, "auth.json"))
	if err != nil {
		t.Fatalf("read applied json: %v", err)
	}

	// Verify Anthropic preserved
	anthropic := asMap(root["anthropic"])
	if anthropic == nil || anthropic["key"] != "sk-ant-keep-me" {
		t.Errorf("anthropic provider not preserved: %v", root["anthropic"])
	}

	// Verify OpenAI API key preserved and NOT overwritten
	openaiApiKey := asMap(root["openai"])
	if openaiApiKey == nil || openaiApiKey["key"] != "sk-openai-keep-me" {
		t.Errorf("openai api key not preserved: %v", root["openai"])
	}

	// Verify OpenAI Codex updated
	codexObj := asMap(root["openai-codex"])
	if codexObj == nil {
		t.Fatalf("expected openai-codex object in root: %v", root)
	}
	if codexObj["access"] != "tok-applied-pi" {
		t.Errorf("got access %v, want tok-applied-pi", codexObj["access"])
	}
	if codexObj["accountId"] != "acc-applied-pi" {
		t.Errorf("got accountId %v, want acc-applied-pi", codexObj["accountId"])
	}

	// Test DeletePiAuthAccount
	if err := DeletePiAuthAccount(acct); err != nil {
		t.Fatalf("DeletePiAuthAccount error: %v", err)
	}

	rootAfterDelete, err := readJSONMap(filepath.Join(tmp, "auth.json"))
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if _, ok := rootAfterDelete["openai-codex"]; ok {
		t.Errorf("expected openai-codex deleted, still present: %v", rootAfterDelete)
	}
	if anthropicAfter := asMap(rootAfterDelete["anthropic"]); anthropicAfter == nil || anthropicAfter["key"] != "sk-ant-keep-me" {
		t.Errorf("anthropic should remain after codex deletion")
	}
	if openaiAfter := asMap(rootAfterDelete["openai"]); openaiAfter == nil || openaiAfter["key"] != "sk-openai-keep-me" {
		t.Errorf("openai api key should remain after codex deletion")
	}
}
func TestDeletePiAuthAccount_ManagedSourceAccountFilePathDoesNotCorruptAccountsJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	piAuth := filepath.Join(tmp, "auth.json")
	t.Setenv("CQ_PI_AUTH_PATH", piAuth)

	// Seed Pi auth.json
	_ = os.WriteFile(piAuth, []byte(`{"openai-codex":{"type":"oauth","access":"tok-pi","accountId":"acc-managed"}}`), 0o600)

	// Seed CQ accounts.json
	cqAccountsPath := filepath.Join(tmp, "accounts.json")
	accountsJSONContent := `{"accounts":[{"account_id":"acc-managed","access_token":"tok-managed"}]}`
	_ = os.WriteFile(cqAccountsPath, []byte(accountsJSONContent), 0o600)

	// Account originated from managed storage (FilePath points to accounts.json)
	managedAcct := &Account{
		AccountID:   "acc-managed",
		AccessToken: "tok-managed",
		Source:      SourceManaged,
		FilePath:    cqAccountsPath,
		Writable:    true,
	}

	if err := DeletePiAuthAccount(managedAcct); err != nil {
		t.Fatalf("DeletePiAuthAccount error: %v", err)
	}

	// Verify Pi auth.json has openai-codex deleted
	piRoot, err := readJSONMap(piAuth)
	if err != nil {
		t.Fatalf("read pi auth: %v", err)
	}
	if _, ok := piRoot["openai-codex"]; ok {
		t.Errorf("expected openai-codex deleted from Pi auth.json")
	}

	// Verify CQ accounts.json was NOT modified or deleted
	contentAfter, err := os.ReadFile(cqAccountsPath)
	if err != nil {
		t.Fatalf("accounts.json should remain intact: %v", err)
	}
	if string(contentAfter) != accountsJSONContent {
		t.Errorf("accounts.json was corrupted by Pi delete! got %s, want %s", string(contentAfter), accountsJSONContent)
	}
}

func TestDeleteOMPAuthAccount_ManagedSourceAccountFilePathDoesNotCorruptAccountsJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	// Seed OMP agent.db
	acct := &Account{
		AccountID:   "acc-managed-omp",
		AccessToken: "tok-omp",
		Source:      SourceOMP,
		Writable:    true,
	}
	if _, err := ApplyAccountToOMP(acct); err != nil {
		t.Fatalf("apply to OMP: %v", err)
	}

	// Seed CQ accounts.json
	cqAccountsPath := filepath.Join(tmp, "accounts.json")
	accountsJSONContent := `{"accounts":[{"account_id":"acc-managed-omp","access_token":"tok-managed"}]}`
	_ = os.WriteFile(cqAccountsPath, []byte(accountsJSONContent), 0o600)

	// Account originated from managed storage (FilePath points to accounts.json)
	managedAcct := &Account{
		AccountID:   "acc-managed-omp",
		AccessToken: "tok-managed",
		Source:      SourceManaged,
		FilePath:    cqAccountsPath,
		Writable:    true,
	}

	if err := DeleteOMPAuthAccount(managedAcct); err != nil {
		t.Fatalf("DeleteOMPAuthAccount error: %v", err)
	}

	// Verify OMP agent.db has account deleted
	ompAccounts, err := loadOMPAccounts(dbPath)
	if err != nil {
		t.Fatalf("load OMP accounts: %v", err)
	}
	if len(ompAccounts) != 0 {
		t.Errorf("expected 0 accounts in OMP db after delete, got %d", len(ompAccounts))
	}

	// Verify CQ accounts.json was NOT modified or deleted
	contentAfter, err := os.ReadFile(cqAccountsPath)
	if err != nil {
		t.Fatalf("accounts.json should remain intact: %v", err)
	}
	if string(contentAfter) != accountsJSONContent {
		t.Errorf("accounts.json was corrupted by OMP delete! got %s, want %s", string(contentAfter), accountsJSONContent)
	}
}

func TestLoadAndApplyOMPAccount_PoolPreservation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	// Account 1
	acct1 := &Account{
		AccessToken:  "tok-omp-1",
		RefreshToken: "ref-omp-1",
		AccountID:    "acc-omp-1",
		Email:        "user1@omp.sh",
		ExpiresAt:    time.UnixMilli(1850000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	appliedPath1, err := ApplyAccountToOMP(acct1)
	if err != nil {
		t.Fatalf("ApplyAccountToOMP acct1 error: %v", err)
	}
	if appliedPath1 != dbPath {
		t.Errorf("appliedPath = %q, want %q", appliedPath1, dbPath)
	}

	// Account 2 (should be added to pool, NOT wiping Account 1)
	acct2 := &Account{
		AccessToken:  "tok-omp-2",
		RefreshToken: "ref-omp-2",
		AccountID:    "acc-omp-2",
		Email:        "user2@omp.sh",
		ExpiresAt:    time.UnixMilli(1860000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	appliedPath2, err := ApplyAccountToOMP(acct2)
	if err != nil {
		t.Fatalf("ApplyAccountToOMP acct2 error: %v", err)
	}
	if appliedPath2 != dbPath {
		t.Errorf("appliedPath = %q, want %q", appliedPath2, dbPath)
	}

	// Verify both accounts exist in OMP database pool
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil {
		t.Fatalf("loadOMPAccounts error: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 pooled accounts in OMP database, got %d", len(accounts))
	}

	// Update Account 1 in pool
	acct1Updated := &Account{
		AccessToken:  "tok-omp-1-refreshed",
		RefreshToken: "ref-omp-1-refreshed",
		AccountID:    "acc-omp-1",
		Email:        "user1@omp.sh",
		ExpiresAt:    time.UnixMilli(1870000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}
	if _, err := ApplyAccountToOMP(acct1Updated); err != nil {
		t.Fatalf("ApplyAccountToOMP update error: %v", err)
	}

	accountsAfterUpdate, err := loadOMPAccounts(dbPath)
	if err != nil {
		t.Fatalf("loadOMPAccounts error: %v", err)
	}
	if len(accountsAfterUpdate) != 2 {
		t.Fatalf("expected still 2 pooled accounts after update, got %d", len(accountsAfterUpdate))
	}

	foundUpdated := false
	for _, a := range accountsAfterUpdate {
		if a.AccountID == "acc-omp-1" && a.AccessToken == "tok-omp-1-refreshed" {
			foundUpdated = true
		}
	}
	if !foundUpdated {
		t.Errorf("expected acc-omp-1 updated in pool")
	}

	// Test deleting Account 1: Account 2 must be preserved in OMP's pool
	if err := DeleteOMPAuthAccount(acct1Updated); err != nil {
		t.Fatalf("DeleteOMPAuthAccount error: %v", err)
	}

	accountsAfterDelete, err := loadOMPAccounts(dbPath)
	if err != nil {
		t.Fatalf("loadOMPAccounts error: %v", err)
	}
	if len(accountsAfterDelete) != 1 {
		t.Fatalf("expected 1 remaining account after deleting account 1, got %d", len(accountsAfterDelete))
	}
	if accountsAfterDelete[0].AccountID != "acc-omp-2" {
		t.Errorf("expected remaining account acc-omp-2, got %q", accountsAfterDelete[0].AccountID)
	}
}
func TestApplyAccountToOMP_ProfileIsolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("OMP_PROFILE", "work")

	acct := &Account{
		AccessToken:  "tok-work-omp",
		RefreshToken: "ref-work-omp",
		AccountID:    "acc-work",
		Email:        "work@company.com",
		ExpiresAt:    time.UnixMilli(1850000000000),
		Source:       SourceOMP,
		Writable:     true,
	}

	appliedPath, err := ApplyAccountToOMP(acct)
	if err != nil {
		t.Fatalf("ApplyAccountToOMP error: %v", err)
	}

	expectedWorkDb := filepath.Join(tmp, ".omp", "profiles", "work", "agent", "agent.db")
	if appliedPath != expectedWorkDb {
		t.Errorf("appliedPath = %q, want %q", appliedPath, expectedWorkDb)
	}

	// Ensure default profile DB was NOT created or written
	defaultDb := filepath.Join(tmp, ".omp", "agent", "agent.db")
	if _, err := os.Stat(defaultDb); err == nil {
		t.Errorf("default profile db should not exist when OMP_PROFILE=work is active")
	}

	// Verify work db contains the account
	workAccounts, err := loadOMPAccounts(expectedWorkDb)
	if err != nil || len(workAccounts) != 1 {
		t.Fatalf("expected 1 account in work db, got %d, err: %v", len(workAccounts), err)
	}
	if workAccounts[0].AccountID != "acc-work" {
		t.Errorf("expected acc-work, got %q", workAccounts[0].AccountID)
	}
}

func TestApplyAccountToOMP_FilePermissionsAndSchemaVersion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	acct := &Account{
		AccessToken:  "tok-perm-test",
		RefreshToken: "ref-perm-test",
		AccountID:    "acc-perm",
		Email:        "perm@omp.sh",
		ExpiresAt:    time.UnixMilli(1850000000000),
		Source:       SourceOMP,
		Writable:     true,
	}

	if _, err := ApplyAccountToOMP(acct); err != nil {
		t.Fatalf("ApplyAccountToOMP error: %v", err)
	}

	// Check file mode is 0600
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat agent.db: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected agent.db permissions 0600, got %#o", perm)
	}

	// Check auth_schema_version is initialized to 7
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var version int
	err = db.QueryRow("SELECT version FROM auth_schema_version WHERE id = 1").Scan(&version)
	if err != nil {
		t.Fatalf("query auth_schema_version error: %v", err)
	}
	if version != 7 {
		t.Errorf("expected auth_schema_version 7, got %d", version)
	}
}

func TestDeleteOMPAuthAccount_EmptyIdentityGuard(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	acct := &Account{
		AccessToken: "tok-valid",
		AccountID:   "acc-keep-me",
		Email:       "keep@omp.sh",
		Source:      SourceOMP,
		Writable:    true,
	}
	if _, err := ApplyAccountToOMP(acct); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Deleting with empty accountID and empty email should be a no-op
	emptyAcct := &Account{
		AccessToken: "tok-empty",
		AccountID:   "",
		Email:       "",
		Source:      SourceOMP,
	}
	if err := DeleteOMPAuthAccount(emptyAcct); err != nil {
		t.Fatalf("delete empty identity error: %v", err)
	}

	// Account should still exist
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 1 {
		t.Fatalf("expected account preserved after empty identity delete, got %d accounts", len(accounts))
	}
}

func TestLoadOMPAccounts_ErrorHandling(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "corrupt.db")

	// Missing file returns nil, nil
	missing, err := loadOMPAccounts(filepath.Join(tmp, "nonexistent.db"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing file, got %v", missing)
	}

	// Corrupted database / bad JSON returns contextual error
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	_, _ = db.Exec(`CREATE TABLE auth_credentials (id INTEGER PRIMARY KEY, provider TEXT, credential_type TEXT, data TEXT, disabled_cause TEXT, identity_key TEXT);`)
	_, _ = db.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', 'not-valid-json', 'account:bad');`)
	db.Close()

	_, err = loadOMPAccounts(dbPath)
	if err == nil {
		t.Fatalf("expected error on corrupt JSON, got nil")
	}
}

func TestLoadAllAccountsWithSources_IncludesPiAndOMP(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("OPENCODE_DATA_DIR", filepath.Join(tmp, "opencode-data"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(tmp, "pi", "auth.json"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	// 1. Codex
	codexPath := filepath.Join(tmp, "codex", "auth.json")
	_ = os.MkdirAll(filepath.Dir(codexPath), 0o700)
	_ = os.WriteFile(codexPath, []byte(`{"tokens":{"access_token":"tok-codex","account_id":"acc-shared"}}`), 0o600)

	// 2. OpenCode
	openCodePath := filepath.Join(tmp, "opencode", "auth.json")
	_ = os.MkdirAll(filepath.Dir(openCodePath), 0o700)
	_ = os.WriteFile(openCodePath, []byte(`{"openai":{"access":"tok-open","accountId":"acc-shared","email":"Shared@Example.com"}}`), 0o600)

	// 3. Pi
	piPath := filepath.Join(tmp, "pi", "auth.json")
	_ = os.MkdirAll(filepath.Dir(piPath), 0o700)
	_ = os.WriteFile(piPath, []byte(`{"openai-codex":{"type":"oauth","access":"tok-pi","accountId":"acc-shared","email":"Shared@Example.com"}}`), 0o600)

	// 4. OMP
	ompDbPath := filepath.Join(tmp, "omp", "agent.db")
	_ = os.MkdirAll(filepath.Dir(ompDbPath), 0o700)
	ompDb, err := sql.Open("sqlite", ompDbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	_, _ = ompDb.Exec(`CREATE TABLE IF NOT EXISTS auth_credentials (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT, credential_type TEXT, data TEXT, disabled_cause TEXT, identity_key TEXT);`)
	ompPayload, _ := json.Marshal(map[string]any{
		"type":      "oauth",
		"access":    "tok-omp",
		"accountId": "acc-shared",
		"email":     "Shared@Example.com",
	})
	_, _ = ompDb.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', ?, 'account:acc-shared');`, string(ompPayload))
	ompDb.Close()

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		t.Fatalf("LoadAllAccountsWithSources error: %v", err)
	}

	gotByAccount := result.ActiveSourcesByIdentity["account:acc-shared"]
	wantByAccount := []string{"codex", "opencode", "pi", "omp"}
	if !reflect.DeepEqual(gotByAccount, wantByAccount) {
		t.Fatalf("active sources by account mismatch: got %v, want %v", gotByAccount, wantByAccount)
	}

	gotByEmail := result.ActiveSourcesByIdentity["email:shared@example.com"]
	wantByEmail := []string{"opencode", "pi", "omp"}
	if !reflect.DeepEqual(gotByEmail, wantByEmail) {
		t.Fatalf("active sources by email mismatch: got %v, want %v", gotByEmail, wantByEmail)
	}
}
func TestLoadAllAccountsWithSources_OMPMultiAccountPoolIndexesAllIdentities(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "omp", "agent.db"))

	ompDbPath := filepath.Join(tmp, "omp", "agent.db")
	_ = os.MkdirAll(filepath.Dir(ompDbPath), 0o700)
	ompDb, err := sql.Open("sqlite", ompDbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	_, _ = ompDb.Exec(`CREATE TABLE IF NOT EXISTS auth_credentials (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT, credential_type TEXT, data TEXT, disabled_cause TEXT, identity_key TEXT);`)

	p1, _ := json.Marshal(map[string]any{"type": "oauth", "access": "tok-omp-1", "accountId": "acc-omp-1", "email": "omp1@example.com"})
	p2, _ := json.Marshal(map[string]any{"type": "oauth", "access": "tok-omp-2", "accountId": "acc-omp-2", "email": "omp2@example.com"})

	_, _ = ompDb.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', ?, 'account:acc-omp-1');`, string(p1))
	_, _ = ompDb.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', ?, 'account:acc-omp-2');`, string(p2))
	ompDb.Close()

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		t.Fatalf("LoadAllAccountsWithSources error: %v", err)
	}

	if sources := result.ActiveSourcesByIdentity["account:acc-omp-1"]; !reflect.DeepEqual(sources, []string{"omp"}) {
		t.Errorf("expected acc-omp-1 to have active source [omp], got %v", sources)
	}
	if sources := result.ActiveSourcesByIdentity["account:acc-omp-2"]; !reflect.DeepEqual(sources, []string{"omp"}) {
		t.Errorf("expected acc-omp-2 to have active source [omp], got %v", sources)
	}
}
