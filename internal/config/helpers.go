package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func asMap(value any) map[string]any {
	if value == nil {
		return nil
	}

	typed, ok := value.(map[string]any)
	if ok {
		return typed
	}

	typedInterface, ok := value.(map[string]interface{})
	if ok {
		result := make(map[string]any, len(typedInterface))
		for k, v := range typedInterface {
			result[k] = v
		}
		return result
	}

	return nil
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case []byte:
		return string(typed)
	default:
		return ""
	}
}

func asInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		return int64(typed), true
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed > ^uint64(0)>>1 {
			return 0, false
		}
		return int64(typed), true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case jsonNumber:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func shortAccountID(accountID string) string {
	trimmed := strings.TrimSpace(accountID)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 12 {
		return trimmed
	}
	return trimmed[:6] + "..." + trimmed[len(trimmed)-4:]
}

func CanonicalAccountID(ids ...string) string {
	trimmed := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		trimmed = append(trimmed, id)
	}
	if len(trimmed) == 0 {
		return ""
	}

	for _, id := range trimmed {
		if isUUIDLike(id) {
			return id
		}
	}

	return trimmed[0]
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ActiveIdentityKeys(account *Account) []string {
	if account == nil {
		return nil
	}

	keys := make([]string, 0, 5)
	if accountID := strings.TrimSpace(account.AccountID); accountID != "" {
		keys = append(keys, "account:"+accountID)
	}
	if id := identityID(account); id != "" && id != strings.TrimSpace(account.AccountID) {
		keys = append(keys, "account:"+id)
	}
	if email := normalizeEmail(account.Email); email != "" {
		keys = append(keys, "email:"+email)
	}
	if tokenKey := tokenKey("access", account.AccessToken); tokenKey != "" {
		keys = append(keys, tokenKey)
	}
	if tokenKey := tokenKey("refresh", account.RefreshToken); tokenKey != "" {
		keys = append(keys, tokenKey)
	}

	return keys
}

func tokenKey(prefix, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return prefix + ":" + hex.EncodeToString(sum[:8])
}

func isUUIDLike(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHexRune(r) {
				return false
			}
		}
	}
	return true
}

func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

type jsonNumber interface {
	Int64() (int64, error)
}
