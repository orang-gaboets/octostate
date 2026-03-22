package syncfromlive

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

// BootstrapOptions defines the inputs required to generate an initial desired
// GitOps config from live organization state.
type BootstrapOptions struct {
	Actual *state.OrganizationState
}

// Validate checks whether the bootstrap inputs are usable.
func (opt *BootstrapOptions) Validate() error {
	switch {
	case opt.Actual == nil:
		return fmt.Errorf("actual state is required: %w", githubpkg.ErrMissingRequiredField)
	case strings.TrimSpace(opt.Actual.Organization) == "":
		return fmt.Errorf("organization is required: %w", githubpkg.ErrMissingRequiredField)
	default:
		return nil
	}
}

// BuildBootstrapConfig converts live organization state into an initial
// canonical desired GitOps config proposal.
func BuildBootstrapConfig(opt BootstrapOptions) (config.OrganizationConfig, error) {
	if err := opt.Validate(); err != nil {
		return config.OrganizationConfig{}, err
	}

	actual := cloneOrganizationState(opt.Actual)
	organization := strings.TrimSpace(actual.Organization)

	membersByTeam, err := bootstrapTeamMembers(actual.Teams, actual.TeamMembers)
	if err != nil {
		return config.OrganizationConfig{}, err
	}
	permissionsByTeam, err := bootstrapTeamRepositoryPermissions(organization, actual.Teams, actual.TeamRepositoryPermissions)
	if err != nil {
		return config.OrganizationConfig{}, err
	}

	return config.OrganizationConfig{
		Organization: organization,
		Invites:      []config.InviteSpec{},
		Repositories: bootstrapRepositories(organization, actual.Repositories),
		Teams:        bootstrapTeams(actual.Teams, membersByTeam, permissionsByTeam),
	}, nil
}
