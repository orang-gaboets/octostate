package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
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
	)

	cmd := &cobra.Command{
		Use:           "plan",
		Short:         "Preview GitOps reconciliation changes",
		Long:          "Load desired GitOps configuration, collect live GitHub state, and print a deterministic reconciliation plan without making changes.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			repo-builder config plan --token <token> --config-dir ./config
			repo-builder config plan --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --config-dir /path/to/control-repo/config`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := planConfig(
				cmd.Context(),
				token,
				appID,
				installationID,
				appKeyPath,
				configDir,
			)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, report)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	github.MarkRequiredFlags(cmd, "config-dir")

	return cmd
}

func planConfig(
	ctx context.Context,
	token string,
	appID, installationID int64,
	appKeyPath, configDir string,
) (*gitopsplan.Report, error) {
	cfg, err := loadPlanConfig(strings.TrimSpace(configDir))
	if err != nil {
		return nil, err
	}

	validation := validatePlanConfig(cfg)
	if !validation.Valid {
		return nil, errors.New("configuration is invalid; run `repo-builder config validate`")
	}

	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		return nil, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}

	client, err := newPlanClient(ctx, token, appID, installationID, appKeyPath)
	if err != nil {
		return nil, err
	}

	actual, err := collectPlanState(ctx, collector.CollectOrganizationOptions{
		OrgName:             organization,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
	})
	if err != nil {
		return nil, err
	}

	return buildPlanReport(ctx, gitopsplan.Options{
		Desired:     cfg,
		Actual:      actual,
		UserService: client.Users(),
	})
}
