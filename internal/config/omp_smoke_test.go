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

	if _, err := ApplyAccountToOMP(&Account{
		AccessToken: "tok-replaced-cli-smoke-test",
		AccountID:   "acc-replaced",
		Email:       "replaced@omp.sh",
	}); err != nil {
		t.Fatalf("seed replaced account: %v", err)
	}

	selected := &Account{
		AccessToken:  "tok-real-cli-smoke-test",
		RefreshToken: "ref-real-cli-smoke-test",
		AccountID:    "acc-smoke-456",
		Email:        "real-smoke@omp.sh",
		ExpiresAt:    time.Now().Add(48 * time.Hour),
		Source:       SourceOMP,
		FilePath:     dbPath,
		Writable:     true,
	}

	appliedPath, err := ApplyAccountToOMP(selected)
	if err != nil {
		t.Fatalf("ApplyAccountToOMP error: %v", err)
	}
	if appliedPath != dbPath {
		t.Errorf("appliedPath = %q, want %q", appliedPath, dbPath)
	}

	// Run actual installed omp CLI with HOME set to the temporary directory
	cmd := exec.Command(ompPath, "token", "openai-codex", "--list")
	cmd.Env = append(os.Environ(), "HOME="+tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("omp token --list failed: %v\nOutput: %s", err, string(out))
	}

	outStr := strings.TrimSpace(string(out))
	t.Logf("omp token output:\n%s", outStr)
	lines := strings.Split(outStr, "\n")
	if len(lines) != 1 || (!strings.Contains(outStr, "real-smoke@omp.sh") && !strings.Contains(outStr, "acc-smoke-456")) || strings.Contains(outStr, "replaced@omp.sh") {
		t.Errorf("expected exactly one selected account from real omp binary, got:\n%s", outStr)
	}
}
