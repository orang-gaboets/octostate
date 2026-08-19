package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// EncodeYAML renders one canonical organization.yaml document from desired
// configuration.
func EncodeYAML(cfg OrganizationConfig) ([]byte, error) {
	normalized := NormalizeDesiredState(cfg)

	document := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			encodeOrganizationConfig(normalized),
		},
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode organization config YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, fmt.Errorf("close organization config YAML encoder: %w", err)
	}

	return buf.Bytes(), nil
}

func encodeOrganizationConfig(cfg OrganizationConfig) *yaml.Node {
	root := mapNode()
	appendMapField(root, "organization", stringNode(strings.TrimSpace(cfg.Organization)))
	appendMapField(root, "members", encodeMembers(cfg.Members))
	appendMapField(root, "invites", encodeInvites(cfg.Invites))
	appendMapField(root, "repositories", encodeRepositories(cfg.Repositories, cfg.Organization))
	appendMapField(root, "teams", encodeTeams(cfg.Teams, cfg.Organization))
	return root
}

func encodeMembers(members []OrganizationMemberSpec) *yaml.Node {
	items := make([]*yaml.Node, 0, len(members))
	for _, member := range members {
		item := mapNode()
		appendMapField(item, "username", stringNode(strings.TrimSpace(member.Username)))
		appendMapField(item, "role", stringNode(strings.TrimSpace(member.Role)))
		items = append(items, item)
	}
	return sequenceNode(items)
}

func encodeInvites(invites []InviteSpec) *yaml.Node {
	items := make([]*yaml.Node, 0, len(invites))
	for _, invite := range invites {
		item := mapNode()
		appendOptionalStringField(item, "username", invite.Username)
		appendOptionalStringField(item, "email", invite.Email)
		appendOptionalInt64Field(item, "user_id", invite.UserID)
		appendMapField(item, "role", stringNode(strings.TrimSpace(invite.Role)))
		if len(invite.TeamSlugs) > 0 {
			appendMapField(item, "team_slugs", stringSequenceNode(invite.TeamSlugs))
		}
		items = append(items, item)
	}
	return sequenceNode(items)
}

func encodeRepositories(repositories []RepositorySpec, organization string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(repositories))
	for _, repo := range repositories {
		item := mapNode()
		if repoOwner := strings.TrimSpace(repo.Owner); repoOwner != "" && !strings.EqualFold(repoOwner, strings.TrimSpace(organization)) {
			appendMapField(item, "owner", stringNode(repoOwner))
		}
		appendMapField(item, "name", stringNode(strings.TrimSpace(repo.Name)))

		template := encodeTemplate(repo.Template)
		if len(template.Content) > 0 {
			appendMapField(item, "template", template)
		}

		appendMapField(item, "visibility", stringNode(strings.TrimSpace(repo.Visibility)))
		appendManagedRepositoryStringField(item, "description", repo.DescriptionOption(), repo.Description)
		appendManagedRepositoryStringField(item, "homepage", repo.HomepageOption(), repo.Homepage)
		if len(repo.Topics) > 0 {
			appendMapField(item, "topics", stringSequenceNode(repo.Topics))
		}
		appendManagedRepositoryBoolField(item, "allow_forking", repo.AllowForkingOption(), repo.AllowForking)
		appendManagedRepositoryBoolField(item, "archived", repo.ArchivedOption(), repo.Archived)
		appendManagedRepositoryBoolField(item, "is_template", repo.IsTemplateOption(), repo.IsTemplate)

		items = append(items, item)
	}
	return sequenceNode(items)
}

func encodeTemplate(template TemplateSpec) *yaml.Node {
	item := mapNode()
	if owner := strings.TrimSpace(template.Owner); owner != "" {
		appendMapField(item, "owner", stringNode(owner))
	}
	if name := strings.TrimSpace(template.Name); name != "" {
		appendMapField(item, "name", stringNode(name))
	}
	if template.IncludeAllBranches {
		appendMapField(item, "include_all_branches", boolNode(template.IncludeAllBranches))
	}
	return item
}

func encodeTeams(teams []TeamSpec, organization string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(teams))
	for _, team := range teams {
		item := mapNode()
		appendMapField(item, "slug", stringNode(strings.TrimSpace(team.Slug)))
		appendMapField(item, "name", stringNode(strings.TrimSpace(team.Name)))
		if description := strings.TrimSpace(team.Description); description != "" {
			appendMapField(item, "description", stringNode(description))
		}
		appendMapField(item, "privacy", stringNode(strings.TrimSpace(team.Privacy)))
		if parentSlug := strings.TrimSpace(team.ParentSlug); parentSlug != "" {
			appendMapField(item, "parent_slug", stringNode(parentSlug))
		}
		if len(team.Members) > 0 {
			appendMapField(item, "members", encodeTeamMembers(team.Members))
		}
		if len(team.Repositories) > 0 {
			appendMapField(item, "repositories", encodeTeamRepositories(team.Repositories, organization))
		}
		items = append(items, item)
	}
	return sequenceNode(items)
}

func encodeTeamMembers(members []TeamMemberSpec) *yaml.Node {
	items := make([]*yaml.Node, 0, len(members))
	for _, member := range members {
		item := mapNode()
		appendMapField(item, "username", stringNode(strings.TrimSpace(member.Username)))
		appendMapField(item, "role", stringNode(strings.TrimSpace(member.Role)))
		items = append(items, item)
	}
	return sequenceNode(items)
}

func encodeTeamRepositories(repositories []TeamRepositorySpec, organization string) *yaml.Node {
	items := make([]*yaml.Node, 0, len(repositories))
	for _, repo := range repositories {
		item := mapNode()
		if owner := strings.TrimSpace(repo.Owner); owner != "" && !strings.EqualFold(owner, strings.TrimSpace(organization)) {
			appendMapField(item, "owner", stringNode(owner))
		}
		appendMapField(item, "name", stringNode(strings.TrimSpace(repo.Name)))
		appendMapField(item, "permission", stringNode(strings.TrimSpace(repo.Permission)))
		items = append(items, item)
	}
	return sequenceNode(items)
}
