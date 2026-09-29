package ui

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"

	_ "modernc.org/sqlite"
)

func setupOpenCode2RestoreUITestEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("CQ_OMP_DB_PATH", filepath.Join(root, "omp", "agent.db"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "codex"))
	t.Setenv("CQ_PI_AUTH_PATH", filepath.Join(root, "pi", "auth.json"))
	t.Setenv("OPENCODE_DATA_DIR", filepath.Join(root, "opencode-data"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("PATH", "")
	t.Setenv("OPENCODE_AUTH_PATH", "")
	t.Setenv("OPENCODE_DB", "")
	return root
}

func createOpenCode2RestoreUITestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
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

func nativeRestoreMenuItem(m Model) (actionMenuItem, bool) {
	for _, item := range m.actionMenuItems() {
		if item.ID == actionMenuRestoreOpenCode2Pool {
			return item, true
		}
	}
	return actionMenuItem{}, false
}

func TestOpenCode2RestoreActionVisibilityDependsOnNativeDB(t *testing.T) {
	root := setupOpenCode2RestoreUITestEnv(t)
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(root, "legacy", "auth.json"))
	m := testModelForHotkeys(1)
	m.openActionMenu()
	if _, ok := nativeRestoreMenuItem(m); ok {
		t.Fatal("legacy-only install exposed OpenCode 2 restore action")
	}

	dbPath := filepath.Join(root, "native", "opencode.db")
	_ = createOpenCode2RestoreUITestDB(t, dbPath)
	t.Setenv("OPENCODE_DB", dbPath)
	m.openActionMenu()
	item, ok := nativeRestoreMenuItem(m)
	if !ok {
		t.Fatal("native OpenCode 2 database did not expose restore action")
	}
	if !strings.Contains(item.Label, "OpenCode 2") || item.Shortcut != "" {
		t.Fatalf("native restore menu item = %#v", item)
	}
}

func TestOpenCode2RestoreActionHiddenForInvalidImplicitDB(t *testing.T) {
	root := setupOpenCode2RestoreUITestEnv(t)
	dbPath := filepath.Join(root, "data", "opencode", "opencode.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := testModelForHotkeys(1)
	m.openActionMenu()
	if _, ok := nativeRestoreMenuItem(m); ok {
		t.Fatal("invalid implicit native database exposed OpenCode 2 restore action")
	}
}

func TestOpenCode2RestoreModalCanOpenCancelAndConfirm(t *testing.T) {
	root := setupOpenCode2RestoreUITestEnv(t)
	dbPath := filepath.Join(root, "native", "opencode.db")
	_ = createOpenCode2RestoreUITestDB(t, dbPath)
	t.Setenv("OPENCODE_DB", dbPath)
	m := testModelForHotkeys(1)
	m.openActionMenu()
	item, ok := nativeRestoreMenuItem(m)
	if !ok {
		t.Fatal("native restore action missing")
	}
	items := m.actionMenuItems()
	for index := range items {
		if items[index].ID == item.ID {
			m.ActionMenuCursor = index
			break
		}
	}
	m.ActionMenuVisible = true
	updated, cmd := m.confirmActionMenu()
	if cmd != nil {
		t.Fatal("opening restore modal unexpectedly started command")
	}
	m = updated.(Model)
	if !m.OpenCode2RestoreConfirm || m.ActionMenuVisible {
		t.Fatal("restore action did not open dedicated confirmation modal")
	}
	modal := m.renderOpenCode2RestoreConfirmModal()
	for _, text := range []string{"OpenCode 2", "Existing OpenCode credentials are preserved", "active CQ account will be activated", "[enter] Restore", "[esc] Cancel"} {
		if !strings.Contains(modal, text) {
			t.Fatalf("restore modal missing %q: %s", text, modal)
		}
	}
	updated, cmd = m.handleOpenCode2RestoreConfirm("esc")
	if cmd != nil || updated.(Model).OpenCode2RestoreConfirm {
		t.Fatal("escape did not cancel restore modal")
	}

	m = updated.(Model)
	m.OpenCode2RestoreConfirm = true
	updated, cmd = m.handleOpenCode2RestoreConfirm("enter")
	m = updated.(Model)
	if cmd == nil || m.OpenCode2RestoreConfirm || !m.Loading {
		t.Fatal("confirm did not start dedicated restore command")
	}
}

func TestOpenCode2RestoreConfirmWithoutActiveAccountShowsNotice(t *testing.T) {
	setupOpenCode2RestoreUITestEnv(t)
	m := Model{OpenCode2RestoreConfirm: true}
	updated, _ := m.handleOpenCode2RestoreConfirm("enter")
	m = updated.(Model)
	if m.OpenCode2RestoreConfirm || m.Loading {
		t.Fatal("missing active account left restore modal/loading state")
	}
	if !strings.Contains(m.Notice, "no active account") {
		t.Fatalf("missing active-account notice: %q", m.Notice)
	}
}

func TestRestoreOpenCode2AccountsCmdReportsSuccessAndReloads(t *testing.T) {
	root := setupOpenCode2RestoreUITestEnv(t)
	dbPath := filepath.Join(root, "native", "opencode.db")
	_ = createOpenCode2RestoreUITestDB(t, dbPath)
	t.Setenv("OPENCODE_DB", dbPath)
	account := &config.Account{Key: "managed:one", Label: "One", AccountID: "acct-one", AccessToken: "access-one", RefreshToken: "refresh-one", ExpiresAt: time.Now().Add(2 * time.Hour), Source: config.SourceManaged, Writable: true}
	if err := config.UpsertManagedAccount(account); err != nil {
		t.Fatal(err)
	}

	msg := RestoreOpenCode2AccountsCmd(account, account.Key)()
	result, ok := msg.(AccountsMsg)
	if !ok {
		t.Fatalf("restore command returned %T: %#v", msg, msg)
	}
	if result.ActiveKey != account.Key || !strings.Contains(result.Notice, "restored 1 CQ accounts") || !strings.Contains(result.Notice, dbPath) {
		t.Fatalf("restore success result = %#v", result)
	}
	m := testModelForHotkeys(1)
	updated, _ := m.Update(result)
	updatedModel := updated.(Model)
	if updatedModel.Notice != result.Notice || len(updatedModel.Accounts) == 0 {
		t.Fatalf("successful restore result was not applied: notice=%q accounts=%d", updatedModel.Notice, len(updatedModel.Accounts))
	}
}

func TestRestoreOpenCode2AccountsCmdSurfacesErrorWithoutToken(t *testing.T) {
	setupOpenCode2RestoreUITestEnv(t)
	secret := "ui-secret-token"
	account := &config.Account{Key: "managed:one", AccountID: "acct-one", AccessToken: secret, Source: config.SourceManaged, Writable: true}
	msg := RestoreOpenCode2AccountsCmd(account, account.Key)()
	errMsg, ok := msg.(ErrMsg)
	if !ok || errMsg.Err == nil {
		t.Fatalf("restore error result = %#v", msg)
	}
	if strings.Contains(errMsg.Err.Error(), secret) {
		t.Fatalf("restore error leaked token: %v", errMsg.Err)
	}
	m := testModelForHotkeys(1)
	updated, _ := m.Update(errMsg)
	if updated.(Model).Err == nil {
		t.Fatal("restore error did not reach model error state")
	}
}
