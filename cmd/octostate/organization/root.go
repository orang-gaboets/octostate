package organization

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/v2/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/v2/pkg/github/repos"
	"github.com/orang-gaboets/octostate/v2/pkg/github/users"
)

// NewOrganizationCmd creates a new "organization" command group for managing organizations on GitHub.
func NewOrganizationCmd(orgSvc organizations.Service, reposSvc repos.Service, usersSvc users.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "organization",
		Aliases: []string{"organizations", "organisation", "organisations", "org", "orgs"},
		Short:   "Organization operation",
		Long:    "Manage organizations on GitHub",
	}

	cmd.AddCommand(
		GetOrgByNameCmd(orgSvc),
		InviteCmd(orgSvc, usersSvc),
		MembershipCmd(orgSvc),
		ListOrgInvitationsCmd(orgSvc),
		ListOrgMembersCmd(orgSvc),
		ListOrgReposCmd(reposSvc),
		ListOrgTeamsCmd(nil),
	)

	return cmd
}
