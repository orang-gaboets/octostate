package user

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/users"
)

// GetUserByIDCmd creates a command to retrieve a GitHub user by their ID.
func GetUserByIDCmd(svc users.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		userID         int64
	)

	cmd := &cobra.Command{
		Use:     "get-by-id",
		Aliases: []string{"gbi", "find-by-id", "fetch-by-id"},
		Short:   "Get user details by ID",
		Long:    "Retrieve details of a GitHub user by their ID.",
		Example: `
			repo-builder user get-by-id --token <token> --id <user-id>
			repo-builder user get-by-id --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --id <user-id>`,
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
			opts := users.GetUserByIDOptions{
				Service: service,
				ID:      userID,
			}

			userInfo, err := users.GetUserByID(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, userInfo)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().Int64VarP(&userID, "id", "i", 0, "GitHub user ID to retrieve")

	github.MarkRequiredFlags(cmd, "id")

	return cmd
}
