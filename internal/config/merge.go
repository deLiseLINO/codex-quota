package config

import (
	"strings"
	"time"
)

func dedupeAccounts(input []*Account) []*Account {
	byKey := make(map[string]*Account)

	for _, account := range input {
		if account == nil {
			continue
		}

		key := dedupeKey(account)
		if existing, ok := byKey[key]; ok {
			byKey[key] = mergeAccounts(existing, account)
			continue
		}
		byKey[key] = account
	}

	output := make([]*Account, 0, len(byKey))
	for _, account := range byKey {
		output = append(output, account)
	}

	return output
}

func dedupeKey(account *Account) string {
	if account.AccountID != "" {
		return "account:" + account.AccountID
	}
	if email := normalizeEmail(account.Email); email != "" {
		return "email:" + email
	}
	if account.RefreshToken != "" {
		return "refresh:" + account.RefreshToken
	}
	return "file:" + account.FilePath
}

func mergeAccounts(left, right *Account) *Account {
	primary := left
	secondary := right

	if accountPriority(secondary) > accountPriority(primary) {
		primary, secondary = secondary, primary
	}

	merged := *primary
	if merged.Email == "" {
		merged.Email = secondary.Email
	}
	if merged.Label == "" {
		merged.Label = secondary.Label
	}
	if merged.AccountID == "" {
		merged.AccountID = secondary.AccountID
	}
	merged.AccountID = CanonicalAccountID(merged.AccountID, secondary.AccountID)
	if merged.ClientID == "" {
		merged.ClientID = secondary.ClientID
	}
	if merged.RefreshToken == "" {
		merged.RefreshToken = secondary.RefreshToken
	}
	merged.AccessToken, merged.ExpiresAt = chooseTokenState(primary, secondary)
	if !merged.Writable && secondary.Writable {
		merged.Writable = true
		merged.Source = secondary.Source
		merged.FilePath = secondary.FilePath
	}

	return &merged
}

func accountPriority(account *Account) int {
	score := 0
	if account.Writable {
		score += 100
	}

	switch account.Source {
	case SourceManaged:
		score += 70
	case SourceOpenCode:
		score += 60
	case SourcePi:
		score += 55
	case SourceCodex:
		score += 50
	}

	if account.RefreshToken != "" {
		score += 20
	}
	if account.AccessToken != "" {
		score += 10
	}
	if !account.ExpiresAt.IsZero() {
		score += 5
	}

	return score
}

func chooseTokenState(primary, secondary *Account) (string, time.Time) {
	if primary == nil {
		if secondary == nil {
			return "", time.Time{}
		}
		return secondary.AccessToken, secondary.ExpiresAt
	}
	if secondary == nil {
		return primary.AccessToken, primary.ExpiresAt
	}

	primaryToken := strings.TrimSpace(primary.AccessToken)
	secondaryToken := strings.TrimSpace(secondary.AccessToken)

	if primaryToken == "" {
		return secondary.AccessToken, secondary.ExpiresAt
	}
	if secondaryToken == "" {
		return primary.AccessToken, primary.ExpiresAt
	}

	if primaryToken == secondaryToken {
		if !secondary.ExpiresAt.IsZero() && (primary.ExpiresAt.IsZero() || secondary.ExpiresAt.After(primary.ExpiresAt)) {
			return secondary.AccessToken, secondary.ExpiresAt
		}
		return primary.AccessToken, primary.ExpiresAt
	}

	if primary.ExpiresAt.IsZero() && !secondary.ExpiresAt.IsZero() {
		return secondary.AccessToken, secondary.ExpiresAt
	}
	if !secondary.ExpiresAt.IsZero() && secondary.ExpiresAt.After(primary.ExpiresAt) {
		return secondary.AccessToken, secondary.ExpiresAt
	}

	return primary.AccessToken, primary.ExpiresAt
}
