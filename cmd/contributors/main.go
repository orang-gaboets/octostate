// Command contributors regenerates the README contributor showcase from the
// repository's GitHub contributor list.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/internal/contributors"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if err := execute(context.Background(), args, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func execute(ctx context.Context, args []string, stdout io.Writer) error {
	var repository, readmePath, configPath string

	fs := flag.NewFlagSet("contributors", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&repository, "repository", "orang-gaboets/octostate", "owner/name of the repository to read contributors from")
	fs.StringVar(&readmePath, "readme", "README.md", "path to the README containing the contributor markers")
	fs.StringVar(&configPath, "config", ".github/contributors.yml", "path to the contributor override file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	owner, name, err := splitRepository(repository)
	if err != nil {
		return err
	}

	cfg, err := contributors.LoadConfig(configPath)
	if err != nil {
		return err
	}

	discovered, err := fetch(ctx, owner, name)
	if err != nil {
		return err
	}

	changed, err := contributors.Update(readmePath, discovered, cfg)
	if err != nil {
		return err
	}

	if changed {
		_, _ = fmt.Fprintf(stdout, "updated contributor showcase in %s\n", readmePath)
		return nil
	}
	_, _ = fmt.Fprintf(stdout, "contributor showcase in %s is already up to date\n", readmePath)
	return nil
}

// fetch reads the repository's contributor list. GITHUB_TOKEN is optional and
// only raises the rate limit; the endpoint needs no privileged scope because
// contributor logins are public data.
func fetch(ctx context.Context, owner, name string) ([]contributors.Contributor, error) {
	var opts []gh.ClientOptionsFunc
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		opts = append(opts, gh.WithAuthToken(token))
	}
	client, err := gh.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("construct GitHub client: %w", err)
	}

	var discovered []contributors.Contributor
	listOpts := &gh.ListContributorsOptions{ListOptions: gh.ListOptions{PerPage: 100}}
	for {
		page, response, err := client.Repositories.ListContributors(ctx, owner, name, listOpts)
		if err != nil {
			return nil, fmt.Errorf("list contributors for %s/%s: %w", owner, name, err)
		}
		for _, c := range page {
			discovered = append(discovered, contributors.Contributor{
				Login: c.GetLogin(),
				Type:  c.GetType(),
			})
		}
		if response == nil || response.NextPage == 0 {
			return discovered, nil
		}
		listOpts.Page = response.NextPage
	}
}

func splitRepository(repository string) (string, string, error) {
	// Split rather than Cut so a trailing path segment is rejected instead of
	// being absorbed into the repository name.
	parts := strings.Split(strings.TrimSpace(repository), "/")
	if len(parts) != 2 {
		return "", "", errors.New("repository must be in owner/name form")
	}
	owner, name := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", errors.New("repository must be in owner/name form")
	}
	return owner, name, nil
}
