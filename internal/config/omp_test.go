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
	t.Setenv("CQ_OMP_DB_PATH", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
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

func TestOMPDatabase_MultiAccountPoolAndPreservation(t *testing.T) {
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
		VALUES ('anthropic', 'api_key', '{"key":"sk-ant-test"}', NULL);
	`)
	db.Close()

	// 1. Insert Account 1
	acct1 := &Account{
		AccessToken:  "tok-acc-1",
		RefreshToken: "ref-acc-1",
		AccountID:    "acc-1",
		Email:        "user1@omp.sh",
		ExpiresAt:    time.UnixMilli(1800000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}
	if _, err := ApplyAccountToOMP(acct1); err != nil {
		t.Fatalf("apply acct1: %v", err)
	}

	// 2. Insert Account 2 (distinct identity: both must coexist in pool)
	acct2 := &Account{
		AccessToken:  "tok-acc-2",
		RefreshToken: "ref-acc-2",
		AccountID:    "acc-2",
		Email:        "user2@omp.sh",
		ExpiresAt:    time.UnixMilli(1810000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}
	if _, err := ApplyAccountToOMP(acct2); err != nil {
		t.Fatalf("apply acct2: %v", err)
	}

	accounts, err := loadOMPAccounts(dbPath)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("expected 2 accounts loaded from OMP pool, got %d (err: %v)", len(accounts), err)
	}

	// 3. Update Account 1 with refreshed token (must update in-place without creating a 3rd duplicate row)
	acct1Refreshed := &Account{
		AccessToken:  "tok-acc-1-refreshed",
		RefreshToken: "ref-acc-1-refreshed",
		AccountID:    "acc-1",
		Email:        "user1@omp.sh",
		ExpiresAt:    time.UnixMilli(1820000000000),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}
	if _, err := ApplyAccountToOMP(acct1Refreshed); err != nil {
		t.Fatalf("apply acct1 update: %v", err)
	}

	accountsAfterUpdate, err := loadOMPAccounts(dbPath)
	if err != nil || len(accountsAfterUpdate) != 2 {
		t.Fatalf("expected still 2 accounts in pool after update, got %d", len(accountsAfterUpdate))
	}

	// 4. SaveAccount via SourceOMP dispatch
	acct1Refreshed.AccessToken = "tok-acc-1-saved"
	if err := SaveAccount(acct1Refreshed); err != nil {
		t.Fatalf("SaveAccount error: %v", err)
	}
	loadedSaved, _ := loadOMPAccountFile(dbPath)
	if loadedSaved == nil || loadedSaved.AccessToken != "tok-acc-1-saved" {
		t.Errorf("SaveAccount failed to persist to OMP agent.db: %+v", loadedSaved)
	}

	// 5. Test DeleteOMPAuthAccount on Account 1: Account 2 and Anthropic must be preserved
	if err := DeleteOMPAuthAccount(acct1Refreshed); err != nil {
		t.Fatalf("delete acct1: %v", err)
	}

	accountsAfterDelete, err := loadOMPAccounts(dbPath)
	if err != nil || len(accountsAfterDelete) != 1 {
		t.Fatalf("expected 1 remaining account, got %d", len(accountsAfterDelete))
	}
	if accountsAfterDelete[0].AccountID != "acc-2" {
		t.Errorf("expected remaining account acc-2, got %q", accountsAfterDelete[0].AccountID)
	}

	// Verify Anthropic row was untouched
	dbCheck, _ := sql.Open("sqlite", dbPath)
	var anthropicCount int
	_ = dbCheck.QueryRow("SELECT COUNT(*) FROM auth_credentials WHERE provider = 'anthropic'").Scan(&anthropicCount)
	dbCheck.Close()
	if anthropicCount != 1 {
		t.Errorf("anthropic provider row was deleted or corrupted!")
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

	// 2. Email-only identity
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
	if err != nil || len(accounts) != 2 {
		t.Fatalf("expected 2 accounts loaded, got %d (err: %v)", len(accounts), err)
	}

	// 3. Disabled rows must be ignored by loadOMPAccounts
	db, _ := sql.Open("sqlite", dbPath)
	disabledPayload, _ := json.Marshal(map[string]any{"access": "tok-disabled", "accountId": "acc-disabled"})
	_, _ = db.Exec(`INSERT INTO auth_credentials (provider, credential_type, data, disabled_cause, identity_key)
		VALUES ('openai-codex', 'oauth', ?, 'disabled', 'account:acc-disabled')`, string(disabledPayload))
	db.Close()

	accountsWithDisabled, err := loadOMPAccounts(dbPath)
	if err != nil || len(accountsWithDisabled) != 2 {
		t.Errorf("disabled row should be filtered out, got %d accounts", len(accountsWithDisabled))
	}

	// 4. Deleting with empty account identity is a safe no-op
	emptyAcct := &Account{
		AccessToken: "tok-empty",
		AccountID:   "",
		Email:       "",
		Source:      SourceOMP,
	}
	if err := DeleteOMPAuthAccount(emptyAcct); err != nil {
		t.Errorf("delete empty account should be safe no-op: %v", err)
	}
	if err := DeleteOMPAuthAccount(nil); err != nil {
		t.Errorf("delete nil account should be safe no-op: %v", err)
	}

	accountsAfterEmptyDelete, _ := loadOMPAccounts(dbPath)
	if len(accountsAfterEmptyDelete) != 2 {
		t.Errorf("accounts were deleted by empty identity delete!")
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

	// Corrupted JSON inside database returns contextual error naming the row
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
	db.Close()

	_, err = loadOMPAccounts(corruptDbPath)
	if err == nil {
		t.Fatalf("expected error on corrupt JSON in database, got nil")
	}
	if !strings.Contains(err.Error(), "corrupt credential JSON") {
		t.Errorf("expected error message to mention corrupt credential JSON, got: %v", err)
	}
}
