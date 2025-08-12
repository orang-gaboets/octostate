package organization

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
)

// NewOrganizationCmd creates a new "organization" command group for managing organizations on GitHub.
func NewOrganizationCmd(svc organizations.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "organization",
		Aliases: []string{"organizations", "organisation", "organisations", "org", "orgs"},
		Short:   "Organization operation",
		Long:    "Manage organizations on GitHub",
	}

	cmd.AddCommand(
		GetOrgByName(svc),
	)

	return cmd
}
