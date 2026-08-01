package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSettingsFile(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	path := filepath.Join(dir, "codex-quota", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func TestLoadSettingsMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if !settings.CheckForUpdateOnStartup {
		t.Fatalf("CheckForUpdateOnStartup = false, want true")
	}
	if !settings.AutoRefreshEnabled {
		t.Fatalf("AutoRefreshEnabled = false, want true")
	}
	if settings.ActiveIntervalSec != 30 {
		t.Fatalf("ActiveIntervalSec = %d, want 30", settings.ActiveIntervalSec)
	}
	if settings.BackgroundIntervalSec != 300 {
		t.Fatalf("BackgroundIntervalSec = %d, want 300", settings.BackgroundIntervalSec)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	initial := Settings{
		CheckForUpdateOnStartup: false,
		AutoRefreshEnabled:      false,
		ActiveIntervalSec:       45,
		BackgroundIntervalSec:   600,
	}
	if err := SaveSettings(initial); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	loaded, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if loaded.CheckForUpdateOnStartup != initial.CheckForUpdateOnStartup {
		t.Fatalf("CheckForUpdateOnStartup = %v, want %v", loaded.CheckForUpdateOnStartup, initial.CheckForUpdateOnStartup)
	}
	if loaded.AutoRefreshEnabled != initial.AutoRefreshEnabled {
		t.Fatalf("AutoRefreshEnabled = %v, want %v", loaded.AutoRefreshEnabled, initial.AutoRefreshEnabled)
	}
	if loaded.ActiveIntervalSec != initial.ActiveIntervalSec {
		t.Fatalf("ActiveIntervalSec = %d, want %d", loaded.ActiveIntervalSec, initial.ActiveIntervalSec)
	}
	if loaded.BackgroundIntervalSec != initial.BackgroundIntervalSec {
		t.Fatalf("BackgroundIntervalSec = %d, want %d", loaded.BackgroundIntervalSec, initial.BackgroundIntervalSec)
	}
}

func TestLoadSettingsClampsIntervals(t *testing.T) {
	defaults := DefaultSettings()
	cases := []struct {
		name           string
		content        string
		wantActive     int
		wantBackground int
	}{
		{"clamp active low", `{"active_interval_sec":5}`, ActiveIntervalMinSec, defaults.BackgroundIntervalSec},
		{"clamp active high", `{"active_interval_sec":1000}`, ActiveIntervalMaxSec, defaults.BackgroundIntervalSec},
		{"clamp background low", `{"background_interval_sec":10}`, defaults.ActiveIntervalSec, BackgroundIntervalMinSec},
		{"clamp background high", `{"background_interval_sec":9999}`, defaults.ActiveIntervalSec, BackgroundIntervalMaxSec},
		{"in bounds unchanged", `{"active_interval_sec":60,"background_interval_sec":120}`, 60, 120},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeSettingsFile(t, tc.content)

			loaded, err := LoadSettings()
			if err != nil {
				t.Fatalf("LoadSettings() error = %v", err)
			}
			if loaded.ActiveIntervalSec != tc.wantActive {
				t.Fatalf("ActiveIntervalSec = %d, want %d", loaded.ActiveIntervalSec, tc.wantActive)
			}
			if loaded.BackgroundIntervalSec != tc.wantBackground {
				t.Fatalf("BackgroundIntervalSec = %d, want %d", loaded.BackgroundIntervalSec, tc.wantBackground)
			}
		})
	}
}

func TestLoadSettingsOldFormatWithoutAutoRefreshKeys(t *testing.T) {
	writeSettingsFile(t, `{"check_for_update_on_startup":false}`)

	loaded, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if loaded.CheckForUpdateOnStartup {
		t.Fatalf("CheckForUpdateOnStartup = true, want false")
	}
	if !loaded.AutoRefreshEnabled {
		t.Fatalf("AutoRefreshEnabled = false, want default true")
	}
	if loaded.ActiveIntervalSec != 30 {
		t.Fatalf("ActiveIntervalSec = %d, want 30", loaded.ActiveIntervalSec)
	}
	if loaded.BackgroundIntervalSec != 300 {
		t.Fatalf("BackgroundIntervalSec = %d, want 300", loaded.BackgroundIntervalSec)
	}
}

func TestLoadSettingsNonNumericIntervalsFallBackToDefaults(t *testing.T) {
	writeSettingsFile(t, `{"active_interval_sec":"fast","background_interval_sec":null}`)

	loaded, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if loaded.ActiveIntervalSec != 30 {
		t.Fatalf("ActiveIntervalSec = %d, want 30", loaded.ActiveIntervalSec)
	}
	if loaded.BackgroundIntervalSec != 300 {
		t.Fatalf("BackgroundIntervalSec = %d, want 300", loaded.BackgroundIntervalSec)
	}
}

func TestLoadSettingsInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	path := filepath.Join(dir, "codex-quota", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	if _, err := LoadSettings(); err == nil {
		t.Fatalf("LoadSettings() error = nil, want non-nil")
	}
}
