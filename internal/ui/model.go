package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
	"github.com/deLiseLINO/codex-quota/internal/update"
)

type Model struct {
	defaultProgress         progress.Model
	shortProgress           progress.Model
	Data                    api.UsageData
	Loading                 bool
	DeleteSourceSelect      bool
	DeleteSourceOptions     []config.Source
	DeleteSources           map[config.Source]bool
	DeleteSourceCursor      int
	DeleteConfirm           bool
	ApplyTargetSelect       bool
	ApplyTargets            map[config.Source]bool
	ApplyTargetCursor       int
	ApplyConfirm            bool
	HelpVisible             bool
	ActionMenuVisible       bool
	ActionMenuCursor        int
	AddAccountLoginVisible  bool
	AddAccountLoginURL      string
	AddAccountBrowserFailed bool
	AddAccountLoginStatus   string
	ShowInfo                bool
	Notice                  string
	noticeSeq               int
	Err                     error
	Width                   int
	Height                  int
	CompactMode             bool
	UsageData               map[string]api.UsageData
	PlanTypeByAccount       map[string]string
	LoadingMap              map[string]bool
	ErrorsMap               map[string]error
	ExhaustedSticky         map[string]bool
	Accounts                []*config.Account
	SourcesByAccountID      map[string][]string
	ActiveSourcesByIdentity map[string][]string
	ActiveAccountIx         int
	compactBarAnimations    map[string]compactBarAnimation
	tabWindowAnimations     map[string]tabWindowAnimation
	animationTicking        bool
	UpdatePromptVisible     bool
	UpdatePromptVersion     string
	UpdatePromptMethod      update.Method
	UpdatePromptCursor      int
	UpdateAvailableHint     string
	pendingUpdateMethod     update.Method
	hasPendingUpdateMethod  bool
}

type StartupUpdatePrompt struct {
	Version string
	Method  update.Method
}

func InitialModel(
	accounts []*config.Account,
	sourcesByAccountID map[string][]string,
	activeSourcesByIdentity map[string][]string,
	initialCompactMode bool,
) Model {
	return InitialModelWithStartupUpdate(
		accounts,
		sourcesByAccountID,
		activeSourcesByIdentity,
		config.UIState{CompactMode: initialCompactMode},
		nil,
	)
}

func InitialModelWithUIState(
	accounts []*config.Account,
	sourcesByAccountID map[string][]string,
	activeSourcesByIdentity map[string][]string,
	uiState config.UIState,
) Model {
	return InitialModelWithStartupUpdate(accounts, sourcesByAccountID, activeSourcesByIdentity, uiState, nil)
}

func InitialModelWithStartupUpdate(
	accounts []*config.Account,
	sourcesByAccountID map[string][]string,
	activeSourcesByIdentity map[string][]string,
	uiState config.UIState,
	startupUpdate *StartupUpdatePrompt,
) Model {
	defaultProgress := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)
	shortProgress := progress.New(
		progress.WithGradient("#4285F4", "#34A853"),
		progress.WithoutPercentage(),
	)

	m := Model{
		defaultProgress:         defaultProgress,
		shortProgress:           shortProgress,
		Loading:                 len(accounts) > 0,
		Accounts:                nil,
		SourcesByAccountID:      sourcesByAccountID,
		ActiveSourcesByIdentity: activeSourcesByIdentity,
		ActiveAccountIx:         0,
		CompactMode:             uiState.CompactMode,
		UsageData:               make(map[string]api.UsageData),
		PlanTypeByAccount:       make(map[string]string),
		LoadingMap:              make(map[string]bool),
		ErrorsMap:               make(map[string]error),
		ExhaustedSticky:         make(map[string]bool),
		compactBarAnimations:    make(map[string]compactBarAnimation),
		tabWindowAnimations:     make(map[string]tabWindowAnimation),
		UpdatePromptCursor:      0,
	}
	m.Accounts = accounts
	if startupUpdate != nil {
		version := strings.TrimSpace(startupUpdate.Version)
		if version != "" && update.SupportsAutoUpdate(startupUpdate.Method) {
			m.UpdatePromptVersion = version
			m.UpdatePromptMethod = startupUpdate.Method
			m.UpdatePromptVisible = true
		}
	}

	for _, key := range uiState.ExhaustedAccountKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		m.ExhaustedSticky[key] = true
	}
	m.pruneExhaustedSticky()
	m.normalizeActiveAccountForView("")

	if account := m.activeAccount(); account != nil {
		m.LoadingMap[account.Key] = true
	}

	return m
}

