package contributors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/orang-gaboets/octostate/internal/filereplace"
)

// LoadConfig reads the maintainer override file. A missing file is normal and
// yields an empty configuration; a malformed or misspelled one is an error, so
// a typo cannot silently disable an intended override.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read contributor overrides %s: %w", path, err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	// An empty file decodes to io.EOF and means "no overrides".
	if err := decoder.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("parse contributor overrides %s: %w", path, err)
	}

	// Decode returns one document per call, so a second one would be dropped in
	// silence - an override moved below a stray separator would simply stop
	// applying. Reject it, matching how the GitOps config loader treats a
	// multi-document organization.yaml.
	var extra yaml.Node
	if err := decoder.Decode(&extra); err == nil {
		return Config{}, fmt.Errorf("parse contributor overrides %s: multiple YAML documents are not allowed", path)
	} else if !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("parse contributor overrides %s: %w", path, err)
	}

	return cfg, nil
}

// Update rewrites the contributor showcase in the README at path and reports
// whether the file changed. It writes only when the rendered content differs,
// so an unchanged repository produces no diff and no spurious commit.
func Update(path string, discovered []Contributor, cfg Config) (bool, error) {
	current, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	updated, err := Apply(string(current), Render(Select(discovered, cfg)))
	if err != nil {
		return false, fmt.Errorf("update %s: %w", path, err)
	}
	if updated == string(current) {
		return false, nil
	}

	// Replace rather than WriteFile: the README was read above, so it must
	// already exist. Replace keeps that requirement, so a file removed between
	// the read and the write fails instead of being recreated from contents
	// that are now stale.
	if err := filereplace.Replace(path, []byte(updated)); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
