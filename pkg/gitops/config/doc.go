// Package config loads the desired-state configuration used by the
// repo-builder GitOps engine.
//
// The package reads the canonical organization configuration file from a
// configuration directory, decodes it strictly so unsupported fields fail
// fast, and applies basic normalization that is safe at load time. Semantic
// validation such as duplicate detection, enum checks, and reference
// validation is intentionally handled separately so callers can build
// higher-level validation and planning workflows on top of the loaded
// desired state.
package config
