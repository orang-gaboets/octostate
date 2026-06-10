package apply

import (
	"context"
	"fmt"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

// CheckResult captures the actions that passed apply preflight validation and
// the unsupported drift skipped by one check run.
type CheckResult struct {
	Organization   string              `json:"organization"`
	PlanSummary    gitopsplan.Summary  `json:"plan_summary"`
	CheckedActions []gitopsplan.Action `json:"checked_actions"`
	SkippedActions []gitopsplan.Action `json:"skipped_actions"`
}

// Normalize initializes nil slices while preserving deterministic action order.
func (r *CheckResult) Normalize() {
	if r == nil {
		return
	}
	if r.CheckedActions == nil {
		r.CheckedActions = []gitopsplan.Action{}
	}
	if r.SkippedActions == nil {
		r.SkippedActions = []gitopsplan.Action{}
	}
	for i := range r.CheckedActions {
		r.CheckedActions[i].Normalize()
	}
	for i := range r.SkippedActions {
		r.SkippedActions[i].Normalize()
	}
}

// Check validates the executable portion of a planner report without mutating
// GitHub state.
func Check(ctx context.Context, opt Options) (*CheckResult, error) {
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

	result := &CheckResult{
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
			if err := executor.preflightTeamCreateGroup(opt.Plan.Actions[index:next]); err != nil {
				return nil, err
			}
			result.CheckedActions = append(result.CheckedActions, opt.Plan.Actions[index:next]...)
			index = next
			continue
		}

		if !action.Executable {
			result.SkippedActions = append(result.SkippedActions, action)
			index++
			continue
		}

		if err := executor.preflightAction(action); err != nil {
			return nil, err
		}
		result.CheckedActions = append(result.CheckedActions, action)
		index++
	}

	result.Normalize()
	return result, nil
}

func (e *executor) preflightTeamCreateGroup(actions []gitopsplan.Action) error {
	pending := append([]gitopsplan.Action(nil), actions...)
	for len(pending) > 0 {
		progress := false
		nextPending := make([]gitopsplan.Action, 0, len(pending))

		for _, action := range pending {
			team, ok := e.desiredTeams[action.ResourceID]
			if !ok {
				return fmt.Errorf("desired team %s not found: %w", action.ResourceID, github.ErrNotFound)
			}
			if team.ParentSlug != "" {
				if _, ok := e.teamIDs[teamSlugKey(team.ParentSlug)]; !ok {
					nextPending = append(nextPending, action)
					continue
				}
			}
			if err := e.preflightCreateTeam(action); err != nil {
				return err
			}
			slugKey := teamSlugKey(team.Slug)
			if slugKey == "" {
				slugKey = teamSlugKey(action.ResourceID)
			}
			e.teamIDs[slugKey] = e.allocateSyntheticTeamID()
			progress = true
		}

		if progress {
			pending = nextPending
			continue
		}

		unresolved := make([]string, 0, len(nextPending))
		for _, action := range nextPending {
			unresolved = append(unresolved, action.ResourceID)
		}
		return fmt.Errorf("unable to preflight team create dependencies for %s: %w", strings.Join(unresolved, ", "), github.ErrInvalidFieldValue)
	}

	return nil
}

func (e *executor) preflightAction(action gitopsplan.Action) error {
	switch action.ResourceType {
	case gitopsplan.ActionResourceTypeRepository:
		return e.preflightRepositoryAction(action)
	case gitopsplan.ActionResourceTypeTeam:
		return e.preflightTeamAction(action)
	case gitopsplan.ActionResourceTypeOrganizationMember:
		return e.preflightOrganizationMemberAction(action)
	case gitopsplan.ActionResourceTypeInvite:
		return e.preflightInviteAction(action)
	case gitopsplan.ActionResourceTypeTeamMember:
		return e.preflightTeamMemberAction(action)
	case gitopsplan.ActionResourceTypeTeamRepositoryPermission:
		return e.preflightTeamRepositoryPermissionAction(action)
	default:
		return fmt.Errorf("unsupported plan resource type %q: %w", action.ResourceType, github.ErrInvalidFieldValue)
	}
}

func (e *executor) preflightRepositoryAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate:
		return e.preflightRepositoryCreate(action)
	case gitopsplan.ActionOperationUpdate:
		return e.preflightRepositoryUpdate(action)
	default:
		return fmt.Errorf("unsupported repository operation %q for %s: %w", action.Operation, action.ResourceID, github.ErrInvalidFieldValue)
	}
}

