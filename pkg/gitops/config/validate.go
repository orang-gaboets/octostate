package config

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"
)

var (
	validOrganizationMemberRoles = map[string]struct{}{
		"admin":  {},
		"member": {},
	}
	validInviteRoles = map[string]struct{}{
		"admin":           {},
		"direct_member":   {},
		"billing_manager": {},
	}
	validRepositoryVisibility = map[string]struct{}{
		"public":  {},
		"private": {},
	}
	validTeamPrivacy = map[string]struct{}{
		"closed": {},
		"secret": {},
	}
	validTeamMemberRoles = map[string]struct{}{
		"member":     {},
		"maintainer": {},
	}
	validTeamRepositoryPermissions = map[string]struct{}{
		"pull":     {},
		"triage":   {},
		"push":     {},
		"maintain": {},
		"admin":    {},
	}
)

// Validate performs semantic validation on a loaded organization config.
func Validate(cfg OrganizationConfig) ValidationReport {
	report := ValidationReport{
		Summary: ValidationSummary{
			Repositories: len(cfg.Repositories),
			Members:      len(cfg.Members),
			Teams:        len(cfg.Teams),
			Invites:      len(cfg.Invites),
		},
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}

	organization := strings.TrimSpace(cfg.Organization)
	if organization == "" {
		report.addError("organization", ValidationIssueCodeMissingRequiredField, "organization is required")
	}

	validateRepositories(&report, cfg.Repositories, organization)

	memberIndex := validateMembers(&report, cfg.Members)
	teamIndex := validateTeams(&report, cfg.Teams, organization, memberIndex)
	validateInvites(&report, cfg.Invites, teamIndex, memberIndex)
	validateTeamParentCycles(&report, cfg.Teams, teamIndex)

	report.Summary.Errors = len(report.Errors)
	report.Summary.Warnings = len(report.Warnings)
	report.Valid = len(report.Errors) == 0

	return report
}

func (r *ValidationReport) addError(path string, code ValidationIssueCode, format string, args ...any) {
	r.Errors = append(r.Errors, ValidationIssue{
		Path:    path,
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	})
}

func validateMembers(report *ValidationReport, members []OrganizationMemberSpec) map[string]int {
	memberIndex := make(map[string]int, len(members))

	for i, member := range members {
		pathPrefix := fmt.Sprintf("members[%d]", i)
		username := strings.TrimSpace(member.Username)
		role := strings.TrimSpace(member.Role)

		if username == "" {
			report.addError(pathPrefix+".username", ValidationIssueCodeMissingRequiredField, "organization member username is required")
		} else if !isValidGitHubUsername(username) {
			report.addError(pathPrefix+".username", ValidationIssueCodeInvalidFieldValue, "organization member username %q is not a valid GitHub username", username)
		}
		if role == "" {
			report.addError(pathPrefix+".role", ValidationIssueCodeMissingRequiredField, "organization member role is required")
		} else if !isAllowed(role, validOrganizationMemberRoles) {
			report.addError(pathPrefix+".role", ValidationIssueCodeInvalidEnum, "organization member role %q is not supported", role)
		}

		if username == "" {
			continue
		}
		usernameKey := strings.ToLower(username)
		if firstIndex, ok := memberIndex[usernameKey]; ok {
			report.addError(pathPrefix, ValidationIssueCodeDuplicateOrganizationMember, "organization member %q duplicates members[%d]", username, firstIndex)
			continue
		}
		memberIndex[usernameKey] = i
	}

	return memberIndex
}

