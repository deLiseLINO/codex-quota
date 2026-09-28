package config

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupOpenCode2TestEnv(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("PATH", "")
	t.Setenv("OPENCODE_AUTH_PATH", "")
	t.Setenv("OPENCODE_DB", "")
	t.Setenv("OPENCODE_DATA_DIR", "")
	resetOpenCodeGenerationCacheForTest()
	t.Cleanup(resetOpenCodeGenerationCacheForTest)
	return root, data
}

func createOpenCode2TestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		CREATE TABLE credential (
			id TEXT PRIMARY KEY,
			integration_id TEXT,
			label TEXT NOT NULL,
			value TEXT NOT NULL,
			connector_id TEXT,
			method_id TEXT,
			active INTEGER,
			time_created INTEGER NOT NULL,
			time_updated INTEGER NOT NULL
		)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func insertOpenCode2TestRow(t *testing.T, db *sql.DB, id, label, integrationID, accountID, access, refresh string, expires int64, active int64, extra map[string]any, created int64) {
	t.Helper()
	value := map[string]any{
		"type":     "oauth",
		"methodID": openCode2OAuthMethod,
		"access":   access,
		"refresh":  refresh,
		"expires":  expires,
		"metadata": map[string]any{"accountID": accountID},
	}
	for key, item := range extra {
		value[key] = item
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credential (id, integration_id, label, value, active, time_created, time_updated) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, integrationID, label, string(encoded), active, created, created); err != nil {
		t.Fatal(err)
	}
}

func openCode2TestAccount(accountID, access, refresh string, expiresAt time.Time) *Account {
	return &Account{
		Label:        accountID,
		AccountID:    accountID,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt,
		Source:       SourceOpenCode,
		Writable:     true,
	}
}

func TestOpenCodeVersionGenerationAndAliases(t *testing.T) {
	if got := openCodeVersionGeneration("OpenCode v1.8.2"); got != 1 {
		t.Fatalf("v1 generation = %d", got)
	}
	if got := openCodeVersionGeneration("OpenCode 2.0.0"); got != 2 {
		t.Fatalf("v2 generation = %d", got)
	}

	var probed []string
	v1, v2 := detectOpenCodeGenerations(
		func(name string) (string, error) {
			return "/same/opencode", nil
		},
		func(_ context.Context, path string) (string, error) {
			probed = append(probed, path)
			return "OpenCode 2.0.0", nil
		},
	)
	if v1 || !v2 {
		t.Fatalf("alias generations = (%v, %v), want (false, true)", v1, v2)
	}
	if len(probed) != 2 {
		t.Fatalf("probed %d executable aliases, want 2 bounded probes", len(probed))
	}

	v1, v2 = detectOpenCodeGenerations(
		func(name string) (string, error) {
			if name == "opencode" {
				return "/v1/opencode", nil
			}
			return "/v2/opencode", nil
		},
		func(_ context.Context, path string) (string, error) {
			if strings.HasPrefix(path, "/v1/") {
				return "OpenCode 1.9.0", nil
			}
			return "OpenCode 2.1.0", nil
		},
	)
	if !v1 || !v2 {
		t.Fatalf("distinct generations = (%v, %v), want (true, true)", v1, v2)
	}

	installed := installedApplyTargetsWithOpenCode(func(string) (string, error) { return "", errors.New("not installed") }, openCodeCapabilities{V2: true})
	if !reflect.DeepEqual(installed, []Source{SourceOpenCode}) {
		t.Fatalf("native target list = %#v, want one OpenCode target", installed)
	}
}

func TestOpenCode2PathAndDSN(t *testing.T) {
	_, data := setupOpenCode2TestEnv(t)
	expected := filepath.Join(data, "opencode", "opencode.db")
	if got := opencode2DBPath(); got != expected {
		t.Fatalf("XDG database path = %q, want %q", got, expected)
	}

	override := filepath.Join(t.TempDir(), "db with spaces", "native.db")
	t.Setenv("OPENCODE_DB", override)
	if got := opencode2DBPath(); got != override {
		t.Fatalf("override database path = %q, want %q", got, override)
	}
	dsn := openCode2DSN(override, false)
	if !strings.Contains(dsn, "%20") || !strings.Contains(dsn, "mode=rw") {
		t.Fatalf("read-write DSN %q does not escape path or select rw mode", dsn)
	}
}

