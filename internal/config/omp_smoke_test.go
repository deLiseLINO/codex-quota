package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOMPRealCLISmokeTest(t *testing.T) {
	ompPath, err := exec.LookPath("omp")
	if err != nil {
		t.Skip("omp binary not found in PATH, skipping live CLI smoke test")
	}

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("PI_CONFIG_DIR", ".omp")
	dbPath := filepath.Join(tmp, ".omp", "agent", "agent.db")
	t.Setenv("CQ_OMP_DB_PATH", dbPath)

	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cq"))
	selected := &Account{
		AccessToken:  "tok-real-cli-smoke-test",
		RefreshToken: "ref-real-cli-smoke-test",
		AccountID:    "acc-smoke-456",
		Email:        "real-smoke@omp.sh",
		ExpiresAt:    time.Now().Add(48 * time.Hour),
		Source:       SourceManaged,
		Writable:     true,
	}
	for _, account := range []*Account{
		selected,
		{AccessToken: "tok-smoke-two", AccountID: "acc-smoke-2", Email: "two@omp.sh"},
		{AccessToken: "tok-smoke-three", AccountID: "acc-smoke-3", Email: "three@omp.sh"},
	} {
		if err := UpsertManagedAccount(account); err != nil {
			t.Fatalf("save smoke account: %v", err)
		}
	}
	listCount := func() int {
		cmd := exec.Command(ompPath, "token", "openai-codex", "--list")
		cmd.Env = append(os.Environ(), "HOME="+tmp)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("omp token --list failed: %v\nOutput: %s", err, string(out))
		}
		outStr := strings.TrimSpace(string(out))
		t.Logf("omp token output:\n%s", outStr)
		if outStr == "" {
			return 0
		}
		return len(strings.Split(outStr, "\n"))
	}
	if count, _, err := RestoreManagedAccountsToOMP(); err != nil || count != 3 || listCount() != 3 {
		t.Fatalf("restore smoke = count %d, err %v", count, err)
	}
	selected.AccessToken = "tok-real-cli-smoke-refreshed"
	if err := saveOMPAccount(selected); err != nil {
		t.Fatalf("refresh smoke: %v", err)
	}
	if got := listCount(); got != 3 {
		t.Fatalf("expected three OMP credentials after refresh, got %d", got)
	}
	if appliedPath, err := ApplyAccountToOMP(selected); err != nil || appliedPath != dbPath {
		t.Fatalf("exclusive apply smoke = %q, %v", appliedPath, err)
	}
	if got := listCount(); got != 1 {
		t.Fatalf("expected one OMP credential after exclusive apply, got %d", got)
	}
	if count, _, err := RestoreManagedAccountsToOMP(); err != nil || count != 3 || listCount() != 3 {
		t.Fatalf("second restore smoke = count %d, err %v", count, err)
	}
}