func validateRepositories(report *ValidationReport, repositories []RepositorySpec, organization string) {
	seen := make(map[string]int, len(repositories))

	for i, repo := range repositories {
		pathPrefix := fmt.Sprintf("repositories[%d]", i)
		owner := strings.TrimSpace(repo.Owner)
		if owner == "" {
			owner = organization
		}
		name := strings.TrimSpace(repo.Name)
		if name == "" {
			report.addError(pathPrefix+".name", ValidationIssueCodeMissingRequiredField, "repository name is required")
		}

		visibility := strings.TrimSpace(repo.Visibility)
		if visibility == "" {
			report.addError(pathPrefix+".visibility", ValidationIssueCodeMissingRequiredField, "repository visibility is required")
		} else if !isAllowed(visibility, validRepositoryVisibility) {
			report.addError(pathPrefix+".visibility", ValidationIssueCodeInvalidEnum, "repository visibility %q is not supported", visibility)
		}

		templateOwner := strings.TrimSpace(repo.Template.Owner)
		templateName := strings.TrimSpace(repo.Template.Name)
		switch {
		case templateOwner == "" && templateName != "":
			report.addError(pathPrefix+".template.owner", ValidationIssueCodeMissingRequiredField, "template owner is required when template name is set")
		case templateOwner != "" && templateName == "":
			report.addError(pathPrefix+".template.name", ValidationIssueCodeMissingRequiredField, "template name is required when template owner is set")
		}
		if repo.DescriptionOption().Null {
			report.addError(pathPrefix+".description", ValidationIssueCodeInvalidFieldValue, "repository description must not be null when provided")
		}
		if repo.HomepageOption().Null {
			report.addError(pathPrefix+".homepage", ValidationIssueCodeInvalidFieldValue, "repository homepage must not be null when provided")
		}
		if repo.AllowForkingOption().Null {
			report.addError(pathPrefix+".allow_forking", ValidationIssueCodeInvalidFieldValue, "repository allow_forking must not be null when provided")
		}
		if repo.ArchivedOption().Null {
			report.addError(pathPrefix+".archived", ValidationIssueCodeInvalidFieldValue, "repository archived must not be null when provided")
		}
		if repo.IsTemplateOption().Null {
			report.addError(pathPrefix+".is_template", ValidationIssueCodeInvalidFieldValue, "repository is_template must not be null when provided")
		}

		validateRepositoryTopics(report, pathPrefix, repo.Topics)

		if owner == "" || name == "" {
			continue
		}
		key := strings.ToLower(owner) + "\x00" + strings.ToLower(name)
		if firstIndex, ok := seen[key]; ok {
			report.addError(pathPrefix, ValidationIssueCodeDuplicateRepository, "repository %s/%s duplicates repositories[%d]", owner, name, firstIndex)
			continue
		}
		seen[key] = i
	}
}

func validateRepositoryTopics(report *ValidationReport, pathPrefix string, topics []string) {
	normalizedTopics := make(map[string]struct{}, len(topics))

	for i, topic := range topics {
		normalized := strings.TrimSpace(topic)
		topicPath := fmt.Sprintf("%s.topics[%d]", pathPrefix, i)
		if normalized == "" {
			report.addError(topicPath, ValidationIssueCodeMissingRequiredField, "repository topic cannot be empty")
			continue
		}
		normalizedTopics[normalized] = struct{}{}
		if len(normalized) > 50 {
			report.addError(topicPath, ValidationIssueCodeInvalidRepositoryTopic, "repository topic %q must be at most 50 bytes", normalized)
			continue
		}

		valid := true
		for j := 0; j < len(normalized); j++ {
			char := normalized[j]
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				valid = false
				break
			}
		}
		if !valid {
			report.addError(topicPath, ValidationIssueCodeInvalidRepositoryTopic, "repository topic %q must contain only lowercase ASCII letters, digits, and hyphens", normalized)
			continue
		}
	}

	if len(normalizedTopics) > 20 {
		report.addError(pathPrefix+".topics", ValidationIssueCodeRepositoryTopicLimit, "repository topics must contain at most 20 distinct normalized topics")
	}
}

