package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/auth"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func StartAddAccountLoginCmd() tea.Cmd {
	return func() tea.Msg {
		status, err := auth.StartOpenAICodexLogin()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("login failed: %w", err)}
		}
		return AddAccountLoginStartedMsg{
			AuthURL:           status.AuthURL,
			BrowserOpenFailed: status.BrowserOpenFailed,
		}
	}
}

func PollAddAccountLoginCmd() tea.Cmd {
	return tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
		account, done, err := auth.PollOpenAICodexLogin()
		if !done {
			return AddAccountLoginPendingMsg{}
		}
		return AddAccountLoginFinishedMsg{Account: account, Err: err}
	})
}

func CancelAddAccountLoginCmd() tea.Cmd {
	return func() tea.Msg {
		_ = auth.CancelOpenAICodexLogin()
		return nil
	}
}

func CopyToClipboardCmd(text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "windows":
			cmd = exec.Command("cmd", "/c", "clip")
		default:
			if _, err := exec.LookPath("wl-copy"); err == nil {
				cmd = exec.Command("wl-copy")
				break
			}
			if _, err := exec.LookPath("xclip"); err == nil {
				cmd = exec.Command("xclip", "-selection", "clipboard")
				break
			}
			if _, err := exec.LookPath("xsel"); err == nil {
				cmd = exec.Command("xsel", "--clipboard", "--input")
				break
			}
			return AddAccountLoginCopyResultMsg{Err: fmt.Errorf("no clipboard command found")}
		}

		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return AddAccountLoginCopyResultMsg{Err: fmt.Errorf("failed to copy URL: %w", err)}
		}
		return AddAccountLoginCopyResultMsg{Text: "Copied URL to clipboard."}
	}
}

func OpenAddAccountLoginURLCmd(url string) tea.Cmd {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}

	return func() tea.Msg {
		if err := auth.OpenBrowserURL(url); err != nil {
			return AddAccountLoginCopyResultMsg{Err: fmt.Errorf("failed to open browser: %w", err)}
		}
		return AddAccountLoginCopyResultMsg{Text: "Opened authorization URL in browser."}
	}
}

func FinalizeAddAccountLoginCmd(account *config.Account) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	return func() tea.Msg {
		if err := config.UpsertManagedAccount(accountSnapshot); err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to save account: %w", err)}
		}

		result, err := config.LoadAllAccountsWithSources()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to reload accounts: %w", err)}
		}

		note := "account added"
		if accountSnapshot.Email != "" {
			note = "account added: " + accountSnapshot.Email
		}

		return AccountsMsg{
			ActiveKey:               accountSnapshot.AccountID,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
			Notice:                  note,
		}
	}
}

func ApplyToTargetsCmd(account *config.Account, targets []config.Source) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	targetsSnapshot := dedupeApplyTargets(targets)

	return func() tea.Msg {
		appliedPaths, applyErrors := config.ApplyAccountToTargets(accountSnapshot, targetsSnapshot)
		if len(appliedPaths) == 0 {
			if len(applyErrors) == 0 {
				return ErrMsg{Err: fmt.Errorf("no apply target selected")}
			}
			return ErrMsg{Err: fmt.Errorf("apply failed: %s", formatTargetErrors(applyErrors))}
		}

		result, loadErr := config.LoadAllAccountsWithSources()
		if loadErr != nil {
			note := fmt.Sprintf("applied to: %s", sourceListText(mapKeysSortedBySource(appliedPaths)))
			if len(applyErrors) > 0 {
				note = fmt.Sprintf("%s (errors: %s)", note, formatTargetErrors(applyErrors))
			}
			return NoticeMsg{Text: note}
		}

		note := fmt.Sprintf("applied to: %s", sourceListText(mapKeysSortedBySource(appliedPaths)))
		if len(appliedPaths) == 1 {
			for source, path := range appliedPaths {
				note = fmt.Sprintf("applied to %s: %s", sourceDisplayName(source), filepath.Base(path))
			}
		}
		if len(applyErrors) > 0 {
			note = fmt.Sprintf("%s (errors: %s)", note, formatTargetErrors(applyErrors))
		}

		return AccountsMsg{
			ActiveKey:               accountSnapshot.Key,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
			Notice:                  note,
		}
	}
}