func TestOpenCode2RelativeDBPathOpensReadOnly(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "native.db")
	fixture := createOpenCode2TestDB(t, dbPath)
	_ = fixture.Close()
	relativePath, err := filepath.Rel(workingDir, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_DB", relativePath)
	if got := opencode2DBPath(); got != dbPath {
		t.Fatalf("relative database path = %q, want %q", got, dbPath)
	}
	db, err := openOpenCode2Database(opencode2DBPath(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("fresh native credential count = %d, want 0", count)
	}
}

func TestImplicitInvalidOpenCode2DBFallsBackToLegacy(t *testing.T) {
	root, data := setupOpenCode2TestEnv(t)
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeOpenCodeVersionStub(t, binDir, "opencode", "OpenCode 1.9.0")
	t.Setenv("PATH", binDir)
	resetOpenCodeGenerationCacheForTest()
	storeDir := filepath.Join(data, "opencode")
	authPath := filepath.Join(storeDir, "auth.json")
	dbPath := filepath.Join(storeDir, "opencode.db")
	if err := writeJSONMap(authPath, map[string]any{
		"openai":    map[string]any{"type": "oauth", "access": "legacy-access", "accountId": "acct-legacy"},
		"providers": map[string]any{"keep": true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		t.Fatal(err)
	}
	if !containsAccountID(result.Accounts, "acct-legacy") {
		t.Fatalf("legacy account was not loaded with an invalid implicit native DB")
	}
	if OpenCode2Available() {
		t.Fatal("invalid implicit native DB was advertised as available")
	}
	if _, err := ApplyAccountToOpenCode(openCode2TestAccount("acct-legacy", "legacy-updated", "", time.Time{})); err != nil {
		t.Fatalf("legacy apply should remain usable: %v", err)
	}
	updated, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "legacy-updated") {
		t.Fatal("legacy target was not updated after invalid implicit native DB was ignored")
	}
}

func TestOpenCodeLegacyApplyClearsOptionalFieldsAndPreservesMode(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("OPENCODE_AUTH_PATH", authPath)
	root := map[string]any{
		"openai": map[string]any{
			"type":      "api",
			"access":    "old-access",
			"refresh":   "old-refresh",
			"accountId": "old-account",
			"email":     "old@example.test",
			"expires":   float64(123),
		},
		"providers": map[string]any{"anthropic": map[string]any{"apiKey": "keep"}},
	}
	if err := writeJSONMap(authPath, root); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(authPath, 0o640); err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(authPath)
	if err != nil {
		t.Fatal(err)
	}

	account := openCode2TestAccount("", "new-access", "", time.Time{})
	if _, err := ApplyAccountToOpenCode(account); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := map[string]any{}
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatal(err)
	}
	openai, ok := updated["openai"].(map[string]any)
	if !ok {
		t.Fatalf("openai object missing: %#v", updated["openai"])
	}
	if openai["type"] != "oauth" || openai["access"] != "new-access" {
		t.Fatalf("legacy OAuth fields = %#v", openai)
	}
	for _, key := range []string{"refresh", "accountId", "email", "expires"} {
		if _, ok := openai[key]; ok {
			t.Errorf("stale optional field %q survived: %#v", key, openai[key])
		}
	}
	if _, ok := updated["providers"]; !ok {
		t.Fatal("unrelated providers were removed")
	}
	afterInfo, err := os.Stat(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if beforeInfo.Mode().Perm() != afterInfo.Mode().Perm() {
		t.Fatalf("mode changed from %o to %o", beforeInfo.Mode().Perm(), afterInfo.Mode().Perm())
	}
}

func TestOpenCodeLegacyNullRootFailsWithoutMutation(t *testing.T) {
	root, _ := setupOpenCode2TestEnv(t)
	authPath := filepath.Join(root, "auth.json")
	t.Setenv("OPENCODE_AUTH_PATH", authPath)
	if err := os.WriteFile(authPath, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	secret := "legacy-null-secret"
	if _, err := ApplyAccountToOpenCode(openCode2TestAccount("acct-null", secret, "", time.Time{})); err == nil {
		t.Fatal("null legacy root unexpectedly accepted")
	} else if strings.Contains(err.Error(), secret) {
		t.Fatalf("null-root error leaked token: %v", err)
	}
	after, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("legacy bytes changed after null-root preflight failure")
	}
}

func TestOpenCodeV2AliasPairLeavesStaleLegacyAuthUntouched(t *testing.T) {
	root, data := setupOpenCode2TestEnv(t)
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeOpenCodeVersionStub(t, binDir, "opencode", "OpenCode 2.0.2")
	writeOpenCodeVersionStub(t, binDir, "opencode2", "OpenCode 2.0.2")
	t.Setenv("PATH", binDir)

	storeDir := filepath.Join(data, "opencode")
	authPath := filepath.Join(storeDir, "auth.json")
	dbPath := filepath.Join(storeDir, "opencode.db")
	if err := writeJSONMap(authPath, map[string]any{
		"openai":    map[string]any{"type": "oauth", "access": "stale-migration-access"},
		"providers": map[string]any{"keep": true},
	}); err != nil {
		t.Fatal(err)
	}
	db := createOpenCode2TestDB(t, dbPath)
	insertOpenCode2TestRow(t, db, "native-alias", "Alias", openCode2Integration, "acct-alias", "old-native-access", "old-native-refresh", 0, 0, nil, 1)
	before, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	resetOpenCodeGenerationCacheForTest()

	if _, err := ApplyAccountToOpenCode(openCode2TestAccount("acct-alias", "new-native-access", "new-native-refresh", time.UnixMilli(1234))); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("v2 alias pair rewrote stale legacy auth")
	}
	assertOpenCode2Row(t, dbPath, "acct-alias", "new-native-access", true)
}

func TestOpenCode2MetadataLessJWTIdentityLoadsUpdatesInPlace(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	dbPath := filepath.Join(t.TempDir(), "native.db")
	t.Setenv("OPENCODE_DB", dbPath)
	db := createOpenCode2TestDB(t, dbPath)
	jwt := openCode2TestJWT("acct-claims", "claims@example.com")
	value, err := json.Marshal(map[string]any{
		"type":     "oauth",
		"methodID": openCode2OAuthMethod,
		"access":   jwt,
		"refresh":  "claims-refresh",
		"expires":  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credential (id, integration_id, label, value, active, time_created, time_updated) VALUES (?, ?, ?, ?, 0, 1, 1)`, "claims-row", openCode2Integration, "OAuth", string(value)); err != nil {
		t.Fatal(err)
	}

	accounts, active, err := loadOpenCode2Accounts(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || len(active) != 0 {
		t.Fatalf("metadata-less JWT load = accounts:%d active:%d", len(accounts), len(active))
	}
	if accounts[0].AccountID != "acct-claims" || accounts[0].Email != "claims@example.com" {
		t.Fatalf("metadata-less JWT identity = id:%q email:%q", accounts[0].AccountID, accounts[0].Email)
	}

	if _, err := ApplyAccountToOpenCode(openCode2TestAccount("acct-claims", "replacement-access", "replacement-refresh", time.Time{})); err != nil {
		t.Fatal(err)
	}
	var rowCount, activeValue int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential WHERE id = 'claims-row'`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT active FROM credential WHERE id = 'claims-row'`).Scan(&activeValue); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 || activeValue != 1 {
		t.Fatalf("metadata-less JWT update row count/active = %d/%d", rowCount, activeValue)
	}
}

func TestOpenCodeCompositeFansOutLegacyAndNative(t *testing.T) {
	root, _ := setupOpenCode2TestEnv(t)
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeOpenCodeVersionStub(t, binDir, "opencode", "OpenCode 1.9.0")
	writeOpenCodeVersionStub(t, binDir, "opencode2", "OpenCode 2.0.0")
	t.Setenv("PATH", binDir)

	authPath := filepath.Join(root, "legacy", "auth.json")
	dbPath := filepath.Join(root, "native", "opencode.db")
	t.Setenv("OPENCODE_AUTH_PATH", authPath)
	t.Setenv("OPENCODE_DB", dbPath)
	if err := writeJSONMap(authPath, map[string]any{"providers": map[string]any{"keep": true}}); err != nil {
		t.Fatal(err)
	}
	_ = createOpenCode2TestDB(t, dbPath)
	resetOpenCodeGenerationCacheForTest()

	account := openCode2TestAccount("acct-fanout", "access-fanout", "refresh-fanout", time.UnixMilli(4567))
	path, err := ApplyAccountToOpenCode(account)
	if err != nil {
		t.Fatal(err)
	}
	if path != authPath {
		t.Fatalf("legacy apply path = %q, want %q", path, authPath)
	}

	legacyData, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{}
	if err := json.Unmarshal(legacyData, &legacy); err != nil {
		t.Fatal(err)
	}
	legacyOpenAI := legacy["openai"].(map[string]any)
	if legacyOpenAI["type"] != "oauth" || legacyOpenAI["accountId"] != "acct-fanout" {
		t.Fatalf("legacy fan-out object = %#v", legacyOpenAI)
	}

	assertOpenCode2Row(t, dbPath, "acct-fanout", "access-fanout", true)
}

func TestOpenCodeCompositeUnavailableRefreshNoOp(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	account := openCode2TestAccount("acct-unavailable", "access-unavailable", "", time.Time{})

	path, err := applyOpenCodeComposite(account, targetWriteRefresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if path != "" {
		t.Fatalf("refresh path = %q, want empty", path)
	}

	if _, err := applyOpenCodeComposite(account, targetWriteApply); err == nil {
		t.Fatal("apply unexpectedly succeeded without an OpenCode store")
	} else if err.Error() != "OpenCode store is not available" {
		t.Fatalf("apply error = %v, want unavailable-store error", err)
	}
}

func TestSaveOpenCodeAccountRefreshFollowsLoadedPath(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		root, _ := setupOpenCode2TestEnv(t)
		loadedPath := filepath.Join(root, "loaded", "auth.json")
		activePath := filepath.Join(root, "active", "auth.json")
		t.Setenv("OPENCODE_AUTH_PATH", loadedPath)

		for _, path := range []string{loadedPath, activePath} {
			if err := writeJSONMap(path, map[string]any{
				"openai": map[string]any{
					"type":      "oauth",
					"access":    "legacy-old",
					"accountId": "acct-refresh",
				},
			}); err != nil {
				t.Fatal(err)
			}
		}

		loaded, err := loadOpenCodeAccountFile(loadedPath, SourceOpenCode, true)
		if err != nil {
			t.Fatal(err)
		}
		if loaded == nil {
			t.Fatal("loaded legacy account is nil")
		}
		loaded.AccessToken = "legacy-refreshed"
		t.Setenv("OPENCODE_AUTH_PATH", activePath)

		if err := SaveAccount(loaded); err != nil {
			t.Fatalf("refresh: %v", err)
		}

		source, err := loadOpenCodeAccountFile(loadedPath, SourceOpenCode, true)
		if err != nil {
			t.Fatal(err)
		}
		active, err := loadOpenCodeAccountFile(activePath, SourceOpenCode, true)
		if err != nil {
			t.Fatal(err)
		}
		if source == nil || source.AccessToken != "legacy-refreshed" {
			t.Fatalf("loaded legacy path was not refreshed: %#v", source)
		}
		if active == nil || active.AccessToken != "legacy-old" {
			t.Fatalf("switched legacy path was changed: %#v", active)
		}
	})

	t.Run("native", func(t *testing.T) {
		root, _ := setupOpenCode2TestEnv(t)
		loadedPath := filepath.Join(root, "loaded", "opencode.db")
		activePath := filepath.Join(root, "active", "opencode.db")
		t.Setenv("OPENCODE_DB", loadedPath)

		loadedDB := createOpenCode2TestDB(t, loadedPath)
		insertOpenCode2TestRow(t, loadedDB, "loaded", "Loaded", openCode2Integration, "acct-refresh", "native-old", "refresh", 0, 0, nil, 1)
		activeDB := createOpenCode2TestDB(t, activePath)
		insertOpenCode2TestRow(t, activeDB, "active", "Active", openCode2Integration, "acct-refresh", "native-old", "refresh", 0, 0, nil, 1)

		accounts, _, err := loadOpenCode2Accounts(loadedPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(accounts) != 1 {
			t.Fatalf("loaded native accounts = %d, want 1", len(accounts))
		}
		loaded := accounts[0]
		loaded.AccessToken = "native-refreshed"
		t.Setenv("OPENCODE_DB", activePath)

		if err := SaveAccount(loaded); err != nil {
			t.Fatalf("refresh: %v", err)
		}

		assertOpenCode2Row(t, loadedPath, "acct-refresh", "native-refreshed", false)
		assertOpenCode2Row(t, activePath, "acct-refresh", "native-old", false)
	})
}

func TestOpenCode2ApplySwitchesActiveAndPreservesRowMetadata(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	dbPath := filepath.Join(t.TempDir(), "native.db")
	t.Setenv("OPENCODE_DB", dbPath)
	db := createOpenCode2TestDB(t, dbPath)
	insertOpenCode2TestRow(t, db, "native-a", "A label", openCode2Integration, "acct-a", "old-a", "refresh-a", 100, 1, map[string]any{"custom": map[string]any{"keep": true}}, 10)
	insertOpenCode2TestRow(t, db, "native-b", "B label", openCode2Integration, "acct-b", "old-b", "refresh-b", 100, 0, nil, 20)
	insertOpenCode2TestRow(t, db, "provider-row", "Provider", "anthropic", "", "api-key", "", 0, 1, map[string]any{"type": "api"}, 30)

	account := openCode2TestAccount("acct-b", "new-b", "new-refresh-b", time.UnixMilli(200))
	if _, err := ApplyAccountToOpenCode(account); err != nil {
		t.Fatal(err)
	}

	var activeA, activeB int64
	if err := db.QueryRow(`SELECT active FROM credential WHERE id = 'native-a'`).Scan(&activeA); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT active FROM credential WHERE id = 'native-b'`).Scan(&activeB); err != nil {
		t.Fatal(err)
	}
	if activeA != 0 || activeB != 1 {
		t.Fatalf("active rows = (%d, %d), want (0, 1)", activeA, activeB)
	}
	var id, label string
	var created int64
	if err := db.QueryRow(`SELECT id, label, time_created FROM credential WHERE integration_id = ? AND id = ?`, openCode2Integration, "native-b").Scan(&id, &label, &created); err != nil {
		t.Fatal(err)
	}
	if id != "native-b" || label != "B label" || created != 20 {
		t.Fatalf("row metadata changed: id=%q label=%q created=%d", id, label, created)
	}
	var providerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential WHERE integration_id = 'anthropic'`).Scan(&providerCount); err != nil {
		t.Fatal(err)
	}
	if providerCount != 1 {
		t.Fatalf("unrelated provider row count = %d", providerCount)
	}
	assertOpenCode2Row(t, dbPath, "acct-b", "new-b", true)
}

func TestOpenCode2LoaderReportsOnlyActiveNativeSource(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	dbPath := filepath.Join(t.TempDir(), "native.db")
	t.Setenv("OPENCODE_DB", dbPath)
	db := createOpenCode2TestDB(t, dbPath)
	insertOpenCode2TestRow(t, db, "active", "Active", openCode2Integration, "acct-active", "access-active", "refresh-active", 0, 1, nil, 1)
	insertOpenCode2TestRow(t, db, "inactive", "Inactive", openCode2Integration, "acct-inactive", "access-inactive", "refresh-inactive", 0, 0, nil, 2)

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.ActiveSourcesByIdentity["account:acct-active"]; !ok {
		t.Fatalf("active native source missing: %#v", result.ActiveSourcesByIdentity)
	}
	if _, ok := result.ActiveSourcesByIdentity["account:acct-inactive"]; ok {
		t.Fatalf("inactive native source reported active: %#v", result.ActiveSourcesByIdentity)
	}
	if !containsAccountID(result.Accounts, "acct-active") || !containsAccountID(result.Accounts, "acct-inactive") {
		t.Fatalf("native pool identities missing from loaded accounts")
	}
}

func TestRestoreManagedAccountsToOpenCode2PreservesNativePool(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	dbPath := filepath.Join(t.TempDir(), "native.db")
	t.Setenv("OPENCODE_DB", dbPath)
	db := createOpenCode2TestDB(t, dbPath)
	insertOpenCode2TestRow(t, db, "existing", "Existing label", openCode2Integration, "acct-one", "native-old", "native-refresh", time.Now().Add(time.Hour).UnixMilli(), 0, map[string]any{"custom": map[string]any{"preserve": "yes"}}, 11)
	insertOpenCode2TestRow(t, db, "stale", "Stale", openCode2Integration, "acct-stale", "stale-access", "", 0, 1, nil, 12)
	insertOpenCode2TestRow(t, db, "other-provider", "Other", "anthropic", "", "api", "", 0, 1, map[string]any{"type": "api"}, 13)

	one := &Account{Label: "One", AccountID: "acct-one", AccessToken: "managed-one", RefreshToken: "managed-refresh-one", ExpiresAt: time.Now().Add(4 * time.Hour), Source: SourceManaged, Writable: true}
	two := &Account{Label: "Two", AccountID: "acct-two", AccessToken: "managed-two", RefreshToken: "managed-refresh-two", ExpiresAt: time.Now().Add(3 * time.Hour), Source: SourceManaged, Writable: true}
	if err := UpsertManagedAccount(one); err != nil {
		t.Fatal(err)
	}
	if err := UpsertManagedAccount(two); err != nil {
		t.Fatal(err)
	}
	managed, err := LoadManagedAccounts()
	if err != nil {
		t.Fatal(err)
	}
	var active *Account
	for _, account := range managed {
		if account.AccountID == "acct-two" {
			active = account
		}
	}
	if active == nil {
		t.Fatal("managed active account missing")
	}

	if _, _, err := RestoreManagedAccountsToOpenCode2(active); err != nil {
		t.Fatal(err)
	}

	var openAI, providers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential WHERE integration_id = ?`, openCode2Integration).Scan(&openAI); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential WHERE integration_id = 'anthropic'`).Scan(&providers); err != nil {
		t.Fatal(err)
	}
	if openAI != 3 || providers != 1 {
		t.Fatalf("native row counts = openai:%d providers:%d", openAI, providers)
	}
	var activeID string
	if err := db.QueryRow(`SELECT id FROM credential WHERE integration_id = ? AND active = 1`, openCode2Integration).Scan(&activeID); err != nil {
		t.Fatal(err)
	}
	if activeID == "stale" {
		t.Fatal("stale native row unexpectedly selected")
	}
	var custom string
	var valueText string
	if err := db.QueryRow(`SELECT value FROM credential WHERE id = 'existing'`).Scan(&valueText); err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(valueText), &value); err != nil {
		t.Fatal(err)
	}
	custom = asMap(value["custom"])["preserve"].(string)
	if custom != "yes" {
		t.Fatalf("native unknown value field was not preserved: %q", custom)
	}
}

func TestOpenCode2SchemaFailureLeavesLegacyBytesUntouchedAndOmitsToken(t *testing.T) {
	root, _ := setupOpenCode2TestEnv(t)
	authPath := filepath.Join(root, "auth.json")
	dbPath := filepath.Join(root, "bad.db")
	t.Setenv("OPENCODE_AUTH_PATH", authPath)
	t.Setenv("OPENCODE_DB", dbPath)
	if err := writeJSONMap(authPath, map[string]any{"openai": map[string]any{"type": "oauth", "access": "old"}, "keep": true}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	badDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := badDB.Exec(`CREATE TABLE credential (id TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	_ = badDB.Close()

	secret := "super-secret-access-token"
	_, err = ApplyAccountToOpenCode(openCode2TestAccount("acct-bad", secret, "", time.Time{}))
	if err == nil {
		t.Fatal("malformed native schema unexpectedly accepted")
	}
	if !strings.Contains(err.Error(), "invalid OpenCode v2 database") {
		t.Fatalf("explicit malformed native schema error = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("schema error leaked credential: %v", err)
	}
	after, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("legacy bytes changed after native preflight failure")
	}
}

func writeOpenCodeVersionStub(t *testing.T, dir, name, version string) {
	t.Helper()
	path := filepath.Join(dir, name)
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q\n", version)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func openCode2TestJWT(accountID, email string) string {
	payload, _ := json.Marshal(map[string]any{
		"https://api.openai.com/auth": accountID,
		"email":                       email,
	})
	return base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func assertOpenCode2Row(t *testing.T, path, accountID, access string, active bool) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT value, active FROM credential WHERE integration_id = ?`, openCode2Integration)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var valueText string
		var activeValue int64
		if err := rows.Scan(&valueText, &activeValue); err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(valueText), &value); err != nil {
			t.Fatal(err)
		}
		if asString(asMap(value["metadata"])["accountID"]) != accountID {
			continue
		}
		found = true
		if asString(value["access"]) != access {
			t.Fatalf("native value = %#v", value)
		}
		if (activeValue == 1) != active {
			t.Fatalf("native active = %d, want %v", activeValue, active)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("native account %q not found", accountID)
	}
}

func containsAccountID(accounts []*Account, accountID string) bool {
	for _, account := range accounts {
		if account != nil && account.AccountID == accountID {
			return true
		}
	}
	return false
}
