package collector

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	gh "github.com/google/go-github/v55/github"
	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

type concurrencyTracker struct {
	mu      sync.Mutex
	current int
	max     int
}

func (t *concurrencyTracker) Start() func() {
	t.mu.Lock()
	t.current++
	if t.current > t.max {
		t.max = t.current
	}
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		t.current--
		t.mu.Unlock()
	}
}

func (t *concurrencyTracker) Snapshot() (current, maxSeen int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.current, t.max
}

func waitForTrackerMaxAtLeast(t *testing.T, tracker *concurrencyTracker, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, maxSeen := tracker.Snapshot()
		if maxSeen >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	current, maxSeen := tracker.Snapshot()
	t.Fatalf("timed out waiting for max concurrency >= %d; current=%d max=%d", want, current, maxSeen)
}

func waitForSignals(t *testing.T, ch <-chan string, count int) []string {
	t.Helper()

	values := make([]string, 0, count)
	deadline := time.After(2 * time.Second)
	for len(values) < count {
		select {
		case value := <-ch:
			values = append(values, value)
		case <-deadline:
			t.Fatalf("timed out waiting for %d signals; got %#v", count, values)
		}
	}
	return values
}

func TestCollectOrganizationTopLevelReadsOverlap(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"

	tracker := &concurrencyTracker{}
	release := make(chan struct{})

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(_ context.Context, org string, _ *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListMembers: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			return []*gh.User{}, &gh.Response{}, nil
		},
		listPendingOrgInvitationsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListPendingOrgInvitations: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			return []*gh.Invitation{}, &gh.Response{}, nil
		},
	}
	repoSvc := &repositoryServiceStub{
		listByOrgFunc: func(_ context.Context, org string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListByOrg: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			return []*gh.Repository{}, &gh.Response{}, nil
		},
	}
	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeams: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			return []*gh.Team{}, &gh.Response{}, nil
		},
	}

	resultCh := make(chan *state.OrganizationState, 1)
	errCh := make(chan error, 1)
	go func() {
		actual, err := collectOrganizationWithLimits(context.Background(), CollectOrganizationOptions{
			OrgName:             orgName,
			OrganizationService: orgSvc,
			RepositoryService:   repoSvc,
			TeamService:         teamSvc,
		}, collectOrganizationBehavior{
			includeMembers:            true,
			includePendingInvitations: true,
			includeRepositories:       true,
			includeTeams:              true,
		}, collectorConcurrencyLimits{
			topLevel:        4,
			memberRoles:     1,
			invitationTeams: 1,
			teamDetails:     1,
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- actual
	}()

	waitForTrackerMaxAtLeast(t, tracker, 4)
	close(release)

	select {
	case err := <-errCh:
		t.Fatalf("collectOrganizationWithLimits returned error: %v", err)
	case actual := <-resultCh:
		if actual.Organization != orgName {
			t.Fatalf("unexpected organization %q", actual.Organization)
		}
	}

	_, maxSeen := tracker.Snapshot()
	if maxSeen < 4 {
		t.Fatalf("expected top-level overlap to reach 4, got %d", maxSeen)
	}
}

func TestCollectPendingInvitationsBoundsInvitationTeamFanOut(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	const invitationCount = 20

	tracker := &concurrencyTracker{}
	release := make(chan struct{})

	orgSvc := &organizationServiceStub{
		listPendingOrgInvitationsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListPendingOrgInvitations: got %q want %q", org, orgName)
			}
			invitations := make([]*gh.Invitation, 0, invitationCount)
			for i := 1; i <= invitationCount; i++ {
				invitationID := int64(i)
				login := fmt.Sprintf("user-%02d", i)
				teamCount := 1
				invitations = append(invitations, &gh.Invitation{
					ID:        githubpkg.Ptr(invitationID),
					Login:     githubpkg.Ptr(login),
					Role:      githubpkg.Ptr("direct_member"),
					TeamCount: githubpkg.Ptr(teamCount),
				})
			}
			return invitations, &gh.Response{}, nil
		},
		listOrgInvitationTeamsFunc: func(_ context.Context, org, invitationID string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListOrgInvitationTeams: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			slug := "team-" + invitationID
			return []*gh.Team{{Slug: githubpkg.Ptr(slug), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}}}, &gh.Response{}, nil
		},
	}

	resultCh := make(chan []state.PendingInvitation, 1)
	errCh := make(chan error, 1)
	go func() {
		invitations, err := collectPendingInvitations(context.Background(), CollectOrganizationOptions{
			OrgName:             orgName,
			OrganizationService: orgSvc,
		}, collectorConcurrencyLimits{
			invitationTeams: 8,
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- invitations
	}()

	waitForTrackerMaxAtLeast(t, tracker, 8)
	close(release)

	select {
	case err := <-errCh:
		t.Fatalf("collectPendingInvitations returned error: %v", err)
	case invitations := <-resultCh:
		if len(invitations) != invitationCount {
			t.Fatalf("unexpected invitation count: got %d want %d", len(invitations), invitationCount)
		}
	}

	_, maxSeen := tracker.Snapshot()
	if maxSeen > 8 {
		t.Fatalf("expected invitation team fan-out <= 8, got %d", maxSeen)
	}
}

func TestCollectTeamStateBoundsPerTeamDetailFanOut(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	const teamCount = 10

	tracker := &concurrencyTracker{}
	release := make(chan struct{})

	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeams: got %q want %q", org, orgName)
			}
			teams := make([]*gh.Team, 0, teamCount)
			for i := 1; i <= teamCount; i++ {
				slug := fmt.Sprintf("team-%02d", i)
				teams = append(teams, &gh.Team{Slug: githubpkg.Ptr(slug), Name: githubpkg.Ptr(slug), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}})
			}
			return teams, &gh.Response{}, nil
		},
		listTeamMembersBySlugFunc: func(_ context.Context, org, slug string, _ *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeamMembersBySlug: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			return []*gh.User{{Login: githubpkg.Ptr(slug + "-user")}}, &gh.Response{}, nil
		},
		listTeamReposBySlugFunc: func(_ context.Context, org, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeamReposBySlug: got %q want %q", org, orgName)
			}
			done := tracker.Start()
			defer done()
			<-release
			return []*gh.Repository{{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr(slug + "-repo"), Permissions: map[string]bool{"push": true}}}, &gh.Response{}, nil
		},
	}

	resultCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	go func() {
		_, _, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
			OrgName:     orgName,
			TeamService: teamSvc,
		}, collectorConcurrencyLimits{teamDetails: 8})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- struct{}{}
	}()

	waitForTrackerMaxAtLeast(t, tracker, 8)
	close(release)

	select {
	case err := <-errCh:
		t.Fatalf("collectTeamState returned error: %v", err)
	case <-resultCh:
	}

	_, maxSeen := tracker.Snapshot()
	if maxSeen > 8 {
		t.Fatalf("expected team detail fan-out <= 8, got %d", maxSeen)
	}
}

