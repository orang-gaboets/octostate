package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/orang-gaboets/octostate/internal/filereplace"
	"time"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

const actualSnapshotRelativePath = "actual/snapshot.json"

// ActualSnapshot is the persisted actual-state snapshot written by audit pull.
type ActualSnapshot struct {
	PulledAt                        time.Time                        `json:"pulled_at"`
	Organization                    string                           `json:"organization"`
	ResolvedInviteUserIDsByUsername map[string]int64                 `json:"resolved_invite_user_ids_by_username"`
	Members                         []state.OrganizationMember       `json:"members"`
	PendingInvitations              []state.PendingInvitation        `json:"pending_invitations"`
	Repositories                    []state.Repository               `json:"repositories"`
	Teams                           []state.Team                     `json:"teams"`
	TeamMembers                     []state.TeamMember               `json:"team_members"`
	TeamRepositoryPermissions       []state.TeamRepositoryPermission `json:"team_repo_permissions"`
}

// NewActualSnapshot builds a snapshot value from a normalized actual-state model.
func NewActualSnapshot(pulledAt time.Time, actual *state.OrganizationState) ActualSnapshot {
	if actual == nil {
		actual = &state.OrganizationState{}
	}

	clone := cloneOrganizationState(actual)
	clone.Normalize()

	return ActualSnapshot{
		PulledAt:                        pulledAt.UTC(),
		Organization:                    clone.Organization,
		ResolvedInviteUserIDsByUsername: map[string]int64{},
		Members:                         clone.Members,
		PendingInvitations:              clone.PendingInvitations,
		Repositories:                    clone.Repositories,
		Teams:                           clone.Teams,
		TeamMembers:                     clone.TeamMembers,
		TeamRepositoryPermissions:       clone.TeamRepositoryPermissions,
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
		return nil, fmt.Errorf("state directory is required")
	}

	path := ActualPath(stateDir)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, err)
	}
	defer func() {
		_ = file.Close() //nolint:errcheck // best-effort cleanup after reading the snapshot
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

	if err := normalizeActualSnapshot(&snapshot); err != nil {
		return nil, fmt.Errorf("normalize actual-state snapshot %s: %w", path, err)
	}
	return &snapshot, nil
}

// WriteActual writes the actual-state snapshot to
// <state-dir>/actual/snapshot.json.
func WriteActual(stateDir string, snapshot ActualSnapshot) (string, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return "", fmt.Errorf("state directory is required")
	}
	if err := normalizeActualSnapshot(&snapshot); err != nil {
		return "", fmt.Errorf("normalize snapshot: %w", err)
	}

	path := ActualPath(stateDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create snapshot directory: %w", err)
	}

	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode snapshot: %w", err)
	}
	// json.Encoder.Encode terminated the document with a newline; MarshalIndent
	// does not, so add it back to keep the file byte-identical.
	encoded = append(encoded, '\n')

	if err := filereplace.WriteFile(path, encoded, 0o644); err != nil {
		return "", fmt.Errorf("write snapshot: %w", err)
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

func normalizeActualSnapshot(snapshot *ActualSnapshot) error {
	if snapshot == nil {
		return nil
	}
	if !snapshot.PulledAt.IsZero() {
		snapshot.PulledAt = snapshot.PulledAt.UTC()
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
	if err := validateOrganizationMemberRoles(actual.Members); err != nil {
		return err
	}

	resolvedInviteUserIDsByUsername, err := normalizeResolvedInviteUserIDsByUsername(snapshot.ResolvedInviteUserIDsByUsername)
	if err != nil {
		return err
	}

	snapshot.Organization = actual.Organization
	snapshot.ResolvedInviteUserIDsByUsername = resolvedInviteUserIDsByUsername
	snapshot.Members = actual.Members
	snapshot.PendingInvitations = actual.PendingInvitations
	snapshot.Repositories = actual.Repositories
	snapshot.Teams = actual.Teams
	snapshot.TeamMembers = actual.TeamMembers
	snapshot.TeamRepositoryPermissions = actual.TeamRepositoryPermissions
	return nil
}

func validateOrganizationMemberRoles(members []state.OrganizationMember) error {
	for _, member := range members {
		username := strings.TrimSpace(member.Username)
		role := strings.TrimSpace(member.Role)
		if username == "" {
			return fmt.Errorf("snapshot organization member username is required: %w", githubpkg.ErrInvalidFieldValue)
		}
		switch role {
		case "admin", "member":
		default:
			return fmt.Errorf(
				"snapshot organization member %q has unsupported or missing role %q; re-run audit pull to refresh actual state: %w",
				username,
				role,
				githubpkg.ErrInvalidFieldValue,
			)
		}
	}
	return nil
}

// NormalizeResolvedInviteUserIDsByUsername canonicalizes username keys and
// rejects conflicting entries that collapse to the same canonical username.
func NormalizeResolvedInviteUserIDsByUsername(values map[string]int64) (map[string]int64, error) {
	return normalizeResolvedInviteUserIDsByUsername(values)
}

func normalizeResolvedInviteUserIDsByUsername(values map[string]int64) (map[string]int64, error) {
	if len(values) == 0 {
		return map[string]int64{}, nil
	}

	normalized := make(map[string]int64, len(values))
	for username, userID := range values {
		username = strings.ToLower(strings.TrimSpace(username))
		if username == "" || userID <= 0 {
			continue
		}
		if existingUserID, ok := normalized[username]; ok && existingUserID != userID {
			return nil, fmt.Errorf(
				"resolved invite user IDs contain conflicting entries for username %q: %w",
				username,
				githubpkg.ErrInvalidFieldValue,
			)
		}
		normalized[username] = userID
	}
	if len(normalized) == 0 {
		return map[string]int64{}, nil
	}
	return normalized, nil
}
