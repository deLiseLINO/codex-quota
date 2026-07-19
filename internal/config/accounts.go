package config

import (
	"fmt"
	"strings"
	"time"
)

type Source string

const (
	SourceManaged  Source = "managed"
	SourceOpenCode Source = "opencode"
	SourceCodex    Source = "codex"
)

type Account struct {
	Key          string
	Label        string
	Email        string
	AccountID    string
	UserID       string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	ClientID     string
	Source       Source
	FilePath     string
	Writable     bool
}

type AccessTokenClaims struct {
	ClientID  string
	AccountID string
	UserID    string
	ExpiresAt time.Time
	Email     string
}

// identityID is the per-user identity key for an account. AccountID is the
// ChatGPT workspace UUID, which is shared by every member of a Team/Business
// org; UserID (from chatgpt_user_id / sub) distinguishes users within that
// workspace. Callers that need to tell two users apart must use this, not the
// bare AccountID. External consumers (the ChatGPT-Account-Id header and the
// account_id written into Codex/opencode auth files) must keep using AccountID.
func identityID(a *Account) string {
	if a == nil {
		return ""
	}
	accountID := strings.TrimSpace(a.AccountID)
	userID := strings.TrimSpace(a.UserID)
	if accountID != "" && userID != "" {
		return accountID + " " + userID
	}
	if accountID != "" {
		return accountID
	}
	return userID
}

// IdentityKey returns the per-user runtime key for an account, matching the
// Account.Key assigned during load. Callers outside this package (the UI) use
// it to reference the active/selected account instead of the bare AccountID,
// which collides for co-workspace users.
func IdentityKey(a *Account) string {
	return identityID(a)
}

type AccountsLoadResult struct {
	Accounts                []*Account
	SourcesByAccountID      map[string][]string
	ActiveSourcesByIdentity map[string][]string
}

func (a *Account) SourceLabel() string {
	switch a.Source {
	case SourceManaged:
		return "app"
	case SourceOpenCode:
		return "opencode"
	case SourceCodex:
		return "codex"
	default:
		return "unknown"
	}
}

func LoadAllAccounts() ([]*Account, error) {
	result, err := LoadAllAccountsWithSources()
	if err != nil {
		return nil, err
	}
	return result.Accounts, nil
}

func SaveAccount(account *Account) error {
	if account == nil || !account.Writable {
		return nil
	}

	switch account.Source {
	case SourceManaged:
		return saveManagedAccount(account)
	case SourceOpenCode:
		if account.FilePath == "" {
			return nil
		}
		return saveOpenCodeAccount(account)
	case SourceCodex:
		if account.FilePath == "" {
			return nil
		}
		return saveCodexAccount(account)
	default:
		return nil
	}
}

func ApplyAccountToTarget(account *Account, target Source) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	accountToApply := account
	if fresh, _, err := ResolveFreshAccount(account); err != nil {
		return "", err
	} else if fresh != nil {
		accountToApply = fresh
	}

	switch target {
	case SourceOpenCode:
		return ApplyAccountToOpenCode(accountToApply)
	case SourceCodex:
		return ApplyAccountToCodex(accountToApply)
	default:
		return "", fmt.Errorf("unsupported apply target: %s", target)
	}
}

func ApplyAccountToTargets(account *Account, targets []Source) (map[Source]string, map[Source]error) {
	paths := make(map[Source]string)
	errorsBySource := make(map[Source]error)

	if account == nil {
		errorsBySource[SourceCodex] = fmt.Errorf("account is nil")
		return paths, errorsBySource
	}

	seen := make(map[Source]bool, len(targets))
	for _, target := range targets {
		if target != SourceCodex && target != SourceOpenCode {
			continue
		}
		if seen[target] {
			continue
		}
		seen[target] = true

		path, err := ApplyAccountToTarget(account, target)
		if err != nil {
			errorsBySource[target] = err
			continue
		}
		paths[target] = path
	}

	return paths, errorsBySource
}

func DeleteAccountFromSource(account *Account, source Source) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}

	switch source {
	case SourceManaged:
		return DeleteManagedAccountByIdentity(account)
	case SourceOpenCode:
		return DeleteOpenCodeAuthAccount()
	case SourceCodex:
		return DeleteCodexAuthAccount()
	default:
		return fmt.Errorf("unsupported source: %s", source)
	}
}
