package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// OrganizationConfig is the desired-state configuration for one GitHub
// organization. LoadFile and LoadDir return it already normalized; a value
// built programmatically is not, and callers should canonicalize it with
// NormalizeDesiredState so it reconciles like its YAML equivalent.
type OrganizationConfig struct {
	Organization string                   `yaml:"organization"`
	Members      []OrganizationMemberSpec `yaml:"members"`
	Invites      []InviteSpec             `yaml:"invites"`
	Repositories []RepositorySpec         `yaml:"repositories"`
	Teams        []TeamSpec               `yaml:"teams"`
}

// OptionalString preserves whether a string field was declared and whether it
// was explicitly set to null in YAML.
type OptionalString struct {
	Present bool   `yaml:"-"`
	Null    bool   `yaml:"-"`
	Value   string `yaml:"-"`
}

// TrimSpace normalizes the stored string value without losing declaration
// metadata.
func (s *OptionalString) TrimSpace() {
	if !s.Present || s.Null {
		return
	}
	s.Value = strings.TrimSpace(s.Value)
}

// OptionalInt64 preserves whether an integer field was declared and whether it
// was explicitly set to null in YAML.
type OptionalInt64 struct {
	Present bool  `yaml:"-"`
	Null    bool  `yaml:"-"`
	Value   int64 `yaml:"-"`
}

// OptionalBool preserves whether a boolean field was declared and whether it
// was explicitly set to null in YAML.
type OptionalBool struct {
	Present bool `yaml:"-"`
	Null    bool `yaml:"-"`
	Value   bool `yaml:"-"`
}

// InviteSpec describes an organization invite desired in organization.yaml.
type InviteSpec struct {
	Username  OptionalString `yaml:"username"`
	Email     OptionalString `yaml:"email"`
	UserID    OptionalInt64  `yaml:"user_id"`
	Role      string         `yaml:"role"`
	TeamSlugs []string       `yaml:"team_slugs"`
}

// OrganizationMemberSpec describes a durable organization member declared in
// organization.yaml.
type OrganizationMemberSpec struct {
	Username string `yaml:"username"`
	Role     string `yaml:"role"`
}

// UnmarshalYAML preserves whether invite identity fields were omitted, null,
// or explicitly declared in YAML.
func (i *InviteSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("invite must be a YAML mapping")
	}

	*i = InviteSpec{}
	seen := make(map[string]struct{}, len(node.Content)/2)

	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		key := keyNode.Value

		if _, ok := seen[key]; ok {
			return fmt.Errorf("field %s already declared in type config.InviteSpec", key)
		}
		seen[key] = struct{}{}

		switch key {
		case "username":
			if err := decodeOptionalStringNode(valueNode, &i.Username); err != nil {
				return err
			}
		case "email":
			if err := decodeOptionalStringNode(valueNode, &i.Email); err != nil {
				return err
			}
		case "user_id":
			if err := decodeOptionalInt64Node(valueNode, &i.UserID); err != nil {
				return err
			}
		case "role":
			if err := valueNode.Decode(&i.Role); err != nil {
				return err
			}
		case "team_slugs":
			if err := valueNode.Decode(&i.TeamSlugs); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field %s not found in type config.InviteSpec", key)
		}
	}

	return nil
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

	description  OptionalString `yaml:"-"`
	homepage     OptionalString `yaml:"-"`
	allowForking OptionalBool   `yaml:"-"`
	archived     OptionalBool   `yaml:"-"`
	isTemplate   OptionalBool   `yaml:"-"`
}

// IsPrivateVisibility reports whether visibility makes private-repository
// settings applicable.
func IsPrivateVisibility(visibility string) bool {
	return strings.EqualFold(strings.TrimSpace(visibility), "private")
}

// SupportsAllowForking reports whether repository forking settings apply to a visibility.
func SupportsAllowForking(visibility string) bool {
	visibility = strings.TrimSpace(visibility)
	return strings.EqualFold(visibility, "private") || strings.EqualFold(visibility, "internal")
}

