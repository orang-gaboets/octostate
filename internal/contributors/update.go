package contributors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
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

	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
