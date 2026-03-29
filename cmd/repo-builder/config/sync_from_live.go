package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/syncfromlive"
)

const (
	syncFromLiveModeBootstrap       = "bootstrap"
	syncFromLiveModeAdopt           = "adopt"
	syncFromLiveOrganizationFile    = "organization.yaml"
	syncFromLiveTempFilePattern     = "organization-*.yaml"
	syncFromLiveConfigDirectoryMode = 0o755
)

var (
	newSyncFromLiveClient         = auth.NewClient
	collectSyncFromLiveState      = collector.CollectOrganizationForSyncFromLive
	loadSyncFromLiveConfig        = gitopsconfig.LoadDir
	buildSyncFromLiveBootstrap    = syncfromlive.BuildBootstrapConfig
	buildSyncFromLiveAdopt        = syncfromlive.BuildAdoptConfig
	validateSyncFromLiveConfig    = gitopsconfig.Validate
	encodeSyncFromLiveConfig      = gitopsconfig.EncodeYAML
	statSyncFromLivePath          = os.Stat
	mkdirAllSyncFromLiveConfigDir = os.MkdirAll
	createTempSyncFromLiveFile    = os.CreateTemp
	linkSyncFromLivePath          = os.Link
	renameSyncFromLivePath        = os.Rename
	removeSyncFromLivePath        = os.Remove
)

