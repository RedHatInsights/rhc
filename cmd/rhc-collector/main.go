package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/redhatinsights/rhc/internal/collector"
	httpapi "github.com/redhatinsights/rhc/internal/http"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/version"
)

// FIXME: Make these configurable (use the values from "rhc configure")
const (
	ingressUrl     = "https://cert.console.redhat.com/api/ingress/v1/upload"
	clientCertPath = "/etc/pki/consumer/cert.pem"
	clientKeyPath  = "/etc/pki/consumer/key.pem"
	rhcTmpDir      = "/var/tmp/rhc"
)

func main() {
	if len(os.Args) <= 2 {
		slog.Error("usage: rhc-collector COMMAND COLLECTOR-ID")
		os.Exit(exitcode.Usage)
	}
	command, collectorId := os.Args[1], os.Args[2]
	slog.Info("starting rhc-collector", slog.String("id", collectorId))
	if err := run(collectorId, command); err != nil {
		slog.Error("rhc-collector exited with error", "error", err)
		os.Exit(exitcode.Err)
	}
}

func run(collectorId, command string) error {
	collectorId, err := collector.ValidateID(collectorId)
	if err != nil {
		slog.Error("invalid collector ID", "error", err)
		return fmt.Errorf("invalid collector ID: %w", err)
	}

	if command != "run" {
		slog.Error("invalid command", "command", command)
		return fmt.Errorf("invalid command %q: must be 'run'", command)
	}

	config, err := getConfig(collectorId)
	if err != nil {
		return err
	}

	tmpDir, err := createTmpDir()
	if err != nil {
		return err
	}
	defer cleanup(tmpDir)

	if err = executeCollector(config, tmpDir); err != nil {
		return err
	}
	archivePath, err := getArchivePath(tmpDir)
	if err != nil {
		return err
	}
	defer cleanup(archivePath)
	if err = uploadArchive(archivePath, config); err != nil {
		return err
	}
	return nil
}

// createTmpDir ensures rhcTmpDir exists with root-only permissions (0700)
// and creates a collector-specific temporary directory inside it. If the
// parent directory exists with different permissions, they are reset to
// 0700. Returns the temporary directory path or an error if any step fails.
func createTmpDir() (string, error) {
	// Ensure the parent directory exists
	if err := os.MkdirAll(rhcTmpDir, 0700); err != nil {
		slog.Error("failed to create rhc temporary directory", "error", err)
		return "", fmt.Errorf("failed to create rhc temporary directory: %w", err)
	}

	// Verify permissions and fix if necessary
	info, err := os.Stat(rhcTmpDir)
	if err != nil {
		slog.Error("failed to stat rhc temporary directory", "error", err)
		return "", fmt.Errorf("failed to stat rhc temporary directory: %w", err)
	}

	if perms := info.Mode().Perm(); perms != 0700 {
		slog.Warn(
			"rhc temporary directory has incorrect permissions, resetting",
			"path", rhcTmpDir,
			"current_permissions", fmt.Sprintf("%#o", perms),
			"expected_permissions", "0700",
		)

		if err := os.Chmod(rhcTmpDir, 0700); err != nil {
			slog.Error("failed to reset permissions on rhc temporary directory", "error", err)
			return "", fmt.Errorf("failed to reset permissions on rhc temporary directory: %w", err)
		}
	}

	tmpDir, err := os.MkdirTemp(rhcTmpDir, "collector-")
	if err != nil {
		slog.Error("failed to create a temporary directory", "error", err)
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	slog.Info("created temporary directory", "dir", tmpDir)
	return tmpDir, nil
}

// getConfig loads collector configuration from collector.ConfigDir/{collectorId}.toml.
// Returns the parsed Config struct or an error if loading/parsing fails.
func getConfig(collectorId string) (collector.Config, error) {
	config, err := collector.GetConfig(collectorId)
	if err != nil {
		slog.Error("failed to get config", "error", err)
		return collector.Config{}, fmt.Errorf("failed to get config: %w", err)
	}
	slog.Info("configuration of the collector", "config", config)
	return config, nil
}

// executeCollector runs the specified collector binary with the collect argument in tmpDir as the working directory.
// The collector process is executed as the user and group defined in the collector configuration.
// Returns an error if the command execution fails.
func executeCollector(config collector.Config, tmpDir string) error {
	collectorPath := fmt.Sprintf("/usr/libexec/rhc/collectors/%s", config.ID)

	sysProcAttr, err := collector.ResolveUserGroupAttr(config.User, config.Group, user.Lookup, user.LookupGroup)
	if err != nil {
		return fmt.Errorf("failed to resolve user and group: %w", err)
	}

	slog.Info("executing collector as configured user/group", "collector", config.ID, "user", config.User, "group", config.Group)

	cmd := exec.Command(collectorPath, "collect")
	cmd.Dir = tmpDir
	cmd.SysProcAttr = sysProcAttr

	// Capture start/end time and execute the command
	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	endTime := time.Now()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Timer payload and writing to cache
	timerPayload := collector.Timer{
		ID:           config.ID,
		LastStarted:  startTime,
		LastFinished: endTime,
		ExitCode:     exitCode,
	}
	cacheErr := collector.WriteTimerCache(config.ID, timerPayload)
	if cacheErr != nil {
		slog.Error("Failed to write timer cache", "error", cacheErr)
	}
	if err != nil {
		slog.Error("failed to execute collector", "error", err, "output", string(output))
		return fmt.Errorf("failed to execute collector: %w", err)
	}

	slog.Info("collector has ran successfully", "output", string(output))
	return nil
}

// getArchivePath creates a compressed .tar.xz archive from the temporary directory.
// Returns the archive file path or an error if compression fails.
func getArchivePath(tmpDir string) (string, error) {
	archivePath, err := collector.GetArchive(tmpDir, "")
	if err != nil {
		slog.Error("failed to compress directory", "error", err)
		return "", fmt.Errorf("failed to compress directory: %w", err)
	}
	slog.Info("archive created", "path", archivePath)
	return archivePath, nil
}

// uploadArchive uploads the created archive to Red Hat Hybrid Cloud Console.
// Returns an error if the upload fails.
func uploadArchive(archivePath string, collectorConfig collector.Config) error {
	archive := collector.ArchiveDto{
		Path:        archivePath,
		ContentType: collectorConfig.ContentType,
	}
	serviceConfig := collector.ServiceConfig{
		URL:            ingressUrl,
		ClientCertPath: clientCertPath,
		ClientKeyPath:  clientKeyPath,
	}
	userAgent := httpapi.GetUserAgent("rhc-collector", version.Version, collectorConfig.ID)
	if err := collector.UploadArchive(archive, serviceConfig, userAgent); err != nil {
		slog.Error("failed to upload archive", "error", err)
		return fmt.Errorf("failed to upload archive: %w", err)
	}
	return nil
}

// cleanup removes the specified file or directory and all its contents.
func cleanup(path string) {
	if err := os.RemoveAll(path); err != nil {
		slog.Debug("failed to remove path", "path", path, "error", err)
		return
	}
	slog.Debug("removed path", "path", path)
}
