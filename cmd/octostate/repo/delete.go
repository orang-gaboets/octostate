package repo

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
)

// DeleteRepoCmd creates a new command to delete a GitHub repository.
func DeleteRepoCmd(svc repos.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
		dryRun         bool
		yes            bool
	)

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete a GitHub repository",
		Long:    "Delete a specified GitHub repository by providing the organization and repository name.",
		Example: `
			octostate repo delete --token <token> --org <org> --name <repo-name> --yes
			octostate repo delete --token <token> --org <org> --name <repo-name> --dry-run
			octostate repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path> --org <org> --name <repo-name> --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := safety.RequireYesOrDryRun(yes, dryRun); err != nil {
				return err
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would delete repository %s/%s", org, name),
					map[string]any{
						"owner": org,
						"name":  name,
					},
				)
			}

			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Repositories()
			}

			opts := repos.DeleteOptions{
				Repo:    name,
				Owner:   org,
				Service: service,
			}

			if err := repos.Delete(ctx, opts); err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Deleted repository %s/%s", org, name),
				map[string]any{
					"owner": org,
					"name":  name,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name to delete")
	safety.AddDryRunFlag(cmd, &dryRun)
	safety.AddYesFlag(cmd, &yes)

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
