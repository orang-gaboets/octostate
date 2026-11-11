package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orang-gaboets/octostate/internal/filereplace"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/sizeguard"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

const (
	actualSnapshotRelativePath        = "actual/snapshot.json"
	maxActualSnapshotJSONSize   int64 = 128 << 20
	actualSnapshotSizeDisplay         = "128 MiB"
	maxActualSnapshotJSONTokens int64 = 12_000_000
	actualSnapshotTokensDisplay       = "12 million JSON tokens"
	// This aggregate ceiling bounds the fixed-width slices and their copies
	// made by normalizeActualSnapshot.
	maxActualSnapshotJSONArrayElements int64 = 1_000_000
	actualSnapshotArrayElementsDisplay       = "1 million JSON array elements"
	// The per-object ceiling bounds dynamic maps such as resolved invite IDs.
	maxActualSnapshotJSONObjectEntries int64 = 250_000
	actualSnapshotObjectEntriesDisplay       = "250,000 entries per JSON object"
)

type snapshotJSONLimits struct {
	tokens                 int64
	arrayElements          int64
	objectEntriesPerObject int64
}

type snapshotJSONContainer struct {
	delimiter          json.Delim
	entries            int64
	expectingObjectKey bool
}

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

// ActualPath returns the lexical path of the actual snapshot under stateDir.
// It does not inspect or resolve filesystem components; ReadActual and
// WriteActual reject symlinked components and, on Windows, reparse points,
// while using rooted operations to prevent path replacement from redirecting
// snapshot access.
func ActualPath(stateDir string) string {
	return filepath.Join(strings.TrimSpace(stateDir), actualSnapshotRelativePath)
}

// ReadActual loads the actual-state snapshot from
// <state-dir>/actual/snapshot.json. It rejects symlinked path components and,
// on Windows, reparse points; an existing snapshot must be a regular file.
// Rooted file operations prevent later path changes from redirecting the read.
func ReadActual(stateDir string) (*ActualSnapshot, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return nil, fmt.Errorf("state directory is required")
	}

	path := ActualPath(stateDir)
	parent, name, err := openSnapshotParent(path, false)
	if err != nil {
		return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, err)
	}
	defer func() {
		_ = parent.Close() //nolint:errcheck // best-effort cleanup after reading the snapshot
	}()

	snapshot, err := readActualSnapshotAt(parent, name, path)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func readActualSnapshotAt(parent *os.Root, name, path string) (*ActualSnapshot, error) {
	return readActualSnapshotAtWithHook(parent, name, path, nil)
}

