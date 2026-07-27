package github

import (
	"encoding/json"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
)

const (
	orgLogin       = "org"
	repoNameA      = "a"
	repoNameB      = "b"
	repoNameRocket = "rocket"
	repoDescWIP    = "WIP"
	repoURLExample = "https://api.github.com/orgs/org/repos"

	teamSlugCore   = "core"
	teamSlugDevs   = "devs"
	teamNameCore   = "Core"
	teamNameDevs   = "Developers"
	teamDescParent = "parent team"
	teamDescChild  = "child team"

	userNameFirst  = "First"
	userEmailFirst = "first@example.com"
	userURLFirst   = "https://github.com/first"

	userLoginSecond = "second-user"
	userNameSecond  = "Second"
	userEmailSecond = "second@example.com"
	userURLSecond   = "https://github.com/second"
	inviteLogin     = "monalisa"
	inviteEmail     = "octocat@example.com"
	inviteRole      = "direct_member"
	inviteTeamURL   = "https://api.github.com/organizations/1/invitations/7/teams"

	idOrg        = int64(7)
	idTeamParent = int64(1)
	idTeamChild  = int64(2)
	idUserAda    = int64(42)
	idUserGrace  = int64(99)
	idInvite     = int64(77)
	inviteTeams  = 2

	rfc3339CreatedOrg = "2020-01-02T03:04:05Z"
	rfc3339UpdatedOrg = "2021-06-07T08:09:10Z"
	rfc3339UserAda    = "2024-07-10T01:02:03Z"
	rfc3339UserGraceC = "2019-05-06T07:08:09Z"
	rfc3339UserGraceU = "2022-03-04T05:06:07Z"
	rfc3339Invite     = "2025-02-03T04:05:06Z"
)

var (
	topicGo = "go"
	topicCI = "ci"
)

func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", s, err)
	}
	return ts
}

func mustBeValidJSON(t *testing.T, s string) {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
}

func TestRepository_String_JSON(t *testing.T) {
	r := &Repository{
		Owner:        orgLogin,
		Name:         repoNameRocket,
		Private:      true,
		Visibility:   "private",
		Description:  repoDescWIP,
		Homepage:     "https://example.com/repo",
		Topics:       []string{topicGo, topicCI},
		AllowForking: true,
		Archived:     false,
		IsTemplate:   true,
	}
	s := r.String()
	if s == "Repository<marshal error>" {
		t.Fatalf("unexpected marshal error")
	}
	mustBeValidJSON(t, s)

	// Round-trip to ensure field names/values are as expected.
	var got Repository
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if got.Owner != orgLogin || got.Name != repoNameRocket || !got.Private ||
		got.Visibility != "private" || got.Description != repoDescWIP || got.Homepage != "https://example.com/repo" ||
		!got.AllowForking || got.Archived || !got.IsTemplate ||
		len(got.Topics) != 2 || got.Topics[0] != topicGo || got.Topics[1] != topicCI {
		t.Fatalf("unexpected Repository JSON round-trip: %#v", got)
	}
}

func TestRepositoryFromGhRepo(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := RepositoryFromGhRepo(nil); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("maps fields", func(t *testing.T) {
		ghRepo := &gh.Repository{
			Owner:        &gh.User{Login: Ptr(orgLogin)},
			Name:         Ptr(repoNameRocket),
			Private:      Ptr(true),
			Visibility:   Ptr("private"),
			Description:  Ptr(repoDescWIP),
			Homepage:     Ptr("https://example.com/repo"),
			Topics:       []string{topicGo, topicCI},
			AllowForking: Ptr(true),
			Archived:     Ptr(false),
			IsTemplate:   Ptr(true),
		}
		got := RepositoryFromGhRepo(ghRepo)
		if got == nil {
			t.Fatalf("got nil")
			return
		}
		if got.Owner != orgLogin || got.Name != repoNameRocket || !got.Private || got.Visibility != "private" ||
			got.Description != repoDescWIP || got.Homepage != "https://example.com/repo" ||
			!got.AllowForking || got.Archived || !got.IsTemplate {
			t.Fatalf("unexpected mapped fields: %#v", got)
		}
		if len(got.Topics) != 2 || got.Topics[0] != topicGo || got.Topics[1] != topicCI {
			t.Fatalf("unexpected topics: %#v", got.Topics)
		}
	})

	t.Run("owner nil -> empty org", func(t *testing.T) {
		ghRepo := &gh.Repository{
			Name:        Ptr("lib"),
			Private:     Ptr(false),
			Description: Ptr("ok"),
			Owner:       nil,
		}
		got := RepositoryFromGhRepo(ghRepo)
		if got.Owner != "" {
			t.Fatalf("expected empty Org, got %q", got.Owner)
		}
	})
}

