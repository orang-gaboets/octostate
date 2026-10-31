package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	desiredStateIdentityDomain = "octostate-config-v1"
	desiredStateIdentityPrefix = desiredStateIdentityDomain + ":sha256:"
)

// ExpectedDesiredStateIdentityErrorKind identifies why an expected desired
// state identity could not be accepted.
type ExpectedDesiredStateIdentityErrorKind string

const (
	// ExpectedDesiredStateIdentityErrorMissingExpected reports an absent
	// caller-provided identity.
	ExpectedDesiredStateIdentityErrorMissingExpected ExpectedDesiredStateIdentityErrorKind = "missing_expected"
	// ExpectedDesiredStateIdentityErrorMalformed reports an invalid identity
	// syntax or digest.
	ExpectedDesiredStateIdentityErrorMalformed ExpectedDesiredStateIdentityErrorKind = "malformed"
	// ExpectedDesiredStateIdentityErrorUnsupportedVersion reports an identity
	// version this implementation does not support.
	ExpectedDesiredStateIdentityErrorUnsupportedVersion ExpectedDesiredStateIdentityErrorKind = "unsupported_version"
	// ExpectedDesiredStateIdentityErrorMismatch reports an identity that does
	// not match the current desired configuration.
	ExpectedDesiredStateIdentityErrorMismatch ExpectedDesiredStateIdentityErrorKind = "mismatch"
)

// ExpectedDesiredStateIdentityError reports an invalid or mismatched
// caller-provided desired state identity. Kind is stable for callers that need
// to distinguish input errors from identity mismatches.
type ExpectedDesiredStateIdentityError struct {
	Kind     ExpectedDesiredStateIdentityErrorKind
	expected string
	actual   string
}

func (e *ExpectedDesiredStateIdentityError) Error() string {
	if e == nil {
		return "expected desired state identity is invalid"
	}
	switch e.Kind {
	case ExpectedDesiredStateIdentityErrorMissingExpected:
		return "expected desired state identity is required"
	case ExpectedDesiredStateIdentityErrorMalformed:
		return "expected desired state identity is malformed"
	case ExpectedDesiredStateIdentityErrorUnsupportedVersion:
		return "expected desired state identity version is not supported"
	case ExpectedDesiredStateIdentityErrorMismatch:
		return fmt.Sprintf("desired state identity %q does not match expected identity %q", e.actual, e.expected)
	default:
		return "expected desired state identity is invalid"
	}
}

// DesiredStateIdentity returns the versioned SHA-256 identity of a valid
// desired configuration. It hashes a domain separator, a NUL byte, and the
// exact canonical bytes produced by EncodeYAML, including the final newline.
func DesiredStateIdentity(cfg OrganizationConfig) (string, error) {
	normalized := NormalizeDesiredState(cfg)
	if err := ValidateAndError(normalized); err != nil {
		return "", err
	}

	canonical, err := EncodeYAML(normalized)
	if err != nil {
		return "", err
	}
	preimage := make([]byte, 0, len(desiredStateIdentityDomain)+1+len(canonical))
	preimage = append(preimage, desiredStateIdentityDomain...)
	preimage = append(preimage, 0)
	preimage = append(preimage, canonical...)
	digest := sha256.Sum256(preimage)
	return desiredStateIdentityPrefix + hex.EncodeToString(digest[:]), nil
}

// CheckExpectedDesiredStateIdentity validates the caller's reviewed identity
// and rejects cfg unless its effective desired state has the same identity.
func CheckExpectedDesiredStateIdentity(cfg OrganizationConfig, expected string) error {
	if expected == "" {
		return &ExpectedDesiredStateIdentityError{Kind: ExpectedDesiredStateIdentityErrorMissingExpected}
	}

	version, algorithm, digest, ok := parseDesiredStateIdentity(expected)
	if !ok {
		return &ExpectedDesiredStateIdentityError{Kind: ExpectedDesiredStateIdentityErrorMalformed}
	}
	if version != "1" {
		return &ExpectedDesiredStateIdentityError{Kind: ExpectedDesiredStateIdentityErrorUnsupportedVersion}
	}
	if algorithm != "sha256" || len(digest) != sha256.Size*2 {
		return &ExpectedDesiredStateIdentityError{Kind: ExpectedDesiredStateIdentityErrorMalformed}
	}
	decoded, err := hex.DecodeString(digest)
	if err != nil || hex.EncodeToString(decoded) != digest {
		return &ExpectedDesiredStateIdentityError{Kind: ExpectedDesiredStateIdentityErrorMalformed}
	}

	actual, err := DesiredStateIdentity(cfg)
	if err != nil {
		return err
	}
	if expected != actual {
		return &ExpectedDesiredStateIdentityError{
			Kind:     ExpectedDesiredStateIdentityErrorMismatch,
			expected: expected,
			actual:   actual,
		}
	}
	return nil
}

func parseDesiredStateIdentity(value string) (version, algorithm, digest string, ok bool) {
	const versionPrefix = "octostate-config-v"
	if !strings.HasPrefix(value, versionPrefix) {
		return "", "", "", false
	}
	versionAndRest := strings.TrimPrefix(value, versionPrefix)
	version, rest, found := strings.Cut(versionAndRest, ":")
	if !found || version == "" {
		return "", "", "", false
	}
	for _, char := range version {
		if char < '0' || char > '9' {
			return "", "", "", false
		}
	}
	algorithm, digest, found = strings.Cut(rest, ":")
	if !found || strings.Contains(digest, ":") {
		return "", "", "", false
	}
	return version, algorithm, digest, true
}
