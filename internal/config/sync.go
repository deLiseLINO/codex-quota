package config

import "strings"

func ResolveFreshAccount(account *Account) (*Account, bool, error) {
	if account == nil {
		return nil, false, nil
	}

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		return nil, false, err
	}

	fresh := freshestAccountForIdentity(account, result.Accounts)
	if fresh == nil {
		return cloneAccountForSync(account), false, nil
	}

	changed := strings.TrimSpace(fresh.AccessToken) != strings.TrimSpace(account.AccessToken) ||
		strings.TrimSpace(fresh.RefreshToken) != strings.TrimSpace(account.RefreshToken) ||
		strings.TrimSpace(fresh.IDToken) != strings.TrimSpace(account.IDToken) ||
		!fresh.ExpiresAt.Equal(account.ExpiresAt)

	return cloneAccountForSync(fresh), changed, nil
}

func freshestAccountForIdentity(seed *Account, candidates []*Account) *Account {
	var best *Account
	for _, candidate := range candidates {
		if !sameIdentity(seed, candidate) {
			continue
		}
		if strings.TrimSpace(candidate.AccessToken) == "" {
			continue
		}
		if best == nil || tokenBundleRank(candidate) > tokenBundleRank(best) {
			best = candidate
		}
	}
	if best == nil && seed != nil && strings.TrimSpace(seed.AccessToken) != "" {
		best = seed
	}
	return best
}

func sameIdentity(left, right *Account) bool {
	if left == nil || right == nil {
		return false
	}
	leftID := strings.TrimSpace(left.AccountID)
	rightID := strings.TrimSpace(right.AccountID)
	if leftID != "" && rightID != "" {
		return leftID == rightID
	}
	leftEmail := normalizeEmail(left.Email)
	rightEmail := normalizeEmail(right.Email)
	if leftEmail != "" && rightEmail != "" {
		return leftEmail == rightEmail
	}
	leftRefresh := strings.TrimSpace(left.RefreshToken)
	rightRefresh := strings.TrimSpace(right.RefreshToken)
	return leftRefresh != "" && leftRefresh == rightRefresh
}

func tokenBundleRank(account *Account) int64 {
	if account == nil {
		return 0
	}
	rank := int64(0)
	if !account.ExpiresAt.IsZero() {
		rank += account.ExpiresAt.UnixMilli() * 10
	}
	if account.Source != SourceManaged {
		rank += 2
	}
	if strings.TrimSpace(account.RefreshToken) != "" {
		rank += 1
	}
	return rank
}

func cloneAccountForSync(account *Account) *Account {
	if account == nil {
		return nil
	}
	clone := *account
	return &clone
}

func SyncAccountEverywhere(account *Account) error {
	if account == nil {
		return nil
	}
	fresh := cloneAccountForSync(account)
	if fresh == nil || strings.TrimSpace(fresh.AccessToken) == "" {
		return nil
	}

	if strings.TrimSpace(fresh.AccountID) == "" {
		claims := ParseAccessToken(fresh.AccessToken)
		fresh.AccountID = CanonicalAccountID(fresh.AccountID, claims.AccountID)
		if fresh.ClientID == "" {
			fresh.ClientID = claims.ClientID
		}
		if fresh.Email == "" {
			fresh.Email = claims.Email
		}
		if fresh.ExpiresAt.IsZero() {
			fresh.ExpiresAt = claims.ExpiresAt
		}
	}

	if err := UpsertManagedAccount(fresh); err != nil {
		return err
	}
	if _, err := applyAccountToCodex(fresh, targetWriteRefresh); err != nil {
		return err
	}
	if _, err := applyAccountToOpenCode(fresh, targetWriteRefresh); err != nil {
		return err
	}
	if _, err := applyAccountToPi(fresh, targetWriteRefresh); err != nil {
		return err
	}
	if _, err := applyAccountToOMP(fresh, targetWriteRefresh); err != nil {
		return err
	}
	return nil
}

type targetWriteMode int

const (
	targetWriteApply targetWriteMode = iota
	targetWriteRefresh
)

func chooseTargetWriteAccount(incoming, existing *Account, mode targetWriteMode) *Account {
	if incoming == nil {
		return nil
	}
	if mode == targetWriteRefresh {
		if existing == nil || !sameIdentity(incoming, existing) {
			return nil
		}
		return incoming
	}
	if existing != nil && !sameIdentity(incoming, existing) {
		return incoming
	}
	if fresh := freshestAccountForIdentity(incoming, []*Account{incoming, existing}); fresh != nil {
		return fresh
	}
	return incoming
}
