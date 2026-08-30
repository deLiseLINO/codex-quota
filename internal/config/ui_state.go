package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type UIState struct {
	CompactMode          bool              `json:"compact_mode"`
	ExhaustedAccountKeys []string          `json:"exhausted_account_keys"`
	AccountOrderKeys     []string          `json:"account_order_keys"`
	ActiveAccountKey     string            `json:"active_account_key"`
	PlanTypes            map[string]string `json:"plan_types,omitempty"`
	LastApplyTargets     []string          `json:"last_apply_targets,omitempty"`
}

func LoadUIState() (UIState, error) {
	path, err := uiStatePath()
	if err != nil {
		return UIState{}, err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return UIState{}, nil
		}
		return UIState{}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	state := UIState{}
	if compact, ok := root["compact_mode"].(bool); ok {
		state.CompactMode = compact
	}
	if exhaustedAny, ok := root["exhausted_account_keys"].([]any); ok {
		keys := make([]string, 0, len(exhaustedAny))
		for _, raw := range exhaustedAny {
			key, ok := raw.(string)
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			keys = append(keys, key)
		}
		state.ExhaustedAccountKeys = keys
	}
	if orderAny, ok := root["account_order_keys"].([]any); ok {
		keys := make([]string, 0, len(orderAny))
		for _, raw := range orderAny {
			key, ok := raw.(string)
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			keys = append(keys, key)
		}
		state.AccountOrderKeys = keys
	}
	if activeKey, ok := root["active_account_key"].(string); ok {
		state.ActiveAccountKey = strings.TrimSpace(activeKey)
	}
	if planTypesAny, ok := root["plan_types"].(map[string]any); ok {
		planTypes := make(map[string]string, len(planTypesAny))
		for key, raw := range planTypesAny {
			value, ok := raw.(string)
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			planTypes[key] = value
		}
		state.PlanTypes = planTypes
	}
	if targetsAny, ok := root["last_apply_targets"].([]any); ok {
		targets := make([]string, 0, len(targetsAny))
		seen := make(map[string]bool, len(targetsAny))
		for _, raw := range targetsAny {
			target, ok := raw.(string)
			target = strings.TrimSpace(target)
			if !ok || target == "" || seen[target] {
				continue
			}
			seen[target] = true
			targets = append(targets, target)
		}
		state.LastApplyTargets = targets
	}

	return state, nil
}

func SaveUIState(state UIState) error {
	path, err := uiStatePath()
	if err != nil {
		return err
	}

	root := map[string]any{
		"compact_mode":           state.CompactMode,
		"exhausted_account_keys": state.ExhaustedAccountKeys,
		"account_order_keys":     state.AccountOrderKeys,
		"active_account_key":     strings.TrimSpace(state.ActiveAccountKey),
	}
	if len(state.PlanTypes) > 0 {
		root["plan_types"] = state.PlanTypes
	}
	if targets := normalizeLastApplyTargets(state.LastApplyTargets); len(targets) > 0 {
		root["last_apply_targets"] = targets
	}
	if err := writeJSONMap(path, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

func normalizeLastApplyTargets(values []string) []string {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		seen[strings.TrimSpace(value)] = true
	}
	ordered := []string{"codex", "opencode", "pi", "omp"}
	targets := make([]string, 0, len(ordered))
	for _, target := range ordered {
		if seen[target] {
			targets = append(targets, target)
		}
	}
	return targets
}

func uiStatePath() (string, error) {
	dir, err := codexQuotaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ui_state.json"), nil
}
