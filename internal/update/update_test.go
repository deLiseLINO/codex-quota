package update

import (
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestDetectMethodFromInputs(t *testing.T) {
	tests := []struct {
		name    string
		exePath string
		gobin   string
		gopath  string
		home    string
		want    Method
	}{
		{
			name:    "go via gobin",
			exePath: "/Users/test/bin/cq",
			gobin:   "/Users/test/bin",
			home:    "/Users/test",
			want:    MethodGo,
		},
		{
			name:    "go via default gopath",
			exePath: "/Users/test/go/bin/cq",
			home:    "/Users/test",
			want:    MethodGo,
		},
		{
			name:    "brew via apple silicon",
			exePath: "/opt/homebrew/bin/cq",
			home:    "/Users/test",
			want:    MethodBrew,
		},
		{
			name:    "brew via linuxbrew",
			exePath: "/home/linuxbrew/.linuxbrew/bin/cq",
			home:    "/home/test",
			want:    MethodBrew,
		},
		{
			name:    "unknown manual binary",
			exePath: "/tmp/cq",
			home:    "/Users/test",
			want:    MethodUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMethodFromInputs(tc.exePath, tc.gobin, tc.gopath, tc.home); got != tc.want {
				t.Fatalf("DetectMethodFromInputs() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestShouldRefresh(t *testing.T) {
	now := time.Date(2026, time.March, 7, 15, 0, 0, 0, time.UTC)

	if !ShouldRefresh(config.UpdateState{}, now) {
		t.Fatalf("ShouldRefresh(zero) = false, want true")
	}

	fresh := config.UpdateState{LastCheckedAt: now.Add(-2 * time.Minute)}
	if ShouldRefresh(fresh, now) {
		t.Fatalf("ShouldRefresh(fresh) = true, want false")
	}

	stale := config.UpdateState{LastCheckedAt: now.Add(-6 * time.Minute)}
	if !ShouldRefresh(stale, now) {
		t.Fatalf("ShouldRefresh(stale) = false, want true")
	}
}

func TestShouldPrompt(t *testing.T) {
	settings := config.Settings{CheckForUpdateOnStartup: true}
	state := config.UpdateState{LatestVersion: "0.1.5"}

	if latest, ok := ShouldPrompt(settings, state, "0.1.4", MethodGo); !ok || latest != "0.1.5" {
		t.Fatalf("ShouldPrompt() = (%q, %v), want (%q, true)", latest, ok, "0.1.5")
	}

	state.DismissedVersion = "0.1.5"
	if latest, ok := ShouldPrompt(settings, state, "0.1.4", MethodGo); ok || latest != "" {
		t.Fatalf("ShouldPrompt() with dismissed version = (%q, %v), want empty/false", latest, ok)
	}

	state.DismissedVersion = ""
	if latest, ok := ShouldPrompt(settings, state, "0.1.4", MethodUnknown); ok || latest != "" {
		t.Fatalf("ShouldPrompt() for unknown method = (%q, %v), want empty/false", latest, ok)
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("0.1.5", "0.1.4") {
		t.Fatalf("IsNewer() = false, want true")
	}
	if IsNewer("0.1.4", "0.1.5") {
		t.Fatalf("IsNewer() = true, want false")
	}
	if IsNewer("0.1.5-beta.1", "0.1.4") {
		t.Fatalf("IsNewer(prerelease) = true, want false")
	}
}

func TestCommand(t *testing.T) {
	command, args, ok := Command(MethodBrew, "0.3.3")
	if !ok || command != "brew" || len(args) != 2 || args[1] != homebrewUpgradeTarget {
		t.Fatalf("Command(MethodBrew) = (%q, %v, %v)", command, args, ok)
	}

	command, args, ok = Command(MethodGo, "0.3.3")
	if !ok || command != "go" || len(args) != 2 || args[1] != "github.com/deLiseLINO/codex-quota/cmd/cq@v0.3.3" {
		t.Fatalf("Command(MethodGo) = (%q, %v, %v)", command, args, ok)
	}

	command, args, ok = Command(MethodGo, "bad")
	if !ok || command != "go" || len(args) != 2 || args[1] != goInstallTarget {
		t.Fatalf("Command(MethodGo invalid version) = (%q, %v, %v)", command, args, ok)
	}

	if _, _, ok := Command(MethodUnknown, "0.3.3"); ok {
		t.Fatalf("Command(MethodUnknown) ok = true, want false")
	}
}

func TestCommandStringUsesExactGoVersion(t *testing.T) {
	got := CommandString(MethodGo, "0.3.3")
	want := "go install github.com/deLiseLINO/codex-quota/cmd/cq@v0.3.3"
	if got != want {
		t.Fatalf("CommandString(MethodGo) = %q, want %q", got, want)
	}
}

func TestReleaseNotesURL(t *testing.T) {
	got := ReleaseNotesURL("0.3.3")
	want := "https://github.com/deLiseLINO/codex-quota/releases/tag/v0.3.3"
	if got != want {
		t.Fatalf("ReleaseNotesURL(valid) = %q, want %q", got, want)
	}

	got = ReleaseNotesURL("bad")
	want = "https://github.com/deLiseLINO/codex-quota/releases/latest"
	if got != want {
		t.Fatalf("ReleaseNotesURL(invalid) = %q, want %q", got, want)
	}
}
