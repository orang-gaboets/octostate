package diff

import (
	"fmt"
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func BenchmarkBuildActionsConcurrency(b *testing.B) {
	builder := benchmarkBuilder()

	benchmark := func(b *testing.B, limit int) {
		b.Helper()
		b.ReportAllocs()
		for b.Loop() {
			if _, err := builder.buildActionsWithLimit(limit); err != nil {
				b.Fatalf("buildActionsWithLimit returned error: %v", err)
			}
		}
	}

	b.Run("sequential", func(b *testing.B) {
		benchmark(b, 1)
	})

	b.Run("bounded_concurrent", func(b *testing.B) {
		benchmark(b, diffPhaseConcurrency)
	})
}

func benchmarkBuilder() builder {
	const organization = "orang-gaboets"
	const memberCount = 120
	const teamCount = 30
	const repositoryCount = 80
	const inviteCount = 45

	desired := config.OrganizationConfig{
		Organization: organization,
		Members:      make([]config.OrganizationMemberSpec, 0, memberCount),
		Invites:      make([]config.InviteSpec, 0, inviteCount),
		Repositories: make([]config.RepositorySpec, 0, repositoryCount),
		Teams:        make([]config.TeamSpec, 0, teamCount),
	}
	actual := state.OrganizationState{
		Organization:              organization,
		Members:                   make([]state.OrganizationMember, 0, memberCount),
		PendingInvitations:        make([]state.PendingInvitation, 0, inviteCount),
		Repositories:              make([]state.Repository, 0, repositoryCount),
		Teams:                     make([]state.Team, 0, teamCount),
		TeamMembers:               make([]state.TeamMember, 0, teamCount*2),
		TeamRepositoryPermissions: make([]state.TeamRepositoryPermission, 0, teamCount*2),
	}

	for i := 0; i < memberCount; i++ {
		username := fmt.Sprintf("member-%03d", i)
		if i < memberCount/2 {
			desired.Members = append(desired.Members, config.OrganizationMemberSpec{
				Username: username,
				Role:     "member",
			})
		}
		actual.Members = append(actual.Members, state.OrganizationMember{
			ID:       int64(i + 1),
			Username: username,
			Role:     "member",
		})
	}

	for i := 0; i < inviteCount; i++ {
		username := fmt.Sprintf("invite-user-%03d", i)
		desired.Invites = append(desired.Invites, config.InviteSpec{
			Username: presentString(username),
		})
		if i%2 == 0 {
			actual.PendingInvitations = append(actual.PendingInvitations, state.PendingInvitation{
				ID:       int64(i + 1),
				Username: username,
				Role:     "direct_member",
			})
		}
	}

	for i := 0; i < repositoryCount; i++ {
		name := fmt.Sprintf("repo-%03d", i)
		desired.Repositories = append(desired.Repositories, config.RepositorySpec{
			Owner:      organization,
			Name:       name,
			Visibility: "private",
		})
		if i%3 != 0 {
			actual.Repositories = append(actual.Repositories, state.Repository{
				Owner:      organization,
				Name:       name,
				Visibility: "private",
			})
		}
	}

	for i := 0; i < teamCount; i++ {
		slug := fmt.Sprintf("team-%03d", i)
		memberUsername := actual.Members[i%len(actual.Members)].Username
		repoName := desired.Repositories[i%len(desired.Repositories)].Name
		desired.Teams = append(desired.Teams, config.TeamSpec{
			Slug:    slug,
			Name:    "Team " + slug,
			Privacy: "closed",
			Members: []config.TeamMemberSpec{
				{Username: memberUsername, Role: "member"},
			},
			Repositories: []config.TeamRepositorySpec{
				{Owner: organization, Name: repoName, Permission: "push"},
			},
		})
		actual.Teams = append(actual.Teams, state.Team{
			Slug:    slug,
			Name:    "Legacy " + slug,
			Privacy: "closed",
		})
		actual.TeamMembers = append(actual.TeamMembers, state.TeamMember{
			TeamSlug: slug,
			Username: memberUsername,
			Role:     "maintainer",
		})
		actual.TeamRepositoryPermissions = append(actual.TeamRepositoryPermissions, state.TeamRepositoryPermission{
			TeamSlug:   slug,
			Owner:      organization,
			Name:       repoName,
			Permission: "pull",
		})
	}

	actual.Normalize()
	return builder{
		desired: desired,
		actual:  actual,
	}
}
