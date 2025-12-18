package github

import (
	"encoding/json"
	"time"

	gh "github.com/google/go-github/v55/github"
)

// Repository contains the repository details.
type Repository struct {
	Owner       string
	Name        string
	Private     bool
	Description string
	Topics      []string
}

// String returns a string representation of the Repository.
func (r *Repository) String() string {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "Repository<marshal error>"
	}
	return string(b)
}

// RepositoryFromGhRepo converts a GitHub repository from the go-github library
// to the internal Repository type.
func RepositoryFromGhRepo(ghRepo *gh.Repository) *Repository {
	if ghRepo == nil {
		return nil
	}

	owner := ghRepo.GetOwner()

	return &Repository{
		Owner:       owner.GetLogin(),
		Name:        ghRepo.GetName(),
		Private:     ghRepo.GetPrivate(),
		Description: ghRepo.GetDescription(),
		Topics:      ghRepo.Topics,
	}
}

// RepositoriesFromGhRepos converts a slice of GitHub repositories to a slice
// of internal Repository types.
func RepositoriesFromGhRepos(ghRepos []*gh.Repository) []Repository {
	repos := make([]Repository, 0, len(ghRepos))
	for _, r := range ghRepos {
		if repo := RepositoryFromGhRepo(r); repo != nil {
			repos = append(repos, *repo)
		}
	}
	return repos
}

// TeamPrivacy defines the privacy level of a GitHub team.
type TeamPrivacy string

// Predefined values for TeamPrivacy.
var (
	// TeamPrivacySecret indicates a secret team.
	TeamPrivacySecret TeamPrivacy = "secret"
	// TeamPrivacyClosed indicates a visible team.
	TeamPrivacyClosed TeamPrivacy = "closed"
)

// Helper functions for TeamPrivacy

// IsValid checks if the TeamPrivacy value is valid.
func (tp TeamPrivacy) IsValid() bool {
	switch tp {
	case TeamPrivacySecret, TeamPrivacyClosed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the TeamPrivacy value.
func (tp TeamPrivacy) String() string {
	return string(tp)
}

// IsSecret checks if the TeamPrivacy is set to secret.
func (tp TeamPrivacy) IsSecret() bool {
	return tp == TeamPrivacySecret
}

// PrivacyFromBool converts a boolean to a TeamPrivacy value.
func PrivacyFromBool(secret bool) TeamPrivacy {
	if secret {
		return TeamPrivacySecret
	}
	return TeamPrivacyClosed
}

// TeamNotificationSettings defines the notification settings for a GitHub team.
type TeamNotificationSettings string

// Predefined values for TeamNotificationSettings.
var (
	// TeamNotificationSettingsEnabled indicates that notifications are enabled for the team.
	TeamNotificationSettingsEnabled TeamNotificationSettings = "notifications_enabled"
	// TeamNotificationSettingsDisabled indicates that notifications are disabled for the team.
	TeamNotificationSettingsDisabled TeamNotificationSettings = "notifications_disabled"
)

// Helper functions for TeamNotificationSettings

// IsValid checks if the TeamNotificationSettings value is valid.
func (tns TeamNotificationSettings) IsValid() bool {
	switch tns {
	case TeamNotificationSettingsEnabled, TeamNotificationSettingsDisabled:
		return true
	default:
		return false
	}
}

// String returns the string representation of the TeamNotificationSettings value.
func (tns TeamNotificationSettings) String() string {
	return string(tns)
}

// IsEnabled checks if the TeamNotificationSettings is set to enabled.
func (tns TeamNotificationSettings) IsEnabled() bool {
	return tns == TeamNotificationSettingsEnabled
}

// NotificationSettingsFromBool converts a boolean to a TeamNotificationSettings value.
func NotificationSettingsFromBool(enabled bool) TeamNotificationSettings {
	if enabled {
		return TeamNotificationSettingsEnabled
	}
	return TeamNotificationSettingsDisabled
}

// Team represents a GitHub team.
type Team struct {
	ID                   int64
	Slug                 string
	Org                  string
	Name                 string
	Description          string
	Privacy              TeamPrivacy
	NotificationSettings *TeamNotificationSettings
	Repos                []Repository
	ParentTeam           *Team
}

// String returns a string representation of the Team.
func (t *Team) String() string {
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "Team<marshal error>"
	}
	return string(b)
}

// TeamFromGhTeam converts a GitHub team from the go-github library to the internal Team type.
func TeamFromGhTeam(ghTeam *gh.Team) *Team {
	if ghTeam == nil {
		return nil
	}

	parentTeam := ghTeam.GetParent()

	return &Team{
		ID:                   ghTeam.GetID(),
		Slug:                 ghTeam.GetSlug(),
		Org:                  ghTeam.GetOrganization().GetLogin(),
		Name:                 ghTeam.GetName(),
		Description:          ghTeam.GetDescription(),
		Privacy:              TeamPrivacy(ghTeam.GetPrivacy()),
		NotificationSettings: nil, // Notification settings are not included in the GitHub team object
		Repos:                nil, // TODO: Left as nil for now, implementation for handling repositories can be added later
		ParentTeam:           TeamFromGhTeam(parentTeam),
	}
}

// TeamsFromGhTeams converts a slice of GitHub teams to a slice of internal
// Team types.
func TeamsFromGhTeams(ghTeams []*gh.Team) []*Team {
	teams := make([]*Team, 0, len(ghTeams))
	for _, t := range ghTeams {
		if team := TeamFromGhTeam(t); team != nil {
			teams = append(teams, team)
		}
	}
	return teams
}

// UsersFromGhUsers converts a slice of GitHub users to a slice of internal
// User types.
func UsersFromGhUsers(ghUsers []*gh.User) []*User {
	users := make([]*User, 0, len(ghUsers))
	for _, u := range ghUsers {
		if user := UserFromGhUser(u); user != nil {
			users = append(users, user)
		}
	}
	return users
}

// Organization represents a GitHub organization.
type Organization struct {
	ID          *int64
	Name        *string
	Description *string
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
	ReposURL    *string
}

// String implements fmt.Stringer and pretty-prints JSON.
func (o Organization) String() string {
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return "Organization<marshal error>"
	}
	return string(b)
}

// OrganizationFromGhOrg converts a GitHub organization from the go-github library to the internal Organization type.
func OrganizationFromGhOrg(ghOrg *gh.Organization) *Organization {
	if ghOrg == nil {
		return nil
	}

	return &Organization{
		ID:          ghOrg.ID,
		Name:        ghOrg.Name,
		Description: ghOrg.Description,
		CreatedAt:   TimestampToTime(ghOrg.CreatedAt),
		UpdatedAt:   TimestampToTime(ghOrg.UpdatedAt),
		ReposURL:    ghOrg.ReposURL,
	}
}

// User represents a GitHub user.
type User struct {
	ID        *int64
	Name      *string
	Email     *string
	URL       *string
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

// String implements fmt.Stringer and pretty-prints JSON.
func (u User) String() string {
	b, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return "User<marshal error>"
	}
	return string(b)
}

// UserFromGhUser converts a GitHub user from the go-github library to the internal User type.
func UserFromGhUser(ghUser *gh.User) *User {
	if ghUser == nil {
		return nil
	}

	return &User{
		ID:        ghUser.ID,
		Name:      ghUser.Name,
		Email:     ghUser.Email,
		URL:       ghUser.HTMLURL,
		CreatedAt: TimestampToTime(ghUser.CreatedAt),
		UpdatedAt: TimestampToTime(ghUser.UpdatedAt),
	}
}
