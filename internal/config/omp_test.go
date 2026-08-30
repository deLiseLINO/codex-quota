package config

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestOMPPath_PrecedenceAndProfileIsolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Case 1: Default resolution
	expectedDefault := filepath.Join(tmp, ".omp", "agent", "agent.db")
	if got := ompAgentDbPath(); got != expectedDefault {
		t.Errorf("ompAgentDbPath() default = %q, want %q", got, expectedDefault)
	}
	if paths := ompAgentDbPaths(); len(paths) != 1 || paths[0] != expectedDefault {
		t.Errorf("ompAgentDbPaths() default = %v, want [%s]", paths, expectedDefault)
	}

	// Case 2: Named profile via OMP_PROFILE
	t.Setenv("OMP_PROFILE", "work")
	expectedWork := filepath.Join(tmp, ".omp", "profiles", "work", "agent", "agent.db")
	if got := ompAgentDbPath(); got != expectedWork {
		t.Errorf("ompAgentDbPath() with OMP_PROFILE=work = %q, want %q", got, expectedWork)
	}

	// Case 3: Named profile via legacy PI_PROFILE (when OMP_PROFILE unset)
	t.Setenv("OMP_PROFILE", "")
	t.Setenv("PI_PROFILE", "legacy")
	expectedLegacy := filepath.Join(tmp, ".omp", "profiles", "legacy", "agent", "agent.db")
	if got := ompAgentDbPath(); got != expectedLegacy {
		t.Errorf("ompAgentDbPath() with PI_PROFILE=legacy = %q, want %q", got, expectedLegacy)
	}
	t.Setenv("PI_PROFILE", "")

	// Case 4: Custom PI_CONFIG_DIR
	t.Setenv("PI_CONFIG_DIR", ".custom-omp")
	expectedCustom := filepath.Join(tmp, ".custom-omp", "agent", "agent.db")
	if got := ompAgentDbPath(); got != expectedCustom {
		t.Errorf("ompAgentDbPath() with PI_CONFIG_DIR = %q, want %q", got, expectedCustom)
	}
	t.Setenv("PI_CONFIG_DIR", "")

	// Case 5: PI_CODING_AGENT_DIR override
	agentDir := filepath.Join(tmp, "custom-agent-dir")
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	expectedAgentDir := filepath.Join(agentDir, "agent.db")
	if got := ompAgentDbPath(); got != expectedAgentDir {
		t.Errorf("ompAgentDbPath() with PI_CODING_AGENT_DIR = %q, want %q", got, expectedAgentDir)
	}
	t.Setenv("PI_CODING_AGENT_DIR", "")

	// Case 6: CQ_OMP_DB_PATH explicit override wins over all
	cqOverride := filepath.Join(tmp, "cq-test", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", cqOverride)
	if got := ompAgentDbPath(); got != cqOverride {
		t.Errorf("ompAgentDbPath() with CQ_OMP_DB_PATH = %q, want %q", got, cqOverride)
	}
	t.Setenv("CQ_OMP_DB_PATH", "")

	// Case 8: XDG_DATA_HOME with existing profile path
	t.Setenv("OMP_PROFILE", "work")
	xdgData := filepath.Join(tmp, "xdg-data")
	t.Setenv("XDG_DATA_HOME", xdgData)
	xdgProfileDb := filepath.Join(xdgData, "omp", "profiles", "work", "agent", "agent.db")
	_ = os.MkdirAll(filepath.Dir(xdgProfileDb), 0o700)
	_ = os.WriteFile(xdgProfileDb, []byte(""), 0o600)
	if got := ompAgentDbPath(); got != xdgProfileDb {
		t.Errorf("ompAgentDbPath() with existing XDG profile = %q, want %q", got, xdgProfileDb)
	}

	// Case 9: XDG_DATA_HOME default path
	t.Setenv("OMP_PROFILE", "")
	xdgDefaultDb := filepath.Join(xdgData, "omp", "agent", "agent.db")
	_ = os.MkdirAll(filepath.Dir(xdgDefaultDb), 0o700)
	_ = os.WriteFile(xdgDefaultDb, []byte(""), 0o600)
	if got := ompAgentDbPath(); got != xdgDefaultDb {
		t.Errorf("ompAgentDbPath() with existing XDG default = %q, want %q", got, xdgDefaultDb)
	}

	// Case 10: Empty home returns empty
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	if got := ompAgentDbPath(); got != "" {
		t.Errorf("ompAgentDbPath() with empty HOME = %q, want empty", got)
	}
	if paths := ompAgentDbPaths(); paths != nil {
		t.Errorf("ompAgentDbPaths() with empty HOME = %v, want nil", paths)
	}
	if HasExistingOMPAuth() {
		t.Errorf("HasExistingOMPAuth() with empty path should be false")
	}
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_OMP_DB_PATH", "")

	// Case 7: HasExistingOMPAuth
	if HasExistingOMPAuth() {
		t.Errorf("HasExistingOMPAuth() should be false before database creation")
	}
	_ = os.MkdirAll(filepath.Dir(expectedDefault), 0o700)
	_ = os.WriteFile(expectedDefault, []byte(""), 0o600)
	if !HasExistingOMPAuth() {
		t.Errorf("HasExistingOMPAuth() should be true after database creation")
	}
}