func validateTeams(report *ValidationReport, teams []TeamSpec, organization string, organizationMemberIndex map[string]int) map[string]int {
	teamIndex := make(map[string]int, len(teams))

	for i, team := range teams {
		pathPrefix := fmt.Sprintf("teams[%d]", i)
		slug := strings.TrimSpace(team.Slug)
		name := strings.TrimSpace(team.Name)

		if slug == "" {
			report.addError(pathPrefix+".slug", ValidationIssueCodeMissingRequiredField, "team slug is required")
		}
		if name == "" {
			report.addError(pathPrefix+".name", ValidationIssueCodeMissingRequiredField, "team name is required")
		}
		if privacy := strings.TrimSpace(team.Privacy); privacy != "" && !isAllowed(privacy, validTeamPrivacy) {
			report.addError(pathPrefix+".privacy", ValidationIssueCodeInvalidEnum, "team privacy %q is not supported", privacy)
		}
		if slug != "" && name != "" {
			normalizedName := normalizeTeamName(name)
			if slug != normalizedName {
				report.addError(pathPrefix+".slug", ValidationIssueCodeSlugNameMismatch, "team slug %q does not match normalized team name %q", slug, normalizedName)
			}
		}
		if slug != "" {
			slugKey := strings.ToLower(slug)
			if firstIndex, ok := teamIndex[slugKey]; ok {
				report.addError(pathPrefix+".slug", ValidationIssueCodeDuplicateTeamSlug, "team slug %q duplicates teams[%d]", slug, firstIndex)
			} else {
				teamIndex[slugKey] = i
			}
		}

		teamMemberIndex := make(map[string]int, len(team.Members))
		for j, member := range team.Members {
			memberPath := fmt.Sprintf("%s.members[%d]", pathPrefix, j)
			username := strings.TrimSpace(member.Username)
			if username == "" {
				report.addError(memberPath+".username", ValidationIssueCodeMissingRequiredField, "team member username is required")
			}
			if role := strings.TrimSpace(member.Role); role != "" && !isAllowed(role, validTeamMemberRoles) {
				report.addError(memberPath+".role", ValidationIssueCodeInvalidEnum, "team member role %q is not supported", role)
			}
			if username == "" {
				continue
			}
			if _, ok := organizationMemberIndex[strings.ToLower(username)]; !ok {
				report.addError(memberPath+".username", ValidationIssueCodeUnknownOrganizationMember, "team member %q must also be declared in top-level members", username)
			}
			usernameKey := strings.ToLower(username)
			if firstIndex, ok := teamMemberIndex[usernameKey]; ok {
				report.addError(memberPath, ValidationIssueCodeDuplicateTeamMember, "team member %q duplicates teams[%d].members[%d]", username, i, firstIndex)
				continue
			}
			teamMemberIndex[usernameKey] = j
		}

		repositoryIndex := make(map[string]int, len(team.Repositories))
		for j, repo := range team.Repositories {
			repoPath := fmt.Sprintf("%s.repositories[%d]", pathPrefix, j)
			owner := strings.TrimSpace(repo.Owner)
			if owner == "" {
				owner = organization
			}
			name := strings.TrimSpace(repo.Name)
			permission := strings.TrimSpace(repo.Permission)

			if name == "" {
				report.addError(repoPath+".name", ValidationIssueCodeMissingRequiredField, "team repository name is required")
			}
			if permission == "" {
				report.addError(repoPath+".permission", ValidationIssueCodeMissingRequiredField, "team repository permission is required")
			} else if !isAllowed(permission, validTeamRepositoryPermissions) {
				report.addError(repoPath+".permission", ValidationIssueCodeInvalidEnum, "team repository permission %q is not supported", permission)
			}
			if owner == "" || name == "" {
				continue
			}
			key := strings.ToLower(owner) + "\x00" + strings.ToLower(name)
			if firstIndex, ok := repositoryIndex[key]; ok {
				report.addError(repoPath, ValidationIssueCodeDuplicateTeamRepository, "team repository %s/%s duplicates teams[%d].repositories[%d]", owner, name, i, firstIndex)
				continue
			}
			repositoryIndex[key] = j
		}
	}

	for i, team := range teams {
		parentSlug := strings.TrimSpace(team.ParentSlug)
		if parentSlug == "" {
			continue
		}
		if _, ok := teamIndex[strings.ToLower(parentSlug)]; !ok {
			report.addError(fmt.Sprintf("teams[%d].parent_slug", i), ValidationIssueCodeUnknownTeamParentSlug, "parent team slug %q does not reference a declared team", parentSlug)
		}
	}

	return teamIndex
}

