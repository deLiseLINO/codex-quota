package config

import (
	"fmt"
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
	AccessToken  string
	RefreshToken string
	IDToken      string
	ExpiresAt    time.Time
	ClientID     string
	Source       Source
	FilePath     string
	Writable     bool
}

type AccessTokenClaims struct {
	ClientID  string
	AccountID string
	ExpiresAt time.Time
	Email     string
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
