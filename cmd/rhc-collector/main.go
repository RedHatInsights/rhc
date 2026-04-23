package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/redhatinsights/rhc/internal/collector"
	httpapi "github.com/redhatinsights/rhc/internal/http"
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
		slog.Error("usage: rhc-collector COLLECTOR-ID COMMAND")
		os.Exit(ExitCodeUsage)
	}
	collectorId, command := os.Args[1], os.Args[2]
	slog.Info("starting rhc-collector", slog.String("id", collectorId))
	if err := run(collectorId, command); err != nil {
		slog.Error("rhc-collector exited with error", "error", err)
		os.Exit(ExitCodeErr)
	}
}

func run(collectorId, command string) error {
	tmpDir, err := createTmpDir()
	if err != nil {
		return err
	}
	defer cleanup(tmpDir)
	config, err := getConfig(collectorId)
	if err != nil {
		return err
	}
	if err = executeCollector(command, tmpDir); err != nil {
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

// createTmpDir creates a temporary directory for collector output in rhcTmpDir.
// Returns the directory path or an error if creation fails.
func createTmpDir() (string, error) {
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

// executeCollector runs the specified collector command and stores output in tmpDir.
// Returns an error if the command execution fails.
func executeCollector(command, tmpDir string) error {
	// TODO: Run collector as specific user and group (defined in config of collector)
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("%s %q", command, tmpDir))
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
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