func TestOMPDatabase_SchemaAndFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	acct := &Account{
		AccessToken:  "tok-omp-test",
		RefreshToken: "ref-omp-test",
		AccountID:    "acc-omp-1",
		Email:        "user@omp.sh",
		ExpiresAt:    time.UnixMilli(1850000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	appliedPath, err := ApplyAccountToOMP(acct)
	if err != nil {
		t.Fatalf("ApplyAccountToOMP error: %v", err)
	}
	if appliedPath != dbPath {
		t.Errorf("appliedPath = %q, want %q", appliedPath, dbPath)
	}

	// 1. File permissions must be 0600
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat agent.db: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected 0600 permissions, got %#o", perm)
	}

	// 2. Validate exact schema version 7 and indexes matching OMP contract
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.QueryRow("SELECT version FROM auth_schema_version WHERE id = 1").Scan(&version); err != nil {
		t.Fatalf("query auth_schema_version: %v", err)
	}
	if version != 7 {
		t.Errorf("auth_schema_version = %d, want 7", version)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Errorf("expected WAL journal mode, got %q", journalMode)
	}

	// Check indexes exist
	var indexCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name IN ('idx_auth_provider', 'idx_auth_provider_identity')").Scan(&indexCount)
	if err != nil || indexCount != 2 {
		t.Errorf("expected 2 indexes created, got %d, err: %v", indexCount, err)
	}
}

