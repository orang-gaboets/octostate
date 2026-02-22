package user

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/users"
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
			repo-builder user get-by-username --token <token> --username <username>
			repo-builder user get-by-username --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --username <username>`,
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
