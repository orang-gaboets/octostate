package state

import (
	"slices"
	"strings"
)

// OrganizationState is the normalized actual state for one GitHub organization.
type OrganizationState struct {
	Organization              string                     `json:"organization"`
	Members                   []OrganizationMember       `json:"members"`
	PendingInvitations        []PendingInvitation        `json:"pending_invitations"`
	Repositories              []Repository               `json:"repositories"`
	Teams                     []Team                     `json:"teams"`
	TeamMembers               []TeamMember               `json:"team_members"`
	TeamRepositoryPermissions []TeamRepositoryPermission `json:"team_repo_permissions"`
}

// OrganizationMember is a current organization member.
type OrganizationMember struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// PendingInvitation is a pending organization invitation.
type PendingInvitation struct {
	ID        int64    `json:"id"`
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`
	TeamSlugs []string `json:"team_slugs"`
}

// Repository is the actual repository state relevant to GitOps planning.
type Repository struct {
	Owner        string   `json:"owner"`
	Name         string   `json:"name"`
	Visibility   string   `json:"visibility"`
	Description  string   `json:"description"`
	Homepage     string   `json:"homepage"`
	Topics       []string `json:"topics"`
	AllowForking bool     `json:"allow_forking"`
	Archived     bool     `json:"archived"`
	IsTemplate   bool     `json:"is_template"`
}

// Team is the actual team state relevant to GitOps planning.
type Team struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Privacy     string `json:"privacy"`
	ParentSlug  string `json:"parent_slug"`
}

// TeamMember is a current team membership.
type TeamMember struct {
	TeamSlug string `json:"team_slug"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// TeamRepositoryPermission is a current team-to-repository permission entry.
type TeamRepositoryPermission struct {
	TeamSlug   string `json:"team_slug"`
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	Permission string `json:"permission"`
}

// Normalize initializes nil slices and sorts collections so downstream
// planning, snapshotting, and tests can rely on deterministic ordering.
func (s *OrganizationState) Normalize() {
	if s == nil {
		return
	}

	if s.Members == nil {
		s.Members = []OrganizationMember{}
	}
	if s.PendingInvitations == nil {
		s.PendingInvitations = []PendingInvitation{}
	}
	if s.Repositories == nil {
		s.Repositories = []Repository{}
	}
	if s.Teams == nil {
		s.Teams = []Team{}
	}
	if s.TeamMembers == nil {
		s.TeamMembers = []TeamMember{}
	}
	if s.TeamRepositoryPermissions == nil {
		s.TeamRepositoryPermissions = []TeamRepositoryPermission{}
	}

	for i := range s.PendingInvitations {
		if s.PendingInvitations[i].TeamSlugs == nil {
			s.PendingInvitations[i].TeamSlugs = []string{}
		}
		slices.SortFunc(s.PendingInvitations[i].TeamSlugs, compareStrings)
	}

	for i := range s.Repositories {
		if s.Repositories[i].Topics == nil {
			s.Repositories[i].Topics = []string{}
		}
		slices.SortFunc(s.Repositories[i].Topics, compareStrings)
	}

	slices.SortFunc(s.Members, compareOrganizationMembers)
	slices.SortFunc(s.PendingInvitations, comparePendingInvitations)
	slices.SortFunc(s.Repositories, compareRepositories)
	slices.SortFunc(s.Teams, compareTeams)
	slices.SortFunc(s.TeamMembers, compareTeamMembers)
	slices.SortFunc(s.TeamRepositoryPermissions, compareTeamRepositoryPermissions)
}

func compareOrganizationMembers(a, b OrganizationMember) int {
	if diff := compareStrings(a.Username, b.Username); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.Role, b.Role); diff != 0 {
		return diff
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	if diff := compareStrings(a.Email, b.Email); diff != 0 {
		return diff
	}
	return compareStrings(a.Name, b.Name)
}

func comparePendingInvitations(a, b PendingInvitation) int {
	if diff := compareStrings(a.Username, b.Username); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.Email, b.Email); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.Role, b.Role); diff != 0 {
		return diff
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return compareStringSlices(a.TeamSlugs, b.TeamSlugs)
}

func compareRepositories(a, b Repository) int {
	if diff := compareStrings(a.Owner, b.Owner); diff != 0 {
		return diff
	}
	return compareStrings(a.Name, b.Name)
}

func compareTeams(a, b Team) int {
	if diff := compareStrings(a.Slug, b.Slug); diff != 0 {
		return diff
	}
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return compareStrings(a.Name, b.Name)
}

func compareTeamMembers(a, b TeamMember) int {
	if diff := compareStrings(a.TeamSlug, b.TeamSlug); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.Username, b.Username); diff != 0 {
		return diff
	}
	return compareStrings(a.Role, b.Role)
}

func compareTeamRepositoryPermissions(a, b TeamRepositoryPermission) int {
	if diff := compareStrings(a.TeamSlug, b.TeamSlug); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.Owner, b.Owner); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.Name, b.Name); diff != 0 {
		return diff
	}
	return compareStrings(a.Permission, b.Permission)
}

func compareStrings(a, b string) int {
	aKey := strings.ToLower(a)
	bKey := strings.ToLower(b)
	if aKey < bKey {
		return -1
	}
	if aKey > bKey {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareStringSlices(a, b []string) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := range limit {
		if diff := compareStrings(a[i], b[i]); diff != 0 {
			return diff
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}
