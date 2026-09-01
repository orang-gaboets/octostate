package user

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/users"
)

// GetUserByUsernameCmd creates a command to retrieve a GitHub user by their username.
func GetUserByUsernameCmd(svc users.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		username       string
	)

	cmd := &cobra.Command{
		Use:     "get-by-username",
		Aliases: []string{"gbu", "find-by-username", "fetch-by-username"},
		Short:   "Get user details by username",
		Long:    "Retrieve details of a GitHub user by their username.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate user get-by-username --username <username>
			octostate user get-by-username --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --username <username>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Users()
			}
			opts := users.GetUserByUsernameOptions{
				Service:  service,
				Username: strings.TrimSpace(username),
			}

			userInfo, err := users.GetUserByUsername(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, userInfo)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVarP(&username, "username", "u", "", "GitHub username to retrieve")

	github.MarkRequiredFlags(cmd, "username")

	return cmd
}