func TestCollectOrganizationReturnsFirstTopLevelErrorInLegacyOrder(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	membersErr := errors.New("members failed")
	reposErr := errors.New("repositories failed")
	release := make(chan struct{})

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(_ context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListMembers: got %q want %q", org, orgName)
			}
			if opts == nil {
				t.Fatal("expected member list options")
			}
			<-release
			if opts.Role == "admin" {
				return nil, nil, membersErr
			}
			return []*gh.User{}, &gh.Response{}, nil
		},
	}
	repoSvc := &repositoryServiceStub{
		listByOrgFunc: func(_ context.Context, org string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListByOrg: got %q want %q", org, orgName)
			}
			<-release
			return nil, nil, reposErr
		},
	}

	close(release)

	_, err := collectOrganizationWithLimits(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
		RepositoryService:   repoSvc,
	}, collectOrganizationBehavior{
		includeMembers:      true,
		includeRepositories: true,
	}, collectorConcurrencyLimits{
		topLevel:        2,
		memberRoles:     1,
		invitationTeams: 1,
		teamDetails:     1,
	})
	if !errors.Is(err, membersErr) {
		t.Fatalf("unexpected error: got %v want %v", err, membersErr)
	}
}

func TestCollectPendingInvitationsReturnsFirstInvitationErrorByInputOrder(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	firstErr := errors.New("first invitation failed")
	secondErr := errors.New("second invitation failed")
	release := make(chan struct{})

	orgSvc := &organizationServiceStub{
		listPendingOrgInvitationsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListPendingOrgInvitations: got %q want %q", org, orgName)
			}
			return []*gh.Invitation{
				{ID: githubpkg.Ptr(int64(1)), Login: githubpkg.Ptr("alpha"), TeamCount: githubpkg.Ptr(1)},
				{ID: githubpkg.Ptr(int64(2)), Login: githubpkg.Ptr("beta"), TeamCount: githubpkg.Ptr(1)},
			}, &gh.Response{}, nil
		},
		listOrgInvitationTeamsFunc: func(_ context.Context, org, invitationID string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListOrgInvitationTeams: got %q want %q", org, orgName)
			}
			<-release
			if invitationID == "1" {
				return nil, nil, firstErr
			}
			return nil, nil, secondErr
		},
	}

	close(release)

	_, err := collectPendingInvitations(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
	}, collectorConcurrencyLimits{invitationTeams: 2})
	if !errors.Is(err, firstErr) {
		t.Fatalf("unexpected error: got %v want %v", err, firstErr)
	}
}

