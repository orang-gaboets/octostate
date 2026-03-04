package config

// OrganizationConfig contains the normalized desired-state configuration loaded
// from organization.yaml.
type OrganizationConfig struct {
	Organization string           `yaml:"organization"`
	Invites      []InviteSpec     `yaml:"invites"`
	Repositories []RepositorySpec `yaml:"repositories"`
	Teams        []TeamSpec       `yaml:"teams"`
}

// InviteSpec describes an organization invite desired in organization.yaml.
type InviteSpec struct {
	Username  string   `yaml:"username"`
	Email     string   `yaml:"email"`
	UserID    int64    `yaml:"user_id"`
	Role      string   `yaml:"role"`
	TeamSlugs []string `yaml:"team_slugs"`
}

// RepositorySpec describes a repository desired in organization.yaml.
type RepositorySpec struct {
	Owner        string       `yaml:"owner"`
	Name         string       `yaml:"name"`
	Template     TemplateSpec `yaml:"template"`
	Visibility   string       `yaml:"visibility"`
	Description  string       `yaml:"description"`
	Homepage     string       `yaml:"homepage"`
	Topics       []string     `yaml:"topics"`
	AllowForking bool         `yaml:"allow_forking"`
	Archived     bool         `yaml:"archived"`
	IsTemplate   bool         `yaml:"is_template"`
}

// TemplateSpec identifies the template repository used to create a repository.
type TemplateSpec struct {
	Owner              string `yaml:"owner"`
	Name               string `yaml:"name"`
	IncludeAllBranches bool   `yaml:"include_all_branches"`
}

// TeamSpec describes a team desired in organization.yaml.
type TeamSpec struct {
	Slug         string               `yaml:"slug"`
	Name         string               `yaml:"name"`
	Description  string               `yaml:"description"`
	Privacy      string               `yaml:"privacy"`
	ParentSlug   string               `yaml:"parent_slug"`
	Members      []TeamMemberSpec     `yaml:"members"`
	Repositories []TeamRepositorySpec `yaml:"repositories"`
}

// TeamMemberSpec describes a desired membership on a team.
type TeamMemberSpec struct {
	Username string `yaml:"username"`
	Role     string `yaml:"role"`
}

// TeamRepositorySpec describes a desired team permission on a repository.
type TeamRepositorySpec struct {
	Owner      string `yaml:"owner"`
	Name       string `yaml:"name"`
	Permission string `yaml:"permission"`
}
