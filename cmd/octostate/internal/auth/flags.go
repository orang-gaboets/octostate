package auth

import "github.com/spf13/cobra"

// AddFlags registers authentication flags for personal access token and GitHub App credentials.
func AddFlags(cmd *cobra.Command, token *string, appID, installationID *int64, appKeyPath *string) {
	cmd.Flags().StringVar(token, "token", "", "GitHub access token (prefer OCTOSTATE_GITHUB_TOKEN)")
	cmd.Flags().Int64Var(appID, "app-id", 0, "GitHub App ID for authentication")
	cmd.Flags().Int64Var(installationID, "installation-id", 0, "GitHub App installation ID for authentication")
	cmd.Flags().StringVar(appKeyPath, "app-key-path", "", "Path to the GitHub App private key file")
}
