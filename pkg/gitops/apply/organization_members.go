package apply

import (
	"fmt"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
)

func (e *executor) executeOrganizationMemberAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
	default:
		return fmt.Errorf("unsupported organization member operation %q for %s: %w", action.Operation, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	member, ok := e.desiredOrgMembers[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired organization member %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	return organizations.SetMembership(e.ctx, organizations.SetMembershipOptions{
		Service:  e.organizationService,
		OrgName:  e.organization,
		Username: member.Username,
		Role:     member.Role,
	})
}