func DeleteAccountSourcesCmd(account *config.Account, sources []config.Source, activeKey string) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	sourcesSnapshot := dedupeSources(sources)
	activeKeySnapshot := activeKey

	return func() tea.Msg {
		deleted := make([]config.Source, 0, len(sourcesSnapshot))
		errorsList := make([]string, 0)
		for _, source := range sourcesSnapshot {
			if err := config.DeleteAccountFromSource(accountSnapshot, source); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", sourceDisplayName(source), err))
				continue
			}
			deleted = append(deleted, source)
		}

		if len(deleted) == 0 {
			errorText := "failed to delete account"
			if len(errorsList) > 0 {
				errorText = fmt.Sprintf("%s: %s", errorText, strings.Join(errorsList, "; "))
			}
			return ErrMsg{Err: fmt.Errorf("%s", errorText)}
		}

		result, err := config.LoadAllAccountsWithSources()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to reload accounts: %w", err)}
		}

		note := fmt.Sprintf("account deleted from: %s", sourceListText(deleted))
		if len(errorsList) > 0 {
			note = fmt.Sprintf("%s (errors: %s)", note, strings.Join(errorsList, "; "))
		}

		return AccountsMsg{
			ActiveKey:               activeKeySnapshot,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
			Notice:                  note,
		}
	}
}

func FetchDataCmd(account *config.Account) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	accountKey := accountSnapshot.Key

	return func() tea.Msg {
		workingAccount := *accountSnapshot
		reloadAccounts := false

		if auth.IsExpired(&workingAccount) {
			if err := auth.RefreshToken(&workingAccount); err != nil {
				return ErrMsg{AccountKey: accountKey, Err: fmt.Errorf("token refresh failed: %w", err)}
			}
		}

		data, err := api.CallAPI(workingAccount.AccessToken, workingAccount.AccountID)
		if err != nil && api.IsUnauthorized(err) && workingAccount.RefreshToken != "" {
			if refreshErr := auth.RefreshToken(&workingAccount); refreshErr != nil {
				return ErrMsg{AccountKey: accountKey, Err: fmt.Errorf("token refresh failed: %w", refreshErr)}
			}
			data, err = api.CallAPI(workingAccount.AccessToken, workingAccount.AccountID)
		}

		if err != nil {
			return ErrMsg{AccountKey: accountKey, Err: err}
		}

		if strings.TrimSpace(workingAccount.Email) == "" {
			email, name, fetchErr := auth.FetchUserEmail(workingAccount.AccessToken)
			if fetchErr == nil && strings.TrimSpace(email) != "" {
				workingAccount.Email = strings.TrimSpace(email)
				workingAccount.Label = workingAccount.Email
				if strings.TrimSpace(workingAccount.Label) == "" {
					workingAccount.Label = strings.TrimSpace(name)
				}
				_ = config.SaveAccount(&workingAccount)
				reloadAccounts = true
			}
		}

		reloadActiveKey := accountKey
		if strings.TrimSpace(workingAccount.AccountID) != "" {
			reloadActiveKey = workingAccount.AccountID
		}
		return DataMsg{
			AccountKey:      accountKey,
			Data:            data,
			Account:         &workingAccount,
			ReloadAccounts:  reloadAccounts,
			ReloadActiveKey: reloadActiveKey,
		}
	}
}

func ReloadAccountsCmd(activeKey string) tea.Cmd {
	return func() tea.Msg {
		result, err := config.LoadAllAccountsWithSources()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to reload accounts: %w", err)}
		}
		return AccountsMsg{
			ActiveKey:               activeKey,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
		}
	}
}

