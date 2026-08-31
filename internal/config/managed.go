package config

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type managedStore struct {
	Accounts []managedAccount `json:"accounts"`
}

type managedAccount struct {
	Label        string `json:"label,omitempty"`
	Email        string `json:"email,omitempty"`
	AccountID    string `json:"account_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at_ms,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
}

func LoadManagedAccounts() ([]*Account, error) {
	path, err := managedAccountsPath()
	if err != nil {
		return nil, err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Account{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	store := managedStore{}
	if rawAccounts, ok := root["accounts"]; ok {
		store.Accounts, err = decodeManagedAccounts(rawAccounts)
		if err != nil {
			return nil, fmt.Errorf("failed to decode accounts in %s: %w", path, err)
		}
	}

	if migrated, changed := migrateManagedAccounts(store.Accounts); changed {
		store.Accounts = migrated
		// Best-effort persistence for automatic migration; in-memory state continues even if write fails.
		_ = writeJSONMap(path, map[string]any{"accounts": store.Accounts})
	}

	accounts := make([]*Account, 0, len(store.Accounts))
	for _, item := range store.Accounts {
		if strings.TrimSpace(item.AccessToken) == "" {
			continue
		}
		account := &Account{
			Label:        strings.TrimSpace(item.Label),
			Email:        strings.TrimSpace(item.Email),
			AccountID:    strings.TrimSpace(item.AccountID),
			AccessToken:  strings.TrimSpace(item.AccessToken),
			RefreshToken: strings.TrimSpace(item.RefreshToken),
			IDToken:      strings.TrimSpace(item.IDToken),
			ClientID:     strings.TrimSpace(item.ClientID),
			Source:       SourceManaged,
			FilePath:     path,
			Writable:     true,
		}
		if item.ExpiresAt > 0 {
			account.ExpiresAt = time.UnixMilli(item.ExpiresAt)
		}

		claims := ParseAccessToken(account.AccessToken)
		account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
		if account.ClientID == "" {
			account.ClientID = claims.ClientID
		}
		if account.Email == "" {
			account.Email = claims.Email
		}
		if account.ExpiresAt.IsZero() {
			account.ExpiresAt = claims.ExpiresAt
		}

		accounts = append(accounts, account)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return strings.ToLower(accounts[i].Label) < strings.ToLower(accounts[j].Label)
	})

	return accounts, nil
}

func UpsertManagedAccount(account *Account) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		return fmt.Errorf("access token is empty")
	}
	claims := ParseAccessToken(account.AccessToken)
	account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
	if account.Email == "" {
		account.Email = claims.Email
	}
	if account.ClientID == "" {
		account.ClientID = claims.ClientID
	}
	if account.ExpiresAt.IsZero() && !claims.ExpiresAt.IsZero() {
		account.ExpiresAt = claims.ExpiresAt
	}
	if strings.TrimSpace(account.AccountID) == "" {
		return fmt.Errorf("account_id is missing")
	}

	path, err := managedAccountsPath()
	if err != nil {
		return err
	}

	store := managedStore{}
	root, err := readJSONMap(path)
	if err == nil {
		if rawAccounts, ok := root["accounts"]; ok {
			store.Accounts, err = decodeManagedAccounts(rawAccounts)
			if err != nil {
				return fmt.Errorf("failed to decode accounts in %s: %w", path, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	item := managedAccount{
		Label:        strings.TrimSpace(account.Label),
		Email:        strings.TrimSpace(account.Email),
		AccountID:    strings.TrimSpace(account.AccountID),
		AccessToken:  strings.TrimSpace(account.AccessToken),
		RefreshToken: strings.TrimSpace(account.RefreshToken),
		IDToken:      strings.TrimSpace(account.IDToken),
		ClientID:     strings.TrimSpace(account.ClientID),
	}
	if !account.ExpiresAt.IsZero() {
		item.ExpiresAt = account.ExpiresAt.UnixMilli()
	}

	updated := false
	for i := range store.Accounts {
		if strings.TrimSpace(store.Accounts[i].AccountID) == item.AccountID {
			store.Accounts[i] = mergeManagedAccount(store.Accounts[i], item)
			updated = true
			break
		}
	}
	if !updated {
		store.Accounts = append(store.Accounts, item)
	}

	if err := writeJSONMap(path, map[string]any{"accounts": store.Accounts}); err != nil {
		return err
	}

	return nil
}

func mergeManagedAccount(existing, incoming managedAccount) managedAccount {
	merged := existing
	existingExpiresAt := merged.ExpiresAt

	if strings.TrimSpace(merged.Label) == "" {
		merged.Label = incoming.Label
	}
	if strings.TrimSpace(merged.Email) == "" {
		merged.Email = incoming.Email
	}
	if strings.TrimSpace(merged.ClientID) == "" {
		merged.ClientID = incoming.ClientID
	}
	if strings.TrimSpace(merged.RefreshToken) == "" {
		merged.RefreshToken = incoming.RefreshToken
	}
	if strings.TrimSpace(merged.IDToken) == "" {
		merged.IDToken = incoming.IDToken
	}

	if incoming.ExpiresAt > 0 && (existingExpiresAt == 0 || incoming.ExpiresAt > existingExpiresAt) {
		merged.AccessToken = incoming.AccessToken
		merged.ExpiresAt = incoming.ExpiresAt
		if strings.TrimSpace(incoming.RefreshToken) != "" {
			merged.RefreshToken = incoming.RefreshToken
		}
		if strings.TrimSpace(incoming.IDToken) != "" {
			merged.IDToken = incoming.IDToken
		}
		if strings.TrimSpace(incoming.ClientID) != "" {
			merged.ClientID = incoming.ClientID
		}
	}

	if merged.ExpiresAt == 0 {
		merged.ExpiresAt = incoming.ExpiresAt
	}

	if strings.TrimSpace(merged.AccessToken) == "" {
		merged.AccessToken = incoming.AccessToken
		if merged.ExpiresAt == 0 {
			merged.ExpiresAt = incoming.ExpiresAt
		}
	}

	return merged
}

func saveManagedAccount(account *Account) error {
	return UpsertManagedAccount(account)
}

func DeleteManagedAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("account_id is empty")
	}

	path, err := managedAccountsPath()
	if err != nil {
		return err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	store := managedStore{}
	if rawAccounts, ok := root["accounts"]; ok {
		store.Accounts, err = decodeManagedAccounts(rawAccounts)
		if err != nil {
			return fmt.Errorf("failed to decode accounts in %s: %w", path, err)
		}
	}

	filtered := make([]managedAccount, 0, len(store.Accounts))
	for _, item := range store.Accounts {
		if strings.TrimSpace(item.AccountID) == accountID {
			continue
		}
		filtered = append(filtered, item)
	}

	if len(filtered) == len(store.Accounts) {
		return nil
	}

	root["accounts"] = filtered
	return writeJSONMap(path, root)
}

func DeleteManagedAccountByIdentity(account *Account) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}

	accountID := strings.TrimSpace(account.AccountID)
	email := normalizeEmail(account.Email)
	if accountID == "" && email == "" {
		return fmt.Errorf("account identity is empty")
	}

	path, err := managedAccountsPath()
	if err != nil {
		return err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	store := managedStore{}
	if rawAccounts, ok := root["accounts"]; ok {
		store.Accounts, err = decodeManagedAccounts(rawAccounts)
		if err != nil {
			return fmt.Errorf("failed to decode accounts in %s: %w", path, err)
		}
	}

	filtered := make([]managedAccount, 0, len(store.Accounts))
	removed := false
	for _, item := range store.Accounts {
		itemAccountID := strings.TrimSpace(item.AccountID)
		itemEmail := normalizeEmail(item.Email)
		itemCanonicalID := CanonicalAccountID(itemAccountID, ParseAccessToken(strings.TrimSpace(item.AccessToken)).AccountID)

		matchID := accountID != "" && (itemAccountID == accountID || itemCanonicalID == accountID)
		matchEmail := email != "" && itemEmail == email
		if matchID || matchEmail {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}

	if !removed {
		return nil
	}

	root["accounts"] = filtered
	return writeJSONMap(path, root)
}

func ApplyAccountToOpenCode(account *Account) (string, error) {
	return applyAccountToOpenCode(account, targetWriteApply)
}

func applyAccountToOpenCode(account *Account, mode targetWriteMode) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	paths := opencodeApplyPaths()
	if len(paths) == 0 {
		return "", fmt.Errorf("OpenCode auth path is unknown")
	}

	successPaths := make([]string, 0, len(paths))
	errorsList := make([]string, 0)
	skipped := false

	for _, path := range paths {
		root, err := readJSONMap(path)
		if err != nil {
			if os.IsNotExist(err) {
				root = make(map[string]any)
			} else {
				errorsList = append(errorsList, fmt.Sprintf("%s: failed to read: %v", path, err))
				continue
			}
		}

		openai := asMap(root["openai"])
		if openai == nil {
			openai = make(map[string]any)
			root["openai"] = openai
		}

		accountToWrite := chooseTargetWriteAccount(account, buildOpenAIAccount(openai, SourceOpenCode, path, true), mode)
		if accountToWrite == nil {
			skipped = true
			continue
		}

		openai["access"] = accountToWrite.AccessToken
		if accountToWrite.RefreshToken != "" {
			openai["refresh"] = accountToWrite.RefreshToken
		}
		if accountToWrite.AccountID != "" {
			openai["accountId"] = accountToWrite.AccountID
		}
		if accountToWrite.Email != "" {
			openai["email"] = accountToWrite.Email
		}
		if !accountToWrite.ExpiresAt.IsZero() {
			openai["expires"] = accountToWrite.ExpiresAt.UnixMilli()
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to ensure directory: %v", path, err))
			continue
		}

		if err := writeJSONMap(path, root); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to write: %v", path, err))
			continue
		}

		successPaths = append(successPaths, path)
	}

	if len(successPaths) == 0 {
		if skipped {
			return "", nil
		}
		if len(errorsList) > 0 {
			return "", fmt.Errorf("apply to OpenCode failed: %s", strings.Join(errorsList, "; "))
		}
		return "", fmt.Errorf("apply to OpenCode failed: no writable auth path")
	}

	return successPaths[0], nil
}

func ApplyAccountToCodex(account *Account) (string, error) {
	return applyAccountToCodex(account, targetWriteApply)
}

func applyAccountToCodex(account *Account, mode targetWriteMode) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	path := codexAuthPath()
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("Codex auth path is unknown")
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			root = make(map[string]any)
		} else {
			return "", fmt.Errorf("failed to read %s: %w", path, err)
		}
	}

	tokens := asMap(root["tokens"])
	if tokens == nil {
		tokens = make(map[string]any)
		root["tokens"] = tokens
	}

	accountToWrite := chooseTargetWriteAccount(account, buildCodexAccountFromTokens(tokens, path), mode)
	if accountToWrite == nil {
		return "", nil
	}

	tokens["access_token"] = accountToWrite.AccessToken
	if accountToWrite.RefreshToken != "" {
		tokens["refresh_token"] = accountToWrite.RefreshToken
	}
	if accountToWrite.AccountID != "" {
		tokens["account_id"] = accountToWrite.AccountID
	}
	tokens["id_token"] = codexIDToken(accountToWrite)
	root["last_refresh"] = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("failed to ensure directory for %s: %w", path, err)
	}

	if err := writeJSONMap(path, root); err != nil {
		return "", err
	}

	return path, nil
}

func DeleteOpenCodeAuthAccount() error {
	paths := opencodeExistingPaths()
	if len(paths) == 0 {
		if len(opencodeAuthPaths()) == 0 {
			return fmt.Errorf("OpenCode auth path is unknown")
		}
		return nil
	}

	errorsList := make([]string, 0)
	for _, path := range paths {
		root, err := readJSONMap(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to read: %v", path, err))
			continue
		}

		openai := asMap(root["openai"])
		if openai == nil {
			continue
		}

		changed := false
		changed = deleteMapKey(openai, "access") || changed
		changed = deleteMapKey(openai, "refresh") || changed
		changed = deleteMapKey(openai, "accountId") || changed
		changed = deleteMapKey(openai, "email") || changed
		changed = deleteMapKey(openai, "expires") || changed
		if !changed {
			continue
		}

		if err := writeJSONMap(path, root); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to write: %v", path, err))
			continue
		}
	}

	if len(errorsList) > 0 {
		return fmt.Errorf("delete from OpenCode failed: %s", strings.Join(errorsList, "; "))
	}

	return nil
}

func opencodeExistingPaths() []string {
	paths := opencodeAuthPaths()
	if len(paths) == 0 {
		return nil
	}

	existing := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

func opencodeApplyPaths() []string {
	existing := opencodeExistingPaths()
	if len(existing) > 0 {
		return existing
	}

	allPaths := opencodeAuthPaths()
	if len(allPaths) > 0 {
		return []string{allPaths[0]}
	}

	path := opencodeAuthPath()
	if strings.TrimSpace(path) != "" {
		return []string{path}
	}
	return nil
}

func DeleteCodexAuthAccount() error {
	path := codexAuthPath()
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("Codex auth path is unknown")
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	tokens := asMap(root["tokens"])
	if tokens == nil {
		return nil
	}

	changed := false
	changed = deleteMapKey(tokens, "access_token") || changed
	changed = deleteMapKey(tokens, "refresh_token") || changed
	changed = deleteMapKey(tokens, "account_id") || changed
	changed = deleteMapKey(tokens, "id_token") || changed
	if !changed {
		return nil
	}

	return writeJSONMap(path, root)
}
func ApplyAccountToPi(account *Account) (string, error) {
	return applyAccountToPi(account, targetWriteApply)
}

func applyAccountToPi(account *Account, mode targetWriteMode) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	path := piAuthPath()
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("Pi auth path is unknown")
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			root = make(map[string]any)
		} else {
			return "", fmt.Errorf("failed to read %s: %w", path, err)
		}
	}

	codexObj := asMap(root["openai-codex"])
	if codexObj == nil {
		codexObj = make(map[string]any)
		root["openai-codex"] = codexObj
	}

	accountToWrite := chooseTargetWriteAccount(account, buildPiAccount(codexObj, SourcePi, path, true), mode)
	if accountToWrite == nil {
		return "", nil
	}

	codexObj["type"] = "oauth"
	codexObj["access"] = accountToWrite.AccessToken
	if accountToWrite.RefreshToken != "" {
		codexObj["refresh"] = accountToWrite.RefreshToken
	}
	if accountToWrite.AccountID != "" {
		codexObj["accountId"] = accountToWrite.AccountID
	}
	if accountToWrite.Email != "" {
		codexObj["email"] = accountToWrite.Email
	}
	if !accountToWrite.ExpiresAt.IsZero() {
		codexObj["expires"] = accountToWrite.ExpiresAt.UnixMilli()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("failed to ensure directory for %s: %w", path, err)
	}

	if err := writeJSONMap(path, root); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", path, err)
	}

	return path, nil
}

func DeletePiAuthAccount(account *Account) error {
	path := piAuthPath()
	if strings.TrimSpace(path) == "" {
		return nil
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	if _, ok := root["openai-codex"]; !ok {
		return nil
	}

	delete(root, "openai-codex")
	if err := writeJSONMap(path, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func HasExistingPiAuth() bool {
	path := piAuthPath()
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func HasExistingOMPAuth() bool {
	path := ompAgentDbPath()
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func ApplyAccountToOMP(account *Account) (string, error) {
	return applyAccountToOMP(account, targetWriteApply)
}

func applyAccountToOMP(account *Account, mode targetWriteMode) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(account.AccountID) == "" && normalizeEmail(account.Email) == "" {
		return "", fmt.Errorf("OMP account identity is empty")
	}

	path := ompAgentDbPath()
	// Refresh follows the database the account was loaded from so a
	// profile switch between load and save cannot retarget the write.
	// Apply always targets the active profile database.
	if mode == targetWriteRefresh && strings.TrimSpace(account.FilePath) != "" {
		path = account.FilePath
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("OMP agent.db path is unknown")
	}
	existingAccounts, err := loadOMPAccounts(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to load existing accounts from %s: %w", path, err)
	}

	var matchingExisting *Account
	for _, existing := range existingAccounts {
		if sameIdentity(account, existing) {
			matchingExisting = existing
			break
		}
	}

	accountToWrite := chooseTargetWriteAccount(account, matchingExisting, mode)
	if accountToWrite == nil {
		return "", nil
	}

	db, err := openOMPDatabase(path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	activeRowID, err := upsertOMPCredential(tx, path, accountToWrite)
	if err != nil {
		return "", err
	}

	if mode == targetWriteRefresh {
		if err := tx.Commit(); err != nil {
			return "", fmt.Errorf("failed to commit OMP credential refresh in %s: %w", path, err)
		}
		return path, nil
	}

	var hasCacheTable int
	if err := tx.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'cache'").Scan(&hasCacheTable); err != nil {
		return "", fmt.Errorf("failed to inspect OMP cache table in %s: %w", path, err)
	}
	if hasCacheTable != 0 {
		_, err = tx.Exec(`
			DELETE FROM cache
			WHERE key LIKE 'session:sticky:openai-codex:%'
			  AND CAST(json_extract(value, '$.credentialId') AS INTEGER) IN (
				SELECT id FROM auth_credentials
				WHERE provider = 'openai-codex' AND id != ?
			  )`,
			activeRowID,
		)
		if err != nil {
			return "", fmt.Errorf("failed to clear stale OMP sticky cache entries in %s: %w", path, err)
		}
	}

	if _, err = tx.Exec("DELETE FROM auth_credentials WHERE provider = 'openai-codex' AND id != ?", activeRowID); err != nil {
		return "", fmt.Errorf("failed to remove inactive OMP credentials in %s: %w", path, err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit OMP credential update in %s: %w", path, err)
	}
	return path, nil
}

func openOMPDatabase(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to ensure directory for %s: %w", path, err)
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600); err == nil {
		_ = f.Close()
		_ = os.Chmod(path, 0o600)
	}
	db, err := openOMPSQLite(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %s: %w", path, err)
	}
	_, err = db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		CREATE TABLE IF NOT EXISTS auth_schema_version (id INTEGER PRIMARY KEY CHECK (id = 1), version INTEGER NOT NULL);
		INSERT OR IGNORE INTO auth_schema_version (id, version) VALUES (1, 7);
		CREATE TABLE IF NOT EXISTS auth_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT, provider TEXT NOT NULL, credential_type TEXT NOT NULL, data TEXT NOT NULL,
			disabled_cause TEXT DEFAULT NULL, identity_key TEXT DEFAULT NULL,
			created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
			updated_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER))
		);
		CREATE INDEX IF NOT EXISTS idx_auth_provider ON auth_credentials(provider);
		CREATE INDEX IF NOT EXISTS idx_auth_provider_identity ON auth_credentials(provider, identity_key) WHERE identity_key IS NOT NULL;
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema in %s: %w", path, err)
	}
	return db, nil
}

func openOMPSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func upsertOMPCredential(tx *sql.Tx, path string, account *Account) (int64, error) {
	accountID := strings.TrimSpace(account.AccountID)
	email := normalizeEmail(account.Email)
	if accountID == "" && email == "" {
		return 0, fmt.Errorf("OMP account identity is empty")
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		return 0, fmt.Errorf("OMP account credential is empty")
	}
	identityKey := "account:" + accountID
	if email != "" {
		identityKey = "email:" + email
	}
	payload := map[string]any{
		"type": "oauth", "access": account.AccessToken, "refresh": account.RefreshToken,
		"accountId": account.AccountID, "email": account.Email, "authorizedAt": time.Now().UnixMilli(),
	}
	if !account.ExpiresAt.IsZero() {
		payload["expires"] = account.ExpiresAt.UnixMilli()
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal credential JSON: %w", err)
	}

	var id int64
	if accountID != "" {
		err = tx.QueryRow(`SELECT id FROM auth_credentials WHERE provider = 'openai-codex' AND (identity_key = ? OR json_extract(data, '$.accountId') = ?) ORDER BY id ASC LIMIT 1`, "account:"+accountID, accountID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) && email != "" {
			err = tx.QueryRow(`SELECT id FROM auth_credentials WHERE provider = 'openai-codex' AND (identity_key = ? OR lower(json_extract(data, '$.email')) = ?) AND (json_extract(data, '$.accountId') IS NULL OR trim(json_extract(data, '$.accountId')) = '') AND (identity_key IS NULL OR identity_key NOT LIKE 'account:%') ORDER BY id ASC LIMIT 1`, "email:"+email, email).Scan(&id)
		}
	} else {
		err = tx.QueryRow(`SELECT id FROM auth_credentials WHERE provider = 'openai-codex' AND (identity_key = ? OR lower(json_extract(data, '$.email')) = ?) ORDER BY id ASC LIMIT 1`, "email:"+email, email).Scan(&id)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("failed to find credential row in %s: %w", path, err)
	}
	if id > 0 {
		_, err = tx.Exec("UPDATE auth_credentials SET credential_type = 'oauth', data = ?, identity_key = ?, disabled_cause = NULL, updated_at = CAST(strftime('%s','now') AS INTEGER) WHERE id = ?", string(data), identityKey, id)
	} else {
		err = tx.QueryRow("INSERT INTO auth_credentials (provider, credential_type, data, disabled_cause, identity_key) VALUES ('openai-codex', 'oauth', ?, NULL, ?) RETURNING id", string(data), identityKey).Scan(&id)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to write credential row in %s: %w", path, err)
	}
	return id, nil
}

// RestoreManagedAccountsToOMP mirrors CQ's managed accounts into the active OMP profile.
func RestoreManagedAccountsToOMP() (int, string, error) {
	if err := validateManagedAccountsForOMPRestore(); err != nil {
		return 0, "", err
	}
	managed, err := LoadManagedAccounts()
	if err != nil {
		return 0, "", fmt.Errorf("failed to load CQ accounts: %w", err)
	}
	unique := make([]*Account, 0, len(managed))
	for _, account := range managed {
		if account == nil {
			continue
		}
		for index, existing := range unique {
			if !sameIdentity(account, existing) {
				continue
			}
			if strings.TrimSpace(existing.AccountID) == "" && strings.TrimSpace(account.AccountID) != "" {
				unique[index] = account
			} else if strings.TrimSpace(existing.AccountID) == strings.TrimSpace(account.AccountID) {
				unique[index] = freshestAccountForIdentity(account, []*Account{account, existing})
			}
			account = nil
			break
		}
		if account != nil {
			unique = append(unique, account)
		}
	}

	prepared := make([]*Account, 0, len(unique))
	var failures []error
	for _, account := range unique {
		key := "email:" + normalizeEmail(account.Email)
		if accountID := strings.TrimSpace(account.AccountID); accountID != "" {
			key = "account:" + accountID
		}
		if key == "email:" {
			failures = append(failures, fmt.Errorf("%q: missing account ID and email", account.Label))
			continue
		}
		fresh, _, refreshErr := ResolveFreshAccount(account)
		if refreshErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", key, refreshErr))
			continue
		}
		if fresh == nil || strings.TrimSpace(fresh.AccessToken) == "" {
			failures = append(failures, fmt.Errorf("%s: missing access token", key))
			continue
		}
		if strings.TrimSpace(account.AccountID) != "" {
			fresh.AccountID = account.AccountID
		}
		if normalizeEmail(fresh.Email) == "" {
			fresh.Email = account.Email
		}
		prepared = append(prepared, fresh)
	}
	if len(failures) > 0 {
		return 0, "", fmt.Errorf("cannot restore OMP pool: %w", errors.Join(failures...))
	}
	if len(prepared) == 0 {
		return 0, "", fmt.Errorf("cannot restore OMP pool: no managed CQ accounts")
	}

	path := ompAgentDbPath()
	if strings.TrimSpace(path) == "" {
		return 0, "", fmt.Errorf("OMP agent.db path is unknown")
	}
	db, err := openOMPDatabase(path)
	if err != nil {
		return 0, "", err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return 0, "", fmt.Errorf("failed to begin OMP restore transaction: %w", err)
	}
	defer tx.Rollback()

	ids := make([]any, 0, len(prepared))
	for _, account := range prepared {
		id, err := upsertOMPCredential(tx, path, account)
		if err != nil {
			return 0, "", err
		}
		ids = append(ids, id)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	var hasCacheTable int
	if err := tx.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'cache'").Scan(&hasCacheTable); err != nil {
		return 0, "", fmt.Errorf("failed to inspect OMP cache table in %s: %w", path, err)
	}
	if hasCacheTable != 0 {
		query := "DELETE FROM cache WHERE key LIKE 'session:sticky:openai-codex:%' AND CAST(json_extract(value, '$.credentialId') AS INTEGER) IN (SELECT id FROM auth_credentials WHERE provider = 'openai-codex' AND id NOT IN (" + placeholders + "))"
		if _, err := tx.Exec(query, ids...); err != nil {
			return 0, "", fmt.Errorf("failed to clear stale OMP sticky cache entries in %s: %w", path, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM auth_credentials WHERE provider = 'openai-codex' AND id NOT IN ("+placeholders+")", ids...); err != nil {
		return 0, "", fmt.Errorf("failed to remove obsolete OMP credentials in %s: %w", path, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, "", fmt.Errorf("failed to commit OMP pool restore in %s: %w", path, err)
	}
	return len(prepared), path, nil
}

func validateManagedAccountsForOMPRestore() error {
	path, err := managedAccountsPath()
	if err != nil {
		return err
	}
	root, err := readJSONMap(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read CQ accounts: %w", err)
	}
	items, err := decodeManagedAccounts(root["accounts"])
	if err != nil {
		return fmt.Errorf("failed to decode CQ accounts: %w", err)
	}
	var failures []error
	for index, item := range items {
		if strings.TrimSpace(item.AccessToken) == "" {
			failures = append(failures, fmt.Errorf("managed account %d: missing access token", index+1))
		}
		if strings.TrimSpace(item.AccountID) == "" && normalizeEmail(item.Email) == "" {
			failures = append(failures, fmt.Errorf("managed account %d: missing account ID and email", index+1))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("cannot restore OMP pool: %w", errors.Join(failures...))
	}
	return nil
}

func DeleteOMPAuthAccount(account *Account) error {
	if account == nil {
		return nil
	}
	accountID := strings.TrimSpace(account.AccountID)
	email := normalizeEmail(account.Email)
	if accountID == "" && email == "" {
		return nil
	}
	path := ompAgentDbPath()
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	db, err := openOMPSQLite(path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction in %s: %w", path, err)
	}
	defer tx.Rollback()

	identityKey := "email:" + email
	query := "provider = 'openai-codex' AND (identity_key = ? OR lower(json_extract(data, '$.email')) = ?)"
	args := []any{identityKey, email}
	if accountID != "" {
		identityKey = "account:" + accountID
		query = "provider = 'openai-codex' AND (identity_key = ? OR json_extract(data, '$.accountId') = ?)"
		args = []any{identityKey, accountID}
		if email != "" {
			query = "provider = 'openai-codex' AND (identity_key = ? OR json_extract(data, '$.accountId') = ? OR ((json_extract(data, '$.accountId') IS NULL OR trim(json_extract(data, '$.accountId')) = '') AND (identity_key IS NULL OR identity_key NOT LIKE 'account:%') AND (identity_key = ? OR lower(json_extract(data, '$.email')) = ?)))"
			args = []any{identityKey, accountID, "email:" + email, email}
		}
	}
	rows, err := tx.Query("SELECT id FROM auth_credentials WHERE "+query, args...)
	if err != nil {
		return fmt.Errorf("failed to select account from %s: %w", path, err)
	}
	var ids []any
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan account in %s: %w", path, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to read accounts in %s: %w", path, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("failed to read accounts in %s: %w", path, err)
	}
	if len(ids) > 0 {
		var hasCacheTable int
		if err := tx.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'cache'").Scan(&hasCacheTable); err != nil {
			return fmt.Errorf("failed to inspect OMP cache table in %s: %w", path, err)
		}
		placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
		if hasCacheTable != 0 {
			if _, err := tx.Exec("DELETE FROM cache WHERE key LIKE 'session:sticky:openai-codex:%' AND CAST(json_extract(value, '$.credentialId') AS INTEGER) IN ("+placeholders+")", ids...); err != nil {
				return fmt.Errorf("failed to clear OMP sticky cache entries in %s: %w", path, err)
			}
		}
		if _, err := tx.Exec("DELETE FROM auth_credentials WHERE id IN ("+placeholders+")", ids...); err != nil {
			return fmt.Errorf("failed to delete account from %s: %w", path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to delete account from %s: %w", path, err)
	}
	return nil
}

func deleteMapKey(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	if _, ok := values[key]; !ok {
		return false
	}
	delete(values, key)
	return true
}

func managedAccountsPath() (string, error) {
	dir, err := codexQuotaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "accounts.json"), nil
}

func decodeManagedAccounts(raw any) ([]managedAccount, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	accounts := make([]managedAccount, 0)
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}

	return accounts, nil
}

func migrateManagedAccounts(input []managedAccount) ([]managedAccount, bool) {
	if len(input) == 0 {
		return input, false
	}

	byID := make(map[string]managedAccount, len(input))
	order := make([]string, 0, len(input))
	changed := false

	for _, item := range input {
		normalized := strings.TrimSpace(item.AccountID)
		accessToken := strings.TrimSpace(item.AccessToken)
		claims := ParseAccessToken(accessToken)
		canonicalID := CanonicalAccountID(normalized, claims.AccountID)
		if canonicalID != normalized {
			changed = true
		}
		item.AccountID = canonicalID

		if item.Email == "" && claims.Email != "" {
			item.Email = claims.Email
			changed = true
		}
		if shouldReplaceManagedLabelWithEmail(item) {
			item.Label = strings.TrimSpace(item.Email)
			changed = true
		}
		if item.ClientID == "" && claims.ClientID != "" {
			item.ClientID = claims.ClientID
			changed = true
		}
		if item.ExpiresAt == 0 && !claims.ExpiresAt.IsZero() {
			item.ExpiresAt = claims.ExpiresAt.UnixMilli()
			changed = true
		}

		key := item.AccountID
		if key == "" {
			key = fmt.Sprintf("__empty__:%d", len(order))
		}

		if existing, ok := byID[key]; ok {
			merged := mergeManagedAccount(existing, item)
			if merged != existing {
				changed = true
			}
			byID[key] = merged
			continue
		}

		byID[key] = item
		order = append(order, key)
	}

	output := make([]managedAccount, 0, len(order))
	for _, accountID := range order {
		if account, ok := byID[accountID]; ok {
			output = append(output, account)
		}
	}

	if len(output) != len(input) {
		changed = true
	}

	return output, changed
}

func shouldReplaceManagedLabelWithEmail(item managedAccount) bool {
	email := strings.TrimSpace(item.Email)
	if email == "" {
		return false
	}
	label := strings.TrimSpace(item.Label)
	if label == "" {
		return true
	}
	if strings.EqualFold(label, "n/a") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(label), "auth0|") {
		return true
	}
	if accountID := strings.TrimSpace(item.AccountID); accountID != "" && label == shortAccountID(accountID) {
		return true
	}
	return false
}
