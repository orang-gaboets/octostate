package repo

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func selectedVisibility(cmd *cobra.Command, legacyPrivate bool, visibility string) (string, error) {
	legacySet := cmd.Flags().Changed("private")
	newSet := cmd.Flags().Changed("visibility")
	if legacySet && newSet {
		return "", fmt.Errorf("--private cannot be combined with --visibility")
	}
	if newSet {
		value := strings.TrimSpace(visibility)
		switch value {
		case "public", "private", "internal":
			return value, nil
		default:
			return "", fmt.Errorf("invalid --visibility value %q (must be public, private, or internal)", visibility)
		}
	}
	if legacySet && legacyPrivate {
		return "private", nil
	}
	return "public", nil
}
