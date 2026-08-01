package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ActiveIntervalMinSec     = 15
	ActiveIntervalMaxSec     = 600
	BackgroundIntervalMinSec = 60
	BackgroundIntervalMaxSec = 3600
)

type Settings struct {
	CheckForUpdateOnStartup bool `json:"check_for_update_on_startup"`
	AutoRefreshEnabled      bool `json:"auto_refresh_enabled"`
	ActiveIntervalSec       int  `json:"active_interval_sec"`
	BackgroundIntervalSec   int  `json:"background_interval_sec"`
}

func DefaultSettings() Settings {
	return Settings{
		CheckForUpdateOnStartup: true,
		AutoRefreshEnabled:      true,
		ActiveIntervalSec:       30,
		BackgroundIntervalSec:   300,
	}
}

func LoadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return DefaultSettings(), err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return DefaultSettings(), fmt.Errorf("failed to read %s: %w", path, err)
	}

	settings := DefaultSettings()
	if check, ok := root["check_for_update_on_startup"].(bool); ok {
		settings.CheckForUpdateOnStartup = check
	}
	if enabled, ok := root["auto_refresh_enabled"].(bool); ok {
		settings.AutoRefreshEnabled = enabled
	}
	if raw, ok := asInt64(root["active_interval_sec"]); ok {
		settings.ActiveIntervalSec = ClampInt(int(raw), ActiveIntervalMinSec, ActiveIntervalMaxSec)
	}
	if raw, ok := asInt64(root["background_interval_sec"]); ok {
		settings.BackgroundIntervalSec = ClampInt(int(raw), BackgroundIntervalMinSec, BackgroundIntervalMaxSec)
	}

	return settings, nil
}

func SaveSettings(settings Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}

	root := map[string]any{
		"check_for_update_on_startup": settings.CheckForUpdateOnStartup,
		"auto_refresh_enabled":        settings.AutoRefreshEnabled,
		"active_interval_sec":         settings.ActiveIntervalSec,
		"background_interval_sec":     settings.BackgroundIntervalSec,
	}
	if err := writeJSONMap(path, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

func settingsPath() (string, error) {
	dir, err := codexQuotaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
