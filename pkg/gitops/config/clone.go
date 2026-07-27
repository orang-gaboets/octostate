package config

// Clone returns a deep copy of the organization config so callers can
// normalize or mutate the copy without affecting the original.
func (c OrganizationConfig) Clone() OrganizationConfig {
	cloned := c
	cloned.Members = append([]OrganizationMemberSpec{}, c.Members...)
	cloned.Invites = CloneInviteSpecs(c.Invites)
	cloned.Repositories = CloneRepositorySpecs(c.Repositories)
	cloned.Teams = CloneTeamSpecs(c.Teams)
	return cloned
}

// CloneInviteSpecs returns a deep copy of invite specs.
func CloneInviteSpecs(invites []InviteSpec) []InviteSpec {
	cloned := append([]InviteSpec{}, invites...)
	for i := range cloned {
		cloned[i].TeamSlugs = append([]string{}, cloned[i].TeamSlugs...)
	}
	return cloned
}

// CloneRepositorySpecs returns a deep copy of repository specs.
func CloneRepositorySpecs(repositories []RepositorySpec) []RepositorySpec {
	cloned := append([]RepositorySpec{}, repositories...)
	for i := range cloned {
		cloned[i].Topics = append([]string{}, cloned[i].Topics...)
	}
	return cloned
}

// CloneTeamSpecs returns a deep copy of team specs.
func CloneTeamSpecs(teams []TeamSpec) []TeamSpec {
	cloned := append([]TeamSpec{}, teams...)
	for i := range cloned {
		cloned[i].Members = append([]TeamMemberSpec{}, cloned[i].Members...)
		cloned[i].Repositories = append([]TeamRepositorySpec{}, cloned[i].Repositories...)
	}
	return cloned
}
