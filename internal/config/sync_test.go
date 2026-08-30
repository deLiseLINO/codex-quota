package config

import (
	"testing"
	"time"
)

func TestFreshestAccountForIdentityPrefersLaterExpiry(t *testing.T) {
	oldExpiry := time.Unix(1000, 0)
	newExpiry := time.Unix(2000, 0)
	seed := &Account{AccountID: "acc-1", AccessToken: "old-access", RefreshToken: "old-refresh", ExpiresAt: oldExpiry, Source: SourceManaged, Writable: true}
	candidates := []*Account{
		seed,
		{AccountID: "acc-1", AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: newExpiry, Source: SourceOpenCode, Writable: true},
	}

	got := freshestAccountForIdentity(seed, candidates)
	if got == nil {
		t.Fatal("expected account")
	}
	if got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" {
		t.Fatalf("selected stale bundle: access=%q refresh=%q", got.AccessToken, got.RefreshToken)
	}
}

func TestFreshestAccountForIdentityPrefersExternalWhenExpiryTies(t *testing.T) {
	expires := time.Unix(1000, 0)
	seed := &Account{AccountID: "acc-1", AccessToken: "managed-access", RefreshToken: "managed-refresh", ExpiresAt: expires, Source: SourceManaged, Writable: true}
	candidates := []*Account{
		seed,
		{AccountID: "acc-1", AccessToken: "external-access", RefreshToken: "external-refresh", ExpiresAt: expires, Source: SourceOpenCode, Writable: true},
	}

	got := freshestAccountForIdentity(seed, candidates)
	if got == nil {
		t.Fatal("expected account")
	}
	if got.AccessToken != "external-access" || got.RefreshToken != "external-refresh" {
		t.Fatalf("selected managed bundle on tie: access=%q refresh=%q", got.AccessToken, got.RefreshToken)
	}
}

func TestSyncAccountEverywhereWritesFreshBundleToLocalStores(t *testing.T) {
	setupSyncTestEnv(t)

	fresh := &Account{
		AccountID:    "acc-1",
		Email:        "user@example.com",
		AccessToken:  "fresh-access",
		RefreshToken: "fresh-refresh",
		ExpiresAt:    time.Unix(2000, 0),
		Source:       SourceManaged,
		Writable:     true,
	}
	if err := UpsertManagedAccount(&Account{AccountID: "acc-1", AccessToken: "old-managed", RefreshToken: "old-managed-refresh", Source: SourceManaged, Writable: true}); err != nil {
		t.Fatalf("seed managed: %v", err)
	}
	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-1", AccessToken: "old-a", RefreshToken: "old-a-refresh"}); err != nil {
		t.Fatalf("seed app a: %v", err)
	}
	if _, err := ApplyAccountToOpenCode(&Account{AccountID: "acc-1", AccessToken: "old-b", RefreshToken: "old-b-refresh"}); err != nil {
		t.Fatalf("seed app b: %v", err)
	}

	if err := SyncAccountEverywhere(fresh); err != nil {
		t.Fatalf("sync everywhere: %v", err)
	}

	managed, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load managed: %v", err)
	}
	if len(managed) != 1 || managed[0].AccessToken != "fresh-access" || managed[0].RefreshToken != "fresh-refresh" {
		t.Fatalf("managed not synced: %#v", managed)
	}

	appA, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load app a: %v", err)
	}
	if appA.AccessToken != "fresh-access" || appA.RefreshToken != "fresh-refresh" {
		t.Fatalf("app a not synced: %#v", appA)
	}

	appB, err := loadOpenCodeAccountFile(opencodeAuthPath(), SourceOpenCode, true)
	if err != nil {
		t.Fatalf("load app b: %v", err)
	}
	if appB.AccessToken != "fresh-access" || appB.RefreshToken != "fresh-refresh" {
		t.Fatalf("app b not synced: %#v", appB)
	}
}

func TestSyncAccountEverywhereSkipsLocalStoresForDifferentAccount(t *testing.T) {
	setupSyncTestEnv(t)

	fresh := &Account{
		AccountID:    "acc-1",
		Email:        "user@example.com",
		AccessToken:  "fresh-access",
		RefreshToken: "fresh-refresh",
		ExpiresAt:    time.Unix(2000, 0),
		Source:       SourceManaged,
		Writable:     true,
	}
	other := &Account{
		AccountID:    "acc-2",
		Email:        "other@example.com",
		AccessToken:  "other-access",
		RefreshToken: "other-refresh",
		ExpiresAt:    time.Unix(3000, 0),
		Writable:     true,
	}
	if _, err := ApplyAccountToCodex(other); err != nil {
		t.Fatalf("seed app a: %v", err)
	}
	if _, err := ApplyAccountToOpenCode(other); err != nil {
		t.Fatalf("seed app b: %v", err)
	}

	if err := SyncAccountEverywhere(fresh); err != nil {
		t.Fatalf("sync everywhere: %v", err)
	}

	managed, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load managed: %v", err)
	}
	if len(managed) != 1 || managed[0].AccountID != "acc-1" || managed[0].AccessToken != "fresh-access" {
		t.Fatalf("managed not synced: %#v", managed)
	}

	appA, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load app a: %v", err)
	}
	if appA.AccountID != "acc-2" || appA.AccessToken != "other-access" || appA.RefreshToken != "other-refresh" {
		t.Fatalf("app a was overwritten: %#v", appA)
	}

	appB, err := loadOpenCodeAccountFile(opencodeAuthPath(), SourceOpenCode, true)
	if err != nil {
		t.Fatalf("load app b: %v", err)
	}
	if appB.AccountID != "acc-2" || appB.AccessToken != "other-access" || appB.RefreshToken != "other-refresh" {
		t.Fatalf("app b was overwritten: %#v", appB)
	}
}

