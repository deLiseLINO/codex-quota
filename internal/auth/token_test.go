package auth

import (
	"testing"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestCopyResolvedAccountUpdatesTokenFields(t *testing.T) {
	target := &config.Account{AccountID: "acc-1", AccessToken: "old-access", RefreshToken: "old-refresh"}
	fresh := &config.Account{AccountID: "acc-1", AccessToken: "fresh-access", RefreshToken: "fresh-refresh"}

	copyResolvedAccount(target, fresh)

	if target.AccessToken != "fresh-access" || target.RefreshToken != "fresh-refresh" {
		t.Fatalf("target not updated: %#v", target)
	}
}