func (m Model) Init() tea.Cmd {
	titleCmd := tea.SetWindowTitle("🚀 Codex Quota Monitor")
	if account := m.activeAccount(); account != nil {
		return tea.Batch(titleCmd, FetchDataCmd(account), m.fetchNextCmd())
	}
	return titleCmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if m.AddAccountLoginVisible && msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			if m.addAccountLoginURLContainsPoint(msg.X, msg.Y) {
				return m, OpenAddAccountLoginURLCmd(m.AddAccountLoginURL)
			}
		}

	case tea.KeyMsg:
		rawKey := msg.String()
		keyStr := normalizeHelpKey(rawKey, normalizeKey(rawKey))

		if m.UpdatePromptVisible {
			return m.handleUpdatePrompt(msg, keyStr)
		}
		if m.HelpVisible {
			return m.handleHelpOverlay(keyStr)
		}
		if m.AddAccountLoginVisible {
			return m.handleAddAccountLogin(keyStr)
		}
		if m.ActionMenuVisible {
			return m.handleActionMenu(keyStr)
		}
		if m.DeleteSourceSelect {
			return m.handleDeleteSourceSelection(keyStr)
		}
		if m.DeleteConfirm {
			return m.handleDeleteConfirm(keyStr)
		}
		if m.ApplyTargetSelect {
			return m.handleApplyTargetSelection(keyStr)
		}
		if m.ApplyConfirm {
			return m.handleApplyConfirm(keyStr)
		}

		switch keyStr {
		case "u":
			if !m.openUpdatePrompt() {
				return m, nil
			}
			return m, nil

		case "help":
			m.openHelpOverlay()
			return m, nil

		case "enter":
			if m.activeAccount() == nil {
				return m, nil
			}
			m.openActionMenu()
			return m, nil

		case "x", "delete":
			return m.beginDeleteFlow()

		case "esc":
			if m.ShowInfo {
				m.ShowInfo = false
				return m, nil
			}
			if m.Err != nil {
				m.Err = nil
				return m, nil
			}
			if m.Notice != "" {
				m.Notice = ""
				return m, nil
			}
			return m, tea.Quit

		case "q", "ctrl+c":
			return m, tea.Quit

		case "r":
			return m.beginRefreshActive()

		case "R":
			return m.beginRefreshAll()

		case "i":
			m.resetHelpState()
			m.resetActionMenuState()
			m.ShowInfo = !m.ShowInfo
			m.resetDeleteState()
			m.resetApplyState()
			m.Notice = ""
			return m, nil

		case "v", "c":
			return m.toggleViewMode()

		case "n":
			return m.beginAddAccount()

		case "o":
			return m.beginApplyFlow()

		case "right", "l", "down", "j":
			if len(m.Accounts) > 1 {
				if m.CompactMode {
					m.moveActiveAccountCompact(1)
				} else {
					m.ActiveAccountIx = (m.ActiveAccountIx + 1) % len(m.Accounts)
				}
				m.syncActiveAccount()
				return m, tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
			}

		case "left", "h", "up", "k":
			if len(m.Accounts) > 1 {
				if m.CompactMode {
					m.moveActiveAccountCompact(-1)
				} else {
					m.ActiveAccountIx = (m.ActiveAccountIx - 1 + len(m.Accounts)) % len(m.Accounts)
				}
				m.syncActiveAccount()
				return m, tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		barWidth := msg.Width - 72
		if barWidth < 20 {
			barWidth = 20
		}
		if barWidth > 50 {
			barWidth = 50
		}
		m.defaultProgress.Width = barWidth
		m.shortProgress.Width = barWidth

	case AccountsMsg:
		m.Accounts = msg.Accounts
		m.SourcesByAccountID = msg.SourcesByAccountID
		m.ActiveSourcesByIdentity = msg.ActiveSourcesByIdentity
		m.ActiveAccountIx = 0
		m.Data = api.UsageData{}
		m.pruneCompactBarAnimations()
		m.pruneKnownPlanTypes()
		stickyPruned := m.pruneExhaustedSticky()
		m.clearTabWindowAnimations()
		m.resetDeleteState()
		m.resetApplyState()

		if len(m.Accounts) == 0 {
			m.Loading = false
			m.Err = fmt.Errorf("no accounts found; press n to add account")
			m.Notice = ""
			if stickyPruned {
				return m, SaveUIStateSnapshotCmd(m.uiStateSnapshot())
			}
			return m, nil
		}

		m.normalizeActiveAccountForView(msg.ActiveKey)

		m.Loading = true
		m.Err = nil
		m.Notice = msg.Notice

		if m.LoadingMap == nil {
			m.LoadingMap = make(map[string]bool)
		}

		var fetchCmd tea.Cmd
		if m.activeAccount() != nil {
			m.LoadingMap[m.activeAccountKey()] = true
			fetchCmd = FetchDataCmd(m.activeAccount())
		}

		cmds := []tea.Cmd{fetchCmd, m.fetchNextCmd()}
		if stickyPruned {
			cmds = append(cmds, SaveUIStateSnapshotCmd(m.uiStateSnapshot()))
		}
		if msg.Notice != "" {
			m.noticeSeq++
			cmds = append(cmds, scheduleNoticeClearCmd(m.noticeSeq))
		}
		return m, tea.Batch(cmds...)

	case DataMsg:
		prevSnapshot := cloneAccount(m.findAccountByKey(msg.AccountKey))
		m.applyAccountSnapshot(msg.AccountKey, msg.Account)

		if m.UsageData == nil {
			m.UsageData = make(map[string]api.UsageData)
			m.LoadingMap = make(map[string]bool)
			m.ErrorsMap = make(map[string]error)
		}
		if m.ExhaustedSticky == nil {
			m.ExhaustedSticky = make(map[string]bool)
		}

		var (
			prevData    api.UsageData
			hadPrevData bool
			wasLoading  bool
		)
		stickyChanged := false
		if msg.AccountKey != "" {
			prevData, hadPrevData = m.UsageData[msg.AccountKey]
			wasLoading = m.LoadingMap[msg.AccountKey]
			m.UsageData[msg.AccountKey] = msg.Data
			m.setKnownPlanType(msg.AccountKey, msg.Data.PlanType)
			stickyChanged = m.setExhaustedStickyIfConfirmed(msg.AccountKey, msg.Data) || stickyChanged
			m.LoadingMap[msg.AccountKey] = false
			delete(m.ErrorsMap, msg.AccountKey)
			if m.CompactMode {
				m.startCompactBarAnimation(msg.AccountKey, prevData, hadPrevData, msg.Data, wasLoading)
			} else {
				delete(m.compactBarAnimations, msg.AccountKey)
			}
			if msg.AccountKey == m.activeAccountKey() {
				m.startTabWindowAnimations(msg.AccountKey, prevData, hadPrevData, msg.Data, wasLoading, tabLoadAnimationDuration)
			}
		}
		cmds := []tea.Cmd{m.fetchNextCmd(), m.ensureAnimationTickCmd()}
		if stickyChanged {
			cmds = append(cmds, SaveUIStateSnapshotCmd(m.uiStateSnapshot()))
		}
		nextCmd := tea.Batch(cmds...)

		if msg.AccountKey != "" && msg.AccountKey != m.activeAccountKey() {
			return m, nextCmd
		}
		m.Data = msg.Data
		m.Loading = false
		m.Err = nil
		if msg.ReloadAccounts {
			activeKey := msg.ReloadActiveKey
			if activeKey == "" {
				activeKey = msg.AccountKey
			}
			return m, tea.Batch(ReloadAccountsCmd(activeKey), nextCmd)
		}
		if prevSnapshot != nil && msg.Account != nil {
			prevEmail := strings.TrimSpace(prevSnapshot.Email)
			nextEmail := strings.TrimSpace(msg.Account.Email)
			prevID := strings.TrimSpace(prevSnapshot.AccountID)
			nextID := strings.TrimSpace(msg.Account.AccountID)
			if (prevEmail == "" && nextEmail != "") || (prevID != "" && nextID != "" && prevID != nextID) {
				return m, tea.Batch(ReloadAccountsCmd(msg.AccountKey), nextCmd)
			}
		}
		return m, nextCmd

	case NoticeMsg:
		m.Loading = false
		m.Err = nil
		m.Notice = msg.Text
		if msg.Text == "" {
			return m, nil
		}
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)

	case NoticeTimeoutMsg:
		if msg.Seq != m.noticeSeq {
			return m, nil
		}
		m.Notice = ""
		return m, nil

	case AddAccountLoginStartedMsg:
		m.AddAccountLoginVisible = true
		m.AddAccountLoginURL = strings.TrimSpace(msg.AuthURL)
		m.AddAccountBrowserFailed = msg.BrowserOpenFailed
		m.AddAccountLoginStatus = ""
		m.Loading = false
		m.Err = nil
		m.Notice = ""
		return m, PollAddAccountLoginCmd()

	case AddAccountLoginPendingMsg:
		if !m.AddAccountLoginVisible {
			return m, nil
		}
		return m, PollAddAccountLoginCmd()

	case AddAccountLoginFinishedMsg:
		if !m.AddAccountLoginVisible {
			return m, nil
		}
		m.AddAccountLoginVisible = false
		m.AddAccountLoginURL = ""
		m.AddAccountBrowserFailed = false
		m.AddAccountLoginStatus = ""
		m.Loading = false
		if msg.Err != nil {
			m.Err = fmt.Errorf("login failed: %w", msg.Err)
			return m, nil
		}
		if msg.Account == nil {
			m.Err = fmt.Errorf("login failed: empty account result")
			return m, nil
		}
		return m, FinalizeAddAccountLoginCmd(msg.Account)

	case AddAccountLoginCopyResultMsg:
		if !m.AddAccountLoginVisible {
			return m, nil
		}
		if msg.Err != nil {
			m.AddAccountLoginStatus = msg.Err.Error()
			return m, nil
		}
		m.AddAccountLoginStatus = strings.TrimSpace(msg.Text)
		return m, nil

	case ErrMsg:
		if m.ErrorsMap == nil {
			m.ErrorsMap = make(map[string]error)
			m.LoadingMap = make(map[string]bool)
		}
		if msg.AccountKey != "" {
			m.ErrorsMap[msg.AccountKey] = msg.Err
			m.LoadingMap[msg.AccountKey] = false
			delete(m.compactBarAnimations, msg.AccountKey)
			if msg.AccountKey == m.activeAccountKey() {
				m.clearTabWindowAnimations()
			}
		}
		nextCmd := tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
		if msg.AccountKey != "" && msg.AccountKey != m.activeAccountKey() {
			return m, nextCmd
		}
		m.Loading = false
		m.Err = msg.Err
		m.Notice = ""
		m.AddAccountLoginVisible = false
		m.AddAccountLoginURL = ""
		m.AddAccountBrowserFailed = false
		m.AddAccountLoginStatus = ""
		m.resetDeleteState()
		m.resetApplyState()
		return m, nextCmd

	case UpdateAvailableMsg:
		version := strings.TrimSpace(msg.Version)
		if version == "" {
			return m, nil
		}
		if update.SupportsAutoUpdate(msg.Method) {
			m.UpdatePromptMethod = msg.Method
		}
		if m.UpdatePromptVersion == "" || update.IsNewer(version, m.UpdatePromptVersion) {
			m.UpdatePromptVersion = version
		}
		if update.SupportsAutoUpdate(m.UpdatePromptMethod) {
			m.UpdateAvailableHint = "Update available • press u"
		}
		return m, nil

	case progress.FrameMsg:
		defaultModel, defaultCmd := m.defaultProgress.Update(msg)
		m.defaultProgress = defaultModel.(progress.Model)

		shortModel, shortCmd := m.shortProgress.Update(msg)
		m.shortProgress = shortModel.(progress.Model)

		return m, tea.Batch(defaultCmd, shortCmd)

	case AnimationFrameMsg:
		if !m.advanceAnimations(msg.Now) {
			m.animationTicking = false
			return m, nil
		}
		return m, animationTickCmd()
	}

	return m, nil
}

func (m Model) PendingUpdate() (update.Method, bool) {
	return m.pendingUpdateMethod, m.hasPendingUpdateMethod
}