// TemplateSpec identifies the template repository used to create a repository.
type TemplateSpec struct {
	Owner              string `yaml:"owner"`
	Name               string `yaml:"name"`
	IncludeAllBranches bool   `yaml:"include_all_branches"`
}

// UnmarshalYAML strictly decodes template fields and rejects duplicates.
func (t *TemplateSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("template must be a YAML mapping")
	}

	*t = TemplateSpec{}
	seen := make(map[string]struct{}, len(node.Content)/2)

	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		key := keyNode.Value

		if _, ok := seen[key]; ok {
			return fmt.Errorf("field %s already declared in type config.TemplateSpec", key)
		}
		seen[key] = struct{}{}

		switch key {
		case "owner":
			if err := valueNode.Decode(&t.Owner); err != nil {
				return err
			}
		case "name":
			if err := valueNode.Decode(&t.Name); err != nil {
				return err
			}
		case "include_all_branches":
			if err := valueNode.Decode(&t.IncludeAllBranches); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field %s not found in type config.TemplateSpec", key)
		}
	}

	return nil
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

// DescriptionOption returns the repository description declaration metadata.
func (r RepositorySpec) DescriptionOption() OptionalString {
	return r.description
}

// SetManagedDescription marks description as explicitly managed in
// programmatic config construction.
func (r *RepositorySpec) SetManagedDescription(value string) {
	r.Description = value
	r.description = OptionalString{
		Present: true,
		Value:   value,
	}
}

// ManagedDescription returns the description value when it should be
// reconciled, along with whether the field is managed at all.
func (r RepositorySpec) ManagedDescription() (string, bool) {
	if r.description.Present {
		if r.description.Null {
			return "", false
		}
		return r.Description, true
	}
	if strings.TrimSpace(r.Description) != "" {
		return r.Description, true
	}
	return "", false
}

// HomepageOption returns the repository homepage declaration metadata.
func (r RepositorySpec) HomepageOption() OptionalString {
	return r.homepage
}

// SetManagedHomepage marks homepage as explicitly managed in programmatic
// config construction.
func (r *RepositorySpec) SetManagedHomepage(value string) {
	r.Homepage = value
	r.homepage = OptionalString{
		Present: true,
		Value:   value,
	}
}

// ManagedHomepage returns the homepage value when it should be reconciled,
// along with whether the field is managed at all.
func (r RepositorySpec) ManagedHomepage() (string, bool) {
	if r.homepage.Present {
		if r.homepage.Null {
			return "", false
		}
		return r.Homepage, true
	}
	if strings.TrimSpace(r.Homepage) != "" {
		return r.Homepage, true
	}
	return "", false
}

// AllowForkingOption returns the repository allow_forking declaration metadata.
func (r RepositorySpec) AllowForkingOption() OptionalBool {
	return r.allowForking
}

// SetManagedAllowForking marks allow_forking as explicitly managed in
// programmatic config construction.
func (r *RepositorySpec) SetManagedAllowForking(value bool) {
	r.AllowForking = value
	r.allowForking = OptionalBool{
		Present: true,
		Value:   value,
	}
}

// ManagedAllowForking returns the allow_forking value when it should be
// reconciled, along with whether the field is managed at all.
func (r RepositorySpec) ManagedAllowForking() (bool, bool) {
	if r.allowForking.Present {
		if r.allowForking.Null {
			return false, false
		}
		return r.AllowForking, true
	}
	if r.AllowForking {
		return true, true
	}
	return false, false
}

// ArchivedOption returns the repository archived declaration metadata.
func (r RepositorySpec) ArchivedOption() OptionalBool {
	return r.archived
}

// SetManagedArchived marks archived as explicitly managed in programmatic
// config construction.
func (r *RepositorySpec) SetManagedArchived(value bool) {
	r.Archived = value
	r.archived = OptionalBool{
		Present: true,
		Value:   value,
	}
}

// ManagedArchived returns the archived value when it should be reconciled,
// along with whether the field is managed at all.
func (r RepositorySpec) ManagedArchived() (bool, bool) {
	if r.archived.Present {
		if r.archived.Null {
			return false, false
		}
		return r.Archived, true
	}
	if r.Archived {
		return true, true
	}
	return false, false
}

