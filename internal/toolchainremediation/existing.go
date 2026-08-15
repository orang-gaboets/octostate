package toolchainremediation

import (
	"fmt"
	"sort"
	"strings"
)

const remediationPRMarker = "<!-- octostate-go-toolchain-remediation:v1 -->"

const remediationBranchPrefix = "ci/go-toolchain-"

// ExistingPR contains the fields used to decide whether remediation work is
// already open or conflicts with the candidate being proposed.
type ExistingPR struct {
	URL            string `json:"url"`
	Body           string `json:"body"`
	HeadRefName    string `json:"headRefName"`
	HeadRepository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"headRepository"`
	BaseRefName string `json:"baseRefName"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

// ExistingWorkResult reports whether the workflow may continue and, when it
// must stop for an exact duplicate, identifies that PR.
type ExistingWorkResult struct {
	Proceed      bool   `json:"proceed"`
	DuplicateURL string `json:"duplicate_url,omitempty"`
}

// CheckExistingWork applies the remediation PR identity and marker rules.
// Any matching bot PR that is not the exact candidate is a conflict, so the
// caller can fail closed rather than risk creating competing automation work.
func CheckExistingWork(prs []ExistingPR, repository, expectedBot, expectedBranch, currentVersion, targetVersion string) (ExistingWorkResult, error) {
	if repository == "" || expectedBot == "" || expectedBranch == "" || currentVersion == "" || targetVersion == "" {
		return ExistingWorkResult{}, fmt.Errorf("repository, bot, branch, current version, and target version are required")
	}

	currentMarker := fmt.Sprintf("<!-- octostate-go-toolchain-remediation-current:%s -->", currentVersion)
	targetMarker := fmt.Sprintf("<!-- octostate-go-toolchain-remediation-target:%s -->", targetVersion)
	var duplicates []string
	var conflicts []string

	for _, pr := range prs {
		if pr.BaseRefName != "main" ||
			pr.HeadRepository.NameWithOwner != repository ||
			pr.Author.Login != expectedBot {
			continue
		}
		marked := strings.Contains(pr.Body, remediationPRMarker)
		remediationBranch := strings.HasPrefix(pr.HeadRefName, remediationBranchPrefix)
		if !marked && !remediationBranch {
			continue
		}

		if marked && pr.HeadRefName == expectedBranch &&
			strings.Contains(pr.Body, currentMarker) &&
			strings.Contains(pr.Body, targetMarker) {
			duplicates = append(duplicates, pr.URL)
			continue
		}
		conflicts = append(conflicts, pr.URL)
	}

	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return ExistingWorkResult{}, fmt.Errorf("a different open bot-generated remediation PR requires maintainer intervention: %s", strings.Join(conflicts, ", "))
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return ExistingWorkResult{DuplicateURL: duplicates[0]}, nil
	}
	return ExistingWorkResult{Proceed: true}, nil
}
