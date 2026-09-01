package config

import (
	"errors"
	"reflect"
	"testing"
)

func TestInstalledApplyTargetsUsesDeterministicSupportedOrder(t *testing.T) {
	installed := map[string]bool{"omp": true, "codex": true, "pi": true}
	got := installedApplyTargets(func(command string) (string, error) {
		if installed[command] {
			return "/bin/" + command, nil
		}
		return "", errors.New("not found")
	})
	want := []Source{SourceCodex, SourcePi, SourceOMP}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installed targets = %v, want %v", got, want)
	}
}

func TestInstalledApplyTargetsCanBeEmpty(t *testing.T) {
	got := installedApplyTargets(func(string) (string, error) {
		return "", errors.New("not found")
	})
	if len(got) != 0 {
		t.Fatalf("installed targets = %v, want none", got)
	}
}
