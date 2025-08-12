package organization

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
)

// GetOrgByNameCmd creates a new command to get organization details by name.
func GetOrgByNameCmd(svc organizations.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
	)

	cmd := &cobra.Command{
		Use:     "get-by-name",
		Aliases: []string{"gby", "find-by-name", "fetch-by-name", "get", "find", "fetch"},
		Short:   "Get organization details by name",
		Long:    "Retrieve details of a GitHub organization by its name.",
		Example: `
			repo-builder organization get-by-name --token <token> --org <org-name>
			repo-builder organization get-by-name --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Organizations()
			}
			opts := organizations.GetOptions{
				Service: service,
				OrgName: strings.TrimSpace(org),
			}

			_, err := organizations.Get(ctx, opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVarP(&org, "org", "o", "", "Name of the organization to retrieve")

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
