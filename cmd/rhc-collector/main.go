package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/redhatinsights/rhc/internal/collector"
	httpapi "github.com/redhatinsights/rhc/internal/http"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/version"
)

// FIXME: Make these configurable (use the values from "rhc configure")
const (
	ingressUrl         = "https://cert.console.redhat.com/api/ingress/v1/upload"
	clientCertPath     = "/etc/pki/consumer/cert.pem"
	clientKeyPath      = "/etc/pki/consumer/key.pem"
	collectorTmpParent = "/var/tmp"
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

	workDir := filepath.Join(tmpDir, "workdir")
	if err := os.Mkdir(workDir, 0700); err != nil {
		return fmt.Errorf("failed to create collector working directory: %w", err)
	}

	if err = executeCollector(config, workDir); err != nil {
		return err
	}
	archivePath, err := getArchivePath(workDir, tmpDir)
	if err != nil {
		return err
	}
	if err = uploadArchive(archivePath, config); err != nil {
		return err
	}
	return nil
}

// createTmpDir creates a private temporary directory for a collector run.
func createTmpDir() (string, error) {
	tmpDir, err := os.MkdirTemp(collectorTmpParent, "rhc-")
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

// executeCollector runs the specified collector binary with the collect argument in workDir as the working directory.
// The collector process is executed as the user and group defined in the collector configuration.
// Returns an error if the command execution fails.
func executeCollector(config collector.Config, workDir string) error {
	collectorPath := fmt.Sprintf("/usr/libexec/rhc/collectors/%s", config.ID)

	sysProcAttr, err := collector.ResolveUserGroupAttr(config.User, config.Group, user.Lookup, user.LookupGroup)
	if err != nil {
		return fmt.Errorf("failed to resolve user and group: %w", err)
	}

	slog.Info("executing collector as configured user/group", "collector", config.ID, "user", config.User, "group", config.Group)

	cmd := exec.Command(collectorPath, "collect")
	cmd.Dir = workDir
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

// getArchivePath creates a compressed .tar.xz archive from the working directory.
// Returns the archive file path or an error if compression fails.
func getArchivePath(workDir, outputDir string) (string, error) {
	archivePath, err := collector.GetArchive(workDir, outputDir)
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
