package github

// Reposiory contains the repository details.
type Repository struct {
	Org         string
	Name        string
	Private     bool
	Description string
	Topics      []string
}
