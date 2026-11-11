package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/gitops/internal/sizeguard"
	"gopkg.in/yaml.v3"
)

const (
	organizationFileName               = "organization.yaml"
	maxDesiredStateYAMLSize      int64 = 16 << 20
	desiredStateYAMLSizeDisplay        = "16 MiB"
	maxDesiredStateYAMLNodes     int64 = 500_000
	desiredStateYAMLNodesDisplay       = "500,000 YAML nodes"
	// This weighted source ceiling bounds parser-side node construction before
	// yaml.v3 allocates the full tree; the exact node ceiling follows below.
	maxDesiredStateYAMLSourceUnits          int64 = 1_100_000
	desiredStateYAMLSourceUnitsDisplayValue       = "1.1 million YAML source complexity units"
)

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

	info, statErr := file.Stat()
	var decodeErr error
	switch {
	case statErr != nil:
		decodeErr = &sizeguard.ReadError{Err: fmt.Errorf("inspect config file: %w", statErr)}
	case info.Size() > maxDesiredStateYAMLSize:
		decodeErr = desiredStateSizeLimitError(&sizeguard.LimitExceededError{Limit: maxDesiredStateYAMLSize})
	default:
		var data []byte
		processErr := sizeguard.Process(file, maxDesiredStateYAMLSize, func(r io.Reader) error {
			var err error
			data, err = io.ReadAll(r)
			if err != nil {
				return &sizeguard.ReadError{Err: err}
			}
			return nil
		})
		decodeErr = desiredStateSizeLimitError(processErr)
		if decodeErr == nil {
			decodeErr = validateDesiredYAMLSourceComplexity(data, maxDesiredStateYAMLSourceUnits)
			if decodeErr == nil {
				decodeErr = validateDesiredYAMLComplexity(data, maxDesiredStateYAMLNodes)
			}
			if decodeErr == nil {
				decodeErr = decodeYAML(bytes.NewReader(data), out)
			}
		}
	}

	closeErr := file.Close()
	switch {
	case decodeErr != nil && closeErr != nil:
		return &LoadError{
			Kind: loadFileErrorKind(decodeErr),
			Path: path,
			Err:  errors.Join(decodeErr, fmt.Errorf("close config file: %w", closeErr)),
		}
	case decodeErr != nil:
		return &LoadError{
			Kind: loadFileErrorKind(decodeErr),
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

func desiredStateSizeLimitError(err error) error {
	if errors.Is(err, sizeguard.ErrLimitExceeded) {
		return fmt.Errorf("desired-state YAML exceeds the supported size limit of %s: %w", desiredStateYAMLSizeDisplay, err)
	}
	return err
}

func desiredStateYAMLNodeLimitError(limit int64) error {
	display := fmt.Sprintf("%d YAML nodes", limit)
	if limit == maxDesiredStateYAMLNodes {
		display = desiredStateYAMLNodesDisplay
	}
	return fmt.Errorf("desired-state YAML exceeds the supported YAML node limit of %s", display)
}

func validateDesiredYAMLComplexity(data []byte, maxNodes int64) error {
	if maxNodes < 0 {
		return fmt.Errorf("YAML node limit must not be negative")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	if err := countDesiredYAMLNodes(&document, maxNodes); err != nil {
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

func countDesiredYAMLNodes(root *yaml.Node, maxNodes int64) error {
	type frame struct {
		node     *yaml.Node
		next     int
		children int
		total    int64
	}
	childrenOf := func(node *yaml.Node) int {
		children := len(node.Content)
		if node.Kind == yaml.AliasNode && node.Alias != nil {
			children++
		}
		return children
	}
	childAt := func(node *yaml.Node, index int) *yaml.Node {
		if node.Kind == yaml.AliasNode {
			if index == 0 && node.Alias != nil {
				return node.Alias
			}
			if node.Alias != nil {
				index--
			}
		}
		if index < 0 || index >= len(node.Content) {
			return nil
		}
		return node.Content[index]
	}

	if root == nil {
		return fmt.Errorf("YAML document is missing")
	}
	frames := []frame{{node: root, children: childrenOf(root)}}
	active := map[*yaml.Node]struct{}{root: {}}
	completed := make(map[*yaml.Node]int64)
	uniqueNodes := int64(1)
	if uniqueNodes > maxNodes {
		return desiredStateYAMLNodeLimitError(maxNodes)
	}

	for len(frames) > 0 {
		current := &frames[len(frames)-1]
		if current.next < current.children {
			child := childAt(current.node, current.next)
			current.next++
			if child == nil {
				continue
			}
			if _, ok := active[child]; ok {
				return fmt.Errorf("YAML alias cycle detected")
			}
			if childCount, ok := completed[child]; ok {
				if childCount > maxNodes-current.total {
					return desiredStateYAMLNodeLimitError(maxNodes)
				}
				current.total += childCount
				continue
			}
			uniqueNodes++
			if uniqueNodes > maxNodes {
				return desiredStateYAMLNodeLimitError(maxNodes)
			}
			active[child] = struct{}{}
			frames = append(frames, frame{node: child, children: childrenOf(child)})
			continue
		}

		if current.total >= maxNodes {
			return desiredStateYAMLNodeLimitError(maxNodes)
		}
		nodeCount := current.total + 1
		node := current.node
		delete(active, node)
		completed[node] = nodeCount
		frames = frames[:len(frames)-1]
		if len(frames) == 0 {
			return nil
		}
		parent := &frames[len(frames)-1]
		if nodeCount > maxNodes-parent.total {
			return desiredStateYAMLNodeLimitError(maxNodes)
		}
		parent.total += nodeCount
	}
	return nil
}

func loadFileErrorKind(err error) LoadErrorKind {
	var readErr *sizeguard.ReadError
	if errors.Is(err, sizeguard.ErrLimitExceeded) || errors.As(err, &readErr) {
		return LoadErrorReadFile
	}
	return LoadErrorDecodeFile
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
	repo.Owner = ResolveRepositoryOwner(repo.Owner, organization)

	repo.Name = strings.TrimSpace(repo.Name)
	repo.Visibility = strings.TrimSpace(repo.Visibility)
	if repo.description.Present {
		if !repo.description.Null {
			repo.description.Value = strings.TrimSpace(repo.Description)
		}
		repo.Description = repo.description.Value
	} else {
		repo.Description = strings.TrimSpace(repo.Description)
	}
	if repo.homepage.Present {
		if !repo.homepage.Null {
			repo.homepage.Value = strings.TrimSpace(repo.Homepage)
		}
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
		team.Repositories[i].Owner = ResolveRepositoryOwner(team.Repositories[i].Owner, organization)
		team.Repositories[i].Name = strings.TrimSpace(team.Repositories[i].Name)
		team.Repositories[i].Permission = strings.TrimSpace(team.Repositories[i].Permission)
	}
}
