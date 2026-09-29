package config

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	openCode2Integration  = "openai"
	openCode2OAuthMethod  = "chatgpt-browser"
	openCode2BusyTimeout  = 5000
	openCode2ProbeTimeout = 2 * time.Second
	openCode2DBTimeout    = 10 * time.Second
)

type openCodeCapabilities struct {
	Legacy bool
	V2     bool

	v1Generation bool
	v2Generation bool

	legacyPaths         []string
	legacyExistingPaths []string
	v2Path              string
}

func opencode2DBPath() string {
	if path := cleanPath(os.Getenv("OPENCODE_DB")); path != "" {
		return absoluteOpenCode2Path(path)
	}
	dataHome := cleanPath(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return ""
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return absoluteOpenCode2Path(filepath.Join(dataHome, "opencode", "opencode.db"))
}

func absoluteOpenCode2Path(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return path
}

func probeOpenCodeVersion(ctx context.Context, command string) (string, error) {
	cmd := exec.CommandContext(ctx, command, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func openCodeVersionGeneration(output string) int {
	for _, field := range strings.Fields(output) {
		field = strings.TrimPrefix(strings.TrimSpace(field), "opencode")
		field = strings.TrimPrefix(field, "v")
		majorText := field
		if dot := strings.IndexByte(majorText, '.'); dot >= 0 {
			majorText = majorText[:dot]
		}
		major, err := strconv.Atoi(majorText)
		if err != nil {
			continue
		}
		if major == 1 {
			return 1
		}
		if major >= 2 {
			return 2
		}
	}
	return 0
}

// detectOpenCodeGenerations is intentionally uncached so tests can inject PATH/probes.
func detectOpenCodeGenerations(
	lookPath func(string) (string, error),
	probe func(context.Context, string) (string, error),
) (bool, bool) {
	var v1, v2 bool
	for _, name := range []string{"opencode", "opencode2"} {
		path, err := lookPath(name)
		if err != nil || strings.TrimSpace(path) == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), openCode2ProbeTimeout)
		output, probeErr := probe(ctx, path)
		cancel()
		if probeErr != nil {
			continue
		}
		switch openCodeVersionGeneration(output) {
		case 1:
			v1 = true
		case 2:
			v2 = true
		}
	}
	return v1, v2
}

var openCodeGenerationCache struct {
	sync.Mutex
	loaded bool
	v1     bool
	v2     bool
}

func cachedOpenCodeGenerations() (bool, bool) {
	openCodeGenerationCache.Lock()
	defer openCodeGenerationCache.Unlock()
	if !openCodeGenerationCache.loaded {
		openCodeGenerationCache.v1, openCodeGenerationCache.v2 = detectOpenCodeGenerations(exec.LookPath, probeOpenCodeVersion)
		openCodeGenerationCache.loaded = true
	}
	return openCodeGenerationCache.v1, openCodeGenerationCache.v2
}

// resetOpenCodeGenerationCacheForTest is used only by package-local tests.
func resetOpenCodeGenerationCacheForTest() {
	openCodeGenerationCache.Lock()
	openCodeGenerationCache.loaded = false
	openCodeGenerationCache.v1 = false
	openCodeGenerationCache.v2 = false
	openCodeGenerationCache.Unlock()
}

func detectOpenCodeCapabilities() (openCodeCapabilities, error) {
	v1, v2 := cachedOpenCodeGenerations()
	return detectOpenCodeCapabilitiesWithGenerations(v1, v2)
}

// OpenCode2Available reports whether the initialized native OpenCode 2 database is present.
func OpenCode2Available() bool {
	capabilities, err := detectOpenCodeCapabilities()
	return err == nil && capabilities.V2
}

// detectOpenCodeCapabilitiesWith keeps generation probing injectable and uncached for tests.
func detectOpenCodeCapabilitiesWith(
	lookPath func(string) (string, error),
	probe func(context.Context, string) (string, error),
) (openCodeCapabilities, error) {
	v1, v2 := detectOpenCodeGenerations(lookPath, probe)
	return detectOpenCodeCapabilitiesWithGenerations(v1, v2)
}

func detectOpenCodeCapabilitiesWithGenerations(v1, v2 bool) (openCodeCapabilities, error) {
	capabilities := openCodeCapabilities{v1Generation: v1, v2Generation: v2}
	dbPath := opencode2DBPath()
	explicitDB := cleanPath(os.Getenv("OPENCODE_DB")) != ""
	if dbPath != "" {
		info, statErr := os.Stat(dbPath)
		switch {
		case statErr == nil && info.IsDir():
			if explicitDB || v2 {
				return capabilities, fmt.Errorf("OpenCode v2 database path is a directory")
			}
		case statErr == nil:
			schemaErr := validateOpenCode2DatabaseReadOnly(dbPath)
			if schemaErr == nil {
				capabilities.V2 = true
				capabilities.v2Path = dbPath
			} else if explicitDB {
				return capabilities, fmt.Errorf("invalid OpenCode v2 database: %w", schemaErr)
			}
		case os.IsNotExist(statErr):
			if explicitDB {
				return capabilities, fmt.Errorf("OpenCode v2 database configured by OPENCODE_DB does not exist")
			}
		default:
			if explicitDB {
				return capabilities, fmt.Errorf("failed to inspect OpenCode v2 database: %w", statErr)
			}
		}
	}

	if explicitAuth := cleanPath(os.Getenv("OPENCODE_AUTH_PATH")); explicitAuth != "" {
		capabilities.Legacy = true
		capabilities.legacyPaths = []string{explicitAuth}
		if _, err := os.Stat(explicitAuth); err == nil {
			capabilities.legacyExistingPaths = []string{explicitAuth}
		} else if !os.IsNotExist(err) {
			return capabilities, fmt.Errorf("failed to inspect OpenCode auth path: %w", err)
		}
	} else if v1 || (!v2 && !capabilities.V2) {
		capabilities.legacyExistingPaths = opencodeExistingPaths()
		if v1 || len(capabilities.legacyExistingPaths) > 0 {
			capabilities.Legacy = true
			capabilities.legacyPaths = opencodeApplyPaths()
		}
	}
	return capabilities, nil
}

func detectOpenCodeCapabilitiesAtPath(path string) (openCodeCapabilities, error) {
	path = cleanPath(path)
	if path == "" {
		return openCodeCapabilities{}, fmt.Errorf("OpenCode store path is unknown")
	}
	info, err := os.Stat(path)
	if err != nil {
		return openCodeCapabilities{}, fmt.Errorf("failed to inspect OpenCode store: %w", err)
	}
	if !info.Mode().IsRegular() {
		return openCodeCapabilities{}, fmt.Errorf("OpenCode store path is not a regular file")
	}
	if err := validateOpenCode2DatabaseReadOnly(path); err == nil {
		return openCodeCapabilities{V2: true, v2Path: path}, nil
	}
	return openCodeCapabilities{
		Legacy:              true,
		legacyPaths:         []string{path},
		legacyExistingPaths: []string{path},
	}, nil
}

type openCode2Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type openCode2Executor interface {
	openCode2Queryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func openCode2DSN(path string, readOnly bool) string {
	path = strings.TrimSpace(path)
	if path != "" {
		path = absoluteOpenCode2Path(path)
	}
	mode := "rw"
	if readOnly {
		mode = "ro"
	}
	fileURL := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := url.Values{}
	query.Set("mode", mode)
	query.Set("_busy_timeout", strconv.Itoa(openCode2BusyTimeout))
	fileURL.RawQuery = query.Encode()
	return fileURL.String()
}

func openOpenCode2Database(path string, readOnly bool) (*sql.DB, error) {
	path = absoluteOpenCode2Path(strings.TrimSpace(path))
	if path == "" {
		return nil, fmt.Errorf("OpenCode v2 database path is unknown")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("OpenCode v2 database is not initialized")
		}
		return nil, fmt.Errorf("failed to inspect OpenCode v2 database: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("OpenCode v2 database path is not a regular file")
	}
	db, err := sql.Open("sqlite", openCode2DSN(path, readOnly))
	if err != nil {
		return nil, fmt.Errorf("failed to open OpenCode v2 database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to open OpenCode v2 database: %w", err)
	}
	return db, nil
}

func validateOpenCode2DatabaseReadOnly(path string) error {
	db, err := openOpenCode2Database(path, true)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	defer cancel()
	return validateOpenCode2Schema(ctx, db)
}

func configureOpenCode2Connection(ctx context.Context, conn openCode2Executor) error {
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("failed to configure OpenCode v2 busy timeout: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to configure OpenCode v2 foreign keys: %w", err)
	}
	return nil
}

func validateOpenCode2Schema(ctx context.Context, queryer openCode2Queryer) error {
	rows, err := queryer.QueryContext(ctx, "PRAGMA table_info(credential)")
	if err != nil {
		return fmt.Errorf("failed to inspect OpenCode v2 credential schema: %w", err)
	}
	defer rows.Close()

	type columnInfo struct {
		seen    bool
		notNull int
		primary int
	}
	columns := map[string]columnInfo{}
	for _, name := range []string{"id", "integration_id", "label", "value", "connector_id", "method_id", "active", "time_created", "time_updated"} {
		columns[name] = columnInfo{}
	}
	for rows.Next() {
		var cid, notNull, primary int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primary); err != nil {
			return fmt.Errorf("failed to read OpenCode v2 credential schema: %w", err)
		}
		if info, ok := columns[name]; ok {
			info.seen = true
			info.notNull = notNull
			info.primary = primary
			columns[name] = info
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read OpenCode v2 credential schema: %w", err)
	}
	for _, name := range []string{"id", "integration_id", "label", "value", "connector_id", "method_id", "active", "time_created", "time_updated"} {
		if !columns[name].seen {
			return fmt.Errorf("OpenCode v2 credential schema is missing required column %s", name)
		}
	}
	if columns["id"].primary == 0 {
		return fmt.Errorf("OpenCode v2 credential schema has no credential primary key")
	}
	for _, name := range []string{"label", "value", "time_created", "time_updated"} {
		if columns[name].notNull == 0 {
			return fmt.Errorf("OpenCode v2 credential column %s must be non-null", name)
		}
	}
	for _, name := range []string{"integration_id", "connector_id", "method_id", "active"} {
		if columns[name].notNull != 0 {
			return fmt.Errorf("OpenCode v2 credential column %s must be nullable", name)
		}
	}
	return nil
}

type openCode2Transaction struct {
	db     *sql.DB
	conn   *sql.Conn
	closed bool
}

func beginOpenCode2Transaction(path string) (*openCode2Transaction, error) {
	db, err := openOpenCode2Database(path, false)
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to acquire OpenCode v2 database connection: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	defer cancel()
	if err := configureOpenCode2Connection(ctx, conn); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, fmt.Errorf("failed to begin OpenCode v2 transaction: %w", err)
	}
	// Revalidate after taking the writer lock and before any mutation.
	if err := validateOpenCode2Schema(ctx, conn); err != nil {
		_, rollbackErr := conn.ExecContext(ctx, "ROLLBACK")
		_ = conn.Close()
		_ = db.Close()
		if rollbackErr != nil {
			return nil, errors.Join(fmt.Errorf("invalid OpenCode v2 credential schema: %w", err), fmt.Errorf("database rollback failed: %w", rollbackErr))
		}
		return nil, fmt.Errorf("invalid OpenCode v2 credential schema: %w", err)
	}
	return &openCode2Transaction{db: db, conn: conn}, nil
}

