package organizations

import (
	"context"
	"fmt"
	"log"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// Get retrieves an organization by its name.
func Get(ctx context.Context, opts GetOptions) (*github.Organization, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	if opts.OrgName == "" {
		return nil, github.ErrMissingRequiredField
	}

	log.Printf("Retrieving organization: %s", opts.OrgName)
	ghOrg, _, err := opts.Service.Get(ctx, opts.OrgName)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get organization %s", opts.OrgName))
	}
	if ghOrg == nil {
		return nil, fmt.Errorf("organization %s not found", opts.OrgName)
	}
	org := github.OrganizationFromGhOrg(ghOrg)
	log.Printf("Organization %s retrieved successfully: %s", opts.OrgName, *org)
	return org, nil
}