func validateInvites(report *ValidationReport, invites []InviteSpec, teamIndex map[string]int, memberIndex map[string]int) {
	for i, invite := range invites {
		pathPrefix := fmt.Sprintf("invites[%d]", i)
		usernameDeclared := invite.Username.Present
		emailDeclared := invite.Email.Present
		userIDDeclared := invite.UserID.Present
		username := invite.Username.Value
		email := invite.Email.Value
		userID := invite.UserID.Value

		identityCount := 0
		if usernameDeclared {
			identityCount++
		}
		if emailDeclared {
			identityCount++
		}
		if userIDDeclared {
			identityCount++
		}

		switch {
		case identityCount == 0:
			report.addError(pathPrefix, ValidationIssueCodeInvalidInviteIdentity, "invite must declare exactly one of username, email, or user_id")
		case identityCount > 1:
			report.addError(pathPrefix, ValidationIssueCodeInvalidInviteIdentity, "invite must not declare more than one of username, email, or user_id")
		}

		if usernameDeclared {
			switch {
			case invite.Username.Null:
				report.addError(pathPrefix+".username", ValidationIssueCodeInvalidInviteIdentity, "invite username must not be null")
			case username == "":
				report.addError(pathPrefix+".username", ValidationIssueCodeInvalidInviteIdentity, "invite username must not be empty when provided")
			case !isValidGitHubUsername(username):
				report.addError(pathPrefix+".username", ValidationIssueCodeInvalidInviteIdentity, "invite username %q is not a valid GitHub username", username)
			case hasOrganizationMember(memberIndex, username):
				report.addError(pathPrefix+".username", ValidationIssueCodeDuplicateOrganizationMemberInvite, "invite username %q duplicates a declared top-level member", username)
			}
		}
		if emailDeclared {
			switch {
			case invite.Email.Null:
				report.addError(pathPrefix+".email", ValidationIssueCodeInvalidInviteIdentity, "invite email must not be null")
			case email == "":
				report.addError(pathPrefix+".email", ValidationIssueCodeInvalidInviteIdentity, "invite email must not be empty when provided")
			case !isValidInviteEmail(email):
				report.addError(pathPrefix+".email", ValidationIssueCodeInvalidInviteIdentity, "invite email %q is not a valid email address", email)
			}
		}
		if userIDDeclared {
			switch {
			case invite.UserID.Null:
				report.addError(pathPrefix+".user_id", ValidationIssueCodeInvalidInviteIdentity, "invite user_id must not be null")
			case userID <= 0:
				report.addError(pathPrefix+".user_id", ValidationIssueCodeInvalidInviteIdentity, "invite user_id must be greater than zero when provided")
			}
		}

		if role := strings.TrimSpace(invite.Role); role != "" && !isAllowed(role, validInviteRoles) {
			report.addError(pathPrefix+".role", ValidationIssueCodeInvalidEnum, "invite role %q is not supported", role)
		}

		for j, teamSlug := range invite.TeamSlugs {
			trimmedTeamSlug := strings.TrimSpace(teamSlug)
			if trimmedTeamSlug == "" {
				report.addError(fmt.Sprintf("%s.team_slugs[%d]", pathPrefix, j), ValidationIssueCodeMissingRequiredField, "invite team slug must not be empty")
				continue
			}
			if _, ok := teamIndex[strings.ToLower(trimmedTeamSlug)]; !ok {
				report.addError(fmt.Sprintf("%s.team_slugs[%d]", pathPrefix, j), ValidationIssueCodeUnknownInviteTeamSlug, "invite references unknown team slug %q", trimmedTeamSlug)
			}
		}
	}
}

func hasOrganizationMember(memberIndex map[string]int, username string) bool {
	_, ok := memberIndex[strings.ToLower(strings.TrimSpace(username))]
	return ok
}

func validateTeamParentCycles(report *ValidationReport, teams []TeamSpec, teamIndex map[string]int) {
	parentBySlug := make(map[string]string, len(teams))
	for _, team := range teams {
		slug := strings.TrimSpace(team.Slug)
		parentSlug := strings.TrimSpace(team.ParentSlug)
		if slug == "" || parentSlug == "" {
			continue
		}
		if _, ok := teamIndex[strings.ToLower(parentSlug)]; !ok {
			continue
		}
		parentBySlug[strings.ToLower(slug)] = strings.ToLower(parentSlug)
	}

	state := make(map[string]int, len(parentBySlug))
	var visit func(string) bool
	visit = func(slug string) bool {
		switch state[slug] {
		case 1:
			return true
		case 2:
			return false
		}

		state[slug] = 1
		parentSlug, ok := parentBySlug[slug]
		if ok && visit(parentSlug) {
			return true
		}
		state[slug] = 2
		return false
	}

	for _, team := range teams {
		slug := strings.TrimSpace(team.Slug)
		if slug == "" {
			continue
		}
		if visit(strings.ToLower(slug)) {
			report.addError("teams", ValidationIssueCodeTeamParentCycle, "team parent cycle detected")
			return
		}
	}
}

func isAllowed(value string, allowed map[string]struct{}) bool {
	_, ok := allowed[value]
	return ok
}

func isValidGitHubUsername(username string) bool {
	if username == "" || len(username) > 39 {
		return false
	}
	if username[0] == '-' || username[len(username)-1] == '-' {
		return false
	}

	previousHyphen := false
	for _, r := range username {
		switch {
		case r >= 'a' && r <= 'z':
			previousHyphen = false
		case r >= 'A' && r <= 'Z':
			previousHyphen = false
		case r >= '0' && r <= '9':
			previousHyphen = false
		case r == '-':
			if previousHyphen {
				return false
			}
			previousHyphen = true
		default:
			return false
		}
	}

	return true
}

func isValidInviteEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return address.Address == email
}

// NormalizeTeamName derives the canonical team slug from a team name using
// the same normalization applied during semantic validation.
func NormalizeTeamName(name string) string {
	return normalizeTeamName(name)
}

func normalizeTeamName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))

	var b strings.Builder
	lastDash := true
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}
