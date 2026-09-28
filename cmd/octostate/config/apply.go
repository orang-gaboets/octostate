package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsapply "github.com/orang-gaboets/octostate/pkg/gitops/apply"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

var (
	loadApplyConfig     = gitopsconfig.LoadDir
	validateApplyConfig = gitopsconfig.Validate
	newApplyClient      = auth.NewClient
	collectApplyState   = collector.CollectOrganization
	buildApplyPlan      = gitopsplan.Build
	checkApply          = gitopsapply.Check
	executeApply        = gitopsapply.Execute
)

type applyRunMode int

const (
	applyRunModeApply applyRunMode = iota
	applyRunModeDryRun
	applyRunModeCheck
)

// ApplyConfigCmd creates the config apply command.
func ApplyConfigCmd() *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		configDir      string
		expectedOrg    string
		check          bool
		dryRun         bool
		requireExec    bool
	)

	cmd := &cobra.Command{
		Use:           "apply",
		Short:         "Apply or preflight GitOps reconciliation changes",
		Long:          "Load desired GitOps configuration, collect live GitHub state, build a reconciliation plan, preflight the supported create and update actions, and execute them when not running in check or dry-run mode.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate config apply --config-dir ./config --check
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate config apply --config-dir ./config --dry-run
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate config apply --config-dir ./config
			octostate config apply --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --config-dir /path/to/control-repo/config`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if check && dryRun {
				err := exitcode.New(validateExitCodeInvalidConfig, errors.New("cannot use --check and --dry-run together"))
				printInvalidConfigError(cmd, err)
				return err
			}
			// Dry run never evaluates the requirement, so accepting the flag
			// there would make a safety flag silently ineffective. Rejected
			// rather than quietly ignored.
			if dryRun && requireExec {
				err := exitcode.New(validateExitCodeInvalidConfig, errors.New("cannot use --require-executable with --dry-run; use --check or a live apply"))
				printInvalidConfigError(cmd, err)
				return err
			}

			mode := applyRunModeApply
			switch {
			case check:
				mode = applyRunModeCheck
			case dryRun:
				mode = applyRunModeDryRun
			}
			var expected *string
			if cmd.Flags().Changed("expected-org") {
				expected = &expectedOrg
			}

			result, preview, checkResult, err := applyConfig(
				cmd.Context(),
				token,
				appID,
				installationID,
				appKeyPath,
				configDir,
				mode,
				requireExec,
				expected,
			)
			if err != nil {
				printInvalidConfigError(cmd, err)
				return err
			}
			switch mode {
			case applyRunModeCheck:
				return cmdoutput.PrintCheck(
					cmd,
					fmt.Sprintf("checked %d GitOps reconciliation action(s)", checkResult.PlanSummary.ExecutableActions),
					checkResult,
				)
			case applyRunModeDryRun:
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("would apply %d GitOps reconciliation action(s)", preview.PlanSummary.ExecutableActions),
					preview,
				)
			default:
				return cmdoutput.PrintSuccess(
					cmd,
					fmt.Sprintf("applied %d GitOps reconciliation action(s)", len(result.Executed)),
					result,
				)
			}
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)
	cmd.Flags().BoolVar(&check, "check", false, "Run apply preflight checks without mutating GitHub")
	cmd.Flags().BoolVar(&requireExec, "require-executable", false,
		"Fail when a desired create or update cannot be executed. Unsupported delete/remove drift is still reported without failing")
	safety.AddDryRunFlag(cmd, &dryRun)

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	cmd.Flags().StringVar(&expectedOrg, "expected-org", "", "Independently expected GitHub organization for this live operation")
	github.MarkRequiredFlags(cmd, "config-dir")

	return cmd
}

func applyConfig(
	ctx context.Context,
	token string,
	appID, installationID int64,
	appKeyPath, configDir string,
	mode applyRunMode,
	requireExecutable bool,
	expectedOrg *string,
) (*gitopsapply.Result, *planPreview, *gitopsapply.CheckResult, error) {
	cfg, err := loadApplyConfig(strings.TrimSpace(configDir))
	if err != nil {
		return nil, nil, nil, invalidConfigPhaseError("load config", err)
	}

	validation := validateApplyConfig(cfg)
	if !validation.Valid {
		return nil, nil, nil, exitcode.New(validateExitCodeInvalidConfig, errors.New("configuration is invalid; run `octostate config validate`"))
	}

	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		return nil, nil, nil, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}
	if expectedOrg != nil {
		if err := gitopsconfig.CheckExpectedOrganization(organization, *expectedOrg); err != nil {
			return nil, nil, nil, invalidConfigPhaseError("verify expected organization", err)
		}
	}

	client, err := newApplyClient(ctx, token, appID, installationID, appKeyPath)
	if err != nil {
		return nil, nil, nil, runtimePhaseError("create GitHub client", err)
	}

	actual, err := collectApplyState(ctx, collector.CollectOrganizationOptions{
		OrgName:             organization,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
	})
	if err != nil {
		return nil, nil, nil, runtimePhaseError("collect live GitHub state", err)
	}

	planOptions := gitopsplan.Options{
		Desired:     cfg,
		Actual:      actual,
		UserService: client.Users(),
	}
	var report *gitopsplan.Report
	if expectedOrg != nil {
		report, err = gitopsplan.BuildForOrganization(ctx, planOptions, *expectedOrg)
	} else {
		report, err = buildApplyPlan(ctx, planOptions)
	}
	if err != nil {
		return nil, nil, nil, runtimePhaseError("build reconciliation plan", err)
	}
	report.Normalize()

	switch mode {
	case applyRunModeCheck:
		applyOptions := gitopsapply.Options{
			Desired:                         cfg,
			Actual:                          actual,
			Plan:                            report,
			RequireExecutableDesiredActions: requireExecutable,
			OrganizationService:             client.Organizations(),
			RepositoryService:               client.Repositories(),
			TeamService:                     client.Teams(),
			UserService:                     client.Users(),
		}
		var checkResult *gitopsapply.CheckResult
		if expectedOrg != nil {
			checkResult, err = gitopsapply.CheckForOrganization(ctx, applyOptions, *expectedOrg)
		} else {
			checkResult, err = checkApply(ctx, applyOptions)
		}
		if err != nil {
			return nil, nil, nil, runtimePhaseError("run apply preflight check", err)
		}
		checkResult.Normalize()
		return nil, nil, checkResult, nil
	case applyRunModeDryRun:
		preview := previewFromPlan(report)
		preview.Normalize()
		return nil, preview, nil, nil
	default:
		applyOptions := gitopsapply.Options{
			Desired:                         cfg,
			Actual:                          actual,
			Plan:                            report,
			RequireExecutableDesiredActions: requireExecutable,
			OrganizationService:             client.Organizations(),
			RepositoryService:               client.Repositories(),
			TeamService:                     client.Teams(),
			UserService:                     client.Users(),
		}
		var result *gitopsapply.Result
		if expectedOrg != nil {
			result, err = gitopsapply.ExecuteForOrganization(ctx, applyOptions, *expectedOrg)
		} else {
			result, err = executeApply(ctx, applyOptions)
		}
		if err != nil {
			return nil, nil, nil, runtimePhaseError("execute apply plan", err)
		}
		result.Normalize()
		return result, nil, nil, nil
	}
}
