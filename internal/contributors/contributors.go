// Package contributors renders the README contributor showcase from GitHub
// contributor data.
//
// The showcase recognizes contributors; it does not rank them. Ordering is
// alphabetical by login, and no contribution-volume signal - commits, lines,
// pull requests, issues, or tenure - influences selection or order. The
// Contributions field exists only because the GitHub API returns it, and is
// deliberately unused.
package contributors

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

const (
	startMarker = "<!-- contributors:start -->"
	endMarker   = "<!-- contributors:end -->"

	// avatarSize keeps every avatar identically sized in rendered Markdown.
	avatarSize = 100
)

// Contributor is one entry in the showcase. Only public identity is carried:
// the login is enough to derive both the profile and avatar URLs, so no
// contributor email or other non-public metadata is ever read or written.
type Contributor struct {
	Login string `yaml:"login"`
	Name  string `yaml:"name,omitempty"`

	// Type distinguishes a GitHub user from a bot account.
	Type string `yaml:"-"`
	// Contributions is reported by the GitHub API and intentionally ignored.
	Contributions int `yaml:"-"`
}

// Config is the maintainer escape hatch for attribution that automatic
// discovery gets wrong, read from .github/contributors.yml.
type Config struct {
	// Exclude drops a login from the showcase, matched case-insensitively.
	Exclude []string `yaml:"exclude"`
	// Include adds a contributor discovery misses, such as someone whose
	// contribution did not land as a commit authored by their account.
	Include []Contributor `yaml:"include"`
}

// DisplayName is the accessible label for a contributor.
func (c Contributor) DisplayName() string {
	if name := strings.TrimSpace(c.Name); name != "" {
		return name
	}
	return c.Login
}

// ProfileURL is the contributor's GitHub profile.
func (c Contributor) ProfileURL() string {
	return "https://github.com/" + c.Login
}

// AvatarURL is the contributor's GitHub avatar at the showcase size. Deriving
// it from the login rather than storing the API's avatar URL keeps rendered
// output stable across API responses.
func (c Contributor) AvatarURL() string {
	return fmt.Sprintf("https://github.com/%s.png?size=%d", c.Login, avatarSize)
}

// IsBot reports whether the account is a bot or service account. GitHub marks
// most of them with an account type, but a login suffix is also checked because
// some app accounts are reported as users.
func (c Contributor) IsBot() bool {
	return strings.EqualFold(c.Type, "Bot") || strings.HasSuffix(strings.ToLower(c.Login), "[bot]")
}

// Select turns discovered contributors and maintainer overrides into the final
// showcase list: bots and excluded logins are dropped, explicit includes are
// merged in, and the result is sorted alphabetically.
func Select(discovered []Contributor, cfg Config) []Contributor {
	excluded := make(map[string]struct{}, len(cfg.Exclude))
	for _, login := range cfg.Exclude {
		if key := key(login); key != "" {
			excluded[key] = struct{}{}
		}
	}

	byLogin := make(map[string]Contributor, len(discovered)+len(cfg.Include))
	add := func(c Contributor) {
		c.Login = strings.TrimSpace(c.Login)
		if c.Login == "" {
			return
		}
		if _, skip := excluded[key(c.Login)]; skip {
			return
		}
		byLogin[key(c.Login)] = c
	}

	for _, c := range discovered {
		if c.IsBot() {
			continue
		}
		add(c)
	}
	// Applied second so an explicit declaration wins over discovered data, and
	// so an include can restore attribution discovery reported incorrectly.
	for _, c := range cfg.Include {
		add(c)
	}

	selected := make([]Contributor, 0, len(byLogin))
	for _, c := range byLogin {
		selected = append(selected, c)
	}
	sort.Slice(selected, func(i, j int) bool {
		if left, right := key(selected[i].Login), key(selected[j].Login); left != right {
			return left < right
		}
		return selected[i].Login < selected[j].Login
	})
	return selected
}

// Render builds the contributor markup placed between the README markers.
func Render(contributors []Contributor) string {
	if len(contributors) == 0 {
		return "_No contributors recorded yet._"
	}

	var b strings.Builder
	b.WriteString("<p>\n")
	for _, c := range contributors {
		label := html.EscapeString(c.DisplayName())
		fmt.Fprintf(
			&b,
			"  <a href=%q title=%q><img src=%q width=%q height=%q alt=%q /></a>\n",
			html.EscapeString(c.ProfileURL()),
			label,
			html.EscapeString(c.AvatarURL()),
			fmt.Sprint(avatarSize),
			fmt.Sprint(avatarSize),
			label,
		)
	}
	b.WriteString("</p>")
	return b.String()
}

// Apply replaces the marked region of readme with block, leaving everything
// outside the markers untouched. Applying the same block twice is a no-op,
// which is what makes an unchanged repository produce no diff.
func Apply(readme, block string) (string, error) {
	start := strings.Index(readme, startMarker)
	if start < 0 {
		return "", fmt.Errorf("contributor start marker %s not found", startMarker)
	}
	if strings.Contains(readme[start+len(startMarker):], startMarker) {
		return "", fmt.Errorf("contributor start marker %s appears more than once", startMarker)
	}

	end := strings.Index(readme, endMarker)
	if end < 0 {
		return "", fmt.Errorf("contributor end marker %s not found", endMarker)
	}
	if strings.Contains(readme[end+len(endMarker):], endMarker) {
		return "", fmt.Errorf("contributor end marker %s appears more than once", endMarker)
	}
	if end < start {
		return "", fmt.Errorf("contributor end marker %s appears before the start marker", endMarker)
	}

	return readme[:start] + startMarker + "\n" + block + "\n" + readme[end:], nil
}

func key(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}