func (e *executor) preflightRepositoryCreate(action gitopsplan.Action) error {
	repository, ok := e.desiredRepositories[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired repository %s not found: %w", action.ResourceID, github.ErrNotFound)
	}
	if repository.Template.Owner == "" || repository.Template.Name == "" {
		return fmt.Errorf("repository %s cannot be created without a template: %w", action.ResourceID, github.ErrInvalidFieldValue)
	}

	private, err := visibilityPrivateFlag(repository.Visibility)
	if err != nil {
		return fmt.Errorf("create repository %s: %w", action.ResourceID, err)
	}

	createOptions := repos.CreateFromTemplateOptions{
		Service:            e.repositoryService,
		Name:               repository.Name,
		Owner:              repository.Owner,
		TemplateOwner:      repository.Template.Owner,
		TemplateRepo:       repository.Template.Name,
		SkipTopicSync:      true,
		IncludeAllBranches: repository.Template.IncludeAllBranches,
		Private:            github.Ptr(private),
	}
	if description, managed := repository.ManagedDescription(); managed {
		createOptions.Description = github.Ptr(description)
	}
	if err := createOptions.Validate(); err != nil {
		return fmt.Errorf("create repository %s: %w", action.ResourceID, err)
	}

	return nil
}

func (e *executor) preflightRepositoryUpdate(action gitopsplan.Action) error {
	repository, ok := e.desiredRepositories[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired repository %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	editOptions := repos.EditOptions{
		Service: e.repositoryService,
		Owner:   repository.Owner,
		Repo:    repository.Name,
	}

	for _, change := range action.Changes {
		switch change.Field {
		case "visibility":
			if _, err := visibilityPrivateFlag(repository.Visibility); err != nil {
				return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
			}
		case "description", "homepage", "topics", "allow_forking", "archived", "is_template":
			if change.Field == "allow_forking" {
				if _, err := visibilityPrivateFlag(repository.Visibility); err != nil {
					return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
				}
			}
		default:
			return fmt.Errorf("unsupported repository change field %q for %s: %w", change.Field, action.ResourceID, github.ErrInvalidFieldValue)
		}
	}

	if err := editOptions.Validate(); err != nil {
		return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
	}
	return nil
}

func (e *executor) preflightTeamAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate:
		return e.preflightCreateTeam(action)
	case gitopsplan.ActionOperationUpdate:
		return e.preflightUpdateTeam(action)
	default:
		return fmt.Errorf("unsupported team operation %q for %s: %w", action.Operation, action.ResourceID, github.ErrInvalidFieldValue)
	}
}

