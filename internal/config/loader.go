package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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

	piPaths := piAuthPaths()
	piWritable := firstExistingPath(piPaths)
	if piWritable == "" && len(piPaths) > 0 {
		piWritable = piPaths[0]
	}
	for _, path := range piPaths {
		piMain, err := loadPiAccountFile(path, SourcePi, path == piWritable)
		if err != nil {
			return AccountsLoadResult{}, err
		}
		if piMain != nil {
			externalAccounts = append(externalAccounts, piMain)
		}
	}
	activePiAccount, err := loadPiAccountFile(piAuthPath(), SourcePi, true)
	if err != nil {
		return AccountsLoadResult{}, err
	}

	ompPaths := ompAgentDbPaths()
	for _, path := range ompPaths {
		ompAccounts, err := loadOMPAccounts(path)
		if err != nil {
			return AccountsLoadResult{}, err
		}
		if len(ompAccounts) > 0 {
			externalAccounts = append(externalAccounts, ompAccounts...)
		}
	}
	activeOMPAccounts, err := loadOMPAccounts(ompAgentDbPath())
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
	appendActiveSource(activeSourcesByIdentity, activePiAccount, SourcePi)
	for _, account := range activeOMPAccounts {
		appendActiveSource(activeSourcesByIdentity, account, SourceOMP)
	}
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
	if source != SourceCodex && source != SourceOpenCode && source != SourcePi && source != SourceOMP {
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
		UserID:       strings.TrimSpace(account.UserID),
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(account.RefreshToken),
		IDToken:      strings.TrimSpace(account.IDToken),
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
	if strings.TrimSpace(existing.IDToken) != strings.TrimSpace(merged.IDToken) {
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
	if account.UserID == "" {
		account.UserID = claims.UserID
	}
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

	return buildCodexAccountFromTokens(tokens, path), nil
}

func buildCodexAccountFromTokens(tokens map[string]any, path string) *Account {
	accessToken := strings.TrimSpace(asString(tokens["access_token"]))
	if accessToken == "" {
		return nil
	}

	account := &Account{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(asString(tokens["refresh_token"])),
		AccountID:    strings.TrimSpace(asString(tokens["account_id"])),
		IDToken:      strings.TrimSpace(asString(tokens["id_token"])),
		Source:       SourceCodex,
		FilePath:     path,
		Writable:     true,
	}

	claims := ParseAccessToken(accessToken)
	account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
	account.UserID = claims.UserID
	account.ClientID = claims.ClientID
	account.ExpiresAt = claims.ExpiresAt

	return account
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
	if account.RefreshToken != "" {
		tokens["refresh_token"] = account.RefreshToken
	}
	if account.AccountID != "" {
		tokens["account_id"] = account.AccountID
	}
	tokens["id_token"] = codexIDToken(account)

	root["last_refresh"] = time.Now().UTC().Format(time.RFC3339)

	return writeJSONMap(account.FilePath, root)
}
func loadPiAccountFile(path string, source Source, writable bool) (*Account, error) {
	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Upstream Pi stores ChatGPT Codex OAuth exclusively under "openai-codex".
	// We strictly inspect "openai-codex" with type "oauth" to avoid conflating with static "openai" API keys.
	codexCred := asMap(root["openai-codex"])
	if codexCred == nil {
		return nil, nil
	}

	account := buildPiAccount(codexCred, source, path, writable)
	if account == nil {
		return nil, nil
	}

	return account, nil
}

func buildPiAccount(cred map[string]any, source Source, path string, writable bool) *Account {
	credType := strings.TrimSpace(asString(cred["type"]))
	if credType != "" && credType != "oauth" {
		return nil
	}

	accessToken := strings.TrimSpace(asString(cred["access"]))
	if accessToken == "" {
		return nil
	}
	account := &Account{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(asString(cred["refresh"])),
		AccountID:    strings.TrimSpace(asString(cred["accountId"])),
		Email:        strings.TrimSpace(asString(cred["email"])),
		Source:       source,
		FilePath:     path,
		Writable:     writable,
	}

	if expiresMillis, ok := asInt64(cred["expires"]); ok && expiresMillis > 0 {
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

func savePiAccount(account *Account) error {
	root, err := readJSONMap(account.FilePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", account.FilePath, err)
	}

	codexObj := asMap(root["openai-codex"])
	if codexObj == nil {
		codexObj = make(map[string]any)
		root["openai-codex"] = codexObj
	}

	codexObj["type"] = "oauth"
	codexObj["access"] = account.AccessToken
	if account.RefreshToken != "" {
		codexObj["refresh"] = account.RefreshToken
	}
	if account.AccountID != "" {
		codexObj["accountId"] = account.AccountID
	}
	if account.Email != "" {
		codexObj["email"] = account.Email
	}
	if !account.ExpiresAt.IsZero() {
		codexObj["expires"] = account.ExpiresAt.UnixMilli()
	}

	return writeJSONMap(account.FilePath, root)
}

func loadOMPAccounts(dbPath string) ([]*Account, error) {
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to stat OMP database %s: %w", dbPath, err)
	}

	db, err := openOMPSQLite(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open OMP database %s: %w", dbPath, err)
	}
	defer db.Close()

	var tableExists int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='auth_credentials'").Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect OMP database schema in %s: %w", dbPath, err)
	}
	if tableExists == 0 {
		return nil, nil
	}

	rows, err := db.Query("SELECT id, data, identity_key FROM auth_credentials WHERE provider = 'openai-codex' AND (disabled_cause IS NULL OR disabled_cause = '') ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query auth_credentials in %s: %w", dbPath, err)
	}
	defer rows.Close()

	accounts := make([]*Account, 0)
	for rows.Next() {
		var id int64
		var dataStr string
		var identityKey sql.NullString
		if err := rows.Scan(&id, &dataStr, &identityKey); err != nil {
			return nil, fmt.Errorf("failed to scan auth_credentials row in %s: %w", dbPath, err)
		}

		var dataMap map[string]any
		if err := json.Unmarshal([]byte(dataStr), &dataMap); err != nil {
			continue
		}

		accessToken := strings.TrimSpace(asString(dataMap["access"]))
		if accessToken == "" {
			continue
		}

		account := &Account{
			AccessToken:  accessToken,
			RefreshToken: strings.TrimSpace(asString(dataMap["refresh"])),
			AccountID:    strings.TrimSpace(asString(dataMap["accountId"])),
			Email:        strings.TrimSpace(asString(dataMap["email"])),
			Source:       SourceOMP,
			FilePath:     dbPath,
			Writable:     true,
		}

		if expiresMillis, ok := asInt64(dataMap["expires"]); ok && expiresMillis > 0 {
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

		accounts = append(accounts, account)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading auth_credentials rows in %s: %w", dbPath, err)
	}

	return accounts, nil
}

func saveOMPAccount(account *Account) error {
	_, err := applyAccountToOMP(account, targetWriteRefresh)
	return err
}

// codexIDToken falls back to the access token when the account has no id_token,
// matching Codex's own fallback for externally-managed tokens.
func codexIDToken(account *Account) string {
	if account == nil {
		return ""
	}
	if idToken := strings.TrimSpace(account.IDToken); idToken != "" {
		return idToken
	}
	return strings.TrimSpace(account.AccessToken)
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
		if id := identityID(account); id != "" {
			account.Key = id
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
	// Key on the per-user identity (workspace UUID + user id) so two users in the
	// same workspace map to distinct entries instead of clobbering each other.
	if id := identityID(account); id != "" {
		keys = append(keys, "account:"+id)
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