// IsTemplateOption returns the repository is_template declaration metadata.
func (r RepositorySpec) IsTemplateOption() OptionalBool {
	return r.isTemplate
}

// SetManagedIsTemplate marks is_template as explicitly managed in programmatic
// config construction.
func (r *RepositorySpec) SetManagedIsTemplate(value bool) {
	r.IsTemplate = value
	r.isTemplate = OptionalBool{
		Present: true,
		Value:   value,
	}
}

// ManagedIsTemplate returns the is_template value when it should be
// reconciled, along with whether the field is managed at all.
func (r RepositorySpec) ManagedIsTemplate() (bool, bool) {
	if r.isTemplate.Present {
		if r.isTemplate.Null {
			return false, false
		}
		return r.IsTemplate, true
	}
	if r.IsTemplate {
		return true, true
	}
	return false, false
}

func isYAMLNull(node *yaml.Node) bool {
	return node != nil && node.Tag == "!!null"
}

func decodeOptionalStringNode(node *yaml.Node, out *OptionalString) error {
	out.Present = true
	if isYAMLNull(node) {
		out.Null = true
		out.Value = ""
		return nil
	}

	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}

	out.Null = false
	out.Value = value
	return nil
}

func decodeOptionalInt64Node(node *yaml.Node, out *OptionalInt64) error {
	out.Present = true
	if isYAMLNull(node) {
		out.Null = true
		out.Value = 0
		return nil
	}

	var value int64
	if err := node.Decode(&value); err != nil {
		return err
	}

	out.Null = false
	out.Value = value
	return nil
}

func decodeOptionalBoolNode(node *yaml.Node, out *OptionalBool) error {
	out.Present = true
	if isYAMLNull(node) {
		out.Null = true
		out.Value = false
		return nil
	}

	var value bool
	if err := node.Decode(&value); err != nil {
		return err
	}

	out.Null = false
	out.Value = value
	return nil
}

// UnmarshalYAML preserves whether repository optional fields were omitted,
// null, or explicitly declared in YAML.
func (r *RepositorySpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("repository must be a YAML mapping")
	}

	*r = RepositorySpec{}
	seen := make(map[string]struct{}, len(node.Content)/2)

	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		key := keyNode.Value

		if _, ok := seen[key]; ok {
			return fmt.Errorf("field %s already declared in type config.RepositorySpec", key)
		}
		seen[key] = struct{}{}

		switch key {
		case "owner":
			if err := valueNode.Decode(&r.Owner); err != nil {
				return err
			}
		case "name":
			if err := valueNode.Decode(&r.Name); err != nil {
				return err
			}
		case "template":
			if err := valueNode.Decode(&r.Template); err != nil {
				return err
			}
		case "visibility":
			if err := valueNode.Decode(&r.Visibility); err != nil {
				return err
			}
		case "description":
			if err := decodeOptionalStringNode(valueNode, &r.description); err != nil {
				return err
			}
			if !r.description.Null {
				r.Description = r.description.Value
			}
		case "homepage":
			if err := decodeOptionalStringNode(valueNode, &r.homepage); err != nil {
				return err
			}
			if !r.homepage.Null {
				r.Homepage = r.homepage.Value
			}
		case "topics":
			if err := valueNode.Decode(&r.Topics); err != nil {
				return err
			}
		case "allow_forking":
			if err := decodeOptionalBoolNode(valueNode, &r.allowForking); err != nil {
				return err
			}
			if !r.allowForking.Null {
				r.AllowForking = r.allowForking.Value
			}
		case "archived":
			if err := decodeOptionalBoolNode(valueNode, &r.archived); err != nil {
				return err
			}
			if !r.archived.Null {
				r.Archived = r.archived.Value
			}
		case "is_template":
			if err := decodeOptionalBoolNode(valueNode, &r.isTemplate); err != nil {
				return err
			}
			if !r.isTemplate.Null {
				r.IsTemplate = r.isTemplate.Value
			}
		default:
			return fmt.Errorf("field %s not found in type config.RepositorySpec", key)
		}
	}

	return nil
}
