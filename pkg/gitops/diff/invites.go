package diff

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
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

func (b builder) appendInviteActions(actions []Action) ([]Action, error) {
	for _, invite := range b.desired.Invites {
		identity, err := b.desiredInviteIdentity(invite)
		if err != nil {
			return nil, err
		}
		satisfied, err := b.inviteIdentitySatisfied(identity)
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

	for _, invitation := range b.actual.PendingInvitations {
		matched, err := b.pendingInvitationDeclared(invitation)
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
			Message:      fmt.Sprintf("pending invitation %s exists in snapshot state but is not declared in desired config", pendingInvitationID(invitation)),
		})
	}

	return actions, nil
}

func (b builder) desiredInviteIdentity(invite config.InviteSpec) (inviteIdentity, error) {
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

func (b builder) inviteIdentitySatisfied(identity inviteIdentity) (bool, error) {
	for _, member := range b.actual.Members {
		matched, err := b.memberMatchesIdentity(member, identity)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	for _, invitation := range b.actual.PendingInvitations {
		matched, err := b.pendingInvitationMatchesIdentity(invitation, identity)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}

	return false, nil
}

func (b builder) pendingInvitationDeclared(invitation state.PendingInvitation) (bool, error) {
	for _, desiredInvite := range b.desired.Invites {
		identity, err := b.desiredInviteIdentity(desiredInvite)
		if err != nil {
			return false, err
		}
		matched, err := b.pendingInvitationMatchesIdentity(invitation, identity)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func (b builder) memberMatchesIdentity(member state.OrganizationMember, identity inviteIdentity) (bool, error) {
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

// Offline snapshots do not carry invitee user IDs, so user_id invites can
// only be matched to pending invitations when the caller provides a resolved
// login mapping.
func (b builder) pendingInvitationMatchesIdentity(invitation state.PendingInvitation, identity inviteIdentity) (bool, error) {
	switch identity.kind {
	case inviteIdentityKindUsername:
		return invitation.Username != "" && strings.EqualFold(invitation.Username, identity.username), nil
	case inviteIdentityKindEmail:
		return invitation.Email != "" && strings.EqualFold(invitation.Email, identity.email), nil
	case inviteIdentityKindUserID:
		if invitation.Username == "" {
			return false, nil
		}
		login, err := b.lookupLoginByUserID(identity.userID)
		if err != nil {
			return false, err
		}
		return strings.EqualFold(invitation.Username, login), nil
	default:
		return false, nil
	}
}

func (b builder) lookupLoginByUserID(userID int64) (string, error) {
	login, ok := b.resolvedInviteLoginsByUserID[userID]
	if !ok || strings.TrimSpace(login) == "" {
		return "", fmt.Errorf(
			"resolved login for invite user_id %d is required for offline diff: %w",
			userID,
			githubpkg.ErrMissingRequiredField,
		)
	}
	return strings.TrimSpace(login), nil
}
