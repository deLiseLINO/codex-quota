package config

import "os/exec"

type applyHarness struct {
	source  Source
	command string
}

var applyHarnesses = [...]applyHarness{
	{SourceCodex, "codex"},
	{SourceOpenCode, "opencode"},
	{SourcePi, "pi"},
	{SourceOMP, "omp"},
}

func SupportedApplyTargets() []Source {
	targets := make([]Source, 0, len(applyHarnesses))
	for _, harness := range applyHarnesses {
		targets = append(targets, harness.source)
	}
	return targets
}

func InstalledApplyTargets() []Source {
	capabilities, err := detectOpenCodeCapabilities()
	if err != nil {
		capabilities = openCodeCapabilities{}
	}
	return installedApplyTargetsWithOpenCode(exec.LookPath, capabilities)
}

func installedApplyTargetsWithOpenCode(lookPath func(string) (string, error), capabilities openCodeCapabilities) []Source {
	targets := make([]Source, 0, len(applyHarnesses))
	for _, harness := range applyHarnesses {
		if harness.source == SourceOpenCode {
			if capabilities.Legacy || capabilities.V2 {
				targets = append(targets, harness.source)
			}
			continue
		}
		if _, err := lookPath(harness.command); err == nil {
			targets = append(targets, harness.source)
		}
	}
	return targets
}

func installedApplyTargets(lookPath func(string) (string, error)) []Source {
	targets := make([]Source, 0, len(applyHarnesses))
	for _, harness := range applyHarnesses {
		if _, err := lookPath(harness.command); err == nil {
			targets = append(targets, harness.source)
		}
	}
	return targets
}
