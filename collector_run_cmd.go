package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

const (
	bashFilePath            = "/bin/bash"
	collectorStdoutFileName = "collector_stdout"
	collectorStderrFileName = "collector_stderr"
	uploaderStdoutFileName  = "uploader_stdout"
	uploaderStderrFileName  = "uploader_stderr"
)

type CollectorOutput struct {
	CollectedDataFilePath string `json:"collector_output"`
	MimeType              string `json:"mime_type"`
	CollectorError        string `json:"collector_error,omitempty"`
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

// collectorRunAction run given collector and uploader according
// the collector configuration file.
func collectorRunAction(ctx *cli.Context) (err error) {
	collectorId := ctx.Args().First()
	keepArtifacts := ctx.Bool("keep")
	noUpload := ctx.Bool("no-upload")

	if noUpload {
		keepArtifacts = true
	}

	fileName := collectorId + ".toml"
	collectorConfigfilePath := filepath.Join(collectorConfigDirPath, fileName)

	collectorConfig, err := readCollectorConfig(collectorConfigfilePath)
	if err != nil {
		return fmt.Errorf("failed to read collector configuration file %s: %v", fileName, err)
	}

	// Try to change the current user when needed
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
	collectedDataFilePath, err := runCollector(collectorConfig, &tempDir, workingDir)
	if err != nil {
		interactivePrintf(
			"%v[%s] Failed to collect data in directory %s\n",
			mediumIndent,
			uiSettings.iconError,
			workingDir,
		)
		interactivePrintf(
			"%v[ ] Skipping uploading the collected data\n\n",
			mediumIndent,
		)
		return fmt.Errorf("failed to run collector '%s': %v", collectorId, err)
	}

	// Upload data
	if noUpload {
		interactivePrintf(
			"%v[ ] Skipping uploading the collected data (enforced by CLI option)\n\n",
			mediumIndent,
		)
	} else {
		_, err = uploadCollectedData(collectorConfig, &tempDir, collectedDataFilePath)
		if err != nil {
			interactivePrintf(
				"%v[%s] Failed to upload %s: %s\n\n",
				mediumIndent,
				uiSettings.iconError,
				*collectedDataFilePath,
				err,
			)
			return fmt.Errorf("failed to run uploader: %s", err)
		}
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

	err = writeTimeStampOfLastRun(collectorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to write last run timestamp: %v", err)
	}

	interactivePrintf("%v[%s] Collected data to %s\n", mediumIndent, uiSettings.iconOK, collectorOutput.CollectedDataFilePath)

	return &collectorOutput.CollectedDataFilePath, nil
}

// uploadCollectedData tries to upload collected data to some server. It is up to the uploader ;-)
func uploadCollectedData(collectorConfig *CollectorInfo, tempDir *string, dataFilePath *string) (*string, error) {
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
		*dataFilePath,
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
		"%v[%s] Uploaded collected data %s to %s\n",
		mediumIndent,
		uiSettings.iconOK,
		*dataFilePath,
		uploaderOutput.Target,
	)

	return nil, nil
}
