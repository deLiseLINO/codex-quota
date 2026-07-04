package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestReconcileWorkingAccountUsesFreshExternalBundle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))
	t.Setenv("OPENCODE_DATA_DIR", filepath.Join(tmp, "opencode-data"))
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	oldAccount := &config.Account{AccountID: "acc-1", AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: time.Unix(1000, 0), Source: config.SourceManaged, Writable: true}
	freshAccount := &config.Account{AccountID: "acc-1", AccessToken: "fresh-access", RefreshToken: "fresh-refresh", ExpiresAt: time.Unix(2000, 0), Source: config.SourceOpenCode, Writable: true}
	if err := config.UpsertManagedAccount(oldAccount); err != nil {
		t.Fatalf("seed managed: %v", err)
	}
	if _, err := config.ApplyAccountToOpenCode(freshAccount); err != nil {
		t.Fatalf("seed external: %v", err)
	}

	working := *oldAccount
	changed, err := reconcileWorkingAccount(&working)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}
	if working.AccessToken != "fresh-access" || working.RefreshToken != "fresh-refresh" {
		t.Fatalf("working account not reconciled: %#v", working)
	}
}