// readActualSnapshotAtWithHook lets tests replace the file after inspection
// and before Open without relying on timing-sensitive races.
func readActualSnapshotAtWithHook(parent *os.Root, name, path string, afterLstat func(path string)) (*ActualSnapshot, error) {
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, err)
	}
	if isSymlinkOrReparsePoint(info) {
		return nil, fmt.Errorf("read actual-state snapshot %s: unsafe snapshot path component %q: %s", path, path, unsafeSnapshotPathReason)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read actual-state snapshot %s: unsafe snapshot path component %q: destination must be a regular file", path, path)
	}
	if afterLstat != nil {
		afterLstat(path)
	}

	file, err := parent.Open(name)
	if err != nil {
		// Do not include the OS error: it can contain a raced symlink target.
		return nil, fmt.Errorf("open actual-state snapshot %s safely", path)
	}
	defer func() {
		_ = file.Close() //nolint:errcheck // best-effort cleanup after reading the snapshot
	}()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened actual-state snapshot %s: %w", path, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, fmt.Errorf("read actual-state snapshot %s: unsafe snapshot path component %q: changed while opening", path, path)
	}
	if openedInfo.Size() > maxActualSnapshotJSONSize {
		return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, actualSnapshotSizeLimitError(&sizeguard.LimitExceededError{Limit: maxActualSnapshotJSONSize}))
	}

	var data []byte
	processErr := sizeguard.Process(file, maxActualSnapshotJSONSize, func(r io.Reader) error {
		var err error
		data, err = io.ReadAll(r)
		if err != nil {
			return &sizeguard.ReadError{Err: err}
		}
		return nil
	})
	if processErr != nil {
		if errors.Is(processErr, sizeguard.ErrLimitExceeded) {
			return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, actualSnapshotSizeLimitError(processErr))
		}
		var readErr *sizeguard.ReadError
		if errors.As(processErr, &readErr) {
			return nil, fmt.Errorf("read actual-state snapshot %s: %w", path, processErr)
		}
		return nil, processErr
	}

	snapshot, err := decodeSnapshotJSON(data, actualSnapshotJSONLimits())
	if err != nil {
		return nil, fmt.Errorf("decode actual-state snapshot %s: %w", path, err)
	}
	if err := normalizeActualSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("normalize actual-state snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

func decodeSnapshotJSON(data []byte, limits snapshotJSONLimits) (*ActualSnapshot, error) {
	if err := validateSnapshotJSONComplexity(data, limits); err != nil {
		return nil, err
	}

	var snapshot ActualSnapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(extra) > 0 {
		return nil, fmt.Errorf("multiple JSON values are not allowed")
	}
	return &snapshot, nil
}

func actualSnapshotJSONLimits() snapshotJSONLimits {
	return snapshotJSONLimits{
		tokens:                 maxActualSnapshotJSONTokens,
		arrayElements:          maxActualSnapshotJSONArrayElements,
		objectEntriesPerObject: maxActualSnapshotJSONObjectEntries,
	}
}

func validateSnapshotJSONComplexity(data []byte, limits snapshotJSONLimits) error {
	if limits.tokens < 0 || limits.arrayElements < 0 || limits.objectEntriesPerObject < 0 {
		return fmt.Errorf("JSON complexity limits must not be negative")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	containers := make([]snapshotJSONContainer, 0, 16)
	var tokenCount int64
	var arrayElementCount int64
	rootStarted := false
	rootComplete := false

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			if !rootStarted {
				return fmt.Errorf("empty JSON input")
			}
			if len(containers) != 0 {
				return fmt.Errorf("unexpected end of JSON input")
			}
			return nil
		}
		if err != nil {
			return err
		}
		if rootComplete {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		if tokenCount == limits.tokens {
			return fmt.Errorf("actual-state snapshot exceeds the supported JSON token limit of %s", snapshotJSONTokenLimitDisplay(limits.tokens))
		}
		tokenCount++

		isObjectKey, err := accountSnapshotJSONParentValue(token, containers, limits, &arrayElementCount)
		if err != nil {
			return err
		}
		if isObjectKey {
			continue
		}

		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{', '[':
				if !rootStarted {
					rootStarted = true
				}
				containers = append(containers, snapshotJSONContainer{
					delimiter:          delimiter,
					expectingObjectKey: delimiter == '{',
				})
			case '}', ']':
				if len(containers) == 0 || !snapshotJSONDelimitersMatch(delimiter, containers[len(containers)-1].delimiter) {
					return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
				}
				containers = containers[:len(containers)-1]
				if len(containers) == 0 {
					rootComplete = true
				}
			}
			continue
		}
		if !rootStarted {
			rootStarted = true
			rootComplete = true
		} else if len(containers) == 0 {
			rootComplete = true
		}
	}
}

func accountSnapshotJSONParentValue(token any, containers []snapshotJSONContainer, limits snapshotJSONLimits, arrayElementCount *int64) (bool, error) {
	if len(containers) == 0 {
		return false, nil
	}
	if delimiter, ok := token.(json.Delim); ok && (delimiter == '}' || delimiter == ']') {
		return false, nil
	}

	parent := &containers[len(containers)-1]
	if parent.delimiter == '{' && parent.expectingObjectKey {
		if _, ok := token.(string); !ok {
			return false, fmt.Errorf("unexpected JSON object entry")
		}
		parent.entries++
		if parent.entries > limits.objectEntriesPerObject {
			return false, fmt.Errorf("actual-state snapshot exceeds the supported JSON object entry limit of %s", snapshotJSONObjectEntryLimitDisplay(limits.objectEntriesPerObject))
		}
		parent.expectingObjectKey = false
		return true, nil
	}
	if parent.delimiter == '[' {
		*arrayElementCount++
		if *arrayElementCount > limits.arrayElements {
			return false, fmt.Errorf("actual-state snapshot exceeds the supported JSON array element limit of %s", snapshotJSONArrayElementLimitDisplay(limits.arrayElements))
		}
	} else {
		parent.expectingObjectKey = true
	}
	return false, nil
}

func snapshotJSONDelimitersMatch(closing, opening json.Delim) bool {
	return (closing == '}' && opening == '{') || (closing == ']' && opening == '[')
}

func snapshotJSONTokenLimitDisplay(limit int64) string {
	if limit == maxActualSnapshotJSONTokens {
		return actualSnapshotTokensDisplay
	}
	return fmt.Sprintf("%d JSON tokens", limit)
}

func snapshotJSONArrayElementLimitDisplay(limit int64) string {
	if limit == maxActualSnapshotJSONArrayElements {
		return actualSnapshotArrayElementsDisplay
	}
	return fmt.Sprintf("%d JSON array elements", limit)
}

func snapshotJSONObjectEntryLimitDisplay(limit int64) string {
	if limit == maxActualSnapshotJSONObjectEntries {
		return actualSnapshotObjectEntriesDisplay
	}
	return fmt.Sprintf("%d entries per JSON object", limit)
}

// WriteActual writes the actual-state snapshot to
// <state-dir>/actual/snapshot.json. It rejects symlinked path components and,
// on Windows, reparse points; an existing snapshot must be a regular file.
// Rooted file operations prevent later path changes from redirecting the write.
func WriteActual(stateDir string, snapshot ActualSnapshot) (string, error) {
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return "", fmt.Errorf("state directory is required")
	}
	if err := normalizeActualSnapshot(&snapshot); err != nil {
		return "", fmt.Errorf("normalize snapshot: %w", err)
	}

	path := ActualPath(stateDir)
	parent, name, err := openSnapshotParent(path, true)
	if err != nil {
		return "", fmt.Errorf("write snapshot %s: %w", path, err)
	}
	defer func() {
		_ = parent.Close() //nolint:errcheck // best-effort cleanup after writing the snapshot
	}()
	if err := writeActualSnapshotAt(parent, name, path, snapshot); err != nil {
		return "", err
	}

	return path, nil
}

