package builder

// RepoCreationOptions contains parameters for creating a repository from a template repository.
type RepoCreationOptions struct {
	Org          string
	Name         string
	Private      bool
	Description  string
	Topics       []string
	TemplateName string
	TemplateOrg  string
	Service      RepoService
}
