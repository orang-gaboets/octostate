package config

import (
	"strings"

	"github.com/spf13/cobra"

	gitopsconfig "github.com/orang-gaboets/octostate/v2/pkg/gitops/config"
)

func requireExpectedOrganizationFlag(cmd *cobra.Command, expected string) error {
	if strings.TrimSpace(expected) != "" {
		return nil
	}

	err := invalidConfigPhaseError("verify expected organization", &gitopsconfig.ExpectedOrganizationError{
		Kind: gitopsconfig.ExpectedOrganizationErrorMissingExpected,
	})
	printInvalidConfigError(cmd, err)
	return err
}
