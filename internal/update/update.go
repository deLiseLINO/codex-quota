package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

type Method string

const (
	MethodUnknown Method = "unknown"
	MethodBrew    Method = "brew"
	MethodGo      Method = "go"
)

const (
	refreshInterval       = 5 * time.Minute
	latestReleaseURL      = "https://api.github.com/repos/deLiseLINO/codex-quota/releases/latest"
	homebrewFormulaURL    = "https://raw.githubusercontent.com/deLiseLINO/homebrew-tap/main/Formula/codex-quota.rb"
	releasesPageURL       = "https://github.com/deLiseLINO/codex-quota/releases"
	goInstallTarget       = "github.com/deLiseLINO/codex-quota/cmd/cq@latest"
	goInstallTargetPrefix = "github.com/deLiseLINO/codex-quota/cmd/cq@"
	homebrewUpgradeTarget = "deLiseLINO/tap/codex-quota"
)

var formulaVersionPattern = regexp.MustCompile(`(?m)^\s*version\s+"([^"]+)"`)

func DetectMethod() Method {
	exePath, err := os.Executable()
	if err != nil {
		return MethodUnknown
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	home, _ := os.UserHomeDir()
	return DetectMethodFromInputs(
		exePath,
		os.Getenv("GOBIN"),
		os.Getenv("GOPATH"),
		home,
	)
}

func DetectMethodFromInputs(exePath, gobin, gopath, home string) Method {
	exePath = filepath.Clean(strings.TrimSpace(exePath))
	if exePath == "." || exePath == "" {
		return MethodUnknown
	}

	exeDir := filepath.Dir(exePath)
	if goBinDir := resolveGoBinDir(gobin, gopath, home); goBinDir != "" && exeDir == goBinDir {
		return MethodGo
	}

	for _, prefix := range []string{
		"/opt/homebrew",
		"/usr/local",
		"/home/linuxbrew/.linuxbrew",
	} {
		if hasPathPrefix(exePath, prefix) {
			return MethodBrew
		}
	}

	return MethodUnknown
}

func resolveGoBinDir(gobin, gopath, home string) string {
	if gobin = strings.TrimSpace(gobin); gobin != "" {
		return filepath.Clean(gobin)
	}

	if gopath = strings.TrimSpace(gopath); gopath != "" {
		for _, entry := range filepath.SplitList(gopath) {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				return filepath.Join(filepath.Clean(entry), "bin")
			}
		}
	}

	if strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(filepath.Clean(home), "go", "bin")
}

func hasPathPrefix(path, prefix string) bool {
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	return path == prefix || strings.HasPrefix(path, prefix+string(filepath.Separator))
}

func SupportsAutoUpdate(method Method) bool {
	return method == MethodBrew || method == MethodGo
}

func ShouldRefresh(state config.UpdateState, now time.Time) bool {
	return state.LastCheckedAt.IsZero() || state.LastCheckedAt.Before(now.Add(-refreshInterval))
}

func ShouldPrompt(settings config.Settings, state config.UpdateState, currentVersion string, method Method) (string, bool) {
	if !settings.CheckForUpdateOnStartup || !SupportsAutoUpdate(method) {
		return "", false
	}

	latest := strings.TrimSpace(state.LatestVersion)
	if latest == "" || !IsNewer(latest, currentVersion) {
		return "", false
	}

	if strings.TrimSpace(state.DismissedVersion) == latest {
		return "", false
	}

	return latest, true
}

func RefreshState(ctx context.Context, state config.UpdateState, method Method) (config.UpdateState, error) {
	latest, err := FetchLatestVersion(ctx, method)
	if err != nil {
		return state, err
	}

	state.LatestVersion = latest
	state.LastCheckedAt = time.Now().UTC()
	return state, nil
}

func FetchLatestVersion(ctx context.Context, method Method) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	if method == MethodBrew {
		if version, err := fetchLatestBrewVersion(ctx, client); err == nil {
			return version, nil
		}
	}

	return fetchLatestReleaseVersion(ctx, client)
}

func fetchLatestBrewVersion(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, homebrewFormulaURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "codex-quota/update-check")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("homebrew formula request failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	matches := formulaVersionPattern.FindStringSubmatch(string(body))
	if len(matches) != 2 {
		return "", fmt.Errorf("failed to parse homebrew formula version")
	}

	return strings.TrimSpace(matches[1]), nil
}

func fetchLatestReleaseVersion(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "codex-quota/update-check")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release request failed: %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	version := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if version == "" {
		return "", fmt.Errorf("latest release tag is empty")
	}

	return version, nil
}

func IsNewer(latest, current string) bool {
	latestVersion, ok := parseVersion(latest)
	if !ok {
		return false
	}
	currentVersion, ok := parseVersion(current)
	if !ok {
		return false
	}
	for idx := range latestVersion {
		if latestVersion[idx] > currentVersion[idx] {
			return true
		}
		if latestVersion[idx] < currentVersion[idx] {
			return false
		}
	}
	return false
}

func parseVersion(raw string) ([3]uint64, bool) {
	var version [3]uint64
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(raw), "v"), ".")
	if len(parts) != 3 {
		return version, false
	}

	for idx, part := range parts {
		if part == "" {
			return version, false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return version, false
			}
		}
		var value uint64
		for _, r := range part {
			value = value*10 + uint64(r-'0')
		}
		version[idx] = value
	}

	return version, true
}

func Command(method Method, latestVersion string) (string, []string, bool) {
	switch method {
	case MethodBrew:
		return "brew", []string{"upgrade", homebrewUpgradeTarget}, true
	case MethodGo:
		return "go", []string{"install", goInstallTargetForVersion(latestVersion)}, true
	default:
		return "", nil, false
	}
}

func CommandString(method Method, latestVersion string) string {
	command, args, ok := Command(method, latestVersion)
	if !ok {
		return ""
	}
	return strings.Join(append([]string{command}, args...), " ")
}

func ReleaseNotesURL(version string) string {
	if _, ok := parseVersion(version); !ok {
		return releasesPageURL + "/latest"
	}
	return releasesPageURL + "/tag/v" + strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func RunUpgrade(method Method, latestVersion string, stdout, stderr io.Writer) error {
	command, args, ok := Command(method, latestVersion)
	if !ok {
		return fmt.Errorf("unsupported update method: %s", method)
	}

	fmt.Fprintf(stdout, "Updating cq via `%s`...\n", CommandString(method, latestVersion))
	cmd := exec.Command(command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "Update completed. Restart cq.")
	return nil
}

func goInstallTargetForVersion(version string) string {
	if _, ok := parseVersion(version); !ok {
		return goInstallTarget
	}
	return goInstallTargetPrefix + "v" + strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func ManualUpgradeInstructions(currentVersion, latestVersion string) string {
	lines := []string{
		fmt.Sprintf("Current version: %s", strings.TrimSpace(currentVersion)),
	}
	if latestVersion = strings.TrimSpace(latestVersion); latestVersion != "" {
		lines = append(lines, fmt.Sprintf("Latest version: %s", latestVersion))
	}
	lines = append(lines,
		"Automatic upgrade is unavailable because the installation method could not be determined.",
		"Manual update options:",
		fmt.Sprintf("  brew upgrade %s", homebrewUpgradeTarget),
		fmt.Sprintf("  go install %s", goInstallTargetForVersion(latestVersion)),
		fmt.Sprintf("  Releases: %s", ReleaseNotesURL(latestVersion)),
	)
	return strings.Join(lines, "\n")
}