func TestApplyAccountToOMP_ExistingSelectedPreservesRowIDProviderAndValidSticky(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	_ = os.MkdirAll(filepath.Dir(dbPath), 0o700)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Pre-seed table with:
	// - Row 1: another Codex account sharing the selected email
	// - Row 2: the selected Codex account
	// - Row 3: an unrelated provider
	// - Sticky mappings for both Codex rows
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_schema_version (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
		INSERT OR IGNORE INTO auth_schema_version (id, version) VALUES (1, 7);
		CREATE TABLE IF NOT EXISTS auth_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			credential_type TEXT NOT NULL,
			data TEXT NOT NULL,
			disabled_cause TEXT DEFAULT NULL,
			identity_key TEXT DEFAULT NULL,
			created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
			updated_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER))
		);
		CREATE TABLE IF NOT EXISTS cache (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at INTEGER NOT NULL
		);

		INSERT INTO auth_credentials (id, provider, credential_type, data, identity_key)
		VALUES (1, 'openai-codex', 'oauth', '{"access":"tok-other","accountId":"acc-other","email":"user1@omp.sh"}', 'account:acc-other');

		INSERT INTO auth_credentials (id, provider, credential_type, data, identity_key)
		VALUES (2, 'openai-codex', 'oauth', '{"access":"tok-old-1","accountId":"acc-1","email":"user1@omp.sh"}', 'account:acc-1');

		INSERT INTO auth_credentials (id, provider, credential_type, data, identity_key)
		VALUES (3, 'anthropic', 'api_key', '{"key":"sk-ant-keep-me"}', NULL);

		INSERT INTO cache (key, value, expires_at)
		VALUES ('session:sticky:openai-codex:ses-1', '{"type":"oauth","index":0,"credentialId":2,"lastUsedAtMs":1700000000000}', 1800000000);

		INSERT INTO cache (key, value, expires_at)
		VALUES ('session:sticky:openai-codex:ses-2', '{"type":"oauth","index":1,"credentialId":1,"lastUsedAtMs":1700000000000}', 1800000000);

		INSERT INTO cache (key, value, expires_at)
		VALUES ('session:sticky:openai-codex:unknown', '{"credentialId":999}', 1800000000);

		INSERT INTO cache (key, value, expires_at)
		VALUES ('other:cache:key', '{"credentialId":2}', 1800000000);
	`)
	db.Close()

	// Apply Account 1 with refreshed token
	acct1Refreshed := &Account{
		AccessToken:  "tok-1-refreshed",
		RefreshToken: "ref-1-refreshed",
		AccountID:    "acc-1",
		Email:        "user1@omp.sh",
		ExpiresAt:    time.UnixMilli(1890000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	appliedPath, err := ApplyAccountToOMP(acct1Refreshed)
	if err != nil {
		t.Fatalf("ApplyAccountToOMP error: %v", err)
	}
	if appliedPath != dbPath {
		t.Errorf("appliedPath = %q, want %q", appliedPath, dbPath)
	}

	// 1. Verify exactly ONE openai-codex row exists in auth_credentials
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil {
		t.Fatalf("loadOMPAccounts error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected exactly 1 active Codex account in OMP after cutover, got %d", len(accounts))
	}
	if accounts[0].AccountID != "acc-1" || accounts[0].AccessToken != "tok-1-refreshed" {
		t.Errorf("account not updated correctly: %+v", accounts[0])
	}

	// 2. Verify selected account-ID row was preserved, despite an earlier email collision.
	dbCheck, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open dbCheck: %v", err)
	}
	defer dbCheck.Close()

	var codexRowID int64
	if err := dbCheck.QueryRow("SELECT id FROM auth_credentials WHERE provider = 'openai-codex'").Scan(&codexRowID); err != nil {
		t.Fatalf("query codex row id error: %v", err)
	}
	if codexRowID != 2 {
		t.Errorf("expected selected row id 2 preserved, got %d", codexRowID)
	}

	// 3. Verify Anthropic (unrelated provider) is completely intact
	var anthropicCount int
	var anthropicKey string
	if err := dbCheck.QueryRow("SELECT COUNT(*), json_extract(data, '$.key') FROM auth_credentials WHERE provider = 'anthropic'").Scan(&anthropicCount, &anthropicKey); err != nil {
		t.Fatalf("query anthropic error: %v", err)
	}
	if anthropicCount != 1 || anthropicKey != "sk-ant-keep-me" {
		t.Errorf("unrelated anthropic provider was corrupted: count=%d, key=%q", anthropicCount, anthropicKey)
	}

	// 4. Verify cache cleanup only removes sticky mappings to deleted rows.
	var ses1Count, ses2Count, unknownStickyCount, unrelatedCacheCount int
	_ = dbCheck.QueryRow("SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:ses-1'").Scan(&ses1Count)
	_ = dbCheck.QueryRow("SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:ses-2'").Scan(&ses2Count)
	_ = dbCheck.QueryRow("SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:unknown'").Scan(&unknownStickyCount)
	_ = dbCheck.QueryRow("SELECT COUNT(*) FROM cache WHERE key = 'other:cache:key'").Scan(&unrelatedCacheCount)
	if ses1Count != 1 {
		t.Errorf("valid sticky cache entry for selected row was deleted")
	}
	if ses2Count != 0 {
		t.Errorf("stale sticky cache entry for deleted row was not removed")
	}
	if unknownStickyCount != 1 || unrelatedCacheCount != 1 {
		t.Errorf("cache cleanup removed an entry not tied to a deleted Codex credential")
	}

	// 5. Test idempotency: repeated apply leaves state unchanged with same row ID
	if _, err := ApplyAccountToOMP(acct1Refreshed); err != nil {
		t.Fatalf("second apply failed: %v", err)
	}
	var repeatedRowID int64
	_ = dbCheck.QueryRow("SELECT id FROM auth_credentials WHERE provider = 'openai-codex'").Scan(&repeatedRowID)
	if repeatedRowID != 2 {
		t.Errorf("repeated apply changed row ID: got %d, want 2", repeatedRowID)
	}
}

func TestRestoreManagedAccountsToOMP_MirrorsPoolAndPreservesRows(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	dbPath := filepath.Join(tmp, "work", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	managedPath, err := managedAccountsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONMap(managedPath, map[string]any{"accounts": []managedAccount{
		{Label: "one", AccountID: "acc-1", Email: "one@omp.sh", AccessToken: "tok-1"},
		{Label: "duplicate", AccountID: "acc-1", Email: "one@omp.sh", AccessToken: "tok-duplicate"},
		{Label: "email-only duplicate", Email: "one@omp.sh", AccessToken: "tok-email-only"},
		{Label: "two", AccountID: "acc-2", Email: "two@omp.sh", AccessToken: "tok-2"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE auth_credentials (id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT, credential_type TEXT, data TEXT, disabled_cause TEXT, identity_key TEXT, created_at INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE cache (key TEXT PRIMARY KEY, value TEXT, expires_at INTEGER);
		INSERT INTO auth_credentials (id, provider, credential_type, data, identity_key) VALUES (1, 'openai-codex', 'oauth', '{"accountId":"acc-1","access":"old"}', 'account:acc-1');
		INSERT INTO auth_credentials (id, provider, credential_type, data, identity_key) VALUES (2, 'openai-codex', 'oauth', '{"accountId":"obsolete"}', 'account:obsolete');
		INSERT INTO auth_credentials (id, provider, credential_type, data, identity_key) VALUES (3, 'anthropic', 'api_key', '{"key":"keep"}', NULL);
		INSERT INTO cache VALUES ('session:sticky:openai-codex:keep', '{"credentialId":1}', 1);
		INSERT INTO cache VALUES ('session:sticky:openai-codex:stale', '{"credentialId":2}', 1);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	count, path, err := RestoreManagedAccountsToOMP()
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if count != 2 || path != dbPath {
		t.Fatalf("restore result = %d, %q", count, path)
	}
	db, _ = sql.Open("sqlite", dbPath)
	defer db.Close()
	var codexCount, anthropicCount, rowID, keepSticky, staleSticky int
	_ = db.QueryRow("SELECT COUNT(*) FROM auth_credentials WHERE provider = 'openai-codex'").Scan(&codexCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM auth_credentials WHERE provider = 'anthropic'").Scan(&anthropicCount)
	_ = db.QueryRow("SELECT id FROM auth_credentials WHERE json_extract(data, '$.accountId') = 'acc-1'").Scan(&rowID)
	_ = db.QueryRow("SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:keep'").Scan(&keepSticky)
	_ = db.QueryRow("SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:stale'").Scan(&staleSticky)
	if codexCount != 2 || anthropicCount != 1 || rowID != 1 || keepSticky != 1 || staleSticky != 0 {
		t.Errorf("unexpected mirrored OMP state: codex=%d anthropic=%d row=%d keep=%d stale=%d", codexCount, anthropicCount, rowID, keepSticky, staleSticky)
	}
}

func TestRestoreManagedAccountsToOMP_RejectsInvalidInputWithoutMutation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)
	managedPath, err := managedAccountsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONMap(managedPath, map[string]any{"accounts": []managedAccount{{AccountID: "bad", AccessToken: ""}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyAccountToOMP(&Account{AccountID: "preserved", AccessToken: "tok"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreManagedAccountsToOMP(); err == nil {
		t.Fatal("expected malformed managed account error")
	}
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 1 || accounts[0].AccountID != "preserved" {
		t.Errorf("invalid restore mutated OMP: %#v, %v", accounts, err)
	}
}

func TestRestoreManagedAccountsToOMP_EmptyManagedStoreDoesNotMutate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)
	if _, err := ApplyAccountToOMP(&Account{AccountID: "preserved", AccessToken: "tok"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreManagedAccountsToOMP(); err == nil {
		t.Fatal("expected empty managed store error")
	}
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 1 || accounts[0].AccountID != "preserved" {
		t.Errorf("empty restore mutated OMP: %#v, %v", accounts, err)
	}
}

func TestApplyAccountToOMP_CollapsesRestoredPool(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)
	managedPath, err := managedAccountsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONMap(managedPath, map[string]any{"accounts": []managedAccount{
		{AccountID: "acc-1", Email: "one@omp.sh", AccessToken: "tok-1"},
		{AccountID: "acc-2", Email: "two@omp.sh", AccessToken: "tok-2"},
	}}); err != nil {
		t.Fatal(err)
	}
	if count, _, err := RestoreManagedAccountsToOMP(); err != nil || count != 2 {
		t.Fatalf("restore = %d, %v", count, err)
	}
	if _, err := ApplyAccountToOMP(&Account{AccountID: "acc-1", Email: "one@omp.sh", AccessToken: "tok-1-new"}); err != nil {
		t.Fatal(err)
	}
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 1 || accounts[0].AccountID != "acc-1" {
		t.Errorf("exclusive apply did not collapse restored pool: %#v, %v", accounts, err)
	}
}

func TestRestoreManagedAccountsToOMP_RollsBackAndRespectsActiveProfile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	t.Setenv("OMP_PROFILE", "work")
	managedPath, err := managedAccountsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSONMap(managedPath, map[string]any{"accounts": []managedAccount{
		{AccountID: "acc-1", Email: "one@omp.sh", AccessToken: "tok-new"},
		{AccountID: "acc-2", Email: "two@omp.sh", AccessToken: "tok-2"},
	}}); err != nil {
		t.Fatal(err)
	}
	workPath := ompAgentDbPath()
	if _, err := ApplyAccountToOMP(&Account{AccountID: "acc-1", Email: "one@omp.sh", AccessToken: "tok-old"}); err != nil {
		t.Fatal(err)
	}
	db, _ := sql.Open("sqlite", workPath)
	_, err = db.Exec(`
		INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', '{"accountId":"obsolete"}', 'account:obsolete');
		CREATE TRIGGER block_restore_delete BEFORE DELETE ON auth_credentials
		WHEN OLD.provider = 'openai-codex' BEGIN SELECT RAISE(ABORT, 'blocked restore'); END;
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RestoreManagedAccountsToOMP(); err == nil {
		t.Fatal("expected atomic restore failure")
	}
	db, _ = sql.Open("sqlite", workPath)
	defer db.Close()
	var count int
	var access string
	_ = db.QueryRow("SELECT COUNT(*) FROM auth_credentials WHERE provider = 'openai-codex'").Scan(&count)
	_ = db.QueryRow("SELECT json_extract(data, '$.access') FROM auth_credentials WHERE json_extract(data, '$.accountId') = 'acc-1'").Scan(&access)
	if count != 2 || access != "tok-old" {
		t.Errorf("restore leaked a partial transaction: count=%d access=%q", count, access)
	}
	defaultPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
		t.Errorf("restore touched default profile: %v", err)
	}
}

