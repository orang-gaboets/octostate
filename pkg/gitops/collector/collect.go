package collector

import (
	"context"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

// CollectOrganizationOptions defines the services required to collect one
// organization's actual state from GitHub.
type CollectOrganizationOptions struct {
	OrgName             string
	OrganizationService organizations.Service
	RepositoryService   repos.Service
	TeamService         teams.Service
}

type collectOrganizationBehavior struct {
	includeMembers            bool
	includePendingInvitations bool
	includeRepositories       bool
	includeTeams              bool
}

// Validate checks if the CollectOrganizationOptions are valid for the full
// organization collection path.
func (opt *CollectOrganizationOptions) Validate() error {
	return opt.validateForBehavior(collectOrganizationBehavior{
		includeMembers:            true,
		includePendingInvitations: true,
		includeRepositories:       true,
		includeTeams:              true,
	})
}

func (opt *CollectOrganizationOptions) validateForBehavior(behavior collectOrganizationBehavior) error {
	switch {
	case opt.OrgName == "":
		return githubpkg.ErrMissingRequiredField
	case (behavior.includeMembers || behavior.includePendingInvitations) && opt.OrganizationService == nil:
		return githubpkg.ErrNilService
	case behavior.includeRepositories && opt.RepositoryService == nil:
		return githubpkg.ErrNilService
	case behavior.includeTeams && opt.TeamService == nil:
		return githubpkg.ErrNilService
	default:
		return nil
	}
}

// CollectOrganization loads the current GitHub state for one organization into
// a normalized OrganizationState value.
func CollectOrganization(ctx context.Context, opt CollectOrganizationOptions) (*state.OrganizationState, error) {
	return collectOrganization(ctx, opt, collectOrganizationBehavior{
		includeMembers:            true,
		includePendingInvitations: true,
		includeRepositories:       true,
		includeTeams:              true,
	})
}

// CollectOrganizationForBootstrap loads only the live organization state
// required for sync-from-live bootstrap generation.
func CollectOrganizationForBootstrap(ctx context.Context, opt CollectOrganizationOptions) (*state.OrganizationState, error) {
	return CollectOrganizationForSyncFromLive(ctx, opt)
}

// CollectOrganizationForSyncFromLive loads the live organization state needed
// for sync-from-live proposal generation.
func CollectOrganizationForSyncFromLive(ctx context.Context, opt CollectOrganizationOptions) (*state.OrganizationState, error) {
	return collectOrganization(ctx, opt, collectOrganizationBehavior{
		includeMembers:      true,
		includeRepositories: true,
		includeTeams:        true,
	})
}

// CollectOrganizationForMaterialize loads only the live organization state
// required for sync-from-live materialize generation.
func CollectOrganizationForMaterialize(ctx context.Context, opt CollectOrganizationOptions) (*state.OrganizationState, error) {
	return collectOrganization(ctx, opt, collectOrganizationBehavior{
		includeRepositories: true,
	})
}

func collectOrganization(
	ctx context.Context,
	opt CollectOrganizationOptions,
	behavior collectOrganizationBehavior,
) (*state.OrganizationState, error) {
	if err := opt.validateForBehavior(behavior); err != nil {
		return nil, err
	}

	actual := &state.OrganizationState{
		Organization: opt.OrgName,
	}

	if behavior.includeMembers {
		members, err := collectOrganizationMembers(ctx, opt)
		if err != nil {
			return nil, err
		}
		actual.Members = members
	}

	if behavior.includePendingInvitations {
		invitations, err := organizations.ListPendingInvitations(ctx, organizations.ListPendingInvitationsOptions{
			Service: opt.OrganizationService,
			OrgName: opt.OrgName,
		})
		if err != nil {
			return nil, err
		}
		actual.PendingInvitations = make([]state.PendingInvitation, 0, len(invitations))
		for _, invitation := range invitations {
			if invitation == nil {
				continue
			}

			pendingInvitation := pendingInvitationFromOrganizationInvitation(invitation)
			if shouldLoadInvitationTeams(invitation) {
				invitationTeams, err := organizations.ListInvitationTeams(ctx, organizations.ListInvitationTeamsOptions{
					Service:      opt.OrganizationService,
					OrgName:      opt.OrgName,
					InvitationID: *invitation.ID,
				})
				if err != nil {
					return nil, err
				}
				pendingInvitation.TeamSlugs = teamSlugsFromTeams(invitationTeams)
			}
			actual.PendingInvitations = append(actual.PendingInvitations, pendingInvitation)
		}
	}

	if behavior.includeRepositories {
		repositories, err := repos.ListOrgRepos(ctx, repos.ListOrgReposOptions{
			Service: opt.RepositoryService,
			Org:     opt.OrgName,
			Type:    repos.RepoTypeAll,
		})
		if err != nil {
			return nil, err
		}
		actual.Repositories = repositoriesFromRepositories(repositories)
	}

	if behavior.includeTeams {
		collectedTeams, err := teams.ListTeams(ctx, teams.ListTeamsOptions{
			Service: opt.TeamService,
			Org:     opt.OrgName,
		})
		if err != nil {
			return nil, err
		}
		actual.Teams = teamsFromTeams(collectedTeams)

		for _, team := range collectedTeams {
			if team == nil || team.Slug == "" {
				continue
			}

			members, err := teams.ListTeamMembersBySlug(ctx, teams.ListTeamMembersBySlugOptions{
				Service: opt.TeamService,
				Org:     opt.OrgName,
				Slug:    team.Slug,
				Role:    teams.TeamMemberRoleMember,
			})
			if err != nil {
				return nil, err
			}
			actual.TeamMembers = append(actual.TeamMembers, teamMembersFromUsers(team.Slug, "member", members)...)

			maintainers, err := teams.ListTeamMembersBySlug(ctx, teams.ListTeamMembersBySlugOptions{
				Service: opt.TeamService,
				Org:     opt.OrgName,
				Slug:    team.Slug,
				Role:    teams.TeamMemberRoleMaintainer,
			})
			if err != nil {
				return nil, err
			}
			actual.TeamMembers = append(actual.TeamMembers, teamMembersFromUsers(team.Slug, "maintainer", maintainers)...)

			repoPermissions, err := teams.ListTeamRepoPermissionsBySlug(ctx, teams.ListTeamRepoPermissionsBySlugOptions{
				Service: opt.TeamService,
				Org:     opt.OrgName,
				Slug:    team.Slug,
			})
			if err != nil {
				return nil, err
			}
			actual.TeamRepositoryPermissions = append(actual.TeamRepositoryPermissions, teamRepositoryPermissionsFromRepositories(team.Slug, repoPermissions)...)
		}
	}

	actual.Normalize()
	return actual, nil
}

func collectOrganizationMembers(ctx context.Context, opt CollectOrganizationOptions) ([]state.OrganizationMember, error) {
	admins, err := organizations.ListMembers(ctx, organizations.ListMembersOptions{
		Service: opt.OrganizationService,
		OrgName: opt.OrgName,
		Role:    organizations.MemberRoleAdmin,
	})
	if err != nil {
		return nil, err
	}

	members, err := organizations.ListMembers(ctx, organizations.ListMembersOptions{
		Service: opt.OrganizationService,
		OrgName: opt.OrgName,
		Role:    organizations.MemberRoleMember,
	})
	if err != nil {
		return nil, err
	}

	collected := append(
		organizationMembersFromUsers(admins, string(organizations.MemberRoleAdmin)),
		organizationMembersFromUsers(members, string(organizations.MemberRoleMember))...,
	)
	return dedupeOrganizationMembers(collected), nil
}

func organizationMembersFromUsers(users []*githubpkg.User, role string) []state.OrganizationMember {
	result := make([]state.OrganizationMember, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		result = append(result, state.OrganizationMember{
			ID:       derefInt64(user.ID),
			Username: derefString(user.Login),
			Role:     role,
			Name:     derefString(user.Name),
			Email:    derefString(user.Email),
		})
	}
	return result
}

func dedupeOrganizationMembers(members []state.OrganizationMember) []state.OrganizationMember {
	if len(members) == 0 {
		return []state.OrganizationMember{}
	}

	byUsername := make(map[string]state.OrganizationMember, len(members))
	order := make([]string, 0, len(members))
	for _, member := range members {
		username := strings.ToLower(strings.TrimSpace(member.Username))
		if username == "" {
			continue
		}
		if existing, ok := byUsername[username]; ok {
			if existing.Role != string(organizations.MemberRoleAdmin) && member.Role == string(organizations.MemberRoleAdmin) {
				byUsername[username] = member
			}
			continue
		}
		byUsername[username] = member
		order = append(order, username)
	}

	result := make([]state.OrganizationMember, 0, len(order))
	for _, username := range order {
		result = append(result, byUsername[username])
	}
	return result
}

func pendingInvitationFromOrganizationInvitation(invitation *githubpkg.OrganizationInvitation) state.PendingInvitation {
	return state.PendingInvitation{
		ID:        derefInt64(invitation.ID),
		Username:  derefString(invitation.Login),
		Email:     derefString(invitation.Email),
		Role:      derefString(invitation.Role),
		TeamSlugs: []string{},
	}
}

func repositoriesFromRepositories(repositories []*githubpkg.Repository) []state.Repository {
	result := make([]state.Repository, 0, len(repositories))
	for _, repository := range repositories {
		if repository == nil {
			continue
		}
		result = append(result, state.Repository{
			Owner:        repository.Owner,
			Name:         repository.Name,
			Visibility:   repository.Visibility,
			Description:  repository.Description,
			Homepage:     repository.Homepage,
			Topics:       append([]string(nil), repository.Topics...),
			AllowForking: repository.AllowForking,
			Archived:     repository.Archived,
			IsTemplate:   repository.IsTemplate,
		})
	}
	return result
}

func teamsFromTeams(teamsIn []*githubpkg.Team) []state.Team {
	result := make([]state.Team, 0, len(teamsIn))
	for _, team := range teamsIn {
		if team == nil {
			continue
		}
		result = append(result, state.Team{
			ID:          team.ID,
			Slug:        team.Slug,
			Name:        team.Name,
			Description: team.Description,
			Privacy:     team.Privacy.String(),
			ParentSlug:  parentSlug(team.ParentTeam),
		})
	}
	return result
}

func teamMembersFromUsers(teamSlug, role string, users []*githubpkg.User) []state.TeamMember {
	result := make([]state.TeamMember, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		result = append(result, state.TeamMember{
			TeamSlug: teamSlug,
			Username: derefString(user.Login),
			Role:     role,
		})
	}
	return result
}

func teamRepositoryPermissionsFromRepositories(teamSlug string, repositories []*githubpkg.TeamRepositoryPermission) []state.TeamRepositoryPermission {
	result := make([]state.TeamRepositoryPermission, 0, len(repositories))
	for _, repository := range repositories {
		if repository == nil {
			continue
		}
		result = append(result, state.TeamRepositoryPermission{
			TeamSlug:   teamSlug,
			Owner:      repository.Owner,
			Name:       repository.Name,
			Permission: dominantPermission(repository.Permissions),
		})
	}
	return result
}

func teamSlugsFromTeams(teamsIn []*githubpkg.Team) []string {
	result := make([]string, 0, len(teamsIn))
	for _, team := range teamsIn {
		if team == nil || team.Slug == "" {
			continue
		}
		result = append(result, team.Slug)
	}
	return result
}

func dominantPermission(permissions map[string]bool) string {
	for _, permission := range []string{"admin", "maintain", "push", "triage", "pull"} {
		if permissions[permission] {
			return permission
		}
	}
	return ""
}

func parentSlug(team *githubpkg.Team) string {
	if team == nil {
		return ""
	}
	return team.Slug
}

func shouldLoadInvitationTeams(invitation *githubpkg.OrganizationInvitation) bool {
	if invitation == nil || invitation.ID == nil || *invitation.ID <= 0 {
		return false
	}
	if invitation.TeamCount != nil && *invitation.TeamCount == 0 {
		return false
	}
	return true
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
