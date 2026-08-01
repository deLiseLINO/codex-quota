package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setupCodexApplyEnv(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp+"/cfg")
	t.Setenv("CODEX_HOME", tmp+"/codex")
}

func TestApplyAccountToCodexWritesIDToken(t *testing.T) {
	setupCodexApplyEnv(t)

	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-1", AccessToken: "tok-a", RefreshToken: "refresh-a", IDToken: "idt-a"}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || got.IDToken != "idt-a" {
		t.Fatalf("expected id_token written, got %#v", got)
	}
}

func TestApplyAccountToCodexFallsBackToAccessToken(t *testing.T) {
	setupCodexApplyEnv(t)

	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-1", AccessToken: "tok-a", RefreshToken: "refresh-a"}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || got.IDToken != "tok-a" {
		t.Fatalf("expected access_token fallback as id_token, got %#v", got)
	}
}

func TestApplyAccountToCodexReplacesStaleIDToken(t *testing.T) {
	setupCodexApplyEnv(t)

	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-1", AccessToken: "tok-a", RefreshToken: "refresh-a", IDToken: "idt-a"}); err != nil {
		t.Fatalf("seed acc-1: %v", err)
	}
	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-2", AccessToken: "tok-b", RefreshToken: "refresh-b", IDToken: "idt-b"}); err != nil {
		t.Fatalf("apply acc-2: %v", err)
	}

	got, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || got.IDToken != "idt-b" || got.AccessToken != "tok-b" || got.AccountID != "acc-2" {
		t.Fatalf("expected stale id_token replaced, got %#v", got)
	}
}

func TestApplyAccountToCodexReplacesStaleIDTokenWithFallback(t *testing.T) {
	setupCodexApplyEnv(t)

	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-1", AccessToken: "tok-a", RefreshToken: "refresh-a", IDToken: "idt-a"}); err != nil {
		t.Fatalf("seed acc-1: %v", err)
	}
	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-2", AccessToken: "tok-b", RefreshToken: "refresh-b"}); err != nil {
		t.Fatalf("apply acc-2 without id_token: %v", err)
	}

	got, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil || got.IDToken != "tok-b" || got.AccessToken != "tok-b" {
		t.Fatalf("expected stale id_token replaced with fallback, got %#v", got)
	}
}

func TestDeleteCodexAuthAccountRemovesIDToken(t *testing.T) {
	setupCodexApplyEnv(t)

	if _, err := ApplyAccountToCodex(&Account{AccountID: "acc-1", AccessToken: "tok-a", RefreshToken: "refresh-a", IDToken: "idt-a"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := DeleteCodexAuthAccount(); err != nil {
		t.Fatalf("delete: %v", err)
	}

	data, err := os.ReadFile(codexAuthPath())
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	root, err := readJSONMap(codexAuthPath())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tokens := asMap(root["tokens"])
	if tokens == nil {
		t.Fatalf("expected tokens block present in %s", filepath.Base(codexAuthPath()))
	}
	if _, ok := tokens["id_token"]; ok {
		t.Fatalf("expected id_token removed, file has it: %s", data)
	}
	if _, ok := tokens["access_token"]; ok {
		t.Fatalf("expected access_token removed")
	}
}
