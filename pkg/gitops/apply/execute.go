package apply

import (
	"context"
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	ghusers "github.com/orang-gaboets/octostate/pkg/github/users"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// Options defines the inputs and GitHub service dependencies required to apply
// one planner report.
//
// UserService must be safe for concurrent read calls. Apply pre-resolves
// username-based invite targets with bounded concurrent lookups before
// sequential invitation writes begin.
type Options struct {
	Desired             config.OrganizationConfig
	Actual              *state.OrganizationState
	Plan                *gitopsplan.Report
	OrganizationService organizations.Service
	RepositoryService   repos.Service
	TeamService         teams.Service
	UserService         ghusers.Service
}

// Validate checks whether the apply inputs and service dependencies are usable.
func (opt *Options) Validate() error {
	desiredOrg := strings.TrimSpace(opt.Desired.Organization)
	switch {
	case desiredOrg == "":
		return fmt.Errorf("organization is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Actual == nil:
		return fmt.Errorf("actual state is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Plan == nil:
		return fmt.Errorf("plan report is required: %w", githubpkg.ErrMissingRequiredField)
	case strings.TrimSpace(opt.Plan.Organization) == "":
		return fmt.Errorf("plan organization is required: %w", githubpkg.ErrMissingRequiredField)
	case !strings.EqualFold(strings.TrimSpace(opt.Plan.Organization), desiredOrg):
		return fmt.Errorf("plan organization %q does not match desired organization %q: %w", opt.Plan.Organization, desiredOrg, githubpkg.ErrInvalidFieldValue)
	case strings.TrimSpace(opt.Actual.Organization) != "" && !strings.EqualFold(strings.TrimSpace(opt.Actual.Organization), desiredOrg):
		return fmt.Errorf("actual organization %q does not match desired organization %q: %w", opt.Actual.Organization, desiredOrg, githubpkg.ErrInvalidFieldValue)
	case opt.OrganizationService == nil:
		return githubpkg.ErrNilService
	case opt.RepositoryService == nil:
		return githubpkg.ErrNilService
	case opt.TeamService == nil:
		return githubpkg.ErrNilService
	case opt.UserService == nil:
		return githubpkg.ErrNilService
	default:
		return nil
	}
}

// Result captures the actions executed and the unsupported drift skipped by one
// apply run.
type Result struct {
	Organization string              `json:"organization"`
	PlanSummary  gitopsplan.Summary  `json:"plan_summary"`
	Executed     []gitopsplan.Action `json:"executed_actions"`
	SkippedDrift []gitopsplan.Action `json:"skipped_actions"`
}

// Normalize initializes nil slices while preserving execution order.
func (r *Result) Normalize() {
	if r == nil {
		return
	}
	if r.Executed == nil {
		r.Executed = []gitopsplan.Action{}
	}
	if r.SkippedDrift == nil {
		r.SkippedDrift = []gitopsplan.Action{}
	}
	for i := range r.Executed {
		r.Executed[i].Normalize()
	}
	for i := range r.SkippedDrift {
		r.SkippedDrift[i].Normalize()
	}
}

// Execute applies the executable portion of a planner report to GitHub.
func Execute(ctx context.Context, opt Options) (*Result, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	if err := validateTeamCreateOrdering(opt.Plan.Actions); err != nil {
		return nil, err
	}

	executor, err := newExecutor(ctx, opt)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Organization: strings.TrimSpace(opt.Desired.Organization),
		PlanSummary:  opt.Plan.Summary,
	}

	for index := 0; index < len(opt.Plan.Actions); {
		action := opt.Plan.Actions[index]
		if isTeamCreateAction(action) {
			next := index
			for next < len(opt.Plan.Actions) && isTeamCreateAction(opt.Plan.Actions[next]) {
				next++
			}
			if err := executor.executeTeamCreateGroup(opt.Plan.Actions[index:next], result); err != nil {
				return nil, err
			}
			index = next
			continue
		}

		if !action.Executable {
			result.SkippedDrift = append(result.SkippedDrift, action)
			index++
			continue
		}

		if action.ResourceType == gitopsplan.ActionResourceTypeInvite &&
			action.Operation == gitopsplan.ActionOperationCreate &&
			!executor.inviteUsersResolved {
			if err := executor.preResolveInviteUsernames(opt.Plan.Actions[index:]); err != nil {
				return nil, err
			}
		}

		if err := executor.executeAction(action); err != nil {
			return nil, err
		}
		result.Executed = append(result.Executed, action)
		index++
	}

	result.Normalize()
	return result, nil
}

type executor struct {
	ctx                   context.Context
	organization          string
	organizationService   organizations.Service
	repositoryService     repos.Service
	teamService           teams.Service
	userService           ghusers.Service
	teamIDs               map[string]int64
	resolvedInviteUserIDs map[string]int64
	inviteUsersResolved   bool
	desiredRepositories   map[string]config.RepositorySpec
	desiredTeams          map[string]config.TeamSpec
	desiredOrgMembers     map[string]config.OrganizationMemberSpec
	desiredMembers        map[string]config.TeamMemberSpec
	desiredPermissions    map[string]config.TeamRepositorySpec
	desiredInvites        map[string]config.InviteSpec
	syntheticTeamID       int64
}

func newExecutor(ctx context.Context, opt Options) (*executor, error) {
	nextSyntheticTeamID := int64(1)
	exec := &executor{
		ctx:                   ctx,
		organization:          strings.TrimSpace(opt.Desired.Organization),
		organizationService:   opt.OrganizationService,
		repositoryService:     opt.RepositoryService,
		teamService:           opt.TeamService,
		userService:           opt.UserService,
		teamIDs:               make(map[string]int64, len(opt.Actual.Teams)),
		resolvedInviteUserIDs: map[string]int64{},
		desiredRepositories:   make(map[string]config.RepositorySpec, len(opt.Desired.Repositories)),
		desiredTeams:          make(map[string]config.TeamSpec, len(opt.Desired.Teams)),
		desiredOrgMembers:     make(map[string]config.OrganizationMemberSpec, len(opt.Desired.Members)),
		desiredMembers:        map[string]config.TeamMemberSpec{},
		desiredPermissions:    map[string]config.TeamRepositorySpec{},
		desiredInvites:        map[string]config.InviteSpec{},
		syntheticTeamID:       nextSyntheticTeamID,
	}

	for _, team := range opt.Actual.Teams {
		if team.ID <= 0 {
			continue
		}
		exec.teamIDs[teamSlugKey(team.Slug)] = team.ID
		if team.ID >= exec.syntheticTeamID {
			exec.syntheticTeamID = team.ID + 1
		}
	}

	for _, repository := range opt.Desired.Repositories {
		exec.desiredRepositories[repositoryResourceID(repository.Owner, repository.Name)] = repository
	}
	for _, member := range opt.Desired.Members {
		exec.desiredOrgMembers[organizationMemberResourceID(member.Username)] = member
	}
	for _, team := range opt.Desired.Teams {
		exec.desiredTeams[teamResourceID(team.Slug)] = team
		for _, member := range team.Members {
			exec.desiredMembers[teamMemberResourceID(team.Slug, member.Username)] = member
		}
		for _, permission := range team.Repositories {
			exec.desiredPermissions[teamRepoPermissionResourceID(team.Slug, permission.Owner, permission.Name)] = permission
		}
	}
	for _, invite := range opt.Desired.Invites {
		resourceID, err := desiredInviteResourceID(invite)
		if err != nil {
			return nil, err
		}
		exec.desiredInvites[resourceID] = invite
	}

	return exec, nil
}

func (e *executor) executeAction(action gitopsplan.Action) error {
	switch action.ResourceType {
	case gitopsplan.ActionResourceTypeRepository:
		return e.executeRepositoryAction(action)
	case gitopsplan.ActionResourceTypeTeam:
		return e.executeTeamAction(action)
	case gitopsplan.ActionResourceTypeOrganizationMember:
		return e.executeOrganizationMemberAction(action)
	case gitopsplan.ActionResourceTypeInvite:
		return e.executeInviteAction(action)
	case gitopsplan.ActionResourceTypeTeamMember:
		return e.executeTeamMemberAction(action)
	case gitopsplan.ActionResourceTypeTeamRepositoryPermission:
		return e.executeTeamRepositoryPermissionAction(action)
	default:
		return fmt.Errorf("unsupported plan resource type %q: %w", action.ResourceType, githubpkg.ErrInvalidFieldValue)
	}
}

func isTeamCreateAction(action gitopsplan.Action) bool {
	return action.Executable && action.ResourceType == gitopsplan.ActionResourceTypeTeam && action.Operation == gitopsplan.ActionOperationCreate
}

func validateTeamCreateOrdering(actions []gitopsplan.Action) error {
	seenTeamCreate := false
	leftTeamCreateBlock := false

	for _, action := range actions {
		if !isTeamCreateAction(action) {
			if seenTeamCreate {
				leftTeamCreateBlock = true
			}
			continue
		}
		if leftTeamCreateBlock {
			return fmt.Errorf("executable team create actions must be contiguous in the plan: %w", githubpkg.ErrInvalidFieldValue)
		}
		seenTeamCreate = true
	}

	return nil
}