func TestSyncAccountEverywhereDoesNotApplyToEmptyLocalStores(t *testing.T) {
	setupSyncTestEnv(t)

	fresh := &Account{
		AccountID:    "acc-1",
		Email:        "user@example.com",
		AccessToken:  "fresh-access",
		RefreshToken: "fresh-refresh",
		ExpiresAt:    time.Unix(2000, 0),
		Source:       SourceManaged,
		Writable:     true,
	}

	if err := SyncAccountEverywhere(fresh); err != nil {
		t.Fatalf("sync everywhere: %v", err)
	}

	managed, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load managed: %v", err)
	}
	if len(managed) != 1 || managed[0].AccountID != "acc-1" || managed[0].AccessToken != "fresh-access" {
		t.Fatalf("managed not synced: %#v", managed)
	}

	appA, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load app a: %v", err)
	}
	if appA != nil {
		t.Fatalf("app a was applied during refresh: %#v", appA)
	}

	appB, err := loadOpenCodeAccountFile(opencodeAuthPath(), SourceOpenCode, true)
	if err != nil {
		t.Fatalf("load app b: %v", err)
	}
	if appB != nil {
		t.Fatalf("app b was applied during refresh: %#v", appB)
	}
}

func setupSyncTestEnv(t *testing.T) {
	t.Helper()

	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")
	t.Setenv("CODEX_HOME", tmp+"/app-a")
	t.Setenv("OPENCODE_AUTH_PATH", tmp+"/app-b/auth.json")
	t.Setenv("OPENCODE_DATA_DIR", tmp+"/opencode-data")
	t.Setenv("CQ_PI_AUTH_PATH", tmp+"/pi/auth.json")
	t.Setenv("CQ_OMP_DB_PATH", tmp+"/omp/agent.db")
	t.Setenv("HOME", tmp+"/home")
}

func TestOMPRefreshPreservesRestoredPool(t *testing.T) {
	setupSyncTestEnv(t)
	first := &Account{AccountID: "acc-1", Email: "one@example.com", AccessToken: "old-1", Source: SourceManaged, Writable: true}
	second := &Account{AccountID: "acc-2", Email: "two@example.com", AccessToken: "old-2", Source: SourceManaged, Writable: true}
	for _, account := range []*Account{first, second} {
		if err := UpsertManagedAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if count, _, err := RestoreManagedAccountsToOMP(); err != nil || count != 2 {
		t.Fatalf("restore pool: count=%d err=%v", count, err)
	}
	first.AccessToken = "sync-1"
	if err := SyncAccountEverywhere(first); err != nil {
		t.Fatal(err)
	}
	accounts, err := loadOMPAccounts(ompAgentDbPath())
	if err != nil || len(accounts) != 2 {
		t.Fatalf("pool after sync: accounts=%d err=%v", len(accounts), err)
	}
	first.AccessToken = "save-1"
	if err := saveOMPAccount(first); err != nil {
		t.Fatal(err)
	}
	accounts, err = loadOMPAccounts(ompAgentDbPath())
	if err != nil || len(accounts) != 2 {
		t.Fatalf("pool after save: accounts=%d err=%v", len(accounts), err)
	}
	for _, account := range accounts {
		if account.AccountID == first.AccountID && account.AccessToken != "save-1" {
			t.Fatalf("refreshed account token = %q", account.AccessToken)
		}
	}
}

func TestApplyAccountToTargetKeepsFresherExistingTargetBundle(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")
	t.Setenv("CODEX_HOME", tmp+"/app-a")

	oldAccess := makeTestJWT(t, map[string]any{"exp": int64(1000), "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acc-1"}})
	newAccess := makeTestJWT(t, map[string]any{"exp": int64(2000), "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acc-1"}})
	oldAccount := &Account{AccountID: "acc-1", AccessToken: oldAccess, RefreshToken: "old-refresh", ExpiresAt: time.Unix(1000, 0), Source: SourceManaged, Writable: true}
	newTarget := &Account{AccountID: "acc-1", AccessToken: newAccess, RefreshToken: "new-refresh", ExpiresAt: time.Unix(2000, 0), Source: SourceCodex, Writable: true}
	if _, err := ApplyAccountToCodex(newTarget); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	if _, err := ApplyAccountToTarget(oldAccount, SourceCodex); err != nil {
		t.Fatalf("apply stale account: %v", err)
	}

	got, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load target: %v", err)
	}
	if got.AccessToken != newAccess || got.RefreshToken != "new-refresh" {
		t.Fatalf("target was overwritten with stale bundle: %#v", got)
	}
}