func TestApplyAccountToOMP_NewSelectedReplacesOtherCodexRows(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	_ = os.MkdirAll(filepath.Dir(dbPath), 0o700)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, _ = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_schema_version (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
		INSERT OR IGNORE INTO auth_schema_version (id, version) VALUES (1, 7);
		CREATE TABLE IF NOT EXISTS auth_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			credential_type TEXT NOT NULL,
			data TEXT NOT NULL,
			disabled_cause TEXT DEFAULT NULL,
			identity_key TEXT DEFAULT NULL,
			created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
			updated_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER))
		);
		INSERT INTO auth_credentials (provider, credential_type, data, identity_key)
		VALUES ('openai-codex', 'oauth', '{"access":"tok-old","accountId":"acc-old"}', 'account:acc-old');
		INSERT INTO auth_credentials (provider, credential_type, data, identity_key)
		VALUES ('anthropic', 'api_key', '{"key":"sk-ant-keep"}', NULL);
	`)
	db.Close()

	// Apply brand new identity
	newAcct := &Account{
		AccessToken:  "tok-new",
		RefreshToken: "ref-new",
		AccountID:    "acc-brand-new",
		Email:        "brandnew@omp.sh",
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	if _, err := ApplyAccountToOMP(newAcct); err != nil {
		t.Fatalf("ApplyAccountToOMP error: %v", err)
	}

	// Verify only the new account exists for openai-codex
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 1 {
		t.Fatalf("expected 1 account after applying new identity, got %d (err: %v)", len(accounts), err)
	}
	if accounts[0].AccountID != "acc-brand-new" {
		t.Errorf("expected acc-brand-new, got %q", accounts[0].AccountID)
	}

	// Verify Anthropic row remains intact
	dbCheck, _ := sql.Open("sqlite", dbPath)
	var anthropicCount int
	_ = dbCheck.QueryRow("SELECT COUNT(*) FROM auth_credentials WHERE provider = 'anthropic'").Scan(&anthropicCount)
	dbCheck.Close()
	if anthropicCount != 1 {
		t.Errorf("anthropic row was corrupted during replacement!")
	}
}

func TestApplyAccountToOMP_RollsBackOnCutoverFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	selected := &Account{AccessToken: "tok-before", AccountID: "acc-selected", Email: "selected@omp.sh"}
	if _, err := ApplyAccountToOMP(selected); err != nil {
		t.Fatalf("seed selected account: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO auth_credentials (provider, credential_type, data, identity_key)
		VALUES ('openai-codex', 'oauth', '{"access":"tok-other","accountId":"acc-other"}', 'account:acc-other');
		CREATE TRIGGER block_codex_cutover
		BEFORE DELETE ON auth_credentials
		WHEN OLD.provider = 'openai-codex' AND OLD.id != 1
		BEGIN SELECT RAISE(ABORT, 'blocked cutover'); END;
	`)
	db.Close()
	if err != nil {
		t.Fatalf("seed rollback fixture: %v", err)
	}

	selected.AccessToken = "tok-after"
	if _, err := ApplyAccountToOMP(selected); err == nil {
		t.Fatal("expected cutover failure")
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db.Close()
	var count int
	var access string
	if err := db.QueryRow("SELECT COUNT(*) FROM auth_credentials WHERE provider = 'openai-codex'").Scan(&count); err != nil {
		t.Fatalf("count credentials: %v", err)
	}
	if err := db.QueryRow("SELECT json_extract(data, '$.access') FROM auth_credentials WHERE id = 1").Scan(&access); err != nil {
		t.Fatalf("read selected credential: %v", err)
	}
	if count != 2 || access != "tok-before" {
		t.Errorf("transaction leaked partial cutover: count=%d access=%q", count, access)
	}
}

