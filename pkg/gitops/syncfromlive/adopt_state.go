package syncfromlive

import "github.com/orang-gaboets/repo-builder/pkg/gitops/config"

func cloneDesiredConfig(cfg config.OrganizationConfig) config.OrganizationConfig {
	return config.OrganizationConfig{
		Organization: cfg.Organization,
		Members:      append([]config.OrganizationMemberSpec{}, cfg.Members...),
		Invites:      cloneDesiredInvites(cfg.Invites),
		Repositories: cloneDesiredRepositories(cfg.Repositories),
		Teams:        cloneDesiredTeams(cfg.Teams),
	}
}

func cloneDesiredInvites(invites []config.InviteSpec) []config.InviteSpec {
	cloned := make([]config.InviteSpec, 0, len(invites))
	for _, invite := range invites {
		inviteClone := invite
		inviteClone.TeamSlugs = append([]string{}, invite.TeamSlugs...)
		cloned = append(cloned, inviteClone)
	}
	return cloned
}

func cloneDesiredRepositories(repositories []config.RepositorySpec) []config.RepositorySpec {
	cloned := make([]config.RepositorySpec, 0, len(repositories))
	for _, repository := range repositories {
		repositoryClone := repository
		repositoryClone.Topics = append([]string{}, repository.Topics...)
		cloned = append(cloned, repositoryClone)
	}
	return cloned
}

func cloneDesiredTeams(teams []config.TeamSpec) []config.TeamSpec {
	cloned := make([]config.TeamSpec, 0, len(teams))
	for _, team := range teams {
		teamClone := team
		teamClone.Members = append([]config.TeamMemberSpec{}, team.Members...)
		teamClone.Repositories = append([]config.TeamRepositorySpec{}, team.Repositories...)
		cloned = append(cloned, teamClone)
	}
	return cloned
}
