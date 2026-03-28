package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func LoadAllAccountsWithSources() (AccountsLoadResult, error) {
	appAccounts, err := LoadManagedAccounts()
	if err != nil {
		return AccountsLoadResult{}, err
	}
	externalAccounts := make([]*Account, 0, 4)

	opencodePaths := opencodeAuthPaths()
	writable := firstExistingPath(opencodePaths)
	if writable == "" && len(opencodePaths) > 0 {
		writable = opencodePaths[0]
	}

	for _, path := range opencodePaths {
		openCodeMain, err := loadOpenCodeAccountFile(path, SourceOpenCode, path == writable)
		if err != nil {
			return AccountsLoadResult{}, err
		}
		if openCodeMain != nil {
			externalAccounts = append(externalAccounts, openCodeMain)
		}
	}

	codexAccount, err := loadCodexAccountFile(codexAuthPath())
	if err != nil {
		return AccountsLoadResult{}, err
	}
	if codexAccount != nil {
		externalAccounts = append(externalAccounts, codexAccount)
	}

	activeOpenCodeAccount, err := loadOpenCodeAccountFile(opencodeAuthPath(), SourceOpenCode, true)
	if err != nil {
		return AccountsLoadResult{}, err
	}

	if syncExternalAccountsToManaged(appAccounts, externalAccounts) {
		refreshedManaged, reloadErr := LoadManagedAccounts()
		if reloadErr == nil {
			appAccounts = refreshedManaged
		}
	}

	accounts := make([]*Account, 0, len(appAccounts)+len(externalAccounts))
	accounts = append(accounts, appAccounts...)
	accounts = append(accounts, externalAccounts...)

	sourcesByAccountID := make(map[string][]string)
	for _, account := range accounts {
		if account == nil {
			continue
		}
		if account.AccountID != "" {
			sourcesByAccountID[account.AccountID] = appendUniqueString(sourcesByAccountID[account.AccountID], account.SourceLabel())
		}
		if email := normalizeEmail(account.Email); email != "" {
			emailKey := "email:" + email
			sourcesByAccountID[emailKey] = appendUniqueString(sourcesByAccountID[emailKey], account.SourceLabel())
		}
	}

	activeSourcesByIdentity := make(map[string][]string)
	appendActiveSource(activeSourcesByIdentity, codexAccount, SourceCodex)
	appendActiveSource(activeSourcesByIdentity, activeOpenCodeAccount, SourceOpenCode)

	accounts = dedupeAccounts(accounts)
	for _, account := range accounts {
		finalizeAccount(account)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return strings.ToLower(accounts[i].Label) < strings.ToLower(accounts[j].Label)
	})

	return AccountsLoadResult{
		Accounts:                accounts,
		SourcesByAccountID:      sourcesByAccountID,
		ActiveSourcesByIdentity: activeSourcesByIdentity,
	}, nil
}

func appendActiveSource(target map[string][]string, account *Account, source Source) {
	if target == nil || account == nil {
		return
	}
	if source != SourceCodex && source != SourceOpenCode {
		return
	}

	sourceLabel := string(source)
	for _, key := range ActiveIdentityKeys(account) {
		target[key] = appendUniqueString(target[key], sourceLabel)
	}
}

func syncExternalAccountsToManaged(managedAccounts []*Account, externalAccounts []*Account) bool {
	candidates := externalImportCandidates(externalAccounts)
	if len(candidates) == 0 {
		return false
	}

	managedByIdentity := make(map[string]*Account, len(managedAccounts))
	for _, account := range managedAccounts {
		if account == nil {
			continue
		}
		for _, key := range accountIdentityKeys(account) {
			managedByIdentity[key] = account
		}
	}

	updated := false
	for _, candidate := range candidates {
		imported := cloneAsManaged(candidate)
		if imported == nil {
			continue
		}

		existing := findManagedByIdentity(managedByIdentity, imported)
		if !needsManagedUpdate(existing, imported) {
			continue
		}

		if err := UpsertManagedAccount(imported); err != nil {
			continue
		}

		merged := imported
		if existing != nil {
			merged = mergeAccounts(existing, imported)
		}
		for _, key := range accountIdentityKeys(merged) {
			managedByIdentity[key] = merged
		}
		updated = true
	}

	return updated
}

