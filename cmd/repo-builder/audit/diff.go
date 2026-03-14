package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsdiff "github.com/orang-gaboets/repo-builder/pkg/gitops/diff"
	gitopssnapshot "github.com/orang-gaboets/repo-builder/pkg/gitops/snapshot"
)

const auditDiffExitCodeDrift = 2

var (
	loadAuditDiffConfig     = gitopsconfig.LoadDir
	validateAuditDiffConfig = gitopsconfig.Validate
	readAuditSnapshot       = gitopssnapshot.ReadActual
	buildAuditDiffReport    = gitopsdiff.Build
)

// DiffCmd creates the audit diff command.
func DiffCmd() *cobra.Command {
	var (
		configDir   string
		stateDir    string
		failOnDrift bool
	)

	cmd := &cobra.Command{
		Use:           "diff",
		Short:         "Compare desired state against the stored snapshot",
		Long:          "Load desired GitOps configuration, load the stored actual-state snapshot, and print a deterministic offline drift report without talking to GitHub.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			repo-builder audit diff --config-dir ./config --state-dir ./state
			repo-builder audit diff --config-dir /path/to/control-repo/config --state-dir /path/to/control-repo/state --fail-on-drift`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, driftDetected, err := auditDiff(cmd.Context(), configDir, stateDir)
			if err != nil {
				return err
			}
			if err := cmdoutput.PrintJSON(cmd, report); err != nil {
				return err
			}
			if failOnDrift && driftDetected {
				return exitcode.New(auditDiffExitCodeDrift, errors.New("drift detected"))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "Path to the state directory containing actual/snapshot.json")
	cmd.Flags().BoolVar(&failOnDrift, "fail-on-drift", false, "Exit with code 2 when drift is detected")

	github.MarkRequiredFlags(cmd, "config-dir", "state-dir")

	return cmd
}

func auditDiff(
	_ context.Context,
	configDir, stateDir string,
) (*gitopsdiff.Report, bool, error) {
	cfg, err := loadAuditDiffConfig(strings.TrimSpace(configDir))
	if err != nil {
		return nil, false, err
	}

	validation := validateAuditDiffConfig(cfg)
	if !validation.Valid {
		return nil, false, errors.New("configuration is invalid; run `repo-builder config validate`")
	}

	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		return nil, false, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}

	actualSnapshot, err := readAuditSnapshot(strings.TrimSpace(stateDir))
	if err != nil {
		return nil, false, err
	}

	report, err := buildAuditDiffReport(gitopsdiff.Options{
		Desired:                         cfg,
		Snapshot:                        actualSnapshot,
		ResolvedInviteUserIDsByUsername: actualSnapshot.ResolvedInviteUserIDsByUsername,
	})
	if err != nil {
		return nil, false, err
	}

	return report, report.Summary.HasChanges, nil
}
