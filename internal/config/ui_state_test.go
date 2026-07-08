package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUIStateRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	initial := UIState{
		CompactMode:          true,
		ExhaustedAccountKeys: []string{"managed:1", "codex:2"},
		AccountOrderKeys:     []string{"acc-2", "acc-1"},
		ActiveAccountKey:     "managed:2",
	}
	if err := SaveUIState(initial); err != nil {
		t.Fatalf("save ui state: %v", err)
	}

	loaded, err := LoadUIState()
	if err != nil {
		t.Fatalf("load ui state: %v", err)
	}
	if loaded.CompactMode != initial.CompactMode {
		t.Fatalf("compact mode mismatch: got %v, want %v", loaded.CompactMode, initial.CompactMode)
	}
	if len(loaded.ExhaustedAccountKeys) != len(initial.ExhaustedAccountKeys) {
		t.Fatalf("exhausted keys length mismatch: got %d, want %d", len(loaded.ExhaustedAccountKeys), len(initial.ExhaustedAccountKeys))
	}
	for i := range initial.ExhaustedAccountKeys {
		if loaded.ExhaustedAccountKeys[i] != initial.ExhaustedAccountKeys[i] {
			t.Fatalf("exhausted key mismatch at %d: got %q, want %q", i, loaded.ExhaustedAccountKeys[i], initial.ExhaustedAccountKeys[i])
		}
	}
	if len(loaded.AccountOrderKeys) != len(initial.AccountOrderKeys) {
		t.Fatalf("account order keys length mismatch: got %d, want %d", len(loaded.AccountOrderKeys), len(initial.AccountOrderKeys))
	}
	for i := range initial.AccountOrderKeys {
		if loaded.AccountOrderKeys[i] != initial.AccountOrderKeys[i] {
			t.Fatalf("account order key mismatch at %d: got %q, want %q", i, loaded.AccountOrderKeys[i], initial.AccountOrderKeys[i])
		}
	}
	if loaded.ActiveAccountKey != initial.ActiveAccountKey {
		t.Fatalf("active account key mismatch: got %q, want %q", loaded.ActiveAccountKey, initial.ActiveAccountKey)
	}
}

func TestLoadUIStateMissingFileReturnsDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	loaded, err := LoadUIState()
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if loaded.CompactMode {
		t.Fatalf("expected default compact mode false for missing file")
	}
	if len(loaded.ExhaustedAccountKeys) != 0 {
		t.Fatalf("expected empty exhausted keys by default")
	}
	if len(loaded.AccountOrderKeys) != 0 {
		t.Fatalf("expected empty account order keys by default")
	}
}

func TestLoadUIStateInvalidJSONReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "codex-quota")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "ui_state.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	if _, err := LoadUIState(); err == nil {
		t.Fatalf("expected error for invalid ui state json")
	}
}

func TestLoadUIStateOldFormatWithoutExhaustedKeys(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "codex-quota")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "ui_state.json")
	if err := os.WriteFile(path, []byte(`{"compact_mode":true}`), 0o600); err != nil {
		t.Fatalf("write ui state: %v", err)
	}

	loaded, err := LoadUIState()
	if err != nil {
		t.Fatalf("load ui state: %v", err)
	}
	if !loaded.CompactMode {
		t.Fatalf("expected compact mode true from old format")
	}
	if len(loaded.ExhaustedAccountKeys) != 0 {
		t.Fatalf("expected empty exhausted keys for old format")
	}
	if len(loaded.AccountOrderKeys) != 0 {
		t.Fatalf("expected empty account order keys for old format")
	}
}
