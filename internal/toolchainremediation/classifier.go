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
	toolchainModule      = "toolchain"
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
	Module   string `json:"module"`
	Package  string `json:"package,omitempty"`
	Function string `json:"function,omitempty"`
}

type findingRecord struct {
	module          string
	fixedVersion    GoVersion
	hasFixedVersion bool
	reachable       bool
}

// Classify consumes a govulncheck streaming JSON report and the current pinned
// Go toolchain version. It returns a deterministic classification result or an
// error if the stream is malformed, incomplete, or unsupported.
func Classify(r io.Reader, current GoVersion) (Result, error) {
	if current.Major <= 0 || current.Minor < 0 || current.Patch < 0 {
		return Result{}, fmt.Errorf("invalid current Go version: %s", current.String())
	}

	records, err := scanRecords(r, current)
	if err != nil {
		return Result{}, err
	}
	eligibleIDs := make(map[string]struct{})
	var target GoVersion
	haveTarget := false
	sawStdlib := false
	sawNonStdlib := false
	sawIneligible := false

	for osv, record := range records {
		if !record.reachable {
			continue
		}
		if isGoModule(record.module) {
			sawStdlib = true
		} else {
			sawNonStdlib = true
		}

		eligible := record.hasFixedVersion && isGoModule(record.module) &&
			record.fixedVersion.Major == current.Major &&
			record.fixedVersion.Minor == current.Minor &&
			record.fixedVersion.Patch > current.Patch
		if !eligible {
			sawIneligible = true
			continue
		}

		eligibleIDs[osv] = struct{}{}
		if !haveTarget || record.fixedVersion.Patch > target.Patch {
			target = record.fixedVersion
			haveTarget = true
		}
	}

	if sawStdlib && sawNonStdlib {
		return Result{}, errors.New("mixed stdlib and third-party findings are not eligible for automated remediation")
	}
	if sawIneligible {
		return Result{}, nil
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

func scanRecords(r io.Reader, current GoVersion) (map[string]findingRecord, error) {
	dec := json.NewDecoder(r)

	first, err := decodeMessage(dec)
	if err != nil {
		return nil, err
	}
	if first.Config == nil {
		return nil, errors.New("govulncheck stream must start with a config message")
	}
	if err := validateConfig(first.Config, current); err != nil {
		return nil, err
	}

	records := make(map[string]findingRecord)
	for {
		msg, err := decodeMessage(dec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records, nil
			}
			return nil, err
		}
		if msg.Config != nil {
			return nil, errors.New("govulncheck config message must appear exactly once at the start of the stream")
		}
		if msg.Finding == nil {
			continue
		}

		record, err := parseFinding(msg.Finding)
		if err != nil {
			return nil, err
		}
		if prior, ok := records[msg.Finding.OSV]; ok {
			if prior.module != record.module {
				return nil, fmt.Errorf("finding %s has conflicting module identities %q and %q", msg.Finding.OSV, prior.module, record.module)
			}
			if prior.reachable && record.reachable && prior.fixedVersion != record.fixedVersion {
				return nil, fmt.Errorf("finding %s has conflicting fixed versions %q and %q", msg.Finding.OSV, prior.fixedVersion.String(), record.fixedVersion.String())
			}
			prior.reachable = prior.reachable || record.reachable
			if record.reachable {
				prior.fixedVersion = record.fixedVersion
				prior.hasFixedVersion = true
			}
			records[msg.Finding.OSV] = prior
			continue
		}
		records[msg.Finding.OSV] = record
	}
}

func parseFinding(finding *findingMessage) (findingRecord, error) {
	if finding.OSV == "" {
		return findingRecord{}, errors.New("finding message missing vulnerability id")
	}
	if len(finding.Trace) == 0 {
		return findingRecord{}, fmt.Errorf("finding %s missing trace frame", finding.OSV)
	}

	module := finding.Trace[0].Module
	if module == "" {
		return findingRecord{}, fmt.Errorf("finding %s missing trace module", finding.OSV)
	}

	reachable := false
	for index, frame := range finding.Trace {
		if frame.Module == "" {
			return findingRecord{}, fmt.Errorf("finding %s has trace frame missing module", finding.OSV)
		}
		if frame.Function != "" && frame.Package == "" {
			return findingRecord{}, fmt.Errorf("finding %s has trace function without package", finding.OSV)
		}
		if index == 0 {
			reachable = frame.Function != ""
		} else if !reachable && frame.Function != "" {
			return findingRecord{}, fmt.Errorf("finding %s has a non-symbol first trace frame followed by a function", finding.OSV)
		}
	}

	record := findingRecord{module: module, reachable: reachable}
	if !reachable {
		return record, nil
	}
	if finding.FixedVersion == "" {
		return findingRecord{}, fmt.Errorf("finding %s missing fixed version", finding.OSV)
	}
	fixedVersion, err := parseGovulncheckFixedVersion(finding.FixedVersion)
	if err != nil {
		return findingRecord{}, fmt.Errorf("finding %s has invalid fixed version %q: %w", finding.OSV, finding.FixedVersion, err)
	}
	record.fixedVersion = fixedVersion
	record.hasFixedVersion = true
	return record, nil
}

func isGoModule(module string) bool {
	return module == stdlibModule || module == toolchainModule
}

func validateConfig(cfg *configMessage, current GoVersion) error {
	if cfg.ProtocolVersion != protocolVersion {
		return fmt.Errorf("unsupported govulncheck protocol version %q", cfg.ProtocolVersion)
	}
	if cfg.ScannerName != supportedScannerName {
		return fmt.Errorf("unsupported govulncheck scanner name %q", cfg.ScannerName)
	}
	if cfg.ScanLevel != "symbol" {
		return fmt.Errorf("unsupported govulncheck scan level %q", cfg.ScanLevel)
	}
	if cfg.ScanMode != "source" {
		return fmt.Errorf("unsupported govulncheck scan mode %q", cfg.ScanMode)
	}
	version, err := parseGovulncheckGoVersion(cfg.GoVersion)
	if err != nil {
		return fmt.Errorf("invalid govulncheck Go version %q: %w", cfg.GoVersion, err)
	}
	if version != current {
		return fmt.Errorf("govulncheck Go version %s does not match current toolchain %s", version.String(), current.String())
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

func parseGovulncheckFixedVersion(raw string) (GoVersion, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "v") {
		return GoVersion{}, fmt.Errorf("fixed version must use v prefix: %q", raw)
	}
	return parseVersion(strings.TrimPrefix(s, "v"))
}

func parseGovulncheckGoVersion(raw string) (GoVersion, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "go") {
		return GoVersion{}, fmt.Errorf("go version must use go prefix: %q", raw)
	}
	return parseVersion(s)
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
