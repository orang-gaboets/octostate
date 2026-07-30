package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const organizationFileName = "organization.yaml"

// LoadDir loads the canonical organization configuration file from configDir,
// decodes it strictly, and applies basic normalization.
func LoadDir(configDir string) (OrganizationConfig, error) {
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return OrganizationConfig{}, &LoadError{
			Kind: LoadErrorInvalidDir,
			Path: configDir,
			Err:  errors.New("config directory is required"),
		}
	}

	info, err := os.Stat(configDir)
	if err != nil {
		return OrganizationConfig{}, &LoadError{
			Kind: LoadErrorInvalidDir,
			Path: configDir,
			Err:  err,
		}
	}
	if !info.IsDir() {
		return OrganizationConfig{}, &LoadError{
			Kind: LoadErrorInvalidDir,
			Path: configDir,
			Err:  errors.New("path is not a directory"),
		}
	}

	return LoadFile(filepath.Join(configDir, organizationFileName))
}

// LoadFile loads an organization configuration from path, decodes it strictly,
// and applies basic normalization.
func LoadFile(path string) (OrganizationConfig, error) {
	var cfg OrganizationConfig
	if err := decodeYAMLFile(path, &cfg); err != nil {
		return OrganizationConfig{}, err
	}
	normalize(&cfg)
	return cfg, nil
}

func decodeYAMLFile(path string, out any) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &LoadError{
				Kind: LoadErrorMissingFile,
				Path: path,
				Err:  err,
			}
		}
		return &LoadError{
			Kind: LoadErrorReadFile,
			Path: path,
			Err:  err,
		}
	}

	decodeErr := decodeYAML(file, out)

	closeErr := file.Close()
	switch {
	case decodeErr != nil && closeErr != nil:
		return &LoadError{
			Kind: LoadErrorDecodeFile,
			Path: path,
			Err:  errors.Join(decodeErr, fmt.Errorf("close config file: %w", closeErr)),
		}
	case decodeErr != nil:
		return &LoadError{
			Kind: LoadErrorDecodeFile,
			Path: path,
			Err:  decodeErr,
		}
	case closeErr != nil:
		return &LoadError{
			Kind: LoadErrorReadFile,
			Path: path,
			Err:  fmt.Errorf("close config file: %w", closeErr),
		}
	}

	return nil
}

func decodeYAML(r io.Reader, out any) error {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)

	if err := decoder.Decode(out); err != nil {
		return err
	}

	var extra yaml.Node
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("multiple YAML documents are not allowed")
	} else if !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

func normalize(cfg *OrganizationConfig) {
	cfg.Organization = strings.TrimSpace(cfg.Organization)
	if cfg.Members == nil {
		cfg.Members = []OrganizationMemberSpec{}
	}
	for i := range cfg.Members {
		cfg.Members[i].Username = strings.TrimSpace(cfg.Members[i].Username)
		cfg.Members[i].Role = strings.TrimSpace(cfg.Members[i].Role)
	}
	cfg.Invites = normalizeInvites(cfg.Invites)

	if cfg.Repositories == nil {
		cfg.Repositories = []RepositorySpec{}
	}
	for i := range cfg.Repositories {
		normalizeRepository(&cfg.Repositories[i], cfg.Organization)
	}

	if cfg.Teams == nil {
		cfg.Teams = []TeamSpec{}
	}
	for i := range cfg.Teams {
		normalizeTeam(&cfg.Teams[i], cfg.Organization)
	}
}

func normalizeInvites(invites []InviteSpec) []InviteSpec {
	if invites == nil {
		return []InviteSpec{}
	}

	for i := range invites {
		invites[i].Username.TrimSpace()
		invites[i].Email.TrimSpace()
		invites[i].Role = strings.TrimSpace(invites[i].Role)

		if invites[i].TeamSlugs == nil {
			invites[i].TeamSlugs = []string{}
		}
		for j := range invites[i].TeamSlugs {
			invites[i].TeamSlugs[j] = strings.TrimSpace(invites[i].TeamSlugs[j])
		}
	}

	return invites
}

func normalizeRepository(repo *RepositorySpec, organization string) {
	repo.Owner = strings.TrimSpace(repo.Owner)
	if repo.Owner == "" {
		repo.Owner = organization
	}

	repo.Name = strings.TrimSpace(repo.Name)
	repo.Visibility = strings.TrimSpace(repo.Visibility)
	if repo.description.Present {
		repo.description.TrimSpace()
		repo.Description = repo.description.Value
	} else {
		repo.Description = strings.TrimSpace(repo.Description)
	}
	if repo.homepage.Present {
		repo.homepage.TrimSpace()
		repo.Homepage = repo.homepage.Value
	} else {
		repo.Homepage = strings.TrimSpace(repo.Homepage)
	}
	repo.Template.Owner = strings.TrimSpace(repo.Template.Owner)
	repo.Template.Name = strings.TrimSpace(repo.Template.Name)

	if repo.Topics == nil {
		repo.Topics = []string{}
	}
	for i := range repo.Topics {
		repo.Topics[i] = strings.TrimSpace(repo.Topics[i])
	}
}

func normalizeTeam(team *TeamSpec, organization string) {
	team.Slug = strings.TrimSpace(team.Slug)
	team.Name = strings.TrimSpace(team.Name)
	team.Description = strings.TrimSpace(team.Description)
	team.Privacy = strings.TrimSpace(team.Privacy)
	team.ParentSlug = strings.TrimSpace(team.ParentSlug)

	if team.Members == nil {
		team.Members = []TeamMemberSpec{}
	}
	for i := range team.Members {
		team.Members[i].Username = strings.TrimSpace(team.Members[i].Username)
		team.Members[i].Role = strings.TrimSpace(team.Members[i].Role)
	}

	if team.Repositories == nil {
		team.Repositories = []TeamRepositorySpec{}
	}
	for i := range team.Repositories {
		team.Repositories[i].Owner = strings.TrimSpace(team.Repositories[i].Owner)
		if team.Repositories[i].Owner == "" {
			team.Repositories[i].Owner = organization
		}
		team.Repositories[i].Name = strings.TrimSpace(team.Repositories[i].Name)
		team.Repositories[i].Permission = strings.TrimSpace(team.Repositories[i].Permission)
	}
}
