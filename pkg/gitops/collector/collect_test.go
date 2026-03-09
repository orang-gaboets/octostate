package collector

import (
	"context"
	"errors"
	"reflect"
	"testing"

	gh "github.com/google/go-github/v55/github"
	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func TestCollectOrganizationOptionsValidate(t *testing.T) {
	t.Parallel()

	validOrgSvc := &organizationServiceStub{}
	validRepoSvc := &repositoryServiceStub{}
	validTeamSvc := &teamServiceStub{}

	tests := []struct {
		name    string
		options CollectOrganizationOptions
		wantErr error
	}{
		{
			name: "missing org name",
			options: CollectOrganizationOptions{
				OrganizationService: validOrgSvc,
				RepositoryService:   validRepoSvc,
				TeamService:         validTeamSvc,
			},
			wantErr: githubpkg.ErrMissingRequiredField,
		},
		{
			name: "nil organization service",
			options: CollectOrganizationOptions{
				OrgName:           "orang-gaboets",
				RepositoryService: validRepoSvc,
				TeamService:       validTeamSvc,
			},
			wantErr: githubpkg.ErrNilService,
		},
		{
			name: "nil repository service",
			options: CollectOrganizationOptions{
				OrgName:             "orang-gaboets",
				OrganizationService: validOrgSvc,
				TeamService:         validTeamSvc,
			},
			wantErr: githubpkg.ErrNilService,
		},
		{
			name: "nil team service",
			options: CollectOrganizationOptions{
				OrgName:             "orang-gaboets",
				OrganizationService: validOrgSvc,
				RepositoryService:   validRepoSvc,
			},
			wantErr: githubpkg.ErrNilService,
		},
		{
			name: "valid",
			options: CollectOrganizationOptions{
				OrgName:             "orang-gaboets",
				OrganizationService: validOrgSvc,
				RepositoryService:   validRepoSvc,
				TeamService:         validTeamSvc,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.options.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: got %v want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollectOrganizationSuccess(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(_ context.Context, org string, _ *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListMembers: got %q want %q", org, orgName)
			}
			return []*gh.User{
				{
					ID:    githubpkg.Ptr(int64(2)),
					Login: githubpkg.Ptr("zulu"),
					Name:  githubpkg.Ptr("Zulu"),
					Email: githubpkg.Ptr("zulu@example.com"),
				},
				{
					ID:    githubpkg.Ptr(int64(1)),
					Login: githubpkg.Ptr("alpha"),
					Name:  githubpkg.Ptr("Alpha"),
					Email: githubpkg.Ptr("alpha@example.com"),
				},
			}, &gh.Response{}, nil
		},
		listPendingOrgInvitationsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListPendingOrgInvitations: got %q want %q", org, orgName)
			}
			return []*gh.Invitation{
				{
					ID:    githubpkg.Ptr(int64(9)),
					Login: githubpkg.Ptr("zoe"),
					Email: githubpkg.Ptr("zoe@example.com"),
					Role:  githubpkg.Ptr("direct_member"),
				},
				{
					ID:    githubpkg.Ptr(int64(3)),
					Login: githubpkg.Ptr("beta"),
					Role:  githubpkg.Ptr("admin"),
				},
			}, &gh.Response{}, nil
		},
		listOrgInvitationTeamsFunc: func(_ context.Context, org, invitationID string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListOrgInvitationTeams: got %q want %q", org, orgName)
			}
			switch invitationID {
			case "9":
				return []*gh.Team{
					{
						Slug:         githubpkg.Ptr("writers"),
						Name:         githubpkg.Ptr("Writers"),
						Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
					},
					{
						Slug:         githubpkg.Ptr("admins"),
						Name:         githubpkg.Ptr("Admins"),
						Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
					},
				}, &gh.Response{}, nil
			case "3":
				return nil, &gh.Response{}, nil
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
			return []*gh.Repository{
				{
					Owner:        &gh.User{Login: githubpkg.Ptr(orgName)},
					Name:         githubpkg.Ptr("repo-builder"),
					Visibility:   githubpkg.Ptr("private"),
					Description:  githubpkg.Ptr("CLI"),
					Homepage:     githubpkg.Ptr("https://example.com/repo-builder"),
					Topics:       []string{"gitops", "go"},
					AllowForking: githubpkg.Ptr(false),
					Archived:     githubpkg.Ptr(false),
					IsTemplate:   githubpkg.Ptr(false),
				},
				{
					Owner:        &gh.User{Login: githubpkg.Ptr(orgName)},
					Name:         githubpkg.Ptr("alpha"),
					Visibility:   githubpkg.Ptr("public"),
					Description:  githubpkg.Ptr("Alpha repo"),
					Homepage:     githubpkg.Ptr(""),
					Topics:       nil,
					AllowForking: githubpkg.Ptr(true),
					Archived:     githubpkg.Ptr(false),
					IsTemplate:   githubpkg.Ptr(true),
				},
			}, &gh.Response{}, nil
		},
	}

	teamRoleCalls := make(map[string][]string)
	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeams: got %q want %q", org, orgName)
			}
			return []*gh.Team{
				{
					ID:           githubpkg.Ptr(int64(2)),
					Slug:         githubpkg.Ptr("platform"),
					Name:         githubpkg.Ptr("Platform"),
					Description:  githubpkg.Ptr("Platform engineering"),
					Privacy:      githubpkg.Ptr("closed"),
					Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
					Parent: &gh.Team{
						ID:           githubpkg.Ptr(int64(1)),
						Slug:         githubpkg.Ptr("admins"),
						Name:         githubpkg.Ptr("Admins"),
						Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
					},
				},
				{
					ID:           githubpkg.Ptr(int64(1)),
					Slug:         githubpkg.Ptr("admins"),
					Name:         githubpkg.Ptr("Admins"),
					Description:  githubpkg.Ptr("Admin team"),
					Privacy:      githubpkg.Ptr("secret"),
					Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
				},
			}, &gh.Response{}, nil
		},
		listTeamMembersBySlugFunc: func(_ context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeamMembersBySlug: got %q want %q", org, orgName)
			}
			if opts == nil {
				t.Fatal("expected member list options")
				return nil, nil, nil
			}
			teamRoleCalls[slug] = append(teamRoleCalls[slug], opts.Role)

			switch slug {
			case "admins":
				switch opts.Role {
				case "member":
					return []*gh.User{{Login: githubpkg.Ptr("zed")}}, &gh.Response{}, nil
				case "maintainer":
					return []*gh.User{{Login: githubpkg.Ptr("alice")}}, &gh.Response{}, nil
				}
			case "platform":
				switch opts.Role {
				case "member":
					return []*gh.User{{Login: githubpkg.Ptr("bob")}}, &gh.Response{}, nil
				case "maintainer":
					return []*gh.User{}, &gh.Response{}, nil
				}
			}

			t.Fatalf("unexpected team member query: slug=%q role=%q", slug, opts.Role)
			return nil, nil, nil
		},
		listTeamReposBySlugFunc: func(_ context.Context, org, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			if org != orgName {
				t.Fatalf("unexpected org in ListTeamReposBySlug: got %q want %q", org, orgName)
			}
			switch slug {
			case "admins":
				return []*gh.Repository{
					{
						Owner:       &gh.User{Login: githubpkg.Ptr(orgName)},
						Name:        githubpkg.Ptr("repo-admin"),
						Permissions: map[string]bool{"pull": true, "admin": true},
					},
				}, &gh.Response{}, nil
			case "platform":
				return []*gh.Repository{
					{
						Owner:       &gh.User{Login: githubpkg.Ptr(orgName)},
						Name:        githubpkg.Ptr("repo-builder"),
						Permissions: map[string]bool{"pull": true, "push": true},
					},
				}, &gh.Response{}, nil
			default:
				t.Fatalf("unexpected team repo query for slug %q", slug)
				return nil, nil, nil
			}
		},
	}

	actual, err := CollectOrganization(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
		RepositoryService:   repoSvc,
		TeamService:         teamSvc,
	})
	if err != nil {
		t.Fatalf("CollectOrganization returned error: %v", err)
	}

	want := &state.OrganizationState{
		Organization: orgName,
		Members: []state.OrganizationMember{
			{ID: 1, Username: "alpha", Name: "Alpha", Email: "alpha@example.com"},
			{ID: 2, Username: "zulu", Name: "Zulu", Email: "zulu@example.com"},
		},
		PendingInvitations: []state.PendingInvitation{
			{ID: 3, Username: "beta", Role: "admin", TeamSlugs: []string{}},
			{ID: 9, Username: "zoe", Email: "zoe@example.com", Role: "direct_member", TeamSlugs: []string{"admins", "writers"}},
		},
		Repositories: []state.Repository{
			{Owner: orgName, Name: "alpha", Visibility: "public", Description: "Alpha repo", Homepage: "", Topics: []string{}, AllowForking: true, Archived: false, IsTemplate: true},
			{Owner: orgName, Name: "repo-builder", Visibility: "private", Description: "CLI", Homepage: "https://example.com/repo-builder", Topics: []string{"gitops", "go"}, AllowForking: false, Archived: false, IsTemplate: false},
		},
		Teams: []state.Team{
			{ID: 1, Slug: "admins", Name: "Admins", Description: "Admin team", Privacy: "secret", ParentSlug: ""},
			{ID: 2, Slug: "platform", Name: "Platform", Description: "Platform engineering", Privacy: "closed", ParentSlug: "admins"},
		},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "admins", Username: "alice", Role: "maintainer"},
			{TeamSlug: "admins", Username: "zed", Role: "member"},
			{TeamSlug: "platform", Username: "bob", Role: "member"},
		},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{
			{TeamSlug: "admins", Owner: orgName, Name: "repo-admin", Permission: "admin"},
			{TeamSlug: "platform", Owner: orgName, Name: "repo-builder", Permission: "push"},
		},
	}

	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("unexpected organization state:\n got %#v\nwant %#v", actual, want)
	}

	wantRoleCalls := map[string][]string{
		"admins":   {"member", "maintainer"},
		"platform": {"member", "maintainer"},
	}
	if !reflect.DeepEqual(teamRoleCalls, wantRoleCalls) {
		t.Fatalf("unexpected team member role calls: got %#v want %#v", teamRoleCalls, wantRoleCalls)
	}
}

