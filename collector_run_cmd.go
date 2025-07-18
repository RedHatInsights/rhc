package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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

// readCollectorConfig tries to read collector information from the configuration .toml file
func readCollectorConfig(filePath string) (*CollectorInfo, error) {
	var collectorInfo CollectorInfo
	_, err := toml.DecodeFile(filePath, &collectorInfo)
	if err != nil {
		return nil, err
	}
	return &collectorInfo, nil
}

// Run

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

// collectData tries to run a given collector
func collectData(args ...string) (*string, error) {
	var outBuffer bytes.Buffer
	collectorCommand := args[0]
	tempDir := args[1]
	cmd := exec.Command(collectorCommand, args[2:]...)
	cmd.Dir = tempDir
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, cli.Exit(fmt.Sprintf("failed to run collector '%s': %v", collectorCommand, err), 1)
	}

	stdOut := outBuffer.String()

	return &stdOut, nil
}

// archiveData tries to run a given archiver
func archiveData(args ...string) (*string, error) {
	var outBuffer bytes.Buffer
	archiverCommand := args[0]
	tempDir := args[1]
	cmd := exec.Command(archiverCommand, args[2:]...)
	cmd.Dir = tempDir
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, cli.Exit(fmt.Sprintf("failed to run archiver '%s': %v", archiverCommand, err), 1)
	}

	stdOut := outBuffer.String()

	return &stdOut, nil
}

func uploadData(args ...string) (*string, error) {
	var outBuffer bytes.Buffer
	uploaderCommand := args[0]
	tempDir := args[1]
	archivePath := args[2]
	cmd := exec.Command(uploaderCommand, archivePath)
	cmd.Dir = tempDir
	cmd.Stdout = &outBuffer
	err := cmd.Run()

	if err != nil {
		return nil, cli.Exit(fmt.Sprintf("failed to run uploader '%s': %v", uploaderCommand, err), 1)
	}

	stdOut := outBuffer.String()
	return &stdOut, nil
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
	collectorConfig.configFilePath = collectorConfigfilePath

	// Create a temporary directory, where collector will collect data
	tempDir, err := os.MkdirTemp("/tmp", fmt.Sprintf("rhc-collector-%s-*", collectorId))
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to create temporary directory: %v", err), 1)
	}
	// If --keep is not used, then delete temporary directory at the end
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

// runCollector tries to run given collector
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

	return &collectorOutput.CollectionDirectory, nil
}
