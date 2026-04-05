package config

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

const (
	validateExitCodeInvalidConfig = 2
)

// ValidateConfigCmd creates the config validation command.
func ValidateConfigCmd() *cobra.Command {
	var configDir string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate GitOps desired-state configuration",
		Long: "Validate a local GitOps configuration directory by loading " +
			"organization.yaml with strict schema checks and semantic rules.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			octostate config validate --config-dir ./config
			octostate config validate --config-dir /path/to/control-repo/config`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := validateConfig(configDir)
			if printErr := cmdoutput.PrintJSON(cmd, report); printErr != nil {
				return printErr
			}
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	github.MarkRequiredFlags(cmd, "config-dir")

	return cmd
}

func validateConfig(configDir string) (gitopsconfig.ValidationReport, error) {
	cfg, err := gitopsconfig.LoadDir(configDir)
	if err != nil {
		return reportForLoadError(err), exitcode.New(1, err)
	}

	report := gitopsconfig.Validate(cfg)
	if !report.Valid {
		return report, exitcode.New(validateExitCodeInvalidConfig, errors.New("configuration is invalid"))
	}

	return report, nil
}

func reportForLoadError(err error) gitopsconfig.ValidationReport {
	issue := gitopsconfig.ValidationIssue{
		Code:    gitopsconfig.ValidationIssueCodeLoadError,
		Message: err.Error(),
	}

	var loadErr *gitopsconfig.LoadError
	if errors.As(err, &loadErr) {
		issue.Code = gitopsconfig.ValidationIssueCode(loadErr.Kind)
		issue.Path = loadErr.Path
	}

	return gitopsconfig.ValidationReport{
		Valid: false,
		Summary: gitopsconfig.ValidationSummary{
			Errors: 1,
		},
		Errors:   []gitopsconfig.ValidationIssue{issue},
		Warnings: []gitopsconfig.ValidationIssue{},
	}
}