// SyncFromLiveConfigCmd creates the config sync-from-live command.
func SyncFromLiveConfigCmd() *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		mode           string
		organization   string
		configDir      string
		write          bool
	)

	cmd := &cobra.Command{
		Use:           "sync-from-live",
		Short:         "Generate GitOps config from live GitHub state",
		Long:          "Collect live GitHub organization state and generate or update a GitOps organization.yaml proposal. Sync-from-live prints YAML by default and only writes files when --write is set.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: `
			repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --token <token>
			repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --token <token> --write
			repo-builder config sync-from-live --mode adopt --org orang-gaboets --config-dir ./config --token <token>
			repo-builder config sync-from-live --mode adopt --org orang-gaboets --config-dir ./config --token <token> --write
			repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir /path/to/control-repo/config --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --write`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			yamlBytes, writeResult, err := syncFromLiveConfig(
				cmd.Context(),
				token,
				appID,
				installationID,
				appKeyPath,
				mode,
				organization,
				configDir,
				write,
			)
			if err != nil {
				if code, ok := exitcode.Code(err); ok && code == validateExitCodeInvalidConfig {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				}
				return err
			}
			if write {
				return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("wrote %s GitOps config", mode), writeResult)
			}
			_, err = cmd.OutOrStdout().Write(yamlBytes)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&mode, "mode", "", "Sync mode to run (bootstrap or adopt)")
	cmd.Flags().StringVar(&organization, "org", "", "GitHub organization to read from live state")
	cmd.Flags().StringVar(&configDir, "config-dir", "", "Path to the config directory containing or receiving organization.yaml")
	cmd.Flags().BoolVar(&write, "write", false, "Write the generated organization.yaml into --config-dir instead of printing YAML to stdout")
	github.MarkRequiredFlags(cmd, "mode", "org", "config-dir")

	return cmd
}

type syncFromLiveWriteResult struct {
	Organization string `json:"organization"`
	Mode         string `json:"mode"`
	Path         string `json:"path"`
}

func syncFromLiveConfig(
	ctx context.Context,
	token string,
	appID, installationID int64,
	appKeyPath, mode, organization, configDir string,
	write bool,
) ([]byte, *syncFromLiveWriteResult, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	organization = strings.TrimSpace(organization)
	configDir = strings.TrimSpace(configDir)

	switch mode {
	case syncFromLiveModeBootstrap:
	case syncFromLiveModeAdopt:
	default:
		return nil, nil, fmt.Errorf("sync-from-live mode %q is not supported", mode)
	}

	if organization == "" {
		return nil, nil, fmt.Errorf("organization is required: %w", github.ErrMissingRequiredField)
	}
	if configDir == "" {
		return nil, nil, fmt.Errorf("config directory is required: %w", github.ErrMissingRequiredField)
	}
	if write && mode == syncFromLiveModeBootstrap {
		if _, err := ensureBootstrapConfigTargetAvailable(configDir); err != nil {
			return nil, nil, err
		}
	}

	var desired gitopsconfig.OrganizationConfig
	var err error
	if mode == syncFromLiveModeAdopt {
		desired, err = loadSyncFromLiveConfig(configDir)
		if err != nil {
			return nil, nil, err
		}
		report := validateSyncFromLiveConfig(desired)
		if !report.Valid {
			return nil, nil, exitcode.New(validateExitCodeInvalidConfig, existingSyncFromLiveConfigValidationError(report))
		}
		if !strings.EqualFold(strings.TrimSpace(desired.Organization), organization) {
			return nil, nil, fmt.Errorf(
				"organization %q in %s does not match --org %q: %w",
				desired.Organization,
				syncFromLiveOrganizationFile,
				organization,
				github.ErrInvalidFieldValue,
			)
		}
	}

	client, err := newSyncFromLiveClient(ctx, token, appID, installationID, appKeyPath)
	if err != nil {
		return nil, nil, err
	}

	actual, err := collectSyncFromLiveState(ctx, collector.CollectOrganizationOptions{
		OrgName:             organization,
		OrganizationService: client.Organizations(),
		RepositoryService:   client.Repositories(),
		TeamService:         client.Teams(),
	})
	if err != nil {
		return nil, nil, err
	}

	var cfg gitopsconfig.OrganizationConfig
	switch mode {
	case syncFromLiveModeBootstrap:
		cfg, err = buildSyncFromLiveBootstrap(syncfromlive.BootstrapOptions{Actual: actual})
	case syncFromLiveModeAdopt:
		cfg, err = buildSyncFromLiveAdopt(syncfromlive.AdoptOptions{
			Desired: desired,
			Actual:  actual,
		})
	}
	if err != nil {
		return nil, nil, err
	}
	report := validateSyncFromLiveConfig(cfg)
	if !report.Valid {
		return nil, nil, exitcode.New(validateExitCodeInvalidConfig, generatedSyncFromLiveConfigValidationError(mode, report))
	}

	yamlBytes, err := encodeSyncFromLiveConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	if !write {
		return yamlBytes, nil, nil
	}

	targetPath, err := writeSyncFromLiveConfigFile(mode, configDir, yamlBytes)
	if err != nil {
		return nil, nil, err
	}

	return nil, &syncFromLiveWriteResult{
		Organization: strings.TrimSpace(cfg.Organization),
		Mode:         mode,
		Path:         targetPath,
	}, nil
}

func writeSyncFromLiveConfigFile(mode, configDir string, yamlBytes []byte) (string, error) {
	switch mode {
	case syncFromLiveModeBootstrap:
		return writeBootstrapConfigFile(configDir, yamlBytes)
	case syncFromLiveModeAdopt:
		return replaceSyncFromLiveConfigFile(configDir, yamlBytes)
	default:
		return "", fmt.Errorf("sync-from-live mode %q is not supported", mode)
	}
}

func writeBootstrapConfigFile(configDir string, yamlBytes []byte) (string, error) {
	targetPath, err := ensureBootstrapConfigTargetAvailable(configDir)
	if err != nil {
		return "", err
	}

	if err := mkdirAllSyncFromLiveConfigDir(strings.TrimSpace(configDir), syncFromLiveConfigDirectoryMode); err != nil {
		return "", fmt.Errorf("create config directory %s: %w", configDir, err)
	}

	file, err := createTempSyncFromLiveFile(filepath.Dir(targetPath), syncFromLiveTempFilePattern)
	if err != nil {
		return "", fmt.Errorf("create bootstrap config temp file: %w", err)
	}
	tempPath := file.Name()

	writeErr := error(nil)
	if _, err := file.Write(yamlBytes); err != nil {
		writeErr = err
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = removeSyncFromLivePath(tempPath)
	}

	switch {
	case writeErr != nil && closeErr != nil:
		return "", fmt.Errorf("write bootstrap config %s: %w", targetPath, errors.Join(writeErr, closeErr))
	case writeErr != nil:
		return "", fmt.Errorf("write bootstrap config %s: %w", targetPath, writeErr)
	case closeErr != nil:
		return "", fmt.Errorf("close bootstrap config temp file %s: %w", tempPath, closeErr)
	}

	if err := linkSyncFromLivePath(tempPath, targetPath); err != nil {
		_ = removeSyncFromLivePath(tempPath)
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("bootstrap config target already exists: %s", targetPath)
		}
		return "", fmt.Errorf("link bootstrap config %s: %w", targetPath, err)
	}
	_ = removeSyncFromLivePath(tempPath)

	return targetPath, nil
}

func replaceSyncFromLiveConfigFile(configDir string, yamlBytes []byte) (string, error) {
	targetPath := filepath.Join(strings.TrimSpace(configDir), syncFromLiveOrganizationFile)

	if err := mkdirAllSyncFromLiveConfigDir(strings.TrimSpace(configDir), syncFromLiveConfigDirectoryMode); err != nil {
		return "", fmt.Errorf("create config directory %s: %w", configDir, err)
	}

	file, err := createTempSyncFromLiveFile(filepath.Dir(targetPath), syncFromLiveTempFilePattern)
	if err != nil {
		return "", fmt.Errorf("create sync-from-live config temp file: %w", err)
	}
	tempPath := file.Name()

	writeErr := error(nil)
	if _, err := file.Write(yamlBytes); err != nil {
		writeErr = err
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = removeSyncFromLivePath(tempPath)
	}

	switch {
	case writeErr != nil && closeErr != nil:
		return "", fmt.Errorf("write sync-from-live config %s: %w", targetPath, errors.Join(writeErr, closeErr))
	case writeErr != nil:
		return "", fmt.Errorf("write sync-from-live config %s: %w", targetPath, writeErr)
	case closeErr != nil:
		return "", fmt.Errorf("close sync-from-live config temp file %s: %w", tempPath, closeErr)
	}

	if err := renameSyncFromLivePath(tempPath, targetPath); err != nil {
		_ = removeSyncFromLivePath(tempPath)
		return "", fmt.Errorf("replace sync-from-live config %s: %w", targetPath, err)
	}

	return targetPath, nil
}

func ensureBootstrapConfigTargetAvailable(configDir string) (string, error) {
	targetPath := filepath.Join(strings.TrimSpace(configDir), syncFromLiveOrganizationFile)

	if _, err := statSyncFromLivePath(targetPath); err == nil {
		return "", fmt.Errorf("bootstrap config target already exists: %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat bootstrap config target %s: %w", targetPath, err)
	}

	return targetPath, nil
}

func existingSyncFromLiveConfigValidationError(report gitopsconfig.ValidationReport) error {
	return syncFromLiveConfigValidationError("existing config is invalid", report)
}

func generatedSyncFromLiveConfigValidationError(mode string, report gitopsconfig.ValidationReport) error {
	return syncFromLiveConfigValidationError(fmt.Sprintf("generated %s config is invalid", strings.TrimSpace(mode)), report)
}

func syncFromLiveConfigValidationError(prefix string, report gitopsconfig.ValidationReport) error {
	if len(report.Errors) == 0 {
		return fmt.Errorf("%s", prefix)
	}

	first := report.Errors[0]
	detail := first.Message
	if path := strings.TrimSpace(first.Path); path != "" {
		detail = fmt.Sprintf("%s: %s", path, first.Message)
	}
	if len(report.Errors) == 1 {
		return fmt.Errorf("%s: %s", prefix, detail)
	}
	return fmt.Errorf(
		"%s: %s (and %d more error(s))",
		prefix,
		detail,
		len(report.Errors)-1,
	)
}
