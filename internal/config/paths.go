package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func opencodeAuthPath() string {
	paths := opencodeAuthPaths()
	if len(paths) == 0 {
		return ""
	}

	if path := firstExistingPath(paths); path != "" {
		return path
	}

	return paths[0]
}

func opencodeAuthPaths() []string {
	paths := make([]string, 0, 4)

	if path := cleanPath(os.Getenv("OPENCODE_AUTH_PATH")); path != "" {
		paths = append(paths, path)
	}

	for _, dir := range opencodeDirsForScan() {
		paths = append(paths, filepath.Join(dir, "auth.json"))
	}

	return uniqueStrings(paths)
}

func opencodeDirsForScan() []string {
	dirs := make([]string, 0, 4)

	if dir := cleanPath(os.Getenv("OPENCODE_DATA_DIR")); dir != "" {
		dirs = append(dirs, dir)
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local", "share", "opencode"),
			filepath.Join(home, ".config", "opencode"),
			filepath.Join(home, "Library", "Application Support", "opencode"),
			filepath.Join(home, ".opencode"),
		)
	}

	return uniqueStrings(dirs)
}

func codexAuthPath() string {
	if path := cleanPath(os.Getenv("CODEX_AUTH_PATH")); path != "" {
		return path
	}

	if dir := cleanPath(os.Getenv("CODEX_HOME")); dir != "" {
		return filepath.Join(dir, "auth.json")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".codex", "auth.json")
}

func piAuthPath() string {
	if dir := cleanPath(os.Getenv("PI_CODING_AGENT_DIR")); dir != "" {
		return filepath.Join(dir, "auth.json")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".pi", "agent", "auth.json")
}

func firstExistingPath(paths []string) string {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			rest := strings.TrimPrefix(path, "~")
			rest = strings.TrimLeft(rest, "/\\")
			if rest == "" {
				path = home
			} else {
				path = filepath.Join(home, rest)
			}
		}
	}
	return filepath.Clean(path)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func codexQuotaConfigDir() (string, error) {
	base := strings.TrimSpace(os.Getenv("CQ_CONFIG_HOME"))
	if base == "" {
		base = strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	}

	if base == "" {
		userConfigDir, err := os.UserConfigDir()
		if err == nil && strings.TrimSpace(userConfigDir) != "" {
			base = userConfigDir
		}
	}

	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to locate user config dir")
		}
		base = filepath.Join(home, ".config")
	}

	dir := filepath.Join(base, "codex-quota")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	return dir, nil
}
