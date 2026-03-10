package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

const actualSnapshotRelativePath = "actual/snapshot.json"

// ActualSnapshot is the persisted actual-state snapshot written by audit pull.
type ActualSnapshot struct {
	PulledAt                  time.Time                        `json:"pulled_at"`
	Organization              string                           `json:"organization"`
	Members                   []state.OrganizationMember       `json:"members"`
	PendingInvitations        []state.PendingInvitation        `json:"pending_invitations"`
	Repositories              []state.Repository               `json:"repositories"`
	Teams                     []state.Team                     `json:"teams"`
	TeamMembers               []state.TeamMember               `json:"team_members"`
	TeamRepositoryPermissions []state.TeamRepositoryPermission `json:"team_repo_permissions"`
}

// NewActualSnapshot builds a snapshot value from a normalized actual-state model.
func NewActualSnapshot(pulledAt time.Time, actual *state.OrganizationState) ActualSnapshot {
	if actual == nil {
		actual = &state.OrganizationState{}
	}

	clone := *actual
	clone.Normalize()

	return ActualSnapshot{
		PulledAt:                  pulledAt.UTC(),
		Organization:              clone.Organization,
		Members:                   append([]state.OrganizationMember(nil), clone.Members...),
		PendingInvitations:        clonePendingInvitations(clone.PendingInvitations),
		Repositories:              cloneRepositories(clone.Repositories),
		Teams:                     append([]state.Team(nil), clone.Teams...),
		TeamMembers:               append([]state.TeamMember(nil), clone.TeamMembers...),
		TeamRepositoryPermissions: append([]state.TeamRepositoryPermission(nil), clone.TeamRepositoryPermissions...),
	}
}

// ActualPath returns the canonical path of the actual snapshot under stateDir.
func ActualPath(stateDir string) string {
	return filepath.Join(strings.TrimSpace(stateDir), actualSnapshotRelativePath)
}

// WriteActual writes the actual-state snapshot to
// <state-dir>/actual/snapshot.json.
func WriteActual(stateDir string, snapshot ActualSnapshot) (string, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return "", fmt.Errorf("state directory is required")
	}

	path := ActualPath(stateDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create snapshot directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Dir(path), "snapshot-*.json")
	if err != nil {
		return "", fmt.Errorf("create snapshot file: %w", err)
	}
	tempPath := file.Name()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	encodeErr := enc.Encode(snapshot)
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		_ = os.Remove(tempPath)
	}

	switch {
	case encodeErr != nil && closeErr != nil:
		return "", fmt.Errorf("encode snapshot: %w", errors.Join(encodeErr, closeErr))
	case encodeErr != nil:
		return "", fmt.Errorf("encode snapshot: %w", encodeErr)
	case closeErr != nil:
		return "", fmt.Errorf("close snapshot file: %w", closeErr)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("replace snapshot file: %w", err)
	}

	return path, nil
}

func clonePendingInvitations(invitations []state.PendingInvitation) []state.PendingInvitation {
	result := make([]state.PendingInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, state.PendingInvitation{
			ID:        invitation.ID,
			Username:  invitation.Username,
			Email:     invitation.Email,
			UserID:    invitation.UserID,
			Role:      invitation.Role,
			TeamSlugs: append([]string(nil), invitation.TeamSlugs...),
		})
	}
	return result
}

func cloneRepositories(repositories []state.Repository) []state.Repository {
	result := make([]state.Repository, 0, len(repositories))
	for _, repository := range repositories {
		result = append(result, state.Repository{
			Owner:        repository.Owner,
			Name:         repository.Name,
			Visibility:   repository.Visibility,
			Description:  repository.Description,
			Homepage:     repository.Homepage,
			Topics:       append([]string(nil), repository.Topics...),
			AllowForking: repository.AllowForking,
			Archived:     repository.Archived,
			IsTemplate:   repository.IsTemplate,
		})
	}
	return result
}