func externalImportCandidates(externalAccounts []*Account) []*Account {
	filtered := make([]*Account, 0, len(externalAccounts))
	for _, account := range externalAccounts {
		if account == nil {
			continue
		}
		if strings.TrimSpace(account.AccessToken) == "" {
			continue
		}
		if strings.TrimSpace(account.AccountID) == "" {
			continue
		}
		filtered = append(filtered, account)
	}
	return dedupeAccounts(filtered)
}

func cloneAsManaged(account *Account) *Account {
	if account == nil {
		return nil
	}
	accountID := strings.TrimSpace(account.AccountID)
	if accountID == "" {
		return nil
	}
	accessToken := strings.TrimSpace(account.AccessToken)
	if accessToken == "" {
		return nil
	}

	return &Account{
		Label:        strings.TrimSpace(account.Label),
		Email:        strings.TrimSpace(account.Email),
		AccountID:    CanonicalAccountID(accountID),
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(account.RefreshToken),
		ExpiresAt:    account.ExpiresAt,
		ClientID:     strings.TrimSpace(account.ClientID),
		Source:       SourceManaged,
		Writable:     true,
	}
}

func needsManagedUpdate(existing *Account, incoming *Account) bool {
	if incoming == nil {
		return false
	}
	if existing == nil {
		return true
	}

	merged := mergeAccounts(existing, incoming)
	if merged == nil {
		return false
	}

	if strings.TrimSpace(existing.AccessToken) != strings.TrimSpace(merged.AccessToken) {
		return true
	}
	if strings.TrimSpace(existing.RefreshToken) != strings.TrimSpace(merged.RefreshToken) {
		return true
	}
	if strings.TrimSpace(existing.ClientID) != strings.TrimSpace(merged.ClientID) {
		return true
	}
	if strings.TrimSpace(existing.Email) != strings.TrimSpace(merged.Email) {
		return true
	}
	if strings.TrimSpace(existing.Label) != strings.TrimSpace(merged.Label) {
		return true
	}
	if !existing.ExpiresAt.Equal(merged.ExpiresAt) {
		return true
	}

	return false
}

func loadOpenCodeAccountFile(path string, source Source, writable bool) (*Account, error) {
	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	openai := asMap(root["openai"])
	if openai == nil {
		return nil, nil
	}

	account := buildOpenAIAccount(openai, source, path, writable)
	if account == nil {
		return nil, nil
	}

	return account, nil
}

func buildOpenAIAccount(openai map[string]any, source Source, path string, writable bool) *Account {
	accessToken := strings.TrimSpace(asString(openai["access"]))
	if accessToken == "" {
		return nil
	}

	account := &Account{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(asString(openai["refresh"])),
		AccountID:    strings.TrimSpace(asString(openai["accountId"])),
		Email:        strings.TrimSpace(asString(openai["email"])),
		Source:       source,
		FilePath:     path,
		Writable:     writable,
	}

	if expiresMillis, ok := asInt64(openai["expires"]); ok && expiresMillis > 0 {
		account.ExpiresAt = time.UnixMilli(expiresMillis)
	}

	claims := ParseAccessToken(accessToken)
	account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
	if account.ClientID == "" {
		account.ClientID = claims.ClientID
	}
	if account.ExpiresAt.IsZero() {
		account.ExpiresAt = claims.ExpiresAt
	}
	if account.Email == "" {
		account.Email = claims.Email
	}

	return account
}

