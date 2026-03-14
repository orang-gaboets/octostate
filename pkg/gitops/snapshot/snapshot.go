package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

const actualSnapshotRelativePath = "actual/snapshot.json"

// ActualSnapshot is the persisted actual-state snapshot written by audit pull.
type ActualSnapshot struct {
	PulledAt                     time.Time                        `json:"pulled_at"`
	Organization                 string                           `json:"organization"`
	ResolvedInviteLoginsByUserID map[int64]string                 `json:"resolved_invite_logins_by_user_id"`
	Members                      []state.OrganizationMember       `json:"members"`
	PendingInvitations           []state.PendingInvitation        `json:"pending_invitations"`
	Repositories                 []state.Repository               `json:"repositories"`
	Teams                        []state.Team                     `json:"teams"`
	TeamMembers                  []state.TeamMember               `json:"team_members"`
	TeamRepositoryPermissions    []state.TeamRepositoryPermission `json:"team_repo_permissions"`
}

// NewActualSnapshot builds a snapshot value from a normalized actual-state model.
func NewActualSnapshot(pulledAt time.Time, actual *state.OrganizationState) ActualSnapshot {
	if actual == nil {
		actual = &state.OrganizationState{}
	}

	clone := cloneOrganizationState(actual)
	clone.Normalize()

	return ActualSnapshot{
		PulledAt:                     pulledAt.UTC(),
		Organization:                 clone.Organization,
		ResolvedInviteLoginsByUserID: map[int64]string{},
		Members:                      clone.Members,
		PendingInvitations:           clone.PendingInvitations,
		Repositories:                 clone.Repositories,
		Teams:                        clone.Teams,
		TeamMembers:                  clone.TeamMembers,
		TeamRepositoryPermissions:    clone.TeamRepositoryPermissions,
	}
}

// ActualPath returns the canonical path of the actual snapshot under stateDir.
func ActualPath(stateDir string) string {
	return filepath.Join(strings.TrimSpace(stateDir), actualSnapshotRelativePath)
}

// ReadActual loads the actual-state snapshot from
// <state-dir>/actual/snapshot.json.
func ReadActual(stateDir string) (*ActualSnapshot, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return nil, fmt.Errorf("state directory must not be empty")
	}

	path := ActualPath(stateDir)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var snapshot ActualSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode actual-state snapshot %s: %w", path, err)
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode actual-state snapshot %s: %w", path, err)
	}
	if len(extra) > 0 {
		return nil, fmt.Errorf("decode actual-state snapshot %s: multiple JSON values are not allowed", path)
	}

	normalizeActualSnapshot(&snapshot)
	return &snapshot, nil
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
			Role:      invitation.Role,
			TeamSlugs: append([]string{}, invitation.TeamSlugs...),
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
			Topics:       append([]string{}, repository.Topics...),
			AllowForking: repository.AllowForking,
			Archived:     repository.Archived,
			IsTemplate:   repository.IsTemplate,
		})
	}
	return result
}

func cloneOrganizationState(actual *state.OrganizationState) state.OrganizationState {
	if actual == nil {
		return state.OrganizationState{}
	}

	return state.OrganizationState{
		Organization:              actual.Organization,
		Members:                   append([]state.OrganizationMember{}, actual.Members...),
		PendingInvitations:        clonePendingInvitations(actual.PendingInvitations),
		Repositories:              cloneRepositories(actual.Repositories),
		Teams:                     append([]state.Team{}, actual.Teams...),
		TeamMembers:               append([]state.TeamMember{}, actual.TeamMembers...),
		TeamRepositoryPermissions: append([]state.TeamRepositoryPermission{}, actual.TeamRepositoryPermissions...),
	}
}

func normalizeActualSnapshot(snapshot *ActualSnapshot) {
	if snapshot == nil {
		return
	}

	actual := state.OrganizationState{
		Organization:              snapshot.Organization,
		Members:                   append([]state.OrganizationMember{}, snapshot.Members...),
		PendingInvitations:        clonePendingInvitations(snapshot.PendingInvitations),
		Repositories:              cloneRepositories(snapshot.Repositories),
		Teams:                     append([]state.Team{}, snapshot.Teams...),
		TeamMembers:               append([]state.TeamMember{}, snapshot.TeamMembers...),
		TeamRepositoryPermissions: append([]state.TeamRepositoryPermission{}, snapshot.TeamRepositoryPermissions...),
	}
	actual.Normalize()

	snapshot.Organization = actual.Organization
	snapshot.ResolvedInviteLoginsByUserID = normalizeResolvedInviteLoginsByUserID(snapshot.ResolvedInviteLoginsByUserID)
	snapshot.Members = actual.Members
	snapshot.PendingInvitations = actual.PendingInvitations
	snapshot.Repositories = actual.Repositories
	snapshot.Teams = actual.Teams
	snapshot.TeamMembers = actual.TeamMembers
	snapshot.TeamRepositoryPermissions = actual.TeamRepositoryPermissions
}

func normalizeResolvedInviteLoginsByUserID(values map[int64]string) map[int64]string {
	if len(values) == 0 {
		return map[int64]string{}
	}

	normalized := make(map[int64]string, len(values))
	for userID, login := range values {
		login = strings.TrimSpace(login)
		if userID <= 0 || login == "" {
			continue
		}
		normalized[userID] = login
	}
	if len(normalized) == 0 {
		return map[int64]string{}
	}
	return normalized
}
