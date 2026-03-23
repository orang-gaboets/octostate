package plan

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	ghusers "github.com/orang-gaboets/repo-builder/pkg/github/users"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

type inviteIdentityKind string

const (
	inviteIdentityKindUsername inviteIdentityKind = "username"
	inviteIdentityKindEmail    inviteIdentityKind = "email"
	inviteIdentityKindUserID   inviteIdentityKind = "user_id"
)

type inviteIdentity struct {
	kind     inviteIdentityKind
	username string
	email    string
	userID   int64
}

func (id inviteIdentity) resourceID() string {
	switch id.kind {
	case inviteIdentityKindUsername:
		return "username:" + id.username
	case inviteIdentityKindEmail:
		return "email:" + id.email
	case inviteIdentityKindUserID:
		return fmt.Sprintf("user_id:%d", id.userID)
	default:
		return "invite"
	}
}

func (p planner) appendInviteActions(actions []Action) ([]Action, error) {
	for _, invite := range p.desired.Invites {
		identity, err := p.desiredInviteIdentity(invite)
		if err != nil {
			return nil, err
		}
		overlapsMember, err := p.desiredMemberMatchesInviteIdentity(identity)
		if err != nil {
			return nil, err
		}
		if overlapsMember {
			return nil, fmt.Errorf(
				"invite %s duplicates a declared top-level member: %w",
				identity.resourceID(),
				githubpkg.ErrInvalidFieldValue,
			)
		}
		satisfied, err := p.inviteIdentitySatisfied(identity)
		if err != nil {
			return nil, err
		}
		if satisfied {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeInvite,
			Operation:    ActionOperationCreate,
			ResourceID:   identity.resourceID(),
			Executable:   true,
			Message:      fmt.Sprintf("create organization invite %s", identity.resourceID()),
		})
	}

	for _, invitation := range p.actual.PendingInvitations {
		matched, err := p.pendingInvitationDeclared(invitation)
		if err != nil {
			return nil, err
		}
		if matched {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeInvite,
			Operation:    ActionOperationRemove,
			ResourceID:   pendingInvitationID(invitation),
			Executable:   false,
			Message:      fmt.Sprintf("pending invitation %s exists in live state but is not declared in desired config", pendingInvitationID(invitation)),
		})
	}

	return actions, nil
}

func (p *planner) desiredInviteIdentity(invite config.InviteSpec) (inviteIdentity, error) {
	declared := 0
	identity := inviteIdentity{}

	if invite.Username.Present {
		declared++
		if !invite.Username.Null {
			identity = inviteIdentity{kind: inviteIdentityKindUsername, username: strings.TrimSpace(invite.Username.Value)}
		}
	}
	if invite.Email.Present {
		declared++
		if !invite.Email.Null {
			identity = inviteIdentity{kind: inviteIdentityKindEmail, email: strings.TrimSpace(invite.Email.Value)}
		}
	}
	if invite.UserID.Present {
		declared++
		if !invite.UserID.Null {
			identity = inviteIdentity{kind: inviteIdentityKindUserID, userID: invite.UserID.Value}
		}
	}

	if declared != 1 {
		return inviteIdentity{}, fmt.Errorf("invite identity declaration is invalid: %w", githubpkg.ErrInvalidFieldValue)
	}

	switch identity.kind {
	case inviteIdentityKindUsername:
		if identity.username == "" {
			return inviteIdentity{}, fmt.Errorf("invite username is invalid: %w", githubpkg.ErrInvalidFieldValue)
		}
	case inviteIdentityKindEmail:
		if identity.email == "" {
			return inviteIdentity{}, fmt.Errorf("invite email is invalid: %w", githubpkg.ErrInvalidFieldValue)
		}
	case inviteIdentityKindUserID:
		if identity.userID <= 0 {
			return inviteIdentity{}, fmt.Errorf("invite user_id is invalid: %w", githubpkg.ErrInvalidFieldValue)
		}
	default:
		return inviteIdentity{}, fmt.Errorf("invite identity declaration is invalid: %w", githubpkg.ErrInvalidFieldValue)
	}

	return identity, nil
}