func TestCollectTeamStateReturnsMemberErrorBeforeMaintainerAndRepoErrors(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	memberErr := errors.New("member read failed")
	maintainerErr := errors.New("maintainer read failed")
	repoErr := errors.New("repo permission read failed")
	release := make(chan struct{})

	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeams: got %q want %q", org, orgName)
			}
			return []*gh.Team{
				{Slug: githubpkg.Ptr("platform"), Name: githubpkg.Ptr("Platform"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
			}, &gh.Response{}, nil
		},
		listTeamMembersBySlugFunc: func(_ context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName || slug != "platform" {
				t.Fatalf("unexpected team member query: org=%q slug=%q", org, slug)
			}
			<-release
			if opts.Role == "member" {
				return nil, nil, memberErr
			}
			return nil, nil, maintainerErr
		},
		listTeamReposBySlugFunc: func(_ context.Context, org, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName || slug != "platform" {
				t.Fatalf("unexpected team repo query: org=%q slug=%q", org, slug)
			}
			<-release
			return nil, nil, repoErr
		},
	}

	close(release)

	_, _, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
		OrgName:     orgName,
		TeamService: teamSvc,
	}, collectorConcurrencyLimits{teamDetails: 3})
	if !errors.Is(err, memberErr) {
		t.Fatalf("unexpected error: got %v want %v", err, memberErr)
	}
}

func TestRunOrderedTasksTreatsUnexpectedContextErrorAsFailure(t *testing.T) {
	t.Parallel()

	err := runOrderedTasks(context.Background(), 2, []orderedTask{
		func(context.Context) error {
			return context.Canceled
		},
		func(context.Context) error {
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: got %v want %v", err, context.Canceled)
	}
}

func TestCollectOrganizationCancelsSiblingReadsOnError(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	wantErr := errors.New("member lookup failed")
	started := make(chan string, 4)
	canceled := make(chan string, 4)

	blockUntilCanceled := func(name string) func(context.Context) error {
		return func(ctx context.Context) error {
			started <- name
			<-ctx.Done()
			canceled <- name
			return ctx.Err()
		}
	}

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(ctx context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListMembers: got %q want %q", org, orgName)
			}
			if opts == nil {
				t.Fatalf("unexpected ListMembers options: %#v", opts)
			}
			switch opts.Role {
			case "admin":
				waitForSignals(t, started, 4)
				return nil, nil, wantErr
			case "member":
				return []*gh.User{}, nil, blockUntilCanceled("members")(ctx)
			default:
				t.Fatalf("unexpected ListMembers role %q", opts.Role)
				return nil, nil, nil
			}
		},
		listPendingOrgInvitationsFunc: func(ctx context.Context, org string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListPendingOrgInvitations: got %q want %q", org, orgName)
			}
			return []*gh.Invitation{}, nil, blockUntilCanceled("invites")(ctx)
		},
	}
	repoSvc := &repositoryServiceStub{
		listByOrgFunc: func(ctx context.Context, org string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListByOrg: got %q want %q", org, orgName)
			}
			return []*gh.Repository{}, nil, blockUntilCanceled("repos")(ctx)
		},
	}
	teamSvc := &teamServiceStub{
		listTeamsFunc: func(ctx context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeams: got %q want %q", org, orgName)
			}
			return []*gh.Team{}, nil, blockUntilCanceled("teams")(ctx)
		},
	}

	_, err := collectOrganizationWithLimits(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
		RepositoryService:   repoSvc,
		TeamService:         teamSvc,
	}, collectOrganizationBehavior{
		includeMembers:            true,
		includePendingInvitations: true,
		includeRepositories:       true,
		includeTeams:              true,
	}, collectorConcurrencyLimits{
		topLevel:        4,
		memberRoles:     2,
		invitationTeams: 1,
		teamDetails:     1,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}

	gotCanceled := waitForSignals(t, canceled, 4)
	slices.Sort(gotCanceled)
	if !reflect.DeepEqual(gotCanceled, []string{"invites", "members", "repos", "teams"}) {
		t.Fatalf("unexpected canceled calls: got %#v", gotCanceled)
	}
}

