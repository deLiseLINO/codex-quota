package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCodexAccountFileReadsIDToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", tmp)
	path := filepath.Join(tmp, "auth.json")
	if err := os.WriteFile(path, []byte(`{"tokens":{"access_token":"tok","account_id":"acc-1","id_token":"idt-1"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	got, err := loadCodexAccountFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("expected account")
	}
	if got.IDToken != "idt-1" {
		t.Fatalf("expected id_token, got %q", got.IDToken)
	}
}

func TestLoadCodexAccountFileWithoutIDToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", tmp)
	path := filepath.Join(tmp, "auth.json")
	if err := os.WriteFile(path, []byte(`{"tokens":{"access_token":"tok","account_id":"acc-1"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	got, err := loadCodexAccountFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("expected account")
	}
	if got.IDToken != "" {
		t.Fatalf("expected empty id_token, got %q", got.IDToken)
	}
}