func TestRepositoriesFromGhRepos(t *testing.T) {
	in := []*gh.Repository{
		{Owner: &gh.User{Login: Ptr(orgLogin)}, Name: Ptr(repoNameA)},
		nil, // should be skipped
		{Owner: &gh.User{Login: Ptr(orgLogin)}, Name: Ptr(repoNameB)},
	}
	got := RepositoriesFromGhRepos(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}
	if got[0].Name != repoNameA || got[1].Name != repoNameB {
		t.Fatalf("unexpected order/content: %#v", got)
	}
}

func TestTeamPrivacyHelpers(t *testing.T) {
	if !TeamPrivacySecret.IsValid() || !TeamPrivacyClosed.IsValid() {
		t.Fatalf("predefined values should be valid")
	}
	if TeamPrivacy("unknown").IsValid() {
		t.Fatalf("unknown value should be invalid")
	}
	if !TeamPrivacySecret.IsSecret() || TeamPrivacyClosed.IsSecret() {
		t.Fatalf("IsSecret mapping incorrect")
	}
	if PrivacyFromBool(true) != TeamPrivacySecret || PrivacyFromBool(false) != TeamPrivacyClosed {
		t.Fatalf("PrivacyFromBool mapping incorrect")
	}
	if TeamPrivacySecret.String() != "secret" || TeamPrivacyClosed.String() != "closed" {
		t.Fatalf("String() unexpected")
	}
}

func TestTeamNotificationSettingsHelpers(t *testing.T) {
	if !TeamNotificationSettingsEnabled.IsValid() || !TeamNotificationSettingsDisabled.IsValid() {
		t.Fatalf("predefined values should be valid")
	}
	if TeamNotificationSettings("x").IsValid() {
		t.Fatalf("unknown value should be invalid")
	}
	if !TeamNotificationSettingsEnabled.IsEnabled() || TeamNotificationSettingsDisabled.IsEnabled() {
		t.Fatalf("IsEnabled mapping incorrect")
	}
	if NotificationSettingsFromBool(true) != TeamNotificationSettingsEnabled ||
		NotificationSettingsFromBool(false) != TeamNotificationSettingsDisabled {
		t.Fatalf("NotificationSettingsFromBool mapping incorrect")
	}
	if TeamNotificationSettingsEnabled.String() != "notifications_enabled" ||
		TeamNotificationSettingsDisabled.String() != "notifications_disabled" {
		t.Fatalf("String() unexpected")
	}
}

func TestTeamFromGhTeam(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := TeamFromGhTeam(nil); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("maps fields including parent", func(t *testing.T) {
		parent := &gh.Team{
			ID:          Ptr(idTeamParent),
			Slug:        Ptr(teamSlugCore),
			Name:        Ptr(teamNameCore),
			Description: Ptr(teamDescParent),
			Privacy:     Ptr(TeamPrivacyClosed.String()),
		}
		child := &gh.Team{
			ID:           Ptr(idTeamChild),
			Slug:         Ptr(teamSlugDevs),
			Name:         Ptr(teamNameDevs),
			Description:  Ptr(teamDescChild),
			Privacy:      Ptr(TeamPrivacySecret.String()),
			Parent:       parent,
			Organization: &gh.Organization{Login: Ptr(orgLogin)},
		}

		got := TeamFromGhTeam(child)
		if got == nil {
			t.Fatalf("got nil team")
		}
		if got.ID != idTeamChild || got.Slug != teamSlugDevs || got.Name != teamNameDevs || got.Org != orgLogin {
			t.Fatalf("unexpected id/slug/name/org: %#v", got)
		}
		if got.Privacy != TeamPrivacySecret {
			t.Fatalf("expected privacy secret, got %q", got.Privacy)
		}
		if got.NotificationSettings != nil {
			t.Fatalf("NotificationSettings should be nil (not mapped)")
		}
		if got.Repos != nil {
			t.Fatalf("Repos should be nil (not mapped)")
		}
		if got.ParentTeam == nil || got.ParentTeam.Slug != teamSlugCore || got.ParentTeam.Name != teamNameCore {
			t.Fatalf("parent not mapped correctly: %#v", got.ParentTeam)
		}
	})
}

func TestTeamsFromGhTeams(t *testing.T) {
	in := []*gh.Team{
		{ID: Ptr(idTeamParent), Slug: Ptr(teamSlugCore), Name: Ptr(teamNameCore), Organization: &gh.Organization{Login: Ptr(orgLogin)}},
		nil, // should be skipped
		{ID: Ptr(idTeamChild), Slug: Ptr(teamSlugDevs), Name: Ptr(teamNameDevs), Organization: &gh.Organization{Login: Ptr(orgLogin)}},
	}
	got := TeamsFromGhTeams(in)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Slug != teamSlugCore || got[1].Slug != teamSlugDevs {
		t.Fatalf("unexpected order/content: %#v", got)
	}
}