func TestCollectOrganizationStableOrderingWithOutOfOrderConcurrentCompletion(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	sleep := func(d time.Duration) { time.Sleep(d) }

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(_ context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListMembers: got %q want %q", org, orgName)
			}
			switch opts.Role {
			case "admin":
				sleep(20 * time.Millisecond)
				return []*gh.User{{ID: githubpkg.Ptr(int64(2)), Login: githubpkg.Ptr("zulu")}}, &gh.Response{}, nil
			case "member":
				sleep(5 * time.Millisecond)
				return []*gh.User{{ID: githubpkg.Ptr(int64(1)), Login: githubpkg.Ptr("alpha")}}, &gh.Response{}, nil
			default:
				t.Fatalf("unexpected role %q", opts.Role)
				return nil, nil, nil
			}
		},
		listPendingOrgInvitationsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListPendingOrgInvitations: got %q want %q", org, orgName)
			}
			return []*gh.Invitation{
				{ID: githubpkg.Ptr(int64(2)), Login: githubpkg.Ptr("zoe"), Role: githubpkg.Ptr("direct_member"), TeamCount: githubpkg.Ptr(1)},
				{ID: githubpkg.Ptr(int64(1)), Login: githubpkg.Ptr("beta"), Role: githubpkg.Ptr("admin"), TeamCount: githubpkg.Ptr(1)},
			}, &gh.Response{}, nil
		},
		listOrgInvitationTeamsFunc: func(_ context.Context, org, invitationID string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListOrgInvitationTeams: got %q want %q", org, orgName)
			}
			switch invitationID {
			case "1":
				sleep(1 * time.Millisecond)
				return []*gh.Team{{Slug: githubpkg.Ptr("admins"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}}}, &gh.Response{}, nil
			case "2":
				sleep(15 * time.Millisecond)
				return []*gh.Team{{Slug: githubpkg.Ptr("writers"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}}}, &gh.Response{}, nil
			default:
				t.Fatalf("unexpected invitation id %q", invitationID)
				return nil, nil, nil
			}
		},
	}
	repoSvc := &repositoryServiceStub{
		listByOrgFunc: func(_ context.Context, org string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListByOrg: got %q want %q", org, orgName)
			}
			sleep(10 * time.Millisecond)
			return []*gh.Repository{
				{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr("repo-b"), Visibility: githubpkg.Ptr("private")},
				{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr("repo-a"), Visibility: githubpkg.Ptr("public")},
			}, &gh.Response{}, nil
		},
	}
	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeams: got %q want %q", org, orgName)
			}
			return []*gh.Team{
				{Slug: githubpkg.Ptr("platform"), Name: githubpkg.Ptr("Platform"), Privacy: githubpkg.Ptr("closed"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
				{Slug: githubpkg.Ptr("admins"), Name: githubpkg.Ptr("Admins"), Privacy: githubpkg.Ptr("secret"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
			}, &gh.Response{}, nil
		},
		listTeamMembersBySlugFunc: func(_ context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeamMembersBySlug: got %q want %q", org, orgName)
			}
			switch slug + "/" + opts.Role {
			case "admins/member":
				sleep(12 * time.Millisecond)
				return []*gh.User{{Login: githubpkg.Ptr("alice")}}, &gh.Response{}, nil
			case "admins/maintainer":
				sleep(2 * time.Millisecond)
				return []*gh.User{}, &gh.Response{}, nil
			case "platform/member":
				sleep(3 * time.Millisecond)
				return []*gh.User{{Login: githubpkg.Ptr("bob")}}, &gh.Response{}, nil
			case "platform/maintainer":
				sleep(18 * time.Millisecond)
				return []*gh.User{{Login: githubpkg.Ptr("carol")}}, &gh.Response{}, nil
			default:
				t.Fatalf("unexpected team member query %s/%s", slug, opts.Role)
				return nil, nil, nil
			}
		},
		listTeamReposBySlugFunc: func(_ context.Context, org, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeamReposBySlug: got %q want %q", org, orgName)
			}
			switch slug {
			case "admins":
				sleep(8 * time.Millisecond)
				return []*gh.Repository{{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr("repo-a"), Permissions: map[string]bool{"admin": true}}}, &gh.Response{}, nil
			case "platform":
				sleep(1 * time.Millisecond)
				return []*gh.Repository{{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr("repo-b"), Permissions: map[string]bool{"push": true}}}, &gh.Response{}, nil
			default:
				t.Fatalf("unexpected team repo query %q", slug)
				return nil, nil, nil
			}
		},
	}

	actual, err := collectOrganizationWithLimits(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
		RepositoryService:   repoSvc,
		TeamService:         teamSvc,
	}, collectOrganizationBehavior{
		includeMembers:            true,
		includePendingInvitations: true,
		includeRepositories:       true,
		includeTeams:              true,
	}, defaultCollectorConcurrencyLimits)
	if err != nil {
		t.Fatalf("collectOrganizationWithLimits returned error: %v", err)
	}

	want := &state.OrganizationState{
		Organization: orgName,
		Members: []state.OrganizationMember{
			{ID: 1, Username: "alpha", Role: "member"},
			{ID: 2, Username: "zulu", Role: "admin"},
		},
		PendingInvitations: []state.PendingInvitation{
			{ID: 1, Username: "beta", Role: "admin", TeamSlugs: []string{"admins"}},
			{ID: 2, Username: "zoe", Role: "direct_member", TeamSlugs: []string{"writers"}},
		},
		Repositories: []state.Repository{
			{Owner: orgName, Name: "repo-a", Visibility: "public", Topics: []string{}},
			{Owner: orgName, Name: "repo-b", Visibility: "private", Topics: []string{}},
		},
		Teams: []state.Team{
			{Slug: "admins", Name: "Admins", Privacy: "secret"},
			{Slug: "platform", Name: "Platform", Privacy: "closed"},
		},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "admins", Username: "alice", Role: "member"},
			{TeamSlug: "platform", Username: "bob", Role: "member"},
			{TeamSlug: "platform", Username: "carol", Role: "maintainer"},
		},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{
			{TeamSlug: "admins", Owner: orgName, Name: "repo-a", Permission: "admin"},
			{TeamSlug: "platform", Owner: orgName, Name: "repo-b", Permission: "push"},
		},
	}
	want.Normalize()

	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("unexpected organization state:\n got %#v\nwant %#v", actual, want)
	}
}

