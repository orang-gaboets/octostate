package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/internal/orderedtasks"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	ghusers "github.com/orang-gaboets/repo-builder/pkg/github/users"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopssnapshot "github.com/orang-gaboets/repo-builder/pkg/gitops/snapshot"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

var (
	loadAuditConfig           = gitopsconfig.LoadDir
	collectAuditState         = collector.CollectOrganization
	newAuditClient            = auth.NewClient
	newAuditSnapshot          = gitopssnapshot.NewActualSnapshot
	writeActualSnapshot       = gitopssnapshot.WriteActual
	nowAuditSnapshotTime      = time.Now
	resolveAuditInviteUserIDs = resolveInviteUserIDsByUsername
)

const auditInviteUserLookupConcurrency = 8

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

	snapshot, err := buildActualSnapshot(ctx, client, actual)
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

func buildActualSnapshot(
	ctx context.Context,
	client auth.Client,
	actual *state.OrganizationState,
) (gitopssnapshot.ActualSnapshot, error) {
	snapshot := newAuditSnapshot(nowAuditSnapshotTime(), actual)

	resolvedInviteUserIDsByUsername, err := resolveAuditInviteUserIDs(ctx, client.Users(), snapshot.PendingInvitations)
	if err != nil {
		return gitopssnapshot.ActualSnapshot{}, err
	}
	snapshot.ResolvedInviteUserIDsByUsername = resolvedInviteUserIDsByUsername

	return snapshot, nil
}

func resolveInviteUserIDsByUsername(
	ctx context.Context,
	service ghusers.Service,
	invitations []state.PendingInvitation,
) (map[string]int64, error) {
	usernames := uniqueInviteUsernames(invitations)
	resolved := make(map[string]int64, len(usernames))
	if len(usernames) == 0 {
		return resolved, nil
	}

	resolvedIDs := make([]int64, len(usernames))
	tasks := make([]orderedtasks.Task, 0, len(usernames))
	for i, username := range usernames {
		index := i
		login := username
		tasks = append(tasks, func(groupCtx context.Context) error {
			user, err := ghusers.GetUserByUsername(groupCtx, ghusers.GetUserByUsernameOptions{
				Service:  service,
				Username: login,
			})
			if err != nil {
				return fmt.Errorf("resolve snapshot invite username %q: %w", login, err)
			}
			if user == nil {
				return fmt.Errorf("resolve snapshot invite username %q: missing user: %w", login, github.ErrInvalidFieldValue)
			}
			userID := derefInt64(user.ID)
			if userID <= 0 {
				return fmt.Errorf("resolve snapshot invite username %q: missing user id: %w", login, github.ErrInvalidFieldValue)
			}
			resolvedIDs[index] = userID
			return nil
		})
	}

	if err := orderedtasks.Run(ctx, auditInviteUserLookupConcurrency, tasks); err != nil {
		return nil, err
	}

	for i, username := range usernames {
		resolved[username] = resolvedIDs[i]
	}
	return resolved, nil
}

func uniqueInviteUsernames(invitations []state.PendingInvitation) []string {
	usernames := make([]string, 0, len(invitations))
	seen := make(map[string]struct{}, len(invitations))
	for _, invitation := range invitations {
		username := strings.ToLower(strings.TrimSpace(invitation.Username))
		if username == "" {
			continue
		}
		if _, ok := seen[username]; ok {
			continue
		}
		seen[username] = struct{}{}
		usernames = append(usernames, username)
	}
	return usernames
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
