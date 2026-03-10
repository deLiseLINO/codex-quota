package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

const (
	oauthClientID   = "app_EMoamEEZ73f0CkXaXp7hrann"
	authorizeURL    = "https://auth.openai.com/oauth/authorize"
	oauthTokenURL   = "https://auth.openai.com/oauth/token"
	redirectURI     = "http://localhost:1455/auth/callback"
	oauthScope      = "openid profile email offline_access"
	callbackAddress = "127.0.0.1:1455"
)

type tokenExchangeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type meResponse struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type OpenAICodexLoginStatus struct {
	AuthURL           string
	BrowserOpenFailed bool
}

var ErrLoginCancelled = errors.New("login cancelled")

type openAICodexLoginSession struct {
	authURL           string
	browserOpenFailed bool
	listener          net.Listener
	server            *http.Server
	done              chan struct{}
	result            *config.Account
	err               error
	finishOnce        sync.Once
}

var (
	openAICodexLoginMu     sync.Mutex
	activeOpenAICodexLogin *openAICodexLoginSession
)

func LoginOpenAICodex() (*config.Account, error) {
	if _, err := StartOpenAICodexLogin(); err != nil {
		return nil, err
	}

	for {
		account, done, err := PollOpenAICodexLogin()
		if done {
			return account, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func StartOpenAICodexLogin() (OpenAICodexLoginStatus, error) {
	openAICodexLoginMu.Lock()
	if activeOpenAICodexLogin != nil && activeOpenAICodexLogin.isDone() {
		activeOpenAICodexLogin = nil
	}
	if activeOpenAICodexLogin != nil {
		status := OpenAICodexLoginStatus{
			AuthURL:           activeOpenAICodexLogin.authURL,
			BrowserOpenFailed: activeOpenAICodexLogin.browserOpenFailed,
		}
		openAICodexLoginMu.Unlock()
		return status, nil
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		openAICodexLoginMu.Unlock()
		return OpenAICodexLoginStatus{}, err
	}
	state, err := randomHex(16)
	if err != nil {
		openAICodexLoginMu.Unlock()
		return OpenAICodexLoginStatus{}, err
	}

	listener, err := net.Listen("tcp", callbackAddress)
	if err != nil {
		openAICodexLoginMu.Unlock()
		return OpenAICodexLoginStatus{}, fmt.Errorf("failed to bind callback server at %s: %w", callbackAddress, err)
	}
	session := &openAICodexLoginSession{
		authURL:  buildAuthorizeURL(state, challenge),
		listener: listener,
		done:     make(chan struct{}),
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/auth/callback" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.URL.Query().Get("state") != state {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("State mismatch"))
				session.finish(nil, fmt.Errorf("state mismatch"))
				return
			}
			code := r.URL.Query().Get("code")
			if code == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Missing code"))
				session.finish(nil, fmt.Errorf("missing authorization code"))
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Authentication successful. You can close this window."))
			go func() {
				tokenResp, err := exchangeCodeForToken(code, verifier)
				if err != nil {
					session.finish(nil, err)
					return
				}

				account, err := accountFromTokenResponse(tokenResp)
				session.finish(account, err)
			}()
		}),
	}
	session.server = server
	activeOpenAICodexLogin = session
	openAICodexLoginMu.Unlock()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			session.finish(nil, err)
		}
	}()

	if err := openBrowser(session.authURL); err != nil {
		session.browserOpenFailed = true
		fmt.Fprintf(os.Stderr, "failed to open browser automatically: %v\n", err)
		fmt.Fprintf(os.Stderr, "open this URL manually to continue login:\n%s\n", session.authURL)
	}

	go func() {
		select {
		case <-time.After(5 * time.Minute):
			session.finish(nil, fmt.Errorf("authentication timed out; open %s", session.authURL))
		case <-session.done:
		}
	}()

	return OpenAICodexLoginStatus{
		AuthURL:           session.authURL,
		BrowserOpenFailed: session.browserOpenFailed,
	}, nil
}

func PollOpenAICodexLogin() (*config.Account, bool, error) {
	openAICodexLoginMu.Lock()
	session := activeOpenAICodexLogin
	if session == nil {
		openAICodexLoginMu.Unlock()
		return nil, true, ErrLoginCancelled
	}
	if !session.isDone() {
		openAICodexLoginMu.Unlock()
		return nil, false, nil
	}
	activeOpenAICodexLogin = nil
	openAICodexLoginMu.Unlock()
	return session.result, true, session.err
}

func CancelOpenAICodexLogin() error {
	openAICodexLoginMu.Lock()
	session := activeOpenAICodexLogin
	activeOpenAICodexLogin = nil
	openAICodexLoginMu.Unlock()
	if session == nil {
		return nil
	}
	session.finish(nil, ErrLoginCancelled)
	return nil
}

func (s *openAICodexLoginSession) isDone() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *openAICodexLoginSession) finish(account *config.Account, err error) {
	s.finishOnce.Do(func() {
		s.result = account
		s.err = err
		if s.server != nil {
			shutdownServer(s.server)
		}
		if s.listener != nil {
			_ = s.listener.Close()
		}
		close(s.done)
	})
}

func accountFromTokenResponse(tokenResp *tokenExchangeResponse) (*config.Account, error) {
	account := &config.Account{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ClientID:     oauthClientID,
		Source:       config.SourceManaged,
		Writable:     true,
	}
	if tokenResp.ExpiresIn > 0 {
		account.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	claims := config.ParseAccessToken(account.AccessToken)
	account.AccountID = config.CanonicalAccountID(account.AccountID, claims.AccountID)
	if claims.ClientID != "" {
		account.ClientID = claims.ClientID
	}

	account.Email = claims.Email
	if account.Email == "" {
		if email, _, err := fetchUserEmail(account.AccessToken); err == nil {
			account.Email = email
		}
	}
	if account.Email != "" {
		account.Label = account.Email
	}

	if account.AccountID == "" {
		return nil, fmt.Errorf("failed to extract account_id from token")
	}

	return account, nil
}

func fetchUserEmail(accessToken string) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/me", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(body))
		if len(bodyText) > 300 {
			bodyText = bodyText[:300]
		}
		return "", "", fmt.Errorf("me endpoint returned %d: %s", resp.StatusCode, bodyText)
	}

	var payload meResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}

	email := strings.TrimSpace(payload.Email)
	name := strings.TrimSpace(payload.Name)
	return email, name, nil
}

func FetchUserEmail(accessToken string) (string, string, error) {
	return fetchUserEmail(accessToken)
}

func buildAuthorizeURL(state, challenge string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", oauthClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", oauthScope)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)
	params.Set("id_token_add_organizations", "true")
	params.Set("codex_cli_simplified_flow", "true")
	params.Set("originator", "codex-quota")
	return authorizeURL + "?" + params.Encode()
}

func exchangeCodeForToken(code, verifier string) (*tokenExchangeResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", oauthClientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, oauthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(body))
		if len(bodyText) > 500 {
			bodyText = bodyText[:500]
		}
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, bodyText)
	}

	var tokenResp tokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("token response missing fields")
	}

	return &tokenResp, nil
}

func generatePKCE() (string, string, error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomHex(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported OS for auto-open: %s", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}

func OpenBrowserURL(url string) error {
	return openBrowser(url)
}