func (p *planner) inviteIdentitySatisfied(identity inviteIdentity) (bool, error) {
	for _, member := range p.actual.Members {
		matched, err := p.memberMatchesIdentity(member, identity)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	for _, invitation := range p.actual.PendingInvitations {
		matched, err := p.pendingInvitationMatchesIdentity(invitation, identity)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

func (p *planner) desiredMemberMatchesInviteIdentity(identity inviteIdentity) (bool, error) {
	switch identity.kind {
	case inviteIdentityKindUsername:
		return p.hasDesiredMember(identity.username), nil
	case inviteIdentityKindUserID:
		if identity.userID <= 0 || p.userService == nil {
			return false, nil
		}
		login, err := p.lookupLoginByUserID(identity.userID)
		if err != nil {
			return false, err
		}
		return p.hasDesiredMember(login), nil
	default:
		return false, nil
	}
}

func (p *planner) hasDesiredMember(username string) bool {
	usernameKey := strings.ToLower(strings.TrimSpace(username))
	if usernameKey == "" {
		return false
	}
	for _, member := range p.desired.Members {
		if strings.EqualFold(member.Username, usernameKey) {
			return true
		}
	}
	return false
}

func (p *planner) pendingInvitationDeclared(invitation state.PendingInvitation) (bool, error) {
	for _, desiredInvite := range p.desired.Invites {
		identity, err := p.desiredInviteIdentity(desiredInvite)
		if err != nil {
			return false, err
		}
		matched, err := p.pendingInvitationMatchesIdentity(invitation, identity)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (p *planner) memberMatchesIdentity(member state.OrganizationMember, identity inviteIdentity) (bool, error) {
	switch identity.kind {
	case inviteIdentityKindUsername:
		return strings.EqualFold(member.Username, identity.username), nil
	case inviteIdentityKindEmail:
		return member.Email != "" && strings.EqualFold(member.Email, identity.email), nil
	case inviteIdentityKindUserID:
		return member.ID > 0 && member.ID == identity.userID, nil
	default:
		return false, nil
	}
}

// Pending invitations do not carry user IDs, so user_id invites need a user
// lookup to resolve the current login before they can be matched by identity.
func (p *planner) pendingInvitationMatchesIdentity(invitation state.PendingInvitation, identity inviteIdentity) (bool, error) {
	switch identity.kind {
	case inviteIdentityKindUsername:
		return invitation.Username != "" && strings.EqualFold(invitation.Username, identity.username), nil
	case inviteIdentityKindEmail:
		return invitation.Email != "" && strings.EqualFold(invitation.Email, identity.email), nil
	case inviteIdentityKindUserID:
		if invitation.Username == "" {
			return false, nil
		}
		login, err := p.lookupLoginByUserID(identity.userID)
		if err != nil {
			return false, err
		}
		return strings.EqualFold(invitation.Username, login), nil
	default:
		return false, nil
	}
}

func (p *planner) lookupLoginByUserID(userID int64) (string, error) {
	if login, ok := p.userLoginsByID[userID]; ok {
		return login, nil
	}
	if p.userService == nil {
		return "", githubpkg.ErrNilService
	}

	user, err := ghusers.GetUserByID(p.ctx, ghusers.GetUserByIDOptions{
		Service: p.userService,
		ID:      userID,
	})
	if err != nil {
		return "", fmt.Errorf("resolve invite user_id %d: %w", userID, err)
	}
	login := strings.TrimSpace(derefString(user.Login))
	if login == "" {
		return "", fmt.Errorf("resolve invite user_id %d: missing login: %w", userID, githubpkg.ErrInvalidFieldValue)
	}
	p.userLoginsByID[userID] = login
	return login, nil
}
