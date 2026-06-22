package config

import (
	"fmt"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
)

func invalidConfigPhaseError(phase string, err error) error {
	if err == nil {
		return nil
	}
	return exitcode.New(validateExitCodeInvalidConfig, fmt.Errorf("failed to %s: %w", phase, err))
}

func runtimePhaseError(phase string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to %s: %w", phase, err)
}
