package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	bashFilePath = "/bin/bash"
)

type CollectorOutput struct {
	CollectionDirectory string `json:"collection_directory"`
	CollectorError      string `json:"collector_error,omitempty"`
}

type ArchiverOutput struct {
	Archive       string `json:"archive"`
	ArchiverError string `json:"archiver_error,omitempty"`
}

type UploaderOutput struct {
	Target        string `json:"target"`
	UploaderError string `json:"uploader_error,omitempty"`
}

// beforeCollectorRunAction validates the collector name argument and ensures format option setup via setupFormatOption.
// Returns an error if validation or setup fails.
func beforeCollectorRunAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	if ctx.Args().Len() != 1 {
		return fmt.Errorf("error: expected 1 argument of collector name, got %d", ctx.Args().Len())
	}
	return nil
}

// collectorRunAction run given collector, archiver and uploader according
// the collector configuration file.
func collectorRunAction(ctx *cli.Context) (err error) {
	collectorId := ctx.Args().First()
	keepArtifacts := ctx.Bool("keep")

	fileName := collectorId + ".toml"
	collectorConfigfilePath := filepath.Join(collectorDirName, fileName)

	collectorConfig, err := readCollectorConfig(collectorConfigfilePath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read collector configuration file %s: %v", fileName, err), 1)
	}

	// Create a temporary directory, where collector will collect data
	tempDir, err := os.MkdirTemp("/tmp", fmt.Sprintf("rhc-collector-%s-*", collectorId))
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create temporary directory: %v", err), 1)
	}
	// If --keep is not used, then delete the temporary directory at the end
	if !keepArtifacts {
		defer func() {
			err := os.RemoveAll(tempDir)
			if err != nil {
				slog.Warn(fmt.Sprintf("failed to remove temporary directory %s: %v", tempDir, err))
			}
		}()
	}

	// Create a working directory inside the temporary directory according name of rhc collector
	workingDir := filepath.Join(tempDir, collectorId)
	err = os.Mkdir(workingDir, 0700)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create working directory %s: %v", workingDir, err), 1)
	}

	// Run collector
	collectionDirectory, err := runCollector(collectorConfig, workingDir)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to run collector '%s': %v", collectorId, err), 1)
	}

	// Archive & compress collected data
	archiveFilePath, err := archiveCollectedData(collectorConfig, &tempDir, collectionDirectory)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to run archiver: %s", err), 1)
	}

	// Upload data
	_, err = uploadArchivedData(collectorConfig, &tempDir, archiveFilePath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to run uploader: %s", err), 1)
	}

	return nil
}

// runCollector tries to run the given collector
func runCollector(collectorConfig *CollectorInfo, workingDir string) (*string, error) {

	collectorCommand := collectorConfig.Exec.Collector.Command
	if collectorCommand == "" {
		msg := fmt.Sprintf("collector command is not set in %s", collectorConfig.configFilePath)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	data, err := showProgressArgs(" Collecting data...", collectData, mediumIndent, collectorCommand, workingDir)
	if err != nil {
		msg := fmt.Sprintf("failed to collect data: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	var collectorOutput CollectorOutput
	err = json.Unmarshal([]byte(*data), &collectorOutput)
	if err != nil {
		msg := fmt.Sprintf("failed to parse collector output: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	interactivePrintf("%v[%s] Collected data to %s\n", mediumIndent, uiSettings.iconOK, workingDir)

	err = writeTimeStampOfLastRun(collectorConfig)
	if err != nil {
		msg := fmt.Sprintf("failed to write last run timestamp: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	return &collectorOutput.CollectionDirectory, nil
}

// uploadArchivedData tries to upload archive probably to some server or somewhere else. It is up to uploader ;-)
func uploadArchivedData(collectorConfig *CollectorInfo, tempDir *string, archiveFilePath *string) (*string, error) {
	uploaderCommand := collectorConfig.Exec.Uploader.Command
	if uploaderCommand == "" {
		msg := fmt.Sprintf("uploader file is not set in %s", collectorConfig.configFilePath)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	data, err := showProgressArgs(
		" Uploading data...",
		uploadData,
		mediumIndent,
		uploaderCommand,
		*tempDir,
		*archiveFilePath,
	)
	if err != nil {
		msg := fmt.Sprintf("failed to upload data: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	var uploaderOutput UploaderOutput
	err = json.Unmarshal([]byte(*data), &uploaderOutput)
	if err != nil {
		msg := fmt.Sprintf("failed to parse uploader output: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	interactivePrintf(
		"%v[%s] Uploaded archive %s to %s\n",
		mediumIndent,
		uiSettings.iconOK,
		*archiveFilePath,
		uploaderOutput.Target,
	)

	return nil, nil
}

// archiveCollectedData tries to run given archiver
func archiveCollectedData(collectorConfig *CollectorInfo, tempDir *string, collectionDir *string) (*string, error) {
	archiverCommand := collectorConfig.Exec.Archiver.Command
	if archiverCommand == "" {
		msg := fmt.Sprintf("archiver command is not set in %s", collectorConfig.configFilePath)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	data, err := showProgressArgs(
		fmt.Sprintf(" Archiving directory '%s'...", *collectionDir),
		archiveData,
		mediumIndent,
		archiverCommand,
		*tempDir,
		*collectionDir,
	)
	if err != nil {
		msg := fmt.Sprintf("failed to archive collected data: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	var archiverOutput ArchiverOutput
	err = json.Unmarshal([]byte(*data), &archiverOutput)
	if err != nil {
		msg := fmt.Sprintf("failed to parse arhiver output: %v", err)
		slog.Error(msg)
		return nil, fmt.Errorf(msg)
	}

	interactivePrintf(
		"%v[%s] Archived directory %s to %s\n",
		mediumIndent,
		uiSettings.iconOK,
		*collectionDir,
		archiverOutput.Archive,
	)

	return &archiverOutput.Archive, nil
}
