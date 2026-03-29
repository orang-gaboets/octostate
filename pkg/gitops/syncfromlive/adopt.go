package syncfromlive

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

// AdoptOptions defines the inputs required to merge supported live GitHub
// state back into an existing desired GitOps config.
type AdoptOptions struct {
	Desired config.OrganizationConfig
	Actual  *state.OrganizationState
}

// Validate checks whether the adopt inputs are usable.
func (opt *AdoptOptions) Validate() error {
	desiredOrganization := strings.TrimSpace(opt.Desired.Organization)
	switch {
	case desiredOrganization == "":
		return fmt.Errorf("desired organization is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Actual == nil:
		return fmt.Errorf("actual state is required: %w", githubpkg.ErrMissingRequiredField)
	case strings.TrimSpace(opt.Actual.Organization) == "":
		return fmt.Errorf("actual organization is required: %w", githubpkg.ErrMissingRequiredField)
	case !strings.EqualFold(strings.TrimSpace(opt.Actual.Organization), desiredOrganization):
		return fmt.Errorf(
			"actual organization %q does not match desired organization %q: %w",
			opt.Actual.Organization,
			opt.Desired.Organization,
			githubpkg.ErrInvalidFieldValue,
		)
	default:
		return nil
	}
}

// BuildAdoptConfig merges supported live GitHub state into an existing desired
// GitOps config without removing config-only declarations.
func BuildAdoptConfig(opt AdoptOptions) (config.OrganizationConfig, error) {
	if err := opt.Validate(); err != nil {
		return config.OrganizationConfig{}, err
	}

	desired := cloneDesiredConfig(opt.Desired)
	actual := cloneOrganizationState(opt.Actual)
	organization := strings.TrimSpace(desired.Organization)

	membersByTeam, err := bootstrapTeamMembers(actual.Teams, actual.TeamMembers)
	if err != nil {
		return config.OrganizationConfig{}, err
	}
	permissionsByTeam, err := bootstrapTeamRepositoryPermissions(organization, actual.Teams, actual.TeamRepositoryPermissions)
	if err != nil {
		return config.OrganizationConfig{}, err
	}
	actualMembers, err := bootstrapOrganizationMembers(actual.Members, actual.TeamMembers)
	if err != nil {
		return config.OrganizationConfig{}, err
	}

	desired.Organization = organization
	desired.Invites = adoptInvites(desired.Invites, actual.Members)
	desired.Members = adoptOrganizationMembers(desired.Members, actualMembers)
	desired.Repositories = adoptRepositories(organization, desired.Repositories, actual.Repositories)
	desired.Teams = adoptTeams(organization, desired.Teams, actual.Teams, membersByTeam, permissionsByTeam)

	return desired, nil
}

func adoptInvites(
	desired []config.InviteSpec,
	actualMembers []state.OrganizationMember,
) []config.InviteSpec {
	adopted := make([]config.InviteSpec, 0, len(desired))
	for _, invite := range desired {
		if inviteSatisfiedByActualMember(invite, actualMembers) {
			continue
		}
		adopted = append(adopted, invite)
	}
	return adopted
}

func inviteSatisfiedByActualMember(
	invite config.InviteSpec,
	actualMembers []state.OrganizationMember,
) bool {
	for _, member := range actualMembers {
		if actualMemberMatchesInvite(member, invite) {
			return true
		}
	}
	return false
}

func actualMemberMatchesInvite(member state.OrganizationMember, invite config.InviteSpec) bool {
	switch {
	case invite.Username.Present && !invite.Username.Null:
		return strings.EqualFold(
			strings.TrimSpace(member.Username),
			strings.TrimSpace(invite.Username.Value),
		)
	case invite.UserID.Present && !invite.UserID.Null:
		return member.ID > 0 && member.ID == invite.UserID.Value
	case invite.Email.Present && !invite.Email.Null:
		memberEmail := strings.TrimSpace(member.Email)
		return memberEmail != "" && strings.EqualFold(
			memberEmail,
			strings.TrimSpace(invite.Email.Value),
		)
	default:
		return false
	}
}

func adoptOrganizationMembers(
	desired []config.OrganizationMemberSpec,
	actual []config.OrganizationMemberSpec,
) []config.OrganizationMemberSpec {
	adopted := append([]config.OrganizationMemberSpec{}, desired...)
	indexByUsername := make(map[string]int, len(adopted))
	for i, member := range adopted {
		indexByUsername[organizationMemberKey(member.Username)] = i
	}

	for _, member := range actual {
		key := organizationMemberKey(member.Username)
		if index, ok := indexByUsername[key]; ok {
			adopted[index].Username = strings.TrimSpace(member.Username)
			adopted[index].Role = strings.TrimSpace(member.Role)
			continue
		}
		indexByUsername[key] = len(adopted)
		adopted = append(adopted, member)
	}

	return adopted
}

func adoptRepositories(
	organization string,
	desired []config.RepositorySpec,
	actual []state.Repository,
) []config.RepositorySpec {
	adopted := cloneDesiredRepositories(desired)
	indexByRepository := make(map[string]int, len(adopted))
	for i, repository := range adopted {
		indexByRepository[repositoryAdoptKey(organization, repository.Owner, repository.Name)] = i
	}

	for _, actualRepository := range actual {
		key := repositoryAdoptKey(organization, actualRepository.Owner, actualRepository.Name)
		if index, ok := indexByRepository[key]; ok {
			adopted[index] = adoptExistingRepository(organization, adopted[index], actualRepository)
			continue
		}
		indexByRepository[key] = len(adopted)
		adopted = append(adopted, adoptNewRepository(organization, actualRepository))
	}

	return adopted
}

func adoptExistingRepository(
	organization string,
	desired config.RepositorySpec,
	actual state.Repository,
) config.RepositorySpec {
	adopted := desired
	adopted.Owner = bootstrapOwner(organization, actual.Owner)
	adopted.Name = strings.TrimSpace(actual.Name)
	adopted.Visibility = strings.TrimSpace(actual.Visibility)
	adopted.Topics = append([]string{}, actual.Topics...)

	if _, managed := desired.ManagedDescription(); managed {
		adopted.SetManagedDescription(actual.Description)
	}
	if _, managed := desired.ManagedHomepage(); managed {
		adopted.SetManagedHomepage(actual.Homepage)
	}
	if _, managed := desired.ManagedAllowForking(); managed {
		adopted.SetManagedAllowForking(actual.AllowForking)
	}
	if _, managed := desired.ManagedArchived(); managed {
		adopted.SetManagedArchived(actual.Archived)
	}
	if _, managed := desired.ManagedIsTemplate(); managed {
		adopted.SetManagedIsTemplate(actual.IsTemplate)
	}

	return adopted
}

func adoptNewRepository(organization string, actual state.Repository) config.RepositorySpec {
	return config.RepositorySpec{
		Owner:      bootstrapOwner(organization, actual.Owner),
		Name:       strings.TrimSpace(actual.Name),
		Visibility: strings.TrimSpace(actual.Visibility),
		Topics:     append([]string{}, actual.Topics...),
	}
}

func adoptTeams(
	organization string,
	desired []config.TeamSpec,
	actualTeams []state.Team,
	membersByTeam map[string][]config.TeamMemberSpec,
	permissionsByTeam map[string][]config.TeamRepositorySpec,
) []config.TeamSpec {
	adopted := cloneDesiredTeams(desired)
	indexBySlug := make(map[string]int, len(adopted))
	for i, team := range adopted {
		indexBySlug[teamAdoptKey(team.Slug)] = i
	}

	for _, actualTeam := range actualTeams {
		key := teamAdoptKey(actualTeam.Slug)
		if index, ok := indexBySlug[key]; ok {
			adopted[index] = adoptExistingTeam(
				organization,
				adopted[index],
				actualTeam,
				membersByTeam[key],
				permissionsByTeam[key],
			)
			continue
		}
		indexBySlug[key] = len(adopted)
		adopted = append(adopted, adoptNewTeam(
			actualTeam,
			membersByTeam[key],
			permissionsByTeam[key],
		))
	}

	return adopted
}

func adoptExistingTeam(
	organization string,
	desired config.TeamSpec,
	actual state.Team,
	actualMembers []config.TeamMemberSpec,
	actualRepositories []config.TeamRepositorySpec,
) config.TeamSpec {
	adopted := desired
	adopted.Slug = strings.TrimSpace(actual.Slug)
	adopted.Name = strings.TrimSpace(actual.Name)
	adopted.Description = strings.TrimSpace(actual.Description)
	adopted.Privacy = strings.TrimSpace(actual.Privacy)
	adopted.ParentSlug = strings.TrimSpace(actual.ParentSlug)
	adopted.Members = adoptTeamMembers(desired.Members, actualMembers)
	adopted.Repositories = adoptTeamRepositories(organization, desired.Repositories, actualRepositories)
	return adopted
}

func adoptNewTeam(
	actual state.Team,
	actualMembers []config.TeamMemberSpec,
	actualRepositories []config.TeamRepositorySpec,
) config.TeamSpec {
	return config.TeamSpec{
		Slug:         strings.TrimSpace(actual.Slug),
		Name:         strings.TrimSpace(actual.Name),
		Description:  strings.TrimSpace(actual.Description),
		Privacy:      strings.TrimSpace(actual.Privacy),
		ParentSlug:   strings.TrimSpace(actual.ParentSlug),
		Members:      append([]config.TeamMemberSpec{}, actualMembers...),
		Repositories: append([]config.TeamRepositorySpec{}, actualRepositories...),
	}
}

func adoptTeamMembers(
	desired []config.TeamMemberSpec,
	actual []config.TeamMemberSpec,
) []config.TeamMemberSpec {
	adopted := append([]config.TeamMemberSpec{}, desired...)
	indexByUsername := make(map[string]int, len(adopted))
	for i, member := range adopted {
		indexByUsername[organizationMemberKey(member.Username)] = i
	}

	for _, member := range actual {
		key := organizationMemberKey(member.Username)
		if index, ok := indexByUsername[key]; ok {
			adopted[index].Username = strings.TrimSpace(member.Username)
			adopted[index].Role = strings.TrimSpace(member.Role)
			continue
		}
		indexByUsername[key] = len(adopted)
		adopted = append(adopted, member)
	}

	return adopted
}

func adoptTeamRepositories(
	organization string,
	desired []config.TeamRepositorySpec,
	actual []config.TeamRepositorySpec,
) []config.TeamRepositorySpec {
	adopted := append([]config.TeamRepositorySpec{}, desired...)
	indexByRepository := make(map[string]int, len(adopted))
	for i, repository := range adopted {
		indexByRepository[repositoryAdoptKey(organization, repository.Owner, repository.Name)] = i
	}

	for _, repository := range actual {
		key := repositoryAdoptKey(organization, repository.Owner, repository.Name)
		if index, ok := indexByRepository[key]; ok {
			adopted[index].Owner = strings.TrimSpace(repository.Owner)
			adopted[index].Name = strings.TrimSpace(repository.Name)
			adopted[index].Permission = strings.TrimSpace(repository.Permission)
			continue
		}
		indexByRepository[key] = len(adopted)
		adopted = append(adopted, repository)
	}

	return adopted
}

func organizationMemberKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func teamAdoptKey(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func repositoryAdoptKey(organization, owner, name string) string {
	keyOwner := strings.TrimSpace(owner)
	if keyOwner == "" {
		keyOwner = strings.TrimSpace(organization)
	}
	return strings.ToLower(keyOwner) + "\x00" + strings.ToLower(strings.TrimSpace(name))
}