func writeActualSnapshotAt(parent *os.Root, name, path string, snapshot ActualSnapshot) error {
	if info, err := parent.Lstat(name); err == nil {
		if isSymlinkOrReparsePoint(info) {
			return fmt.Errorf("write snapshot %s: unsafe snapshot path component %q: %s", path, path, unsafeSnapshotPathReason)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("write snapshot %s: unsafe snapshot path component %q: destination must be a regular file", path, path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("write snapshot %s: inspect destination: %w", path, err)
	}

	encoded, err := encodeActualSnapshot(snapshot, actualSnapshotJSONLimits())
	if err != nil {
		return fmt.Errorf("write snapshot %s: encode snapshot: %w", path, err)
	}

	// 0600 matches what the previous os.CreateTemp path produced on first
	// write. The snapshot carries organization member and invitation data,
	// so first creation must not widen it.
	if err := filereplace.WriteFileInRoot(parent, name, encoded, 0o600); err != nil {
		return fmt.Errorf("write snapshot %s: %w", path, err)
	}

	return nil
}

func encodeActualSnapshot(snapshot ActualSnapshot, limits snapshotJSONLimits) ([]byte, error) {
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	// json.Encoder.Encode terminated the document with a newline; MarshalIndent
	// does not, so add it back to keep the file byte-identical.
	encoded = append(encoded, '\n')
	if err := sizeguard.CheckSize(encoded, maxActualSnapshotJSONSize); err != nil {
		return nil, actualSnapshotSizeLimitError(err)
	}
	if err := validateSnapshotJSONComplexity(encoded, limits); err != nil {
		return nil, err
	}
	return encoded, nil
}

func actualSnapshotSizeLimitError(err error) error {
	return fmt.Errorf("actual-state snapshot exceeds the supported size limit of %s: %w", actualSnapshotSizeDisplay, err)
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