func TestCollectOrganizationReturnsInvitationTeamError(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	wantErr := errors.New("invitation team lookup failed")

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(_ context.Context, _ string, _ *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			return nil, &gh.Response{}, nil
		},
		listPendingOrgInvitationsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			return []*gh.Invitation{
				{ID: githubpkg.Ptr(int64(42)), Login: githubpkg.Ptr("octocat")},
			}, &gh.Response{}, nil
		},
		listOrgInvitationTeamsFunc: func(_ context.Context, _ string, invitationID string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			if invitationID != "42" {
				t.Fatalf("unexpected invitation id: got %q want %q", invitationID, "42")
			}
			return nil, nil, wantErr
		},
	}

	repoSvc := &repositoryServiceStub{
		listByOrgFunc: func(_ context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
			t.Fatal("ListByOrg should not be called after invitation team error")
			return nil, nil, nil
		},
	}

	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			t.Fatal("ListTeams should not be called after invitation team error")
			return nil, nil, nil
		},
	}

	_, err := CollectOrganization(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
		RepositoryService:   repoSvc,
		TeamService:         teamSvc,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
}

func TestCollectOrganizationReturnsTeamMemberError(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	wantErr := errors.New("team member lookup failed")

	orgSvc := &organizationServiceStub{
		listMembersFunc: func(_ context.Context, _ string, _ *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
			return nil, &gh.Response{}, nil
		},
		listPendingOrgInvitationsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
			return nil, &gh.Response{}, nil
		},
		listOrgInvitationTeamsFunc: func(_ context.Context, _ string, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			t.Fatal("ListOrgInvitationTeams should not be called without invitations")
			return nil, nil, nil
		},
	}

	repoSvc := &repositoryServiceStub{
		listByOrgFunc: func(_ context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
			return nil, &gh.Response{}, nil
		},
	}

	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			return []*gh.Team{
				{
					Slug:         githubpkg.Ptr("platform"),
					Name:         githubpkg.Ptr("Platform"),
					Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
				},
			}, &gh.Response{}, nil
		},
		listTeamMembersBySlugFunc: func(_ context.Context, _ string, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			if slug != "platform" {
				t.Fatalf("unexpected slug: got %q want %q", slug, "platform")
			}
			if opts == nil || opts.Role != "member" {
				t.Fatalf("unexpected team member options: %#v", opts)
			}
			return nil, nil, wantErr
		},
		listTeamReposBySlugFunc: func(_ context.Context, _ string, _ string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			t.Fatal("ListTeamReposBySlug should not be called after team member error")
			return nil, nil, nil
		},
	}

	_, err := CollectOrganization(context.Background(), CollectOrganizationOptions{
		OrgName:             orgName,
		OrganizationService: orgSvc,
		RepositoryService:   repoSvc,
		TeamService:         teamSvc,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
}

type organizationServiceStub struct {
	listMembersFunc               func(context.Context, string, *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error)
	listPendingOrgInvitationsFunc func(context.Context, string, *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error)
	listOrgInvitationTeamsFunc    func(context.Context, string, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}

func (s *organizationServiceStub) CreateOrgInvitation(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	panic("unexpected CreateOrgInvitation call")
}

func (s *organizationServiceStub) Get(context.Context, string) (*gh.Organization, *gh.Response, error) {
	panic("unexpected Get call")
}

func (s *organizationServiceStub) ListMembers(ctx context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
	if s.listMembersFunc == nil {
		panic("unexpected ListMembers call")
	}
	return s.listMembersFunc(ctx, org, opts)
}

func (s *organizationServiceStub) ListPendingOrgInvitations(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
	if s.listPendingOrgInvitationsFunc == nil {
		panic("unexpected ListPendingOrgInvitations call")
	}
	return s.listPendingOrgInvitationsFunc(ctx, org, opts)
}

func (s *organizationServiceStub) ListOrgInvitationTeams(ctx context.Context, org, invitationID string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	if s.listOrgInvitationTeamsFunc == nil {
		panic("unexpected ListOrgInvitationTeams call")
	}
	return s.listOrgInvitationTeamsFunc(ctx, org, invitationID, opts)
}

type repositoryServiceStub struct {
	listByOrgFunc func(context.Context, string, *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error)
}

func (s *repositoryServiceStub) CreateFromTemplate(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	panic("unexpected CreateFromTemplate call")
}

func (s *repositoryServiceStub) Delete(context.Context, string, string) (*gh.Response, error) {
	panic("unexpected Delete call")
}

func (s *repositoryServiceStub) Edit(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
	panic("unexpected Edit call")
}

func (s *repositoryServiceStub) Get(context.Context, string, string) (*gh.Repository, *gh.Response, error) {
	panic("unexpected Get call")
}

func (s *repositoryServiceStub) ListByOrg(ctx context.Context, org string, opts *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	if s.listByOrgFunc == nil {
		panic("unexpected ListByOrg call")
	}
	return s.listByOrgFunc(ctx, org, opts)
}

func (s *repositoryServiceStub) ReplaceAllTopics(context.Context, string, string, []string) ([]string, *gh.Response, error) {
	panic("unexpected ReplaceAllTopics call")
}

func (s *repositoryServiceStub) ListAllTopics(context.Context, string, string) ([]string, *gh.Response, error) {
	panic("unexpected ListAllTopics call")
}

type teamServiceStub struct {
	listTeamsFunc             func(context.Context, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
	listTeamMembersBySlugFunc func(context.Context, string, string, *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error)
	listTeamReposBySlugFunc   func(context.Context, string, string, *gh.ListOptions) ([]*gh.Repository, *gh.Response, error)
}

func (s *teamServiceStub) CreateTeam(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error) {
	panic("unexpected CreateTeam call")
}

func (s *teamServiceStub) EditTeamBySlug(context.Context, string, string, gh.NewTeam, bool) (*gh.Team, *gh.Response, error) {
	panic("unexpected EditTeamBySlug call")
}

func (s *teamServiceStub) DeleteTeamBySlug(context.Context, string, string) (*gh.Response, error) {
	panic("unexpected DeleteTeamBySlug call")
}

func (s *teamServiceStub) GetTeamBySlug(context.Context, string, string) (*gh.Team, *gh.Response, error) {
	panic("unexpected GetTeamBySlug call")
}

func (s *teamServiceStub) AddTeamMembershipBySlug(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	panic("unexpected AddTeamMembershipBySlug call")
}

func (s *teamServiceStub) RemoveTeamMembershipBySlug(context.Context, string, string, string) (*gh.Response, error) {
	panic("unexpected RemoveTeamMembershipBySlug call")
}

func (s *teamServiceStub) ListTeamReposBySlug(ctx context.Context, org, slug string, opts *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
	if s.listTeamReposBySlugFunc == nil {
		panic("unexpected ListTeamReposBySlug call")
	}
	return s.listTeamReposBySlugFunc(ctx, org, slug, opts)
}

func (s *teamServiceStub) AddTeamRepoBySlug(context.Context, string, string, string, string, *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
	panic("unexpected AddTeamRepoBySlug call")
}

func (s *teamServiceStub) RemoveTeamRepoBySlug(context.Context, string, string, string, string) (*gh.Response, error) {
	panic("unexpected RemoveTeamRepoBySlug call")
}

func (s *teamServiceStub) ListTeamMembersBySlug(ctx context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	if s.listTeamMembersBySlugFunc == nil {
		panic("unexpected ListTeamMembersBySlug call")
	}
	return s.listTeamMembersBySlugFunc(ctx, org, slug, opts)
}

func (s *teamServiceStub) ListTeams(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	if s.listTeamsFunc == nil {
		panic("unexpected ListTeams call")
	}
	return s.listTeamsFunc(ctx, org, opts)
}