func (e *executor) preflightCreateTeam(action gitopsplan.Action) error {
	team, ok := e.desiredTeams[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	privacy, err := teamPrivacyPointer(team.Privacy)
	if err != nil {
		return fmt.Errorf("create team %s: %w", action.ResourceID, err)
	}

	options := teams.CreateTeamOptions{
		Service:     e.teamService,
		Org:         e.organization,
		Name:        team.Name,
		Description: github.Ptr(team.Description),
		Privacy:     privacy,
	}
	if team.ParentSlug != "" {
		options.ParentTeamSlug = github.Ptr(team.ParentSlug)
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("create team %s: %w", action.ResourceID, err)
	}

	return nil
}

func (e *executor) preflightUpdateTeam(action gitopsplan.Action) error {
	team, ok := e.desiredTeams[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	options := teams.EditTeamBySlugOptions{
		Service: e.teamService,
		Org:     e.organization,
		Slug:    team.Slug,
	}

	for _, change := range action.Changes {
		switch change.Field {
		case "name":
			options.Name = github.Ptr(team.Name)
		case "description":
			options.Description = github.Ptr(team.Description)
		case "privacy":
			privacy, err := teamPrivacyPointer(team.Privacy)
			if err != nil {
				return fmt.Errorf("update team %s: %w", action.ResourceID, err)
			}
			options.Privacy = privacy
		case "parent_slug":
			if strings.TrimSpace(team.ParentSlug) == "" {
				options.RemoveParent = true
			} else {
				options.ParentTeamSlug = github.Ptr(team.ParentSlug)
			}
		default:
			return fmt.Errorf("unsupported team change field %q for %s: %w", change.Field, action.ResourceID, github.ErrInvalidFieldValue)
		}
	}

	if options.ParentTeamSlug != nil {
		if _, ok := e.teamIDs[teamSlugKey(*options.ParentTeamSlug)]; !ok {
			return fmt.Errorf("update team %s: parent team %s not found: %w", action.ResourceID, *options.ParentTeamSlug, github.ErrNotFound)
		}
	}

	if err := options.Validate(); err != nil {
		return fmt.Errorf("update team %s: %w", action.ResourceID, err)
	}
	return nil
}

func (e *executor) preflightOrganizationMemberAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
	default:
		return fmt.Errorf("unsupported organization member operation %q for %s: %w", action.Operation, action.ResourceID, github.ErrInvalidFieldValue)
	}

	member, ok := e.desiredOrgMembers[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired organization member %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	options := organizations.SetMembershipOptions{
		Service:  e.organizationService,
		OrgName:  e.organization,
		Username: member.Username,
		Role:     member.Role,
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("organization member %s: %w", action.ResourceID, err)
	}

	return nil
}

func (e *executor) preflightInviteAction(action gitopsplan.Action) error {
	if action.Operation != gitopsplan.ActionOperationCreate {
		return fmt.Errorf("unsupported invite operation %q for %s: %w", action.Operation, action.ResourceID, github.ErrInvalidFieldValue)
	}

	invite, ok := e.desiredInvites[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired invite %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	if _, err := desiredInviteResourceID(invite); err != nil {
		return fmt.Errorf("invite %s: %w", action.ResourceID, err)
	}
	if len(invite.TeamSlugs) > 0 {
		if _, err := e.resolveInvitationTeamIDs(invite.TeamSlugs); err != nil {
			return fmt.Errorf("invite %s: %w", action.ResourceID, err)
		}
	}

	return nil
}

func (e *executor) preflightTeamMemberAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
	default:
		return fmt.Errorf("unsupported team member operation %q for %s: %w", action.Operation, action.ResourceID, github.ErrInvalidFieldValue)
	}

	member, ok := e.desiredMembers[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team membership %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	teamSlug, _, err := splitTeamMemberResourceID(action.ResourceID)
	if err != nil {
		return err
	}
	if _, ok := e.teamIDs[teamSlugKey(teamSlug)]; !ok {
		return fmt.Errorf("team %s not found: %w", teamSlug, github.ErrNotFound)
	}

	role := teams.TeamMemberAddRole(member.Role)
	if !role.IsValid() {
		return fmt.Errorf("team membership role %q is invalid for %s: %w", member.Role, action.ResourceID, github.ErrInvalidFieldValue)
	}

	options := teams.AddTeamMemberBySlugOptions{
		Service:  e.teamService,
		Org:      e.organization,
		Slug:     teamSlug,
		Username: member.Username,
		Role:     role,
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("team member %s: %w", action.ResourceID, err)
	}

	return nil
}

func (e *executor) preflightTeamRepositoryPermissionAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
	default:
		return fmt.Errorf("unsupported team repository permission operation %q for %s: %w", action.Operation, action.ResourceID, github.ErrInvalidFieldValue)
	}

	permission, ok := e.desiredPermissions[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team repository permission %s not found: %w", action.ResourceID, github.ErrNotFound)
	}

	teamSlug, _, _, err := splitTeamRepoPermissionResourceID(action.ResourceID)
	if err != nil {
		return err
	}
	if _, ok := e.teamIDs[teamSlugKey(teamSlug)]; !ok {
		return fmt.Errorf("team %s not found: %w", teamSlug, github.ErrNotFound)
	}

	repoPermission := teams.TeamRepoPermission(permission.Permission)
	if !repoPermission.IsValid() {
		return fmt.Errorf("team repository permission %q is invalid for %s: %w", permission.Permission, action.ResourceID, github.ErrInvalidFieldValue)
	}

	options := teams.AddTeamRepoPermissionBySlugOptions{
		Service:    e.teamService,
		Org:        e.organization,
		Slug:       teamSlug,
		RepoOwner:  permission.Owner,
		RepoName:   permission.Name,
		Permission: repoPermission,
	}
	if err := options.Validate(); err != nil {
		return fmt.Errorf("team repository permission %s: %w", action.ResourceID, err)
	}

	return nil
}

func (e *executor) allocateSyntheticTeamID() int64 {
	if e.syntheticTeamID <= 0 {
		e.syntheticTeamID = 1
	}
	id := e.syntheticTeamID
	e.syntheticTeamID++
	return id
}