func BenchmarkCollectOrganizationConcurrency(b *testing.B) {
	const orgName = "orang-gaboets"
	const invitationCount = 8
	const teamCount = 12
	const callDelay = 200 * time.Microsecond

	buildServices := func() (*organizationServiceStub, *repositoryServiceStub, *teamServiceStub) {
		orgSvc := &organizationServiceStub{
			listMembersFunc: func(_ context.Context, _ string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
				time.Sleep(callDelay)
				switch opts.Role {
				case "admin":
					return []*gh.User{{ID: githubpkg.Ptr(int64(1)), Login: githubpkg.Ptr("alpha")}}, &gh.Response{}, nil
				case "member":
					return []*gh.User{{ID: githubpkg.Ptr(int64(2)), Login: githubpkg.Ptr("beta")}}, &gh.Response{}, nil
				default:
					return []*gh.User{}, &gh.Response{}, nil
				}
			},
			listPendingOrgInvitationsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
				time.Sleep(callDelay)
				invitations := make([]*gh.Invitation, 0, invitationCount)
				for i := 0; i < invitationCount; i++ {
					id := int64(i + 1)
					login := fmt.Sprintf("invite-%02d", i)
					teamCountValue := 1
					invitations = append(invitations, &gh.Invitation{ID: githubpkg.Ptr(id), Login: githubpkg.Ptr(login), Role: githubpkg.Ptr("direct_member"), TeamCount: githubpkg.Ptr(teamCountValue)})
				}
				return invitations, &gh.Response{}, nil
			},
			listOrgInvitationTeamsFunc: func(_ context.Context, _ string, invitationID string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
				time.Sleep(callDelay)
				slug := "team-" + invitationID
				return []*gh.Team{{Slug: githubpkg.Ptr(slug), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}}}, &gh.Response{}, nil
			},
		}
		repoSvc := &repositoryServiceStub{
			listByOrgFunc: func(_ context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
				time.Sleep(callDelay)
				return []*gh.Repository{{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr("repo-builder"), Visibility: githubpkg.Ptr("private")}}, &gh.Response{}, nil
			},
		}
		teamSvc := &teamServiceStub{
			listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
				time.Sleep(callDelay)
				teams := make([]*gh.Team, 0, teamCount)
				for i := 0; i < teamCount; i++ {
					slug := fmt.Sprintf("team-%02d", i)
					teams = append(teams, &gh.Team{Slug: githubpkg.Ptr(slug), Name: githubpkg.Ptr(slug), Privacy: githubpkg.Ptr("closed"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}})
				}
				return teams, &gh.Response{}, nil
			},
			listTeamMembersBySlugFunc: func(_ context.Context, _ string, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
				time.Sleep(callDelay)
				if opts.Role == "member" {
					return []*gh.User{{Login: githubpkg.Ptr(slug + "-member")}}, &gh.Response{}, nil
				}
				return []*gh.User{{Login: githubpkg.Ptr(slug + "-maintainer")}}, &gh.Response{}, nil
			},
			listTeamReposBySlugFunc: func(_ context.Context, _ string, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
				time.Sleep(callDelay)
				return []*gh.Repository{{Owner: &gh.User{Login: githubpkg.Ptr(orgName)}, Name: githubpkg.Ptr(slug + "-repo"), Permissions: map[string]bool{"push": true}}}, &gh.Response{}, nil
			},
		}
		return orgSvc, repoSvc, teamSvc
	}

	benchmark := func(b *testing.B, limits collectorConcurrencyLimits) {
		b.Helper()
		b.ReportAllocs()
		for b.Loop() {
			orgSvc, repoSvc, teamSvc := buildServices()
			if _, err := collectOrganizationWithLimits(context.Background(), CollectOrganizationOptions{
				OrgName:             orgName,
				OrganizationService: orgSvc,
				RepositoryService:   repoSvc,
				TeamService:         teamSvc,
			}, collectOrganizationBehavior{
				includeMembers:            true,
				includePendingInvitations: true,
				includeRepositories:       true,
				includeTeams:              true,
			}, limits); err != nil {
				b.Fatalf("collectOrganizationWithLimits returned error: %v", err)
			}
		}
	}

	b.Run("sequential", func(b *testing.B) {
		benchmark(b, collectorConcurrencyLimits{
			topLevel:        1,
			memberRoles:     1,
			invitationTeams: 1,
			teamDetails:     1,
		})
	})

	b.Run("bounded_concurrent", func(b *testing.B) {
		benchmark(b, defaultCollectorConcurrencyLimits)
	})
}
