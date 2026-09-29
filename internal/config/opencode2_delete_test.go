package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestOpenCodeCompositeDeleteUsesIdentityAndNativeFallback(t *testing.T) {
	root, _ := setupOpenCode2TestEnv(t)
	authPath := filepath.Join(root, "legacy", "auth.json")
	dbPath := filepath.Join(root, "native", "opencode.db")
	t.Setenv("OPENCODE_AUTH_PATH", authPath)
	t.Setenv("OPENCODE_DB", dbPath)
	if err := writeJSONMap(authPath, map[string]any{
		"openai": map[string]any{
			"type":      "oauth",
			"access":    "access-a",
			"refresh":   "refresh-a",
			"accountId": "acct-a",
		},
		"providers": map[string]any{"keep": true},
	}); err != nil {
		t.Fatal(err)
	}
	db := createOpenCode2TestDB(t, dbPath)
	insertOpenCode2TestRow(t, db, "native-a", "A", openCode2Integration, "acct-a", "access-a", "refresh-a", 0, 1, nil, 1)
	insertOpenCode2TestRow(t, db, "native-b", "B", openCode2Integration, "acct-b", "access-b", "refresh-b", 0, 0, nil, 2)

	if err := DeleteOpenCodeAuthAccount(openCode2TestAccount("acct-a", "access-a", "refresh-a", time.Time{})); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential WHERE id = 'native-a'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("matching native credential was not deleted")
	}
	var fallbackActive int64
	if err := db.QueryRow(`SELECT active FROM credential WHERE id = 'native-b'`).Scan(&fallbackActive); err != nil {
		t.Fatal(err)
	}
	if fallbackActive != 1 {
		t.Fatalf("native fallback active = %d, want 1", fallbackActive)
	}
	legacyData, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{}
	if err := json.Unmarshal(legacyData, &legacy); err != nil {
		t.Fatal(err)
	}
	if got := legacy["openai"].(map[string]any); len(got) != 0 {
		t.Fatalf("legacy active auth fields remain: %#v", got)
	}
	if !reflect.DeepEqual(legacy["providers"], map[string]any{"keep": true}) {
		t.Fatalf("unrelated legacy providers changed: %#v", legacy["providers"])
	}
}

func TestOpenCode2DeleteMatchesMetadataLessJWTIdentity(t *testing.T) {
	_, _ = setupOpenCode2TestEnv(t)
	dbPath := filepath.Join(t.TempDir(), "native.db")
	t.Setenv("OPENCODE_DB", dbPath)
	db := createOpenCode2TestDB(t, dbPath)
	value, err := json.Marshal(map[string]any{
		"type":     "oauth",
		"methodID": openCode2OAuthMethod,
		"access":   openCode2TestJWT("acct-delete", "delete@example.com"),
		"refresh":  "refresh-delete",
		"expires":  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO credential (id, integration_id, label, value, active, time_created, time_updated) VALUES (?, ?, ?, ?, 1, 1, 1)`, "claims-delete", openCode2Integration, "OAuth", string(value)); err != nil {
		t.Fatal(err)
	}

	if err := DeleteOpenCodeAuthAccount(openCode2TestAccount("acct-delete", "access-delete", "refresh-delete", time.Time{})); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM credential WHERE id = 'claims-delete'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("metadata-less JWT credential was not deleted")
	}
}
