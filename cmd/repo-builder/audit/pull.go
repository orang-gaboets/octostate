package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	ghusers "github.com/orang-gaboets/repo-builder/pkg/github/users"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopssnapshot "github.com/orang-gaboets/repo-builder/pkg/gitops/snapshot"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

var (
	loadAuditConfig          = gitopsconfig.LoadDir
	collectAuditState        = collector.CollectOrganization
	newAuditClient           = auth.NewClient
	newAuditSnapshot         = gitopssnapshot.NewActualSnapshot
	writeActualSnapshot      = gitopssnapshot.WriteActual
	nowAuditSnapshotTime     = time.Now
	resolveAuditInviteLogins = resolveInviteLoginsByUserID
)

// PullCmd creates the audit pull command.
func PullCmd() *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		configDir      string
		stateDir       string
	)

	cmd := &cobra.Command{
		Use:           "pull",
		Short:         "Pull actual GitHub state into a snapshot",
		Long:          "Collect the current actual GitHub state for the configured organization and persist it as a JSON snapshot under state/actual/.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			repo-builder audit pull --token <token> --config-dir ./config --state-dir ./state
			repo-builder audit pull --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --config-dir /path/to/control-repo/config --state-dir /path/to/control-repo/state`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := pullActualState(
				cmd.Context(),
				token,
				appID,
				installationID,
				appKeyPath,
				configDir,
				stateDir,
			)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(cmd, "wrote actual-state snapshot", result)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing organization.yaml")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "Path to the state directory where actual/snapshot.json will be written")

	github.MarkRequiredFlags(cmd, "config-dir", "state-dir")

	return cmd
}

type auditPullResult struct {
	Path         string    `json:"path"`
	Organization string    `json:"organization"`
	PulledAt     time.Time `json:"pulled_at"`
}

func pullActualState(
	ctx context.Context,
	token string,
	appID, installationID int64,
	appKeyPath, configDir, stateDir string,
) (auditPullResult, error) {
	cfg, err := loadAuditConfig(strings.TrimSpace(configDir))
	if err != nil {
		return auditPullResult{}, err
	}
	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		return auditPullResult{}, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}

	client, err := newAuditClient(ctx, token, appID, installationID, appKeyPath)
	if err != nil {
		return auditPullResult{}, err
	}

	actual, err := collectActualState(ctx, client, organization)
	if err != nil {
		return auditPullResult{}, err
	}

	snapshot := newAuditSnapshot(nowAuditSnapshotTime(), actual)
	snapshot.ResolvedInviteLoginsByUserID, err = resolveAuditInviteLogins(ctx, client.Users(), cfg)
	if err != nil {
		return auditPullResult{}, err
	}
	path, err := writeActualSnapshot(strings.TrimSpace(stateDir), snapshot)
	if err != nil {
		return auditPullResult{}, fmt.Errorf("write actual-state snapshot: %w", err)
	}

	return auditPullResult{
		Path:         path,
		Organization: snapshot.Organization,
		PulledAt:     snapshot.PulledAt,
	}, nil
}

func collectActualState(ctx context.Context, client auth.Client, organization string) (*state.OrganizationState, error) {
	return collectAuditState(ctx, collector.CollectOrganizationOptions{
		OrgName:             organization,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
	})
}

func resolveInviteLoginsByUserID(
	ctx context.Context,
	service ghusers.Service,
	cfg gitopsconfig.OrganizationConfig,
) (map[int64]string, error) {
	resolved := make(map[int64]string)

	for _, invite := range cfg.Invites {
		if !invite.UserID.Present || invite.UserID.Null {
			continue
		}
		userID := invite.UserID.Value
		if userID <= 0 {
			continue
		}
		if _, ok := resolved[userID]; ok {
			continue
		}

		user, err := ghusers.GetUserByID(ctx, ghusers.GetUserByIDOptions{
			Service: service,
			ID:      userID,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve snapshot invite user_id %d: %w", userID, err)
		}
		login := strings.TrimSpace(derefString(user.Login))
		if login == "" {
			return nil, fmt.Errorf("resolve snapshot invite user_id %d: missing login: %w", userID, github.ErrInvalidFieldValue)
		}
		resolved[userID] = login
	}

	return resolved, nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
