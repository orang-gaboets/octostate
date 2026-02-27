package topic

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// NewTopicCmd creates a new "topic" command group for managing topic on GitHub
func NewTopicCmd(svc topics.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "topic",
		Aliases: []string{"topics"},
		Short:   "Topic operation",
		Long:    "Manage repositories' topics on GitHub",
	}

	cmd.AddCommand(
		AddTopicsCmd(svc),
		ListAllTopicsCmd(svc),
		ReplaceAllTopicsCmd(svc),
	)

	return cmd
}
