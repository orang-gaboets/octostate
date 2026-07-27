package state

// Clone returns a deep copy of the organization state. A nil receiver yields
// an empty state.
func (s *OrganizationState) Clone() OrganizationState {
	if s == nil {
		return OrganizationState{}
	}

	return OrganizationState{
		Organization:              s.Organization,
		Members:                   append([]OrganizationMember{}, s.Members...),
		PendingInvitations:        ClonePendingInvitations(s.PendingInvitations),
		Repositories:              CloneRepositories(s.Repositories),
		Teams:                     append([]Team{}, s.Teams...),
		TeamMembers:               append([]TeamMember{}, s.TeamMembers...),
		TeamRepositoryPermissions: append([]TeamRepositoryPermission{}, s.TeamRepositoryPermissions...),
	}
}

// ClonePendingInvitations returns a deep copy of pending invitations.
func ClonePendingInvitations(invitations []PendingInvitation) []PendingInvitation {
	cloned := append([]PendingInvitation{}, invitations...)
	for i := range cloned {
		cloned[i].TeamSlugs = append([]string{}, cloned[i].TeamSlugs...)
	}
	return cloned
}

// CloneRepositories returns a deep copy of repositories.
func CloneRepositories(repositories []Repository) []Repository {
	cloned := append([]Repository{}, repositories...)
	for i := range cloned {
		cloned[i].Topics = append([]string{}, cloned[i].Topics...)
	}
	return cloned
}
