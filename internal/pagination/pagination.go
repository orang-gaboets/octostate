// Package pagination keeps GitHub page loops from running forever on
// pagination metadata that does not advance.
package pagination

import (
	"fmt"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/pkg/github"
)

// Next returns the page to request after current, or 0 when resp reports no
// further page. GitHub pages start at 1 and a zero ListOptions.Page requests
// the first page, so a valid next page is always greater than the page just
// read. A repeated, regressing, or cyclic page number therefore fails here
// instead of being requested again.
//
// There is deliberately no maximum page count: a sequence that keeps
// advancing is real progress, and large organizations must still collect.
func Next(resp *gh.Response, current int) (int, error) {
	if resp == nil || resp.NextPage == 0 {
		return 0, nil
	}
	read := max(current, 1)
	if resp.NextPage <= read {
		return 0, fmt.Errorf("GitHub pagination did not advance: next page %d after page %d: %w", resp.NextPage, read, github.ErrValidationFailed)
	}
	return resp.NextPage, nil
}
