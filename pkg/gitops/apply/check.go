package apply

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

// CheckResult captures the actions that passed apply preflight validation and
// the actions skipped by one check run.//
// The skipped set holds every non-executable action, which covers two different
// situations: destructive drift Octostate intentionally declines to reconcile,
// and a desired create or update that planning determined cannot execute. Use
// UnfulfilledDesiredActions to tell them apart, or
// Options.RequireExecutableDesiredActions to fail on the second kind.
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
	opt.Desired = config.NormalizeDesiredState(opt.Desired)
	if err := requireExecutableDesiredActions(opt); err != nil {
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
	failures := &preflightFailures{}

	for index := 0; index < len(opt.Plan.Actions); {
		action := opt.Plan.Actions[index]
		if isTeamCreateAction(action) {
			next := index
			for next < len(opt.Plan.Actions) && isTeamCreateAction(opt.Plan.Actions[next]) {
				next++
			}
			result.CheckedActions = append(result.CheckedActions, executor.preflightTeamCreateGroup(opt.Plan.Actions[index:next], failures)...)
			index = next
			continue
		}

		if !action.Executable {
			result.SkippedActions = append(result.SkippedActions, action)
			index++
			continue
		}

		if err := executor.preflightAction(action); err != nil {
			failures.add(action, err)
			index++
			continue
		}
		result.CheckedActions = append(result.CheckedActions, action)
		index++
	}

	if err := failures.err(); err != nil {
		return nil, err
	}
	result.Normalize()
	return result, nil
}