func TestUsersFromGhUsers(t *testing.T) {
	created := mustParseRFC3339(t, rfc3339UserAda)

	in := []*gh.User{
		{
			ID:        Ptr(idUserAda),
			Name:      Ptr(userNameFirst),
			Email:     Ptr(userEmailFirst),
			HTMLURL:   Ptr(userURLFirst),
			CreatedAt: &gh.Timestamp{Time: created},
			UpdatedAt: &gh.Timestamp{Time: created},
		},
		nil, // should be skipped
	}
	got := UsersFromGhUsers(in)
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	u := got[0]
	if u.ID == nil || *u.ID != idUserAda {
		t.Fatalf("unexpected user id: %#v", u.ID)
	}
	if u.Name == nil || *u.Name != userNameFirst {
		t.Fatalf("unexpected Name: %#v", u.Name)
	}
	if u.Email == nil || *u.Email != userEmailFirst {
		t.Fatalf("unexpected Email: %#v", u.Email)
	}
	if u.URL == nil || *u.URL != userURLFirst {
		t.Fatalf("unexpected URL: %#v", u.URL)
	}
	if u.CreatedAt == nil || !u.CreatedAt.Equal(created) {
		t.Fatalf("CreatedAt not mapped: %#v", u.CreatedAt)
	}
	if u.UpdatedAt == nil || !u.UpdatedAt.Equal(created) {
		t.Fatalf("UpdatedAt not mapped: %#v", u.UpdatedAt)
	}
}

func TestOrganizationFromGhOrg(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := OrganizationFromGhOrg(nil); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("maps fields with timestamps", func(t *testing.T) {
		created := mustParseRFC3339(t, rfc3339CreatedOrg)
		updated := mustParseRFC3339(t, rfc3339UpdatedOrg)

		ghOrg := &gh.Organization{
			ID:          Ptr(idOrg),
			Name:        Ptr("Acme Inc"),
			Description: Ptr("best org"),
			ReposURL:    Ptr(repoURLExample),
			CreatedAt:   &gh.Timestamp{Time: created},
			UpdatedAt:   &gh.Timestamp{Time: updated},
		}
		got := OrganizationFromGhOrg(ghOrg)
		if got == nil {
			t.Fatalf("got nil")
		}
		if got.ID == nil || *got.ID != idOrg {
			t.Fatalf("unexpected ID: %#v", got.ID)
		}
		if got.Name == nil || *got.Name != "Acme Inc" {
			t.Fatalf("unexpected Name: %#v", got.Name)
		}
		if got.Description == nil || *got.Description != "best org" {
			t.Fatalf("unexpected Description: %#v", got.Description)
		}
		if got.ReposURL == nil || *got.ReposURL != repoURLExample {
			t.Fatalf("unexpected ReposURL: %#v", got.ReposURL)
		}
		if got.CreatedAt == nil || !got.CreatedAt.Equal(created) {
			t.Fatalf("CreatedAt mismatch: %#v", got.CreatedAt)
		}
		if got.UpdatedAt == nil || !got.UpdatedAt.Equal(updated) {
			t.Fatalf("UpdatedAt mismatch: %#v", got.UpdatedAt)
		}
	})

	t.Run("nil timestamps map to nil pointers", func(t *testing.T) {
		ghOrg := &gh.Organization{
			ID:          Ptr(int64(1)),
			Name:        Ptr("X"),
			Description: Ptr("Y"),
			ReposURL:    Ptr("Z"),
			CreatedAt:   nil,
			UpdatedAt:   nil,
		}
		got := OrganizationFromGhOrg(ghOrg)
		if got.CreatedAt != nil || got.UpdatedAt != nil {
			t.Fatalf("expected nil timestamps, got: %#v %#v", got.CreatedAt, got.UpdatedAt)
		}
	})
}

func TestOrganizationInvitation_String_JSON(t *testing.T) {
	invitation := OrganizationInvitation{
		ID:                Ptr(idInvite),
		Login:             Ptr(inviteLogin),
		Email:             Ptr(inviteEmail),
		Role:              Ptr(inviteRole),
		TeamCount:         Ptr(inviteTeams),
		InvitationTeamURL: Ptr(inviteTeamURL),
	}

	s := invitation.String()
	if s == "OrganizationInvitation<marshal error>" {
		t.Fatalf("unexpected marshal error")
	}
	mustBeValidJSON(t, s)
}

