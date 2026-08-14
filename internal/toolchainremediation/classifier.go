// Package toolchainremediation classifies govulncheck streams for Go toolchain
// remediation decisions.
package toolchainremediation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

const (
	protocolVersion      = "v1.0.0"
	supportedScannerName = "govulncheck"
	stdlibModule         = "stdlib"
)

// GoVersion identifies the pinned Go toolchain version being classified.
type GoVersion struct {
	Major int
	Minor int
	Patch int
}

func (v GoVersion) String() string {
	return fmt.Sprintf("go%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Result is the deterministic classification outcome for a govulncheck stream.
type Result struct {
	Eligible         bool
	TargetVersion    string
	VulnerabilityIDs []string
}

type streamMessage struct {
	Config   *configMessage   `json:"config,omitempty"`
	Progress *progressMessage `json:"progress,omitempty"`
	SBOM     *sbomMessage     `json:"SBOM,omitempty"`
	OSV      *osvMessage      `json:"osv,omitempty"`
	Finding  *findingMessage  `json:"finding,omitempty"`
}

type configMessage struct {
	ProtocolVersion string `json:"protocol_version"`
	ScannerName     string `json:"scanner_name,omitempty"`
	ScannerVersion  string `json:"scanner_version,omitempty"`
	DB              string `json:"db,omitempty"`
	GoVersion       string `json:"go_version,omitempty"`
	ScanLevel       string `json:"scan_level,omitempty"`
	ScanMode        string `json:"scan_mode,omitempty"`
}

type progressMessage struct {
	Message string `json:"message,omitempty"`
}

type sbomMessage struct {
	GoVersion string `json:"go_version,omitempty"`
}

type osvMessage struct {
	ID string `json:"id,omitempty"`
}

type findingMessage struct {
	OSV          string         `json:"osv,omitempty"`
	FixedVersion string         `json:"fixed_version,omitempty"`
	Trace        []findingFrame `json:"trace,omitempty"`
}

type findingFrame struct {
	Module string `json:"module"`
}

type findingRecord struct {
	module       string
	fixedVersion string
	eligible     bool
}

type findingDecision struct {
	module    string
	record    findingRecord
	duplicate bool
}

// Classify consumes a govulncheck streaming JSON report and the current pinned
// Go toolchain version. It returns a deterministic classification result or an
// error if the stream is malformed, incomplete, or unsupported.
func Classify(r io.Reader, current GoVersion) (Result, error) {
	if current.Major <= 0 || current.Minor < 0 || current.Patch < 0 {
		return Result{}, fmt.Errorf("invalid current Go version: %s", current.String())
	}

	dec := json.NewDecoder(r)

	first, err := decodeMessage(dec)
	if err != nil {
		return Result{}, err
	}
	if first.Config == nil {
		return Result{}, errors.New("govulncheck stream must start with a config message")
	}
	if err := validateConfig(first.Config); err != nil {
		return Result{}, err
	}

	records := make(map[string]findingRecord)
	eligibleIDs := make(map[string]struct{})
	var target GoVersion
	haveTarget := false
	sawStdlib := false
	sawNonStdlib := false

	for {
		msg, err := decodeMessage(dec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Result{}, err
		}
		if msg.Config != nil {
			return Result{}, errors.New("govulncheck config message must appear exactly once at the start of the stream")
		}
		if msg.Finding == nil {
			continue
		}

		decision, err := classifyFinding(msg.Finding, current, records)
		if err != nil {
			return Result{}, err
		}
		if decision.module == stdlibModule {
			sawStdlib = true
		} else {
			sawNonStdlib = true
		}
		if decision.duplicate {
			continue
		}
		records[msg.Finding.OSV] = decision.record
		if decision.record.eligible {
			eligibleIDs[msg.Finding.OSV] = struct{}{}
			fixedVersion, err := parseVersion(decision.record.fixedVersion)
			if err != nil {
				return Result{}, err
			}
			if !haveTarget || fixedVersion.Patch > target.Patch {
				target = fixedVersion
				haveTarget = true
			}
		}
	}

	if len(records) == 0 {
		return Result{}, nil
	}
	if sawStdlib && sawNonStdlib {
		return Result{}, errors.New("mixed stdlib and third-party findings are not eligible for automated remediation")
	}

	if !haveTarget {
		return Result{}, nil
	}

	ids := make([]string, 0, len(eligibleIDs))
	for id := range eligibleIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	return Result{
		Eligible:         true,
		TargetVersion:    target.String(),
		VulnerabilityIDs: ids,
	}, nil
}

func classifyFinding(finding *findingMessage, current GoVersion, records map[string]findingRecord) (findingDecision, error) {
	if finding.OSV == "" {
		return findingDecision{}, errors.New("finding message missing vulnerability id")
	}
	if finding.FixedVersion == "" {
		return findingDecision{}, fmt.Errorf("finding %s missing fixed version", finding.OSV)
	}
	if len(finding.Trace) == 0 {
		return findingDecision{}, fmt.Errorf("finding %s missing trace frame", finding.OSV)
	}

	fixedVersion, err := parseVersion(finding.FixedVersion)
	if err != nil {
		return findingDecision{}, fmt.Errorf("finding %s has invalid fixed version %q: %w", finding.OSV, finding.FixedVersion, err)
	}

	module := finding.Trace[0].Module
	if module == "" {
		return findingDecision{}, fmt.Errorf("finding %s missing trace module", finding.OSV)
	}

	decision := findingDecision{
		module: module,
		record: findingRecord{
			module:       module,
			fixedVersion: fixedVersion.String(),
		},
	}
	if prior, ok := records[finding.OSV]; ok {
		if prior.module != module {
			return findingDecision{}, fmt.Errorf("finding %s has conflicting module identities %q and %q", finding.OSV, prior.module, module)
		}
		if prior.fixedVersion != fixedVersion.String() {
			return findingDecision{}, fmt.Errorf("finding %s has conflicting fixed versions %q and %q", finding.OSV, prior.fixedVersion, fixedVersion.String())
		}
		decision.duplicate = true
		return decision, nil
	}

	decision.record.eligible = module == stdlibModule &&
		fixedVersion.Major == current.Major &&
		fixedVersion.Minor == current.Minor &&
		fixedVersion.Patch > current.Patch
	return decision, nil
}

func validateConfig(cfg *configMessage) error {
	if cfg.ProtocolVersion != protocolVersion {
		return fmt.Errorf("unsupported govulncheck protocol version %q", cfg.ProtocolVersion)
	}
	if cfg.ScannerName != supportedScannerName {
		return fmt.Errorf("unsupported govulncheck scanner name %q", cfg.ScannerName)
	}
	return nil
}

func decodeMessage(dec *json.Decoder) (streamMessage, error) {
	var msg streamMessage
	if err := dec.Decode(&msg); err != nil {
		return streamMessage{}, err
	}

	fields := 0
	if msg.Config != nil {
		fields++
	}
	if msg.Progress != nil {
		fields++
	}
	if msg.SBOM != nil {
		fields++
	}
	if msg.OSV != nil {
		fields++
	}
	if msg.Finding != nil {
		fields++
	}
	if fields != 1 {
		return streamMessage{}, fmt.Errorf("expected exactly one govulncheck message field, got %d", fields)
	}

	return msg, nil
}

func parseVersion(raw string) (GoVersion, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return GoVersion{}, errors.New("empty version")
	}
	s = strings.TrimPrefix(s, "go")

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return GoVersion{}, fmt.Errorf("expected major.minor.patch version, got %q", raw)
	}

	major, err := parseVersionPart(parts[0])
	if err != nil {
		return GoVersion{}, fmt.Errorf("invalid major version in %q: %w", raw, err)
	}
	minor, err := parseVersionPart(parts[1])
	if err != nil {
		return GoVersion{}, fmt.Errorf("invalid minor version in %q: %w", raw, err)
	}
	patch, err := parseVersionPart(parts[2])
	if err != nil {
		return GoVersion{}, fmt.Errorf("invalid patch version in %q: %w", raw, err)
	}

	return GoVersion{Major: major, Minor: minor, Patch: patch}, nil
}

func parseVersionPart(raw string) (int, error) {
	if raw == "" {
		return 0, errors.New("empty version part")
	}
	if raw[0] == '+' || raw[0] == '-' {
		return 0, fmt.Errorf("invalid numeric component %q", raw)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("negative numeric component %q", raw)
	}
	return value, nil
}
