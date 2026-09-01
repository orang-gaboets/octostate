package auth

import "github.com/spf13/cobra"

const explicitEmptyToken = "\x00"

type tokenFlagValue struct {
	token *string
}

func (v tokenFlagValue) Set(value string) error {
	if value == "" {
		*v.token = explicitEmptyToken
		return nil
	}
	*v.token = value
	return nil
}

func (v tokenFlagValue) String() string {
	if *v.token == explicitEmptyToken {
		return ""
	}
	return *v.token
}

func (tokenFlagValue) Type() string { return "string" }

// AddFlags registers authentication flags for personal access token and GitHub App credentials.
func AddFlags(cmd *cobra.Command, token *string, appID, installationID *int64, appKeyPath *string) {
	cmd.Flags().Var(tokenFlagValue{token: token}, "token", "GitHub access token (prefer OCTOSTATE_GITHUB_TOKEN)")
	cmd.Flags().Int64Var(appID, "app-id", 0, "GitHub App ID for authentication")
	cmd.Flags().Int64Var(installationID, "installation-id", 0, "GitHub App installation ID for authentication")
	cmd.Flags().StringVar(appKeyPath, "app-key-path", "", "Path to the GitHub App private key file")
}
