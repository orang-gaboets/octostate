package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/v2/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/v2/cmd/octostate/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/octostate/v2/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/v2/pkg/github"
	"github.com/orang-gaboets/octostate/v2/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/v2/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/v2/pkg/gitops/plan"
)

var (
	loadPlanConfig     = gitopsconfig.LoadDir
	validatePlanConfig = gitopsconfig.Validate
	newPlanClient      = auth.NewClient
	collectPlanState   = collector.CollectOrganization
	buildPlanReport    = gitopsplan.Build
)

// PlanConfigCmd creates the config planning command.
func PlanConfigCmd() *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		configDir      string
		expectedOrg    string
	)

	cmd := &cobra.Command{
		Use:           "plan",
		Short:         "Preview GitOps reconciliation changes",
		Long:          "Load desired GitOps configuration, verify the required caller-supplied expected organization, collect live GitHub state, and print a deterministic reconciliation plan without making changes.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate config plan --config-dir ./config --expected-org <expected-org>
			octostate config plan --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --config-dir /path/to/control-repo/config --expected-org <expected-org>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := planConfig(
				cmd.Context(),
				token,
				appID,
				installationID,
				appKeyPath,
				configDir,
				expectedOrg,
			)
			if err != nil {
				printInvalidConfigError(cmd, err)
				return err
			}
			report.Normalize()
			preview := previewFromPlan(report)
			preview.Normalize()
			return cmdoutput.PrintJSON(cmd, preview)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	cmd.Flags().StringVar(&expectedOrg, "expected-org", "", "Independently expected GitHub organization for this live operation")
	github.MarkRequiredFlags(cmd, "config-dir", "expected-org")

	return cmd
}

func planConfig(
	ctx context.Context,
	token string,
	appID, installationID int64,
	appKeyPath, configDir string,
	expectedOrg string,
) (*gitopsplan.Report, error) {
	cfg, err := loadPlanConfig(strings.TrimSpace(configDir))
	if err != nil {
		return nil, invalidConfigPhaseError("load config", err)
	}

	validation := validatePlanConfig(cfg)
	if !validation.Valid {
		return nil, exitcode.New(validateExitCodeInvalidConfig, errors.New("configuration is invalid; run `octostate config validate`"))
	}

	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		return nil, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}
	if err := gitopsconfig.CheckExpectedOrganization(organization, expectedOrg); err != nil {
		return nil, invalidConfigPhaseError("verify expected organization", err)
	}

	client, err := newPlanClient(ctx, token, appID, installationID, appKeyPath)
	if err != nil {
		return nil, runtimePhaseError("create GitHub client", err)
	}

	actual, err := collectPlanState(ctx, collector.CollectOrganizationOptions{
		OrgName:             organization,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
	}, expectedOrg)
	if err != nil {
		return nil, runtimePhaseError("collect live GitHub state", err)
	}

	planOptions := gitopsplan.Options{
		Desired:     cfg,
		Actual:      actual,
		UserService: client.Users(),
	}
	report, err := buildPlanReport(ctx, planOptions, expectedOrg)
	if err != nil {
		return nil, runtimePhaseError("build reconciliation plan", err)
	}
	return report, nil
}
