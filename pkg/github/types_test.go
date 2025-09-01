package github

import (
	"encoding/json"
	"testing"
	"time"

	gh "github.com/google/go-github/v55/github"
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

	userNameSecond  = "Second"
	userEmailSecond = "second@example.com"
	userURLSecond   = "https://github.com/second"

	idOrg        = int64(7)
	idTeamParent = int64(1)
	idTeamChild  = int64(2)
	idUserAda    = int64(42)
	idUserGrace  = int64(99)

	rfc3339CreatedOrg = "2020-01-02T03:04:05Z"
	rfc3339UpdatedOrg = "2021-06-07T08:09:10Z"
	rfc3339UserAda    = "2024-07-10T01:02:03Z"
	rfc3339UserGraceC = "2019-05-06T07:08:09Z"
	rfc3339UserGraceU = "2022-03-04T05:06:07Z"
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
		Org:         orgLogin,
		Name:        repoNameRocket,
		Private:     true,
		Description: repoDescWIP,
		Topics:      []string{topicGo, topicCI},
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
	if got.Org != orgLogin || got.Name != repoNameRocket || !got.Private ||
		got.Description != repoDescWIP || len(got.Topics) != 2 || got.Topics[0] != topicGo || got.Topics[1] != topicCI {
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
			Owner:       &gh.User{Login: gh.String(orgLogin)},
			Name:        gh.String(repoNameRocket),
			Private:     gh.Bool(true),
			Description: gh.String(repoDescWIP),
			Topics:      []string{topicGo, topicCI},
		}
		got := RepositoryFromGhRepo(ghRepo)
		if got == nil {
			t.Fatalf("got nil")
		}
		if got.Org != orgLogin || got.Name != repoNameRocket || !got.Private || got.Description != repoDescWIP {
			t.Fatalf("unexpected mapped fields: %#v", got)
		}
		if len(got.Topics) != 2 || got.Topics[0] != topicGo || got.Topics[1] != topicCI {
			t.Fatalf("unexpected topics: %#v", got.Topics)
		}
	})

	t.Run("owner nil -> empty org", func(t *testing.T) {
		ghRepo := &gh.Repository{
			Name:        gh.String("lib"),
			Private:     gh.Bool(false),
			Description: gh.String("ok"),
			Owner:       nil,
		}
		got := RepositoryFromGhRepo(ghRepo)
		if got.Org != "" {
			t.Fatalf("expected empty Org, got %q", got.Org)
		}
	})
}

func TestRepositoriesFromGhRepos(t *testing.T) {
	in := []*gh.Repository{
		{Owner: &gh.User{Login: gh.String(orgLogin)}, Name: gh.String(repoNameA)},
		nil, // should be skipped
		{Owner: &gh.User{Login: gh.String(orgLogin)}, Name: gh.String(repoNameB)},
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
			ID:          gh.Int64(idTeamParent),
			Slug:        gh.String(teamSlugCore),
			Name:        gh.String(teamNameCore),
			Description: gh.String(teamDescParent),
			Privacy:     gh.String(TeamPrivacyClosed.String()),
		}
		child := &gh.Team{
			ID:           gh.Int64(idTeamChild),
			Slug:         gh.String(teamSlugDevs),
			Name:         gh.String(teamNameDevs),
			Description:  gh.String(teamDescChild),
			Privacy:      gh.String(TeamPrivacySecret.String()),
			Parent:       parent,
			Organization: &gh.Organization{Login: gh.String(orgLogin)},
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
		{ID: gh.Int64(idTeamParent), Slug: gh.String(teamSlugCore), Name: gh.String(teamNameCore), Organization: &gh.Organization{Login: gh.String(orgLogin)}},
		nil, // should be skipped
		{ID: gh.Int64(idTeamChild), Slug: gh.String(teamSlugDevs), Name: gh.String(teamNameDevs), Organization: &gh.Organization{Login: gh.String(orgLogin)}},
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
			ID:        gh.Int64(idUserAda),
			Name:      gh.String(userNameFirst),
			Email:     gh.String(userEmailFirst),
			HTMLURL:   gh.String(userURLFirst),
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
			ID:          gh.Int64(idOrg),
			Name:        gh.String("Acme Inc"),
			Description: gh.String("best org"),
			ReposURL:    gh.String(repoURLExample),
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
			ID:          gh.Int64(1),
			Name:        gh.String("X"),
			Description: gh.String("Y"),
			ReposURL:    gh.String("Z"),
			CreatedAt:   nil,
			UpdatedAt:   nil,
		}
		got := OrganizationFromGhOrg(ghOrg)
		if got.CreatedAt != nil || got.UpdatedAt != nil {
			t.Fatalf("expected nil timestamps, got: %#v %#v", got.CreatedAt, got.UpdatedAt)
		}
	})
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
			ID:        gh.Int64(idUserGrace),
			Name:      gh.String(userNameSecond),
			Email:     gh.String(userEmailSecond),
			HTMLURL:   gh.String(userURLSecond),
			CreatedAt: &gh.Timestamp{Time: created},
			UpdatedAt: &gh.Timestamp{Time: updated},
		}
		got := UserFromGhUser(ghUser)
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