func TestApplyAccountToOMP_ProfileIsolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	defaultPath := ompAgentDbPath()
	if _, err := ApplyAccountToOMP(&Account{AccessToken: "tok-default", AccountID: "acc-default", Email: "default@omp.sh"}); err != nil {
		t.Fatalf("apply default profile: %v", err)
	}

	t.Setenv("OMP_PROFILE", "work")
	workPath := ompAgentDbPath()
	if _, err := ApplyAccountToOMP(&Account{AccessToken: "tok-work", AccountID: "acc-work", Email: "work@omp.sh"}); err != nil {
		t.Fatalf("apply named profile: %v", err)
	}

	defaultAccounts, err := loadOMPAccounts(defaultPath)
	if err != nil {
		t.Fatalf("load default profile: %v", err)
	}
	workAccounts, err := loadOMPAccounts(workPath)
	if err != nil {
		t.Fatalf("load work profile: %v", err)
	}
	if len(defaultAccounts) != 1 || defaultAccounts[0].AccountID != "acc-default" {
		t.Errorf("default profile changed: %#v", defaultAccounts)
	}
	if len(workAccounts) != 1 || workAccounts[0].AccountID != "acc-work" {
		t.Errorf("work profile not isolated: %#v", workAccounts)
	}
}

func TestApplyAccountToOMP_EmptyIdentityDoesNotCutOver(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	if _, err := ApplyAccountToOMP(&Account{AccessToken: "tok-valid", AccountID: "acc-valid"}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := ApplyAccountToOMP(&Account{AccessToken: "tok-anonymous"}); err == nil {
		t.Fatal("expected empty identity error")
	}
	accounts, err := loadOMPAccounts(dbPath)
	if err != nil {
		t.Fatalf("load account after rejected apply: %v", err)
	}
	if len(accounts) != 1 || accounts[0].AccountID != "acc-valid" {
		t.Errorf("empty identity changed OMP credential: %#v", accounts)
	}
}
func TestOMPDatabase_DeleteAndManagedSafety(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	acct := &Account{
		AccessToken:  "tok-del",
		RefreshToken: "ref-del",
		AccountID:    "acc-del",
		Email:        "del@omp.sh",
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	if _, err := ApplyAccountToOMP(acct); err != nil {
		t.Fatalf("apply error: %v", err)
	}

	// Case 1: Delete with a SourceManaged account (whose FilePath is accounts.json)
	// Must target ompAgentDbPath() without corrupting accounts.json
	cqAccountsPath := filepath.Join(tmp, "accounts.json")
	accountsContent := `{"accounts":[{"account_id":"acc-del"}]}`
	_ = os.WriteFile(cqAccountsPath, []byte(accountsContent), 0o600)

	managedAcct := &Account{
		AccountID: "acc-del",
		Email:     "del@omp.sh",
		Source:    SourceManaged,
		FilePath:  cqAccountsPath,
		Writable:  true,
	}

	if err := DeleteOMPAuthAccount(managedAcct); err != nil {
		t.Fatalf("DeleteOMPAuthAccount error: %v", err)
	}

	accountsAfterDelete, err := loadOMPAccounts(dbPath)
	if err != nil || len(accountsAfterDelete) != 0 {
		t.Errorf("expected 0 accounts in OMP db after delete, got %d", len(accountsAfterDelete))
	}

	// Verify CQ accounts.json was NOT touched
	rawAccounts, _ := os.ReadFile(cqAccountsPath)
	if string(rawAccounts) != accountsContent {
		t.Errorf("accounts.json was corrupted by OMP delete!")
	}

	// Case 2: Delete with empty identity is a safe no-op
	emptyAcct := &Account{AccountID: "", Email: "", Source: SourceOMP}
	if err := DeleteOMPAuthAccount(emptyAcct); err != nil {
		t.Errorf("delete empty account should be safe no-op: %v", err)
	}
	if err := DeleteOMPAuthAccount(nil); err != nil {
		t.Errorf("delete nil should be safe no-op: %v", err)
	}
}

func TestDeleteOMPClearsOnlyDeletedStickyCacheTransactionally(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	dbPath := filepath.Join(tmp, "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)
	for _, account := range []*Account{
		{AccountID: "acc-1", Email: "one@example.com", AccessToken: "tok-1"},
		{AccountID: "acc-2", Email: "two@example.com", AccessToken: "tok-2"},
	} {
		if err := UpsertManagedAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := RestoreManagedAccountsToOMP(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE cache (key TEXT PRIMARY KEY, value TEXT);
		INSERT INTO cache SELECT 'session:sticky:openai-codex:one', json_object('credentialId', id) FROM auth_credentials WHERE json_extract(data, '$.accountId') = 'acc-1';
		INSERT INTO cache SELECT 'session:sticky:openai-codex:two', json_object('credentialId', id) FROM auth_credentials WHERE json_extract(data, '$.accountId') = 'acc-2';
		INSERT INTO cache VALUES ('unrelated', '{"credentialId":999}');
		CREATE TRIGGER block_delete BEFORE DELETE ON auth_credentials BEGIN SELECT RAISE(ABORT, 'blocked'); END;
	`); err != nil {
		t.Fatal(err)
	}
	first := &Account{AccountID: "acc-1", Email: "one@example.com"}
	if err := DeleteOMPAuthAccount(first); err == nil {
		t.Fatal("expected triggered delete failure")
	}
	var stickyOne int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:one'`).Scan(&stickyOne); err != nil || stickyOne != 1 {
		t.Fatalf("cache deletion was not rolled back: count=%d err=%v", stickyOne, err)
	}
	if _, err := db.Exec("DROP TRIGGER block_delete"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteOMPAuthAccount(first); err != nil {
		t.Fatal(err)
	}
	var stickyTwo, unrelated, secondCredential int
	_ = db.QueryRow(`SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:one'`).Scan(&stickyOne)
	_ = db.QueryRow(`SELECT COUNT(*) FROM cache WHERE key = 'session:sticky:openai-codex:two'`).Scan(&stickyTwo)
	_ = db.QueryRow(`SELECT COUNT(*) FROM cache WHERE key = 'unrelated'`).Scan(&unrelated)
	_ = db.QueryRow(`SELECT COUNT(*) FROM auth_credentials WHERE json_extract(data, '$.accountId') = 'acc-2'`).Scan(&secondCredential)
	if stickyOne != 0 || stickyTwo != 1 || unrelated != 1 || secondCredential != 1 {
		t.Fatalf("unexpected retained rows: deleted=%d other=%d unrelated=%d credential=%d", stickyOne, stickyTwo, unrelated, secondCredential)
	}
}

func TestOpenOMPSQLiteConfiguresEveryConnection(t *testing.T) {
	db, err := openOMPSQLite(filepath.Join(t.TempDir(), "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if db.Stats().MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections = %d", db.Stats().MaxOpenConnections)
	}
	var timeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatalf("busy_timeout = %d, err=%v", timeout, err)
	}
}

func TestOMPDatabase_IdentityEdgeCasesAndDisabledRows(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dbPath := filepath.Join(tmp, "edge_cases.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	// 1. AccountID-only identity
	acctIDOnly := &Account{
		AccessToken: "tok-id-only",
		AccountID:   "acc-id-only-99",
		Source:      SourceOMP,
		Writable:    true,
	}
	if _, err := ApplyAccountToOMP(acctIDOnly); err != nil {
		t.Fatalf("apply acctIDOnly: %v", err)
	}

	// 2. Email-only identity replaces previous row
	acctEmailOnly := &Account{
		AccessToken: "tok-email-only",
		Email:       "email-only@omp.sh",
		Source:      SourceOMP,
		Writable:    true,
	}
	if _, err := ApplyAccountToOMP(acctEmailOnly); err != nil {
		t.Fatalf("apply acctEmailOnly: %v", err)
	}

	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 1 {
		t.Fatalf("expected 1 active account loaded after replacement, got %d (err: %v)", len(accounts), err)
	}
	if accounts[0].Email != "email-only@omp.sh" {
		t.Errorf("expected email-only@omp.sh, got %q", accounts[0].Email)
	}

	// 3. Disabled rows must be ignored by loadOMPAccounts
	db, _ := sql.Open("sqlite", dbPath)
	disabledPayload, _ := json.Marshal(map[string]any{"access": "tok-disabled", "accountId": "acc-disabled"})
	_, _ = db.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, disabled_cause, identity_key)
		VALUES ('openai-codex', 'oauth', ?, 'disabled', 'account:acc-disabled')`, string(disabledPayload))
	db.Close()

	accountsWithDisabled, err := loadOMPAccounts(dbPath)
	if err != nil || len(accountsWithDisabled) != 1 {
		t.Errorf("disabled row should be filtered out, got %d accounts", len(accountsWithDisabled))
	}
}

func TestOMPDatabase_ErrorHandlingAndCorruption(t *testing.T) {
	tmp := t.TempDir()

	// Missing file returns nil, nil
	accts, err := loadOMPAccounts(filepath.Join(tmp, "nonexistent.db"))
	if err != nil || accts != nil {
		t.Errorf("missing file: got %v, err %v", accts, err)
	}

	// Uninitialized database (empty file) returns nil, nil without crashing
	emptyDbPath := filepath.Join(tmp, "empty.db")
	_ = os.WriteFile(emptyDbPath, []byte(""), 0o600)
	accts, err = loadOMPAccounts(emptyDbPath)
	if err != nil || accts != nil {
		t.Errorf("uninitialized db: got %v, err %v", accts, err)
	}
	corruptFilePath := filepath.Join(tmp, "not-sqlite.db")
	if err := os.WriteFile(corruptFilePath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOMPAccounts(corruptFilePath); err == nil {
		t.Fatal("expected database error from corrupt SQLite file")
	}

	// Malformed JSON rows are skipped while valid rows still load.
	corruptDbPath := filepath.Join(tmp, "corrupt.db")
	db, err := sql.Open("sqlite", corruptDbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, _ = db.Exec(`
		CREATE TABLE auth_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			credential_type TEXT NOT NULL,
			data TEXT NOT NULL,
			disabled_cause TEXT DEFAULT NULL,
			identity_key TEXT DEFAULT NULL
		);
		INSERT INTO auth_credentials (provider, credential_type, data, identity_key)
		VALUES ('openai-codex', 'oauth', 'not-valid-json{{{', 'account:corrupt');
	`)
	_, _ = db.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, identity_key) VALUES ('openai-codex', 'oauth', '{"access":"valid","accountId":"valid"}', 'account:valid')`)
	db.Close()

	accounts, err := loadOMPAccounts(corruptDbPath)
	if err != nil {
		t.Fatalf("malformed row should be skipped: %v", err)
	}
	if len(accounts) != 1 || accounts[0].AccountID != "valid" {
		t.Fatalf("loaded accounts = %#v, want only valid row", accounts)
	}
}

func TestApplyAndDeleteOMP_ErrorsAndEmptyPaths(t *testing.T) {
	tmp := t.TempDir()

	// 1. Apply nil account returns error
	if _, err := ApplyAccountToOMP(nil); err == nil {
		t.Errorf("expected error applying nil account to OMP")
	}

	// 2. Apply when path is empty returns error
	t.Setenv("CQ_OMP_DB_PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "")
	acct := &Account{AccessToken: "tok", Source: SourceOMP}
	if _, err := ApplyAccountToOMP(acct); err == nil {
		t.Errorf("expected error when OMP DB path is empty")
	}
	t.Setenv("HOME", tmp)

	// 3. Delete when path is empty is safe no-op
	t.Setenv("CQ_OMP_DB_PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "")
	if err := DeleteOMPAuthAccount(acct); err != nil {
		t.Errorf("expected safe no-op on delete with empty path: %v", err)
	}
	t.Setenv("HOME", tmp)

	// 4. Delete when DB file is missing is safe no-op
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(tmp, "missing.db"))
	if err := DeleteOMPAuthAccount(acct); err != nil {
		t.Errorf("expected safe no-op on delete with missing DB: %v", err)
	}
}
