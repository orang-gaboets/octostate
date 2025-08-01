package builder

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v55/github"
)

// CreateRepo creates a repository from a template and optionally sets topics.
func CreateRepo(ctx context.Context, opts RepoCreationOptions) (*github.Repository, error) {
	if opts.Service == nil {
		return nil, fmt.Errorf("repo service is nil")
	}

	req := &github.TemplateRepoRequest{
		Owner:       github.String(opts.Org),
		Name:        github.String(opts.Name),
		Description: github.String(opts.Description),
		Private:     github.Bool(opts.Private),
	}

	log.Printf("Creating repository %s/%s from template %s", opts.TemplateOrg, opts.Name, opts.TemplateName)

	newRepo, _, err := opts.Service.CreateFromTemplate(ctx, opts.TemplateOrg, opts.TemplateName, req)
	if err != nil {
		return nil, fmt.Errorf("create from template: %w", err)
	}

	newRepoURL := newRepo.GetHTMLURL()
	if newRepoURL == "" {
		newRepoURL = "https://github.com/" + opts.Org + "/" + opts.Name
	}
	log.Printf("Repository created: %s", newRepoURL)

	if len(opts.Topics) > 0 {
		// Clean empty topics resulting from consecutive commas
		cleaned := make([]string, 0, len(opts.Topics))
		for _, t := range opts.Topics {
			if v := strings.TrimSpace(t); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		if len(cleaned) > 0 {
			log.Printf("Setting topics for repository %s/%s: %v", opts.Org, opts.Name, cleaned)
			_, _, err = opts.Service.ReplaceAllTopics(ctx, opts.Org, opts.Name, cleaned)
			if err != nil {
				return nil, fmt.Errorf("replace topics: %w", err)
			}
		}
	}
	return newRepo, nil
}
