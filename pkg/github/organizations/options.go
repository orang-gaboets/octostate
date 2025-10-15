package organizations

import (
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// GetOptions defines the options for retrieving organization details.
type GetOptions struct {
	Service Service
	OrgName string
}

// Validate checks if the GetOptions are valid.
func (opt *GetOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}

// InviteUserOptions defines the options for inviting a user to an organization.
type InviteUserOptions struct {
	Service Service
	OrgName string
	UserID  int64
}

// Validate checks if the InviteUserOptions are valid.
func (opt *InviteUserOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	if opt.UserID <= 0 {
		return fmt.Errorf("invalid user ID: %d", opt.UserID)
	}
	return nil
}