func TestOrganizationInvitationFromGhInvitation(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := OrganizationInvitationFromGhInvitation(nil); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("maps fields", func(t *testing.T) {
		created := mustParseRFC3339(t, rfc3339Invite)

		ghInvitation := &gh.Invitation{
			ID:                Ptr(idInvite),
			Login:             Ptr(inviteLogin),
			Email:             Ptr(inviteEmail),
			Role:              Ptr(inviteRole),
			CreatedAt:         &gh.Timestamp{Time: created},
			TeamCount:         Ptr(inviteTeams),
			InvitationTeamURL: Ptr(inviteTeamURL),
		}

		got := OrganizationInvitationFromGhInvitation(ghInvitation)
		if got == nil {
			t.Fatalf("got nil")
			return
		}
		if got.ID == nil || *got.ID != idInvite {
			t.Fatalf("unexpected ID: %#v", got.ID)
		}
		if got.Login == nil || *got.Login != inviteLogin {
			t.Fatalf("unexpected Login: %#v", got.Login)
		}
		if got.Email == nil || *got.Email != inviteEmail {
			t.Fatalf("unexpected Email: %#v", got.Email)
		}
		if got.Role == nil || *got.Role != inviteRole {
			t.Fatalf("unexpected Role: %#v", got.Role)
		}
		if got.CreatedAt == nil || !got.CreatedAt.Equal(created) {
			t.Fatalf("unexpected CreatedAt: %#v", got.CreatedAt)
		}
		if got.TeamCount == nil || *got.TeamCount != inviteTeams {
			t.Fatalf("unexpected TeamCount: %#v", got.TeamCount)
		}
		if got.InvitationTeamURL == nil || *got.InvitationTeamURL != inviteTeamURL {
			t.Fatalf("unexpected InvitationTeamURL: %#v", got.InvitationTeamURL)
		}
	})
}

func TestOrganizationInvitationsFromGhInvitations(t *testing.T) {
	in := []*gh.Invitation{
		{ID: Ptr(idInvite), Login: Ptr(inviteLogin)},
		nil,
		{ID: Ptr(idInvite + 1), Login: Ptr("second")},
	}

	got := OrganizationInvitationsFromGhInvitations(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 invitations, got %d", len(got))
	}
	if got[0].Login == nil || *got[0].Login != inviteLogin {
		t.Fatalf("unexpected first invitation: %#v", got[0])
	}
	if got[1].Login == nil || *got[1].Login != "second" {
		t.Fatalf("unexpected second invitation: %#v", got[1])
	}
}

func TestUserFromGhUser(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := UserFromGhUser(nil); got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("maps fields", func(t *testing.T) {
		created := mustParseRFC3339(t, rfc3339UserGraceC)
		updated := mustParseRFC3339(t, rfc3339UserGraceU)

		ghUser := &gh.User{
			Login:     Ptr(userLoginSecond),
			ID:        Ptr(idUserGrace),
			Name:      Ptr(userNameSecond),
			Email:     Ptr(userEmailSecond),
			HTMLURL:   Ptr(userURLSecond),
			CreatedAt: &gh.Timestamp{Time: created},
			UpdatedAt: &gh.Timestamp{Time: updated},
		}
		got := UserFromGhUser(ghUser)
		if got.Login == nil || *got.Login != userLoginSecond {
			t.Fatalf("unexpected Login: %#v", got.Login)
		}
		if got.ID == nil || *got.ID != idUserGrace {
			t.Fatalf("unexpected ID: %#v", got.ID)
		}
		if got.Name == nil || *got.Name != userNameSecond {
			t.Fatalf("unexpected Name: %#v", got.Name)
		}
		if got.Email == nil || *got.Email != userEmailSecond {
			t.Fatalf("unexpected Email: %#v", got.Email)
		}
		if got.URL == nil || *got.URL != userURLSecond {
			t.Fatalf("unexpected URL: %#v", got.URL)
		}
		if got.CreatedAt == nil || !got.CreatedAt.Equal(created) {
			t.Fatalf("CreatedAt mismatch: %#v", got.CreatedAt)
		}
		if got.UpdatedAt == nil || !got.UpdatedAt.Equal(updated) {
			t.Fatalf("UpdatedAt mismatch: %#v", got.UpdatedAt)
		}
	})
}

func TestTeam_String_JSON(t *testing.T) {
	tm := &Team{
		ID:          idTeamParent,
		Slug:        teamSlugCore,
		Org:         orgLogin,
		Name:        teamNameCore,
		Description: teamDescParent,
		Privacy:     TeamPrivacyClosed,
	}
	s := tm.String()
	if s == "Team<marshal error>" {
		t.Fatalf("unexpected marshal error")
	}
	mustBeValidJSON(t, s)
}
