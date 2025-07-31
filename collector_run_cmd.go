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
	bashFilePath            = "/bin/bash"
	collectorStdoutFileName = "collector_stdout"
	collectorStderrFileName = "collector_stderr"
	archiverStdoutFileName  = "archiver_stdout"
	archiverStderrFileName  = "archiver_stderr"
	uploaderStdoutFileName  = "uploader_stdout"
	uploaderStderrFileName  = "uploader_stderr"
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
		return fmt.Errorf("failed to read collector configuration file %s: %v", fileName, err)
	}

	// Try to change current user, when needed
	err = changeCurrentUser(collectorConfig)
	if err != nil {
		return fmt.Errorf("failed to change current user: %v", err)
	}

	// Create a temporary directory, where collector will collect data
	tempDir, err := os.MkdirTemp("/tmp", fmt.Sprintf("rhc-collector-%s-*", collectorId))
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %v", err)
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
		return fmt.Errorf("failed to create working directory %s: %v", workingDir, err)
	}

	// Run collector
	collectionDirectory, err := runCollector(collectorConfig, &tempDir, workingDir)
	if err != nil {
		interactivePrintf(
			"%v[%s] Failed to collect data in directory %s\n",
			mediumIndent,
			uiSettings.iconError,
			workingDir,
		)
		interactivePrintf(
			"%v[ ] Skipping creating the archive\n",
			mediumIndent,
		)
		interactivePrintf(
			"%v[ ] Skipping uploading the archive\n\n",
			mediumIndent,
		)
		return fmt.Errorf("failed to run collector '%s': %v", collectorId, err)
	}

	// Archive & compress collected data
	archiveFilePath, err := archiveCollectedData(collectorConfig, &tempDir, collectionDirectory)
	if err != nil {
		interactivePrintf(
			"%v[%s] Failed to create archive from %s\n",
			mediumIndent,
			uiSettings.iconError,
			*collectionDirectory,
		)
		interactivePrintf(
			"%v[ ] Skipping uploading the archive\n\n",
			mediumIndent,
		)
		return fmt.Errorf("failed to run archiver: %s", err)
	}

	// Upload data
	_, err = uploadArchivedData(collectorConfig, &tempDir, archiveFilePath)
	if err != nil {
		interactivePrintf(
			"%v[%s] Failed to upload archive %s\n\n",
			mediumIndent,
			uiSettings.iconError,
			*archiveFilePath,
		)
		return fmt.Errorf("failed to run uploader: %s", err)
	}

	return nil
}

// runCollector tries to run the given collector
func runCollector(collectorConfig *CollectorInfo, tempDir *string, workingDir string) (*string, error) {

	collectorCommand := collectorConfig.Exec.Collector.Command
	if collectorCommand == "" {
		return nil, fmt.Errorf("collector command is not set in %s", collectorConfig.configFilePath)
	}

	collectorStdoutFilePath := filepath.Join(*tempDir, collectorStdoutFileName)
	collectorStderrFilePath := filepath.Join(*tempDir, collectorStderrFileName)

	stdout, stderr, err := showProgressArgs(" Collecting data...", collectData, mediumIndent, collectorCommand, workingDir)
	// Write stdout and stderr to the files in the temporary directory
	writeCommandOutputsToFiles(&collectorCommand, collectorStdoutFilePath, collectorStderrFilePath, stdout, stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to collect data: %v", err)
	}

	var collectorOutput CollectorOutput
	err = json.Unmarshal([]byte(*stdout), &collectorOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse collector output: %v", err)
	}

	interactivePrintf("%v[%s] Collected data to %s\n", mediumIndent, uiSettings.iconOK, workingDir)

	err = writeTimeStampOfLastRun(collectorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to write last run timestamp: %v", err)
	}

	return &collectorOutput.CollectionDirectory, nil
}

// uploadArchivedData tries to upload archive probably to some server or somewhere else. It is up to the uploader ;-)
func uploadArchivedData(collectorConfig *CollectorInfo, tempDir *string, archiveFilePath *string) (*string, error) {
	uploaderCommand := collectorConfig.Exec.Uploader.Command
	if uploaderCommand == "" {
		return nil, fmt.Errorf("uploader file is not set in %s", collectorConfig.configFilePath)
	}

	uploaderStdoutFilePath := filepath.Join(*tempDir, uploaderStdoutFileName)
	uploaderStderrFilePath := filepath.Join(*tempDir, uploaderStderrFileName)

	stdout, stderr, err := showProgressArgs(
		" Uploading data...",
		uploadData,
		mediumIndent,
		uploaderCommand,
		*tempDir,
		*archiveFilePath,
	)
	// Write stdout and stderr to the files in the temporary directory
	writeCommandOutputsToFiles(&uploaderCommand, uploaderStdoutFilePath, uploaderStderrFilePath, stdout, stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to upload data: %v", err)
	}

	var uploaderOutput UploaderOutput
	err = json.Unmarshal([]byte(*stdout), &uploaderOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uploader output: %v", err)
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
		return nil, fmt.Errorf("archiver command is not set in %s", collectorConfig.configFilePath)
	}

	archiverStdoutFilePath := filepath.Join(*tempDir, archiverStdoutFileName)
	archiverStderrFilePath := filepath.Join(*tempDir, archiverStderrFileName)

	stdout, stderr, err := showProgressArgs(
		fmt.Sprintf(" Archiving directory '%s'...", *collectionDir),
		archiveData,
		mediumIndent,
		archiverCommand,
		*tempDir,
		*collectionDir,
	)
	// Write stdout and stderr to the files in the temporary directory
	writeCommandOutputsToFiles(&archiverCommand, archiverStdoutFilePath, archiverStderrFilePath, stdout, stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to archive collected data: %v", err)
	}

	var archiverOutput ArchiverOutput
	err = json.Unmarshal([]byte(*stdout), &archiverOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to parse arhiver output: %v", err)
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