func (e *executor) preflightTeamCreateGroup(actions []gitopsplan.Action, failures *preflightFailures) []gitopsplan.Action {
	pending := append([]gitopsplan.Action(nil), actions...)
	checked := make([]gitopsplan.Action, 0, len(actions))
	for len(pending) > 0 {
		progress := false
		nextPending := make([]gitopsplan.Action, 0, len(pending))

		for _, action := range pending {
			team, ok := e.desiredTeams[action.ResourceID]
			if !ok {
				failures.add(action, fmt.Errorf("desired team %s not found: %w", action.ResourceID, github.ErrNotFound))
				continue
			}
			if team.ParentSlug != "" {
				if _, ok := e.teamIDs[teamSlugKey(team.ParentSlug)]; !ok {
					nextPending = append(nextPending, action)
					continue
				}
			}
			if err := e.preflightCreateTeam(action); err != nil {
				failures.add(action, err)
				continue
			}
			slugKey := teamSlugKey(team.Slug)
			if slugKey == "" {
				slugKey = teamSlugKey(action.ResourceID)
			}
			e.teamIDs[slugKey] = e.allocateSyntheticTeamID()
			e.markPreflightCreatedTeam(slugKey)
			checked = append(checked, action)
			progress = true
		}

		if progress {
			pending = nextPending
			continue
		}
		if len(nextPending) == 0 {
			break
		}

		for _, action := range nextPending {
			team, ok := e.desiredTeams[action.ResourceID]
			if !ok {
				failures.add(action, fmt.Errorf("desired team %s not found: %w", action.ResourceID, github.ErrNotFound))
				continue
			}
			failures.add(action, fmt.Errorf("parent team %s was not found or could not be preflighted earlier in the same plan: %w", team.ParentSlug, github.ErrNotFound))
		}
		break
	}

	return checked
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
	private, err := visibilityPrivateFlag(repository.Visibility)
	if err != nil {
		return fmt.Errorf("create repository %s: %w", action.ResourceID, err)
	}

	var validateErr error
	if repository.Template.Owner == "" && repository.Template.Name == "" {
		createOptions := repos.CreateOptions{Service: e.repositoryService, Name: repository.Name, Owner: repository.Owner, Private: github.Ptr(private)}
		if description, managed := repository.ManagedDescription(); managed {
			createOptions.Description = github.Ptr(description)
		}
		validateErr = createOptions.Validate()
	} else {
		createOptions := repos.CreateFromTemplateOptions{
			Service: e.repositoryService, Name: repository.Name, Owner: repository.Owner,
			TemplateOwner: repository.Template.Owner, TemplateRepo: repository.Template.Name,
			SkipTopicSync: true, IncludeAllBranches: repository.Template.IncludeAllBranches,
			Private: github.Ptr(private),
		}
		if description, managed := repository.ManagedDescription(); managed {
			createOptions.Description = github.Ptr(description)
		}
		validateErr = createOptions.Validate()
	}
	if validateErr != nil {
		return fmt.Errorf("create repository %s: %w", action.ResourceID, validateErr)
	}

	if repository.Template.Owner != "" || repository.Template.Name != "" {
		templateRepository, err := e.preflightTemplateRepository(repository.Template.Owner, repository.Template.Name)
		if err != nil {
			return fmt.Errorf("create repository %s: template repository %s/%s: %w", action.ResourceID, repository.Template.Owner, repository.Template.Name, err)
		}
		if templateRepository == nil || !templateRepository.IsTemplate {
			return fmt.Errorf("create repository %s: template repository %s/%s is not marked as a template: %w", action.ResourceID, repository.Template.Owner, repository.Template.Name, github.ErrInvalidFieldValue)
		}
	}

	_, err = e.preflightGetRepository(repository.Owner, repository.Name)
	switch {
	case err == nil:
		return fmt.Errorf("create repository %s: target repository %s/%s already exists: %w", action.ResourceID, repository.Owner, repository.Name, github.ErrInvalidFieldValue)
	case !errors.Is(err, github.ErrNotFound):
		return fmt.Errorf("create repository %s: target repository %s/%s: %w", action.ResourceID, repository.Owner, repository.Name, err)
	}

	e.markPreflightCreatedRepo(repository.Owner, repository.Name)
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
	templateStateUpdated := false

	for _, change := range action.Changes {
		switch change.Field {
		case "visibility":
			if _, err := visibilityPrivateFlag(repository.Visibility); err != nil {
				return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
			}
		case "description", "homepage", "topics", "allow_forking", "archived", "is_template":
			if change.Field == "allow_forking" {
				if !config.IsPrivateVisibility(repository.Visibility) {
					return fmt.Errorf("update repository %s: allow_forking is only applicable to private repositories: %w", action.ResourceID, github.ErrInvalidFieldValue)
				}
			}
			if change.Field == "is_template" {
				templateStateUpdated = true
			}
		default:
			return fmt.Errorf("unsupported repository change field %q for %s: %w", change.Field, action.ResourceID, github.ErrInvalidFieldValue)
		}
	}

	if err := editOptions.Validate(); err != nil {
		return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
	}

	liveRepository, err := e.preflightGetRepository(repository.Owner, repository.Name)
	if err != nil {
		return fmt.Errorf("update repository %s: target repository %s/%s: %w", action.ResourceID, repository.Owner, repository.Name, err)
	}
	if liveRepository == nil {
		return fmt.Errorf("update repository %s: target repository %s/%s did not resolve to a repository: %w", action.ResourceID, repository.Owner, repository.Name, github.ErrInvalidFieldValue)
	}
	if templateStateUpdated {
		liveRepository.IsTemplate = repository.IsTemplate
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

	targetSlug := strings.TrimSpace(team.Slug)
	if targetSlug == "" {
		targetSlug = strings.TrimSpace(action.ResourceID)
	}
	_, err = teams.GetTeamBySlug(e.ctx, teams.GetTeamBySlugOptions{
		Service: e.teamService,
		Org:     e.organization,
		Slug:    targetSlug,
	})
	switch {
	case err == nil:
		return fmt.Errorf("create team %s: target team %s already exists live: %w", action.ResourceID, targetSlug, github.ErrInvalidFieldValue)
	case !errors.Is(err, github.ErrNotFound):
		return fmt.Errorf("create team %s: target team %s: %w", action.ResourceID, targetSlug, err)
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

	liveTeam, err := teams.GetTeamBySlug(e.ctx, teams.GetTeamBySlugOptions{
		Service: e.teamService,
		Org:     e.organization,
		Slug:    team.Slug,
	})
	if err != nil {
		return fmt.Errorf("update team %s: target team %s: %w", action.ResourceID, team.Slug, err)
	}
	if liveTeam == nil || liveTeam.ID <= 0 {
		return fmt.Errorf("update team %s: target team %s did not return a valid team ID: %w", action.ResourceID, team.Slug, github.ErrInvalidFieldValue)
	}
	e.teamIDs[teamSlugKey(team.Slug)] = liveTeam.ID
	e.preflightVerifiedTeams[teamSlugKey(team.Slug)] = struct{}{}

	if options.ParentTeamSlug != nil {
		if _, err := e.preflightEnsureTeamExists(*options.ParentTeamSlug); err != nil {
			return fmt.Errorf("update team %s: parent team %s: %w", action.ResourceID, *options.ParentTeamSlug, err)
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
	if invite.Username.Present && !invite.Username.Null {
		if _, err := e.preflightResolveInviteUserID(strings.TrimSpace(invite.Username.Value)); err != nil {
			return fmt.Errorf("invite %s: %w", action.ResourceID, err)
		}
	}
	if len(invite.TeamSlugs) > 0 {
		if _, err := e.preflightResolveInvitationTeamIDs(invite.TeamSlugs); err != nil {
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
	if _, err := e.preflightEnsureTeamExists(teamSlug); err != nil {
		return fmt.Errorf("team %s not found: %w", teamSlug, err)
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
	if e.hasPreflightCreatedRepo(permission.Owner, permission.Name) {
		return nil
	}
	liveRepository, err := e.preflightGetRepository(permission.Owner, permission.Name)
	if err != nil {
		return fmt.Errorf("team repository permission %s: target repository %s/%s: %w", action.ResourceID, permission.Owner, permission.Name, err)
	}
	if liveRepository == nil {
		return fmt.Errorf("team repository permission %s: target repository %s/%s did not resolve to a repository: %w", action.ResourceID, permission.Owner, permission.Name, github.ErrInvalidFieldValue)
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