func (tx *openCode2Transaction) close() {
	if tx == nil || tx.closed {
		return
	}
	tx.closed = true
	if tx.conn != nil {
		_ = tx.conn.Close()
	}
	if tx.db != nil {
		_ = tx.db.Close()
	}
}

func (tx *openCode2Transaction) rollback() error {
	if tx == nil || tx.closed {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	defer cancel()
	_, err := tx.conn.ExecContext(ctx, "ROLLBACK")
	tx.close()
	return err
}

func (tx *openCode2Transaction) commit() error {
	if tx == nil || tx.closed {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	defer cancel()
	_, err := tx.conn.ExecContext(ctx, "COMMIT")
	if err != nil {
		return err
	}
	tx.close()
	return nil
}

type openCode2CredentialRow struct {
	ID          string
	Label       string
	Value       string
	Active      sql.NullInt64
	TimeCreated int64
	TimeUpdated int64
}

func queryOpenCode2CredentialRows(ctx context.Context, queryer openCode2Queryer) ([]openCode2CredentialRow, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, label, value, active, time_created, time_updated
		FROM credential
		WHERE integration_id = ?
		ORDER BY time_created ASC, id ASC`, openCode2Integration)
	if err != nil {
		return nil, fmt.Errorf("failed to query OpenCode v2 credentials: %w", err)
	}
	defer rows.Close()
	result := make([]openCode2CredentialRow, 0)
	for rows.Next() {
		var row openCode2CredentialRow
		if err := rows.Scan(&row.ID, &row.Label, &row.Value, &row.Active, &row.TimeCreated, &row.TimeUpdated); err != nil {
			return nil, fmt.Errorf("failed to read OpenCode v2 credential: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read OpenCode v2 credentials: %w", err)
	}
	return result, nil
}

func cloneJSONMap(input map[string]any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	output := map[string]any{}
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}
	return output, nil
}

func openCode2ValueIdentity(value map[string]any, label string) (string, string) {
	metadata := asMap(value["metadata"])
	metadataID := ""
	email := ""
	if metadata != nil {
		metadataID = strings.TrimSpace(asString(metadata["accountID"]))
		if metadataID == "" {
			metadataID = strings.TrimSpace(asString(metadata["accountId"]))
		}
		email = normalizeEmail(asString(metadata["email"]))
	}
	if email == "" {
		email = normalizeEmail(asString(value["email"]))
	}

	claims := ParseAccessToken(asString(value["access"]))
	accountID := CanonicalAccountID(metadataID, claims.AccountID)
	if email == "" {
		email = normalizeEmail(claims.Email)
	}
	if email == "" && plausibleOpenCodeEmail(label) {
		email = normalizeEmail(label)
	}
	return accountID, email
}

func plausibleOpenCodeEmail(value string) bool {
	if value == "" || strings.ContainsAny(value, " \t\r\n") || strings.Count(value, "@") != 1 {
		return false
	}
	parts := strings.SplitN(value, "@", 2)
	return parts[0] != "" && parts[1] != ""
}

func openCode2CredentialMatches(value map[string]any, label string, account *Account) bool {
	if account == nil || strings.TrimSpace(asString(value["type"])) != "oauth" {
		return false
	}
	rowID, rowEmail := openCode2ValueIdentity(value, label)
	accountID := strings.TrimSpace(account.AccountID)
	if rowID != "" {
		return accountID != "" && rowID == accountID
	}
	accountEmail := normalizeEmail(account.Email)
	return rowEmail != "" && accountEmail != "" && rowEmail == accountEmail
}

func normalizeOpenCode2Account(account *Account) *Account {
	if account == nil {
		return nil
	}
	clone := *account
	clone.AccessToken = strings.TrimSpace(clone.AccessToken)
	clone.RefreshToken = strings.TrimSpace(clone.RefreshToken)
	clone.AccountID = strings.TrimSpace(clone.AccountID)
	clone.Email = strings.TrimSpace(clone.Email)
	claims := ParseAccessToken(clone.AccessToken)
	clone.AccountID = CanonicalAccountID(clone.AccountID, claims.AccountID)
	if clone.Email == "" {
		clone.Email = claims.Email
	}
	if clone.ClientID == "" {
		clone.ClientID = claims.ClientID
	}
	if clone.ExpiresAt.IsZero() {
		clone.ExpiresAt = claims.ExpiresAt
	}
	return &clone
}

func buildOpenCode2OAuthValue(account *Account, existing map[string]any) (map[string]any, error) {
	account = normalizeOpenCode2Account(account)
	if account == nil || account.AccessToken == "" {
		return nil, fmt.Errorf("OpenCode v2 OAuth access token is empty")
	}
	value, err := cloneJSONMap(existing)
	if err != nil {
		return nil, fmt.Errorf("failed to preserve OpenCode v2 credential value")
	}
	value["type"] = "oauth"
	value["methodID"] = openCode2OAuthMethod
	value["access"] = account.AccessToken
	value["refresh"] = account.RefreshToken
	expires := int64(0)
	if !account.ExpiresAt.IsZero() {
		expires = account.ExpiresAt.UnixMilli()
	}
	value["expires"] = expires
	metadata := asMap(value["metadata"])
	if metadata == nil {
		metadata = map[string]any{}
	}
	if account.AccountID != "" {
		metadata["accountID"] = account.AccountID
	} else {
		delete(metadata, "accountID")
	}
	value["metadata"] = metadata
	return value, nil
}

func newOpenCode2CredentialID(ctx context.Context, queryer openCode2Queryer) (string, error) {
	for range 8 {
		buffer := make([]byte, 16)
		if _, err := rand.Read(buffer); err != nil {
			return "", fmt.Errorf("failed to generate OpenCode v2 credential ID")
		}
		id := "cred_" + hex.EncodeToString(buffer)
		var count int
		if err := queryer.QueryRowContext(ctx, "SELECT COUNT(*) FROM credential WHERE id = ?", id).Scan(&count); err != nil {
			return "", fmt.Errorf("failed to check OpenCode v2 credential ID")
		}
		if count == 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to allocate OpenCode v2 credential ID")
}

func activateOpenCode2Credential(ctx context.Context, executor openCode2Executor, id string) error {
	if _, err := executor.ExecContext(ctx, "UPDATE credential SET active = 0 WHERE integration_id = ?", openCode2Integration); err != nil {
		return fmt.Errorf("failed to deactivate OpenCode v2 credentials: %w", err)
	}
	result, err := executor.ExecContext(ctx, "UPDATE credential SET active = 1 WHERE integration_id = ? AND id = ?", openCode2Integration, id)
	if err != nil {
		return fmt.Errorf("failed to activate OpenCode v2 credential: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return fmt.Errorf("failed to activate OpenCode v2 credential")
	}
	return nil
}

func readbackOpenCode2Credential(ctx context.Context, queryer openCode2Queryer, id string, account *Account, expectedActive bool) error {
	var valueText string
	var active sql.NullInt64
	if err := queryer.QueryRowContext(ctx, "SELECT value, active FROM credential WHERE integration_id = ? AND id = ?", openCode2Integration, id).Scan(&valueText, &active); err != nil {
		return fmt.Errorf("OpenCode v2 credential readback failed")
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(valueText), &value); err != nil {
		return fmt.Errorf("OpenCode v2 credential readback failed")
	}
	expected, err := buildOpenCode2OAuthValue(account, nil)
	if err != nil {
		return err
	}
	if asString(value["type"]) != asString(expected["type"]) ||
		asString(value["methodID"]) != asString(expected["methodID"]) ||
		asString(value["access"]) != asString(expected["access"]) ||
		asString(value["refresh"]) != asString(expected["refresh"]) {
		return fmt.Errorf("OpenCode v2 credential readback failed")
	}
	expectedExpires, _ := asInt64(expected["expires"])
	actualExpires, ok := asInt64(value["expires"])
	if !ok || actualExpires != expectedExpires {
		return fmt.Errorf("OpenCode v2 credential readback failed")
	}
	expectedID, _ := openCode2ValueIdentity(expected, "")
	actualID, _ := openCode2ValueIdentity(value, "")
	if expectedID != actualID || (active.Valid && active.Int64 == 1) != expectedActive {
		return fmt.Errorf("OpenCode v2 credential readback failed")
	}
	return nil
}

func findOpenCode2Credential(ctx context.Context, queryer openCode2Queryer, account *Account) (*openCode2CredentialRow, map[string]any, error) {
	rows, err := queryOpenCode2CredentialRows(ctx, queryer)
	if err != nil {
		return nil, nil, err
	}
	for index := range rows {
		var value map[string]any
		if err := json.Unmarshal([]byte(rows[index].Value), &value); err != nil {
			continue
		}
		if openCode2CredentialMatches(value, rows[index].Label, account) {
			return &rows[index], value, nil
		}
	}
	return nil, nil, nil
}

func upsertOpenCode2Credential(ctx context.Context, executor openCode2Executor, account *Account, activate, allowInsert bool) (string, bool, bool, error) {
	account = normalizeOpenCode2Account(account)
	if account == nil || account.AccessToken == "" {
		return "", false, false, fmt.Errorf("OpenCode v2 OAuth access token is empty")
	}
	row, existing, err := findOpenCode2Credential(ctx, executor, account)
	if err != nil {
		return "", false, false, err
	}
	if row == nil && !allowInsert {
		return "", false, false, nil
	}
	value, err := buildOpenCode2OAuthValue(account, existing)
	if err != nil {
		return "", false, false, err
	}
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return "", false, false, fmt.Errorf("failed to encode OpenCode v2 credential")
	}
	id := ""
	wasActive := false
	if row != nil {
		id = row.ID
		wasActive = row.Active.Valid && row.Active.Int64 == 1
		if _, err := executor.ExecContext(ctx, "UPDATE credential SET value = ?, time_updated = ? WHERE id = ? AND integration_id = ?", string(valueBytes), time.Now().UnixMilli(), id, openCode2Integration); err != nil {
			return "", false, false, fmt.Errorf("failed to update OpenCode v2 credential: %w", err)
		}
	} else {
		id, err = newOpenCode2CredentialID(ctx, executor)
		if err != nil {
			return "", false, false, err
		}
		label := strings.TrimSpace(account.Label)
		if label == "" {
			label = strings.TrimSpace(account.Email)
		}
		if label == "" {
			label = strings.TrimSpace(account.AccountID)
		}
		if label == "" {
			label = "OAuth"
		}
		now := time.Now().UnixMilli()
		if _, err := executor.ExecContext(ctx, `
			INSERT INTO credential (id, integration_id, label, value, active, time_created, time_updated)
			VALUES (?, ?, ?, ?, 0, ?, ?)`, id, openCode2Integration, label, string(valueBytes), now, now); err != nil {
			return "", false, false, fmt.Errorf("failed to insert OpenCode v2 credential: %w", err)
		}
	}
	if activate {
		if err := activateOpenCode2Credential(ctx, executor, id); err != nil {
			return "", false, false, err
		}
	}
	expectedActive := activate || wasActive
	if err := readbackOpenCode2Credential(ctx, executor, id, account, expectedActive); err != nil {
		return "", false, false, err
	}
	return id, row != nil, expectedActive, nil
}

func openCode2AccountFromRow(row openCode2CredentialRow, path string) (*Account, bool) {
	var value map[string]any
	if json.Unmarshal([]byte(row.Value), &value) != nil || asString(value["type"]) != "oauth" || asString(value["methodID"]) != openCode2OAuthMethod {
		return nil, false
	}
	access := strings.TrimSpace(asString(value["access"]))
	if access == "" {
		return nil, false
	}
	accountID, email := openCode2ValueIdentity(value, row.Label)
	account := &Account{Label: strings.TrimSpace(row.Label), AccessToken: access, RefreshToken: strings.TrimSpace(asString(value["refresh"])), AccountID: accountID, Email: email, Source: SourceOpenCode, FilePath: path, Writable: true}
	if expires, ok := asInt64(value["expires"]); ok && expires > 0 {
		account.ExpiresAt = time.UnixMilli(expires)
	} else if claims := ParseAccessToken(access); !claims.ExpiresAt.IsZero() {
		account.ExpiresAt = claims.ExpiresAt
	}
	return account, row.Active.Valid && row.Active.Int64 == 1
}

func loadOpenCode2Accounts(path string) ([]*Account, []*Account, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil, nil
	}
	db, err := openOpenCode2Database(path, true)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	defer cancel()
	rows, err := queryOpenCode2CredentialRows(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	accounts := make([]*Account, 0, len(rows))
	active := make([]*Account, 0, 1)
	for _, row := range rows {
		account, isActive := openCode2AccountFromRow(row, path)
		if account == nil {
			continue
		}
		accounts = append(accounts, account)
		if isActive {
			active = append(active, account)
		}
	}
	return accounts, active, nil
}

func prepareOpenCodeLegacyOAuth(root map[string]any, account *Account) {
	openai := asMap(root["openai"])
	if openai == nil {
		openai = map[string]any{}
		root["openai"] = openai
	}
	openai["type"] = "oauth"
	openai["access"] = account.AccessToken
	if value := strings.TrimSpace(account.RefreshToken); value != "" {
		openai["refresh"] = value
	} else {
		delete(openai, "refresh")
	}
	if value := strings.TrimSpace(account.AccountID); value != "" {
		openai["accountId"] = value
	} else {
		delete(openai, "accountId")
	}
	if value := strings.TrimSpace(account.Email); value != "" {
		openai["email"] = value
	} else {
		delete(openai, "email")
	}
	if !account.ExpiresAt.IsZero() {
		openai["expires"] = account.ExpiresAt.UnixMilli()
	} else {
		delete(openai, "expires")
	}
}

type openCodeJSONSnapshot struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}

type openCodeLegacyChange struct {
	path     string
	root     map[string]any
	snapshot openCodeJSONSnapshot
	written  bool
}

func captureOpenCodeJSON(path string) (openCodeJSONSnapshot, map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return openCodeJSONSnapshot{path: path}, map[string]any{}, nil
		}
		return openCodeJSONSnapshot{}, nil, fmt.Errorf("failed to read OpenCode auth file: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return openCodeJSONSnapshot{}, nil, fmt.Errorf("failed to inspect OpenCode auth file: %w", err)
	}
	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return openCodeJSONSnapshot{}, nil, fmt.Errorf("failed to decode OpenCode auth file: %w", err)
	}
	if root == nil {
		return openCodeJSONSnapshot{}, nil, fmt.Errorf("failed to decode OpenCode auth file: root must be a JSON object")
	}
	return openCodeJSONSnapshot{path: path, data: data, mode: info.Mode().Perm(), exists: true}, root, nil
}

func restoreOpenCodeJSON(snapshot openCodeJSONSnapshot) error {
	if !snapshot.exists {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writeJSONBytesAtomic(snapshot.path, snapshot.data, snapshot.mode)
}

func restoreOpenCodeChanges(changes []openCodeLegacyChange) error {
	var failures []error
	for index := len(changes) - 1; index >= 0; index-- {
		if !changes[index].written {
			continue
		}
		if err := restoreOpenCodeJSON(changes[index].snapshot); err != nil {
			failures = append(failures, fmt.Errorf("%s: %v", changes[index].path, err))
		}
	}
	return errors.Join(failures...)
}

func prepareOpenCodeLegacyChanges(capabilities openCodeCapabilities, account *Account, mode targetWriteMode) ([]openCodeLegacyChange, error) {
	changes := make([]openCodeLegacyChange, 0, len(capabilities.legacyPaths))
	for _, path := range capabilities.legacyPaths {
		snapshot, root, err := captureOpenCodeJSON(path)
		if err != nil {
			return nil, err
		}
		var existing *Account
		if openai := asMap(root["openai"]); openai != nil {
			existing = buildOpenAIAccount(openai, SourceOpenCode, path, true)
		}
		accountToWrite := chooseTargetWriteAccount(account, existing, mode)
		if accountToWrite == nil {
			continue
		}
		prepareOpenCodeLegacyOAuth(root, normalizeOpenCode2Account(accountToWrite))
		changes = append(changes, openCodeLegacyChange{path: path, root: root, snapshot: snapshot})
	}
	return changes, nil
}

func prepareOpenCodeLegacyDeletes(capabilities openCodeCapabilities, account *Account) ([]openCodeLegacyChange, error) {
	changes := make([]openCodeLegacyChange, 0, len(capabilities.legacyExistingPaths))
	for _, path := range capabilities.legacyExistingPaths {
		snapshot, root, err := captureOpenCodeJSON(path)
		if err != nil {
			return nil, err
		}
		openai := asMap(root["openai"])
		if openai == nil {
			continue
		}
		existing := buildOpenAIAccount(openai, SourceOpenCode, path, true)
		if existing == nil || !sameIdentity(account, existing) {
			continue
		}
		for _, key := range []string{"access", "refresh", "accountId", "email", "expires", "type"} {
			delete(openai, key)
		}
		changes = append(changes, openCodeLegacyChange{path: path, root: root, snapshot: snapshot})
	}
	return changes, nil
}

func rollbackOpenCodeComposite(tx *openCode2Transaction, changes []openCodeLegacyChange) error {
	var failures []error
	if tx != nil {
		if err := tx.rollback(); err != nil {
			failures = append(failures, fmt.Errorf("database rollback failed: %w", err))
		}
	}
	if err := restoreOpenCodeChanges(changes); err != nil {
		failures = append(failures, fmt.Errorf("legacy restore failed: %w", err))
	}
	return errors.Join(failures...)
}

func applyOpenCodeComposite(account *Account, mode targetWriteMode) (string, error) {
	account = normalizeOpenCode2Account(account)
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	if account.AccessToken == "" {
		return "", fmt.Errorf("OpenCode OAuth access token is empty")
	}
	var capabilities openCodeCapabilities
	var err error
	// Refresh stays on the loaded store; apply resolves current configured stores.
	if mode == targetWriteRefresh && strings.TrimSpace(account.FilePath) != "" {
		capabilities, err = detectOpenCodeCapabilitiesAtPath(account.FilePath)
	} else {
		capabilities, err = detectOpenCodeCapabilities()
	}
	if err != nil {
		return "", err
	}
	if !capabilities.Legacy && !capabilities.V2 {
		if mode == targetWriteRefresh {
			return "", nil
		}
		return "", fmt.Errorf("OpenCode store is not available")
	}
	changes, err := prepareOpenCodeLegacyChanges(capabilities, account, mode)
	if err != nil {
		return "", err
	}
	var tx *openCode2Transaction
	var v2ID string
	if capabilities.V2 {
		tx, err = beginOpenCode2Transaction(capabilities.v2Path)
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
		v2ID, _, _, err = upsertOpenCode2Credential(ctx, tx.conn, account, mode == targetWriteApply, mode == targetWriteApply)
		cancel()
		if err != nil {
			rollbackErr := rollbackOpenCodeComposite(tx, changes)
			if rollbackErr != nil {
				return "", errors.Join(fmt.Errorf("OpenCode composite apply failed: %w", err), rollbackErr)
			}
			return "", err
		}
	}
	for index := range changes {
		if err := writeJSONMap(changes[index].path, changes[index].root); err != nil {
			rollbackErr := rollbackOpenCodeComposite(tx, changes)
			if rollbackErr != nil {
				return "", errors.Join(fmt.Errorf("OpenCode composite apply failed: %w", err), rollbackErr)
			}
			return "", fmt.Errorf("OpenCode composite apply failed: %w", err)
		}
		changes[index].written = true
	}
	if tx != nil {
		if err := tx.commit(); err != nil {
			rollbackErr := rollbackOpenCodeComposite(tx, changes)
			if rollbackErr != nil {
				return "", errors.Join(fmt.Errorf("OpenCode composite apply failed: database commit: %w", err), rollbackErr)
			}
			return "", fmt.Errorf("OpenCode composite apply failed: database commit: %w", err)
		}
	}
	if len(changes) > 0 {
		return changes[0].path, nil
	}
	if v2ID != "" {
		return capabilities.v2Path, nil
	}
	return "", nil
}

func deleteOpenCode2Identity(ctx context.Context, executor openCode2Executor, account *Account) error {
	rows, err := queryOpenCode2CredentialRows(ctx, executor)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	activeDeleted := false
	for _, row := range rows {
		var value map[string]any
		if json.Unmarshal([]byte(row.Value), &value) != nil || !openCode2CredentialMatches(value, row.Label, account) {
			continue
		}
		ids = append(ids, row.ID)
		activeDeleted = activeDeleted || (row.Active.Valid && row.Active.Int64 == 1)
	}
	for _, id := range ids {
		if _, err := executor.ExecContext(ctx, "DELETE FROM credential WHERE integration_id = ? AND id = ?", openCode2Integration, id); err != nil {
			return fmt.Errorf("failed to delete OpenCode v2 credential: %w", err)
		}
	}
	if !activeDeleted {
		return nil
	}
	var fallback string
	err = executor.QueryRowContext(ctx, `
		SELECT id FROM credential
		WHERE integration_id = ?
		ORDER BY time_created DESC, id DESC
		LIMIT 1`, openCode2Integration).Scan(&fallback)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to select OpenCode v2 active fallback: %w", err)
	}
	return activateOpenCode2Credential(ctx, executor, fallback)
}

func deleteOpenCodeComposite(account *Account) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}
	account = normalizeOpenCode2Account(account)
	capabilities, err := detectOpenCodeCapabilities()
	if err != nil {
		return err
	}
	if !capabilities.Legacy && !capabilities.V2 {
		return nil
	}
	changes, err := prepareOpenCodeLegacyDeletes(capabilities, account)
	if err != nil {
		return err
	}
	var tx *openCode2Transaction
	if capabilities.V2 {
		tx, err = beginOpenCode2Transaction(capabilities.v2Path)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
		err = deleteOpenCode2Identity(ctx, tx.conn, account)
		cancel()
		if err != nil {
			rollbackErr := rollbackOpenCodeComposite(tx, changes)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("OpenCode composite delete failed: %w", err), rollbackErr)
			}
			return err
		}
	}
	for index := range changes {
		if err := writeJSONMap(changes[index].path, changes[index].root); err != nil {
			rollbackErr := rollbackOpenCodeComposite(tx, changes)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("OpenCode composite delete failed: %w", err), rollbackErr)
			}
			return fmt.Errorf("OpenCode composite delete failed: %w", err)
		}
		changes[index].written = true
	}
	if tx != nil {
		if err := tx.commit(); err != nil {
			rollbackErr := rollbackOpenCodeComposite(tx, changes)
			if rollbackErr != nil {
				return errors.Join(fmt.Errorf("OpenCode composite delete failed: database commit: %w", err), rollbackErr)
			}
			return fmt.Errorf("OpenCode composite delete failed: database commit: %w", err)
		}
	}
	return nil
}

func RestoreManagedAccountsToOpenCode2(activeAccount *Account) (int, string, error) {
	if activeAccount == nil {
		return 0, "", fmt.Errorf("OpenCode v2 active account is nil")
	}
	capabilities, err := detectOpenCodeCapabilities()
	if err != nil {
		return 0, "", err
	}
	if !capabilities.V2 {
		return 0, "", fmt.Errorf("OpenCode v2 database is not initialized")
	}
	managed, err := LoadManagedAccounts()
	if err != nil {
		return 0, "", fmt.Errorf("failed to load CQ accounts: %w", err)
	}
	if len(managed) == 0 {
		return 0, "", fmt.Errorf("cannot restore OpenCode v2 pool: no managed CQ accounts")
	}
	prepared := make([]*Account, 0, len(managed)+1)
	for _, account := range managed {
		fresh, _, refreshErr := ResolveFreshAccount(account)
		if refreshErr != nil {
			return 0, "", fmt.Errorf("cannot restore OpenCode v2 pool: account refresh failed")
		}
		if fresh == nil {
			fresh = account
		}
		fresh = normalizeOpenCode2Account(fresh)
		if fresh == nil || fresh.AccessToken == "" || (fresh.AccountID == "" && fresh.Email == "") {
			return 0, "", fmt.Errorf("cannot restore OpenCode v2 pool: invalid managed account")
		}
		duplicate := false
		for index, existing := range prepared {
			if !sameIdentity(fresh, existing) {
				continue
			}
			duplicate = true
			if freshestAccountForIdentity(fresh, []*Account{fresh, existing}) == fresh {
				prepared[index] = fresh
			}
			break
		}
		if !duplicate {
			prepared = append(prepared, fresh)
		}
	}
	activeAccount = normalizeOpenCode2Account(activeAccount)
	if activeAccount.AccessToken == "" || (activeAccount.AccountID == "" && activeAccount.Email == "") {
		return 0, "", fmt.Errorf("cannot restore OpenCode v2 pool: invalid active account")
	}
	selectedPresent := false
	for _, account := range prepared {
		if sameIdentity(account, activeAccount) {
			selectedPresent = true
			break
		}
	}
	if !selectedPresent {
		prepared = append(prepared, activeAccount)
	}

	tx, err := beginOpenCode2Transaction(capabilities.v2Path)
	if err != nil {
		return 0, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), openCode2DBTimeout)
	ids := make(map[string]string, len(prepared))
	for _, account := range prepared {
		id, _, _, upsertErr := upsertOpenCode2Credential(ctx, tx.conn, account, false, true)
		if upsertErr != nil {
			cancel()
			rollbackErr := tx.rollback()
			if rollbackErr != nil {
				return 0, "", errors.Join(upsertErr, fmt.Errorf("database rollback failed: %w", rollbackErr))
			}
			return 0, "", upsertErr
		}
		ids[accountIdentityKey(account)] = id
	}
	selectedID := ""
	for _, account := range prepared {
		if sameIdentity(account, activeAccount) {
			selectedID = ids[accountIdentityKey(account)]
			break
		}
	}
	if selectedID == "" {
		cancel()
		rollbackErr := tx.rollback()
		if rollbackErr != nil {
			return 0, "", errors.Join(fmt.Errorf("cannot restore OpenCode v2 pool: active account was not upserted"), fmt.Errorf("database rollback failed: %w", rollbackErr))
		}
		return 0, "", fmt.Errorf("cannot restore OpenCode v2 pool: active account was not upserted")
	}
	if err := activateOpenCode2Credential(ctx, tx.conn, selectedID); err != nil {
		cancel()
		rollbackErr := tx.rollback()
		if rollbackErr != nil {
			return 0, "", errors.Join(err, fmt.Errorf("database rollback failed: %w", rollbackErr))
		}
		return 0, "", err
	}
	for _, account := range prepared {
		id := ids[accountIdentityKey(account)]
		if err := readbackOpenCode2Credential(ctx, tx.conn, id, account, id == selectedID); err != nil {
			cancel()
			rollbackErr := tx.rollback()
			if rollbackErr != nil {
				return 0, "", errors.Join(err, fmt.Errorf("database rollback failed: %w", rollbackErr))
			}
			return 0, "", err
		}
	}
	cancel()
	if err := tx.commit(); err != nil {
		rollbackErr := tx.rollback()
		if rollbackErr != nil {
			return 0, "", errors.Join(fmt.Errorf("failed to commit OpenCode v2 pool restore: %w", err), fmt.Errorf("database rollback failed: %w", rollbackErr))
		}
		return 0, "", fmt.Errorf("failed to commit OpenCode v2 pool restore: %w", err)
	}
	return len(prepared), capabilities.v2Path, nil
}

func accountIdentityKey(account *Account) string {
	if account == nil {
		return ""
	}
	if id := strings.TrimSpace(account.AccountID); id != "" {
		return "account:" + id
	}
	return "email:" + normalizeEmail(account.Email)
}

func writeJSONBytesAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	if mode == 0 {
		mode = 0o600
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
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
	if err := os.Chmod(tmpPath, mode.Perm()); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