func scheduleNoticeClearCmd(seq int) tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg {
		return NoticeTimeoutMsg{Seq: seq}
	})
}

func animationTickCmd() tea.Cmd {
	return tea.Tick(animationFrameInterval, func(now time.Time) tea.Msg {
		return AnimationFrameMsg{Now: now}
	})
}

func SaveUIStateCmd(compact bool) tea.Cmd {
	return SaveUIStateSnapshotCmd(config.UIState{CompactMode: compact})
}

func SaveUIStateSnapshotCmd(state config.UIState) tea.Cmd {
	return func() tea.Msg {
		_ = config.SaveUIState(state)
		return nil
	}
}

func DismissUpdateVersionCmd(version string) tea.Cmd {
	return func() tea.Msg {
		_ = config.SetDismissedUpdateVersion(version)
		return nil
	}
}

func cloneAccount(account *config.Account) *config.Account {
	if account == nil {
		return nil
	}

	cloned := *account
	return &cloned
}

func (m Model) findAccountByKey(accountKey string) *config.Account {
	if accountKey == "" {
		return nil
	}
	for _, account := range m.Accounts {
		if account == nil {
			continue
		}
		if account.Key == accountKey {
			return account
		}
	}
	return nil
}

func (m *Model) applyAccountSnapshot(accountKey string, snapshot *config.Account) {
	if snapshot == nil || accountKey == "" {
		return
	}

	for _, account := range m.Accounts {
		if account == nil || account.Key != accountKey {
			continue
		}

		account.AccessToken = snapshot.AccessToken
		account.RefreshToken = snapshot.RefreshToken
		account.ExpiresAt = snapshot.ExpiresAt
		if snapshot.ClientID != "" {
			account.ClientID = snapshot.ClientID
		}
		if snapshot.AccountID != "" {
			account.AccountID = snapshot.AccountID
		}
		if snapshot.Email != "" {
			account.Email = snapshot.Email
		}
		if snapshot.Label != "" {
			account.Label = snapshot.Label
		}

		return
	}
}

func (m *Model) fetchNextCmd() tea.Cmd {
	if m.UsageData == nil {
		m.UsageData = make(map[string]api.UsageData)
	}
	if m.LoadingMap == nil {
		m.LoadingMap = make(map[string]bool)
	}
	if m.ErrorsMap == nil {
		m.ErrorsMap = make(map[string]error)
	}

	const maxConcurrentLoads = 3
	currentlyLoading := 0
	for _, isLoading := range m.LoadingMap {
		if isLoading {
			currentlyLoading++
		}
	}
	if currentlyLoading >= maxConcurrentLoads {
		return nil
	}
	availableSlots := maxConcurrentLoads - currentlyLoading

	checkAccount := func(acc *config.Account) tea.Cmd {
		if acc == nil || acc.Key == "" {
			return nil
		}
		if m.LoadingMap[acc.Key] {
			return nil
		}
		_, hasData := m.UsageData[acc.Key]
		_, hasErr := m.ErrorsMap[acc.Key]

		if !hasData && !hasErr {
			m.LoadingMap[acc.Key] = true
			return FetchDataCmd(acc)
		}
		return nil
	}

	cmds := make([]tea.Cmd, 0, availableSlots)
	orderedAccounts := m.Accounts
	if m.CompactMode {
		orderedAccounts = make([]*config.Account, 0, len(m.Accounts))
		for _, idx := range m.compactVisualOrderIndices() {
			if idx < 0 || idx >= len(m.Accounts) {
				continue
			}
			orderedAccounts = append(orderedAccounts, m.Accounts[idx])
		}
	}

	for _, acc := range orderedAccounts {
		if cmd := checkAccount(acc); cmd != nil {
			cmds = append(cmds, cmd)
			availableSlots--
			if availableSlots == 0 {
				return tea.Batch(cmds...)
			}
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
