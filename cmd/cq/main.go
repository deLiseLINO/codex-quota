package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/config"
	"github.com/deLiseLINO/codex-quota/internal/ui"
	"github.com/deLiseLINO/codex-quota/internal/update"
	"github.com/deLiseLINO/codex-quota/internal/version"
)

type command int

const (
	commandRunTUI command = iota
	commandHelp
	commandVersion
	commandUpgrade
)

var (
	currentVersionFn   = version.Current
	detectUpdateMethod = update.DetectMethod
	fetchLatestVersion = update.FetchLatestVersion
	runUpgradeFn       = update.RunUpgrade
	runInteractiveFn   = runInteractive
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd, err := parseCommand(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		io.WriteString(stderr, usageText())
		return 2
	}

	switch cmd {
	case commandHelp:
		io.WriteString(stdout, usageText())
		return 0
	case commandVersion:
		fmt.Fprintln(stdout, currentVersionFn())
		return 0
	case commandUpgrade:
		return runUpgradeCommand(stdout, stderr)
	default:
		if err := runInteractiveFn(stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return commandRunTUI, nil
	}

	switch args[0] {
	case "--help", "-h", "help":
		if len(args) > 1 {
			return commandHelp, fmt.Errorf("help does not accept additional arguments")
		}
		return commandHelp, nil
	case "--version", "-version", "-v", "version":
		if len(args) > 1 {
			return commandVersion, fmt.Errorf("version does not accept additional arguments")
		}
		return commandVersion, nil
	case "upgrade":
		if len(args) > 1 {
			return commandUpgrade, fmt.Errorf("upgrade does not accept additional arguments")
		}
		return commandUpgrade, nil
	default:
		return commandRunTUI, fmt.Errorf("unknown command or flag: %s", args[0])
	}
}

func usageText() string {
	return strings.TrimSpace(`
Usage:
  cq
  cq --help
  cq --version
  cq upgrade

Commands:
  --help      Show this help text
  --version   Print the current cq version
  upgrade     Upgrade cq when the install method is known
`) + "\n"
}

func runUpgradeCommand(stdout, stderr io.Writer) int {
	method := detectUpdateMethod()
	currentVersion := currentVersionFn()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latestVersion, latestErr := fetchLatestVersion(ctx, method)
	if method == update.MethodUnknown {
		if latestErr != nil {
			fmt.Fprintf(stderr, "warning: failed to resolve latest version: %v\n", latestErr)
		}
		fmt.Fprintln(stdout, update.ManualUpgradeInstructions(currentVersion, latestVersion))
		return 1
	}

	if latestErr == nil && !update.IsNewer(latestVersion, currentVersion) {
		fmt.Fprintf(stdout, "cq is already up to date (%s)\n", currentVersion)
		return 0
	}

	if latestErr != nil {
		fmt.Fprintf(stderr, "warning: failed to resolve latest version: %v\n", latestErr)
	}

	if err := runUpgradeFn(method, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

func runInteractive(stdout, stderr io.Writer) error {
	loadResult, err := config.LoadAllAccountsWithSources()
	if err != nil {
		return fmt.Errorf("failed to load accounts: %w", err)
	}

	uiState, uiStateErr := config.LoadUIState()
	if uiStateErr != nil {
		fmt.Fprintf(stderr, "warning: failed to load ui state: %v\n", uiStateErr)
	}

	settings, settingsErr := config.LoadSettings()
	if settingsErr != nil {
		fmt.Fprintf(stderr, "warning: failed to load settings: %v\n", settingsErr)
		settings = config.DefaultSettings()
	}

	updateState, updateStateErr := config.LoadUpdateState()
	if updateStateErr != nil {
		fmt.Fprintf(stderr, "warning: failed to load update state: %v\n", updateStateErr)
	}

	method := detectUpdateMethod()
	currentVersion := currentVersionFn()
	var startupUpdate *ui.StartupUpdatePrompt
	if latestVersion, ok := update.ShouldPrompt(settings, updateState, currentVersion, method); ok {
		startupUpdate = &ui.StartupUpdatePrompt{
			Version: latestVersion,
			Method:  method,
		}
	}

	program := tea.NewProgram(
		ui.InitialModelWithStartupUpdate(
			loadResult.Accounts,
			loadResult.SourcesByAccountID,
			loadResult.ActiveSourcesByIdentity,
			uiState,
			startupUpdate,
		),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if settings.CheckForUpdateOnStartup && update.ShouldRefresh(updateState, time.Now()) {
		startBackgroundRefresh(program, updateState, method, currentVersion, settings)
	}
	model, err := program.Run()
	if err != nil {
		return err
	}

	finalModel, ok := model.(ui.Model)
	if !ok {
		return nil
	}

	if pendingMethod, ok := finalModel.PendingUpdate(); ok {
		return runUpgradeFn(pendingMethod, stdout, stderr)
	}

	return nil
}

func startBackgroundRefresh(program *tea.Program, state config.UpdateState, method update.Method, currentVersion string, settings config.Settings) {
	go func(prev config.UpdateState, detectedMethod update.Method) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		refreshed, err := update.RefreshState(ctx, prev, detectedMethod)
		if err != nil {
			return
		}

		if currentState, err := config.LoadUpdateState(); err == nil {
			refreshed.DismissedVersion = currentState.DismissedVersion
		}

		if err := config.SaveUpdateState(refreshed); err != nil {
			return
		}

		if latestVersion, ok := update.ShouldPrompt(settings, refreshed, currentVersion, detectedMethod); ok && program != nil {
			program.Send(ui.UpdateAvailableMsg{Version: latestVersion, Method: detectedMethod})
		}
	}(state, method)
}
