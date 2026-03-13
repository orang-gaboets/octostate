package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitopsapply "github.com/orang-gaboets/repo-builder/pkg/gitops/apply"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
)

var (
	loadApplyConfig     = gitopsconfig.LoadDir
	validateApplyConfig = gitopsconfig.Validate
	newApplyClient      = auth.NewClient
	collectApplyState   = collector.CollectOrganization
	buildApplyPlan      = gitopsplan.Build
	executeApply        = gitopsapply.Execute
)

// ApplyConfigCmd creates the config apply command.
func ApplyConfigCmd() *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		configDir      string
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:           "apply",
		Short:         "Apply GitOps reconciliation changes",
		Long:          "Load desired GitOps configuration, collect live GitHub state, build a reconciliation plan, and execute the supported create and update actions.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			repo-builder config apply --token <token> --config-dir ./config --dry-run
			repo-builder config apply --token <token> --config-dir ./config
			repo-builder config apply --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --config-dir /path/to/control-repo/config`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, preview, err := applyConfig(
				cmd.Context(),
				token,
				appID,
				installationID,
				appKeyPath,
				configDir,
				dryRun,
			)
			if err != nil {
				return err
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("would apply %d GitOps reconciliation action(s)", preview.PlanSummary.ExecutableActions),
					preview,
				)
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("applied %d GitOps reconciliation action(s)", len(result.Executed)),
				result,
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)
	safety.AddDryRunFlag(cmd, &dryRun)

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	github.MarkRequiredFlags(cmd, "config-dir")

	return cmd
}

type applyPreview struct {
	Organization      string              `json:"organization"`
	PlanSummary       gitopsplan.Summary  `json:"plan_summary"`
	ExecutableActions []gitopsplan.Action `json:"executable_actions"`
	SkippedActions    []gitopsplan.Action `json:"skipped_actions"`
}

func (p *applyPreview) Normalize() {
	if p == nil {
		return
	}
	if p.ExecutableActions == nil {
		p.ExecutableActions = []gitopsplan.Action{}
	}
	if p.SkippedActions == nil {
		p.SkippedActions = []gitopsplan.Action{}
	}
}

func applyConfig(
	ctx context.Context,
	token string,
	appID, installationID int64,
	appKeyPath, configDir string,
	dryRun bool,
) (*gitopsapply.Result, *applyPreview, error) {
	cfg, err := loadApplyConfig(strings.TrimSpace(configDir))
	if err != nil {
		return nil, nil, err
	}

	validation := validateApplyConfig(cfg)
	if !validation.Valid {
		return nil, nil, exitcode.New(validateExitCodeInvalidConfig, errors.New("configuration is invalid; run `repo-builder config validate`"))
	}

	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		return nil, nil, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}

	client, err := newApplyClient(ctx, token, appID, installationID, appKeyPath)
	if err != nil {
		return nil, nil, err
	}

	actual, err := collectApplyState(ctx, collector.CollectOrganizationOptions{
		OrgName:             organization,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
	})
	if err != nil {
		return nil, nil, err
	}

	report, err := buildApplyPlan(ctx, gitopsplan.Options{
		Desired:     cfg,
		Actual:      actual,
		UserService: client.Users(),
	})
	if err != nil {
		return nil, nil, err
	}
	report.Normalize()

	if dryRun {
		preview := previewFromPlan(report)
		preview.Normalize()
		return nil, preview, nil
	}

	result, err := executeApply(ctx, gitopsapply.Options{
		Desired:             cfg,
		Actual:              actual,
		Plan:                report,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
		UserService:         client.Users(),
	})
	if err != nil {
		return nil, nil, err
	}
	result.Normalize()
	return result, nil, nil
}

func previewFromPlan(report *gitopsplan.Report) *applyPreview {
	preview := &applyPreview{
		Organization: strings.TrimSpace(report.Organization),
		PlanSummary:  report.Summary,
	}
	for _, action := range report.Actions {
		if action.Executable {
			preview.ExecutableActions = append(preview.ExecutableActions, action)
			continue
		}
		preview.SkippedActions = append(preview.SkippedActions, action)
	}
	return preview
}
