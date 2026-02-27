package user

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/users"
)

// NewUserCmd creates a new "user" command group for managing GitHub users.
func NewUserCmd(svc users.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "user",
		Aliases: []string{"users"},
		Short:   "User operation",
		Long:    "Manage users on GitHub",
	}

	cmd.AddCommand(
		GetUserByIDCmd(svc),
		GetUserByUsernameCmd(svc),
	)

	return cmd
}
