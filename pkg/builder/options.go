package builder

// Reposiory contains the repository details.
type Repository struct {
	Org         string
	Name        string
	Private     bool
	Description string
	Topics      []string
}

// RepoCreationOptions contains parameters for creating a repository from a template repository.
type RepoCreationOptions struct {
	NewRepo      Repository
	TemplateRepo Repository
	Service      RepoService
}