func loadCodexAccountFile(path string) (*Account, error) {
	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	tokens := asMap(root["tokens"])
	if tokens == nil {
		return nil, nil
	}

	accessToken := strings.TrimSpace(asString(tokens["access_token"]))
	if accessToken == "" {
		return nil, nil
	}

	account := &Account{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(asString(tokens["refresh_token"])),
		AccountID:    strings.TrimSpace(asString(tokens["account_id"])),
		Source:       SourceCodex,
		FilePath:     path,
		Writable:     true,
	}

	claims := ParseAccessToken(accessToken)
	account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
	account.ClientID = claims.ClientID
	account.ExpiresAt = claims.ExpiresAt

	return account, nil
}

func saveOpenCodeAccount(account *Account) error {
	root, err := readJSONMap(account.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", account.FilePath, err)
	}

	openai := asMap(root["openai"])
	if openai == nil {
		openai = make(map[string]any)
		root["openai"] = openai
	}

	openai["access"] = account.AccessToken
	if account.RefreshToken != "" {
		openai["refresh"] = account.RefreshToken
	}
	if account.AccountID != "" {
		openai["accountId"] = account.AccountID
	}
	if !account.ExpiresAt.IsZero() {
		openai["expires"] = account.ExpiresAt.UnixMilli()
	}

	return writeJSONMap(account.FilePath, root)
}

func saveCodexAccount(account *Account) error {
	root, err := readJSONMap(account.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", account.FilePath, err)
	}

	tokens := asMap(root["tokens"])
	if tokens == nil {
		tokens = make(map[string]any)
		root["tokens"] = tokens
	}

	tokens["access_token"] = account.AccessToken
	tokens["id_token"] = account.AccessToken
	if account.RefreshToken != "" {
		tokens["refresh_token"] = account.RefreshToken
	}
	if account.AccountID != "" {
		tokens["account_id"] = account.AccountID
	}

	root["last_refresh"] = time.Now().UTC().Format(time.RFC3339)

	return writeJSONMap(account.FilePath, root)
}

func finalizeAccount(account *Account) {
	if account == nil {
		return
	}

	if shouldReplaceLabelWithEmail(account) {
		account.Label = account.Email
	}

	if account.Label == "" {
		if account.Email != "" {
			account.Label = account.Email
		} else if account.AccountID != "" {
			account.Label = shortAccountID(account.AccountID)
		} else {
			account.Label = account.SourceLabel()
		}
	}

	if account.Key == "" {
		if account.AccountID != "" {
			account.Key = account.AccountID
		} else {
			account.Key = fmt.Sprintf("%s:%s", account.Source, filepath.Base(account.FilePath))
		}
	}
}

func shouldReplaceLabelWithEmail(account *Account) bool {
	if account == nil {
		return false
	}
	email := strings.TrimSpace(account.Email)
	if email == "" {
		return false
	}
	label := strings.TrimSpace(account.Label)
	if label == "" {
		return true
	}
	if label == account.SourceLabel() {
		return true
	}
	if strings.EqualFold(label, "n/a") {
		return true
	}
	if accountID := strings.TrimSpace(account.AccountID); accountID != "" && label == shortAccountID(accountID) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(label), "auth0|") {
		return true
	}
	return false
}

func accountIdentityKeys(account *Account) []string {
	if account == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if email := normalizeEmail(account.Email); email != "" {
		keys = append(keys, "email:"+email)
	}
	if accountID := strings.TrimSpace(account.AccountID); accountID != "" {
		keys = append(keys, "account:"+accountID)
	}
	return keys
}

func findManagedByIdentity(index map[string]*Account, account *Account) *Account {
	for _, key := range accountIdentityKeys(account) {
		if current, ok := index[key]; ok {
			return current
		}
	}
	return nil
}

func appendUniqueString(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func readJSONMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	root := make(map[string]any)
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	return root, nil
}

func writeJSONMap(path string, root map[string]any) error {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", path, err)
	}

	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	cleanup = false
	return nil
}
