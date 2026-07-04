package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

const (
	tokenURL = "https://auth.openai.com/oauth/token"
)

type refreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
}

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func IsExpired(account *config.Account) bool {
	if account == nil {
		return true
	}

	if account.ExpiresAt.IsZero() {
		claims := config.ParseAccessToken(account.AccessToken)
		if !claims.ExpiresAt.IsZero() {
			account.ExpiresAt = claims.ExpiresAt
		}
	}

	if account.ExpiresAt.IsZero() {
		return false
	}

	return time.Now().After(account.ExpiresAt.Add(-5 * time.Minute))
}

func RefreshToken(account *config.Account) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}
	if fresh, changed, err := config.ResolveFreshAccount(account); err != nil {
		return fmt.Errorf("failed to resolve fresh account before refresh: %w", err)
	} else if changed && fresh != nil {
		copyResolvedAccount(account, fresh)
	}
	if account.RefreshToken == "" {
		return fmt.Errorf("refresh token is missing")
	}

	clientID := strings.TrimSpace(account.ClientID)
	if clientID == "" {
		claims := config.ParseAccessToken(account.AccessToken)
		clientID = claims.ClientID
	}
	if clientID == "" {
		return fmt.Errorf("cannot refresh token: missing client_id; re-login required")
	}

	body, err := json.Marshal(refreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: account.RefreshToken,
		ClientID:     clientID,
	})
	if err != nil {
		return fmt.Errorf("failed to build refresh request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, tokenURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(responseBody))
		if len(bodyText) > 500 {
			bodyText = bodyText[:500]
		}
		return fmt.Errorf("refresh failed with status %d: %s", resp.StatusCode, bodyText)
	}

	var tokenResp refreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("received empty access token")
	}

	account.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		account.RefreshToken = tokenResp.RefreshToken
	}

	claims := config.ParseAccessToken(tokenResp.AccessToken)
	if claims.ClientID != "" {
		account.ClientID = claims.ClientID
	} else {
		account.ClientID = clientID
	}
	if claims.AccountID != "" {
		account.AccountID = config.CanonicalAccountID(account.AccountID, claims.AccountID)
	}

	if tokenResp.ExpiresIn > 0 {
		account.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else if !claims.ExpiresAt.IsZero() {
		account.ExpiresAt = claims.ExpiresAt
	}

	if err := config.SyncAccountEverywhere(account); err != nil {
		return fmt.Errorf("failed to persist refreshed token: %w", err)
	}

	return nil
}

func copyResolvedAccount(target, source *config.Account) {
	if target == nil || source == nil {
		return
	}
	target.Label = source.Label
	target.Email = source.Email
	target.AccountID = source.AccountID
	target.AccessToken = source.AccessToken
	target.RefreshToken = source.RefreshToken
	target.ExpiresAt = source.ExpiresAt
	target.ClientID = source.ClientID
	target.Source = source.Source
	target.FilePath = source.FilePath
	target.Writable = source.Writable
}
