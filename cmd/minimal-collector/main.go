package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/redhatinsights/rhc/pkg/exitcode"
)

func main() {
	if len(os.Args) != 2 {
		slog.Error("usage: com.redhat.minimal collect")
		os.Exit(exitcode.Usage)
	}

	command := os.Args[1]
	if command != "collect" {
		slog.Error("unknown command", "command", command)
		os.Exit(exitcode.Usage)
	}

	// Use current working directory as output directory (set by rhc-collector)
	outputDir, err := os.Getwd()
	if err != nil {
		slog.Error("failed to get working directory", "error", err)
		os.Exit(exitcode.Err)
	}

	if err := run(outputDir); err != nil {
		slog.Error("minimal-collector failed", "error", err)
		os.Exit(exitcode.Err)
	}
}

func run(outputDir string) error {
	slog.Info("starting minimal collector", "outputDir", outputDir)

	dataDir, metaDataDir, err := createArchiveStructure(outputDir)
	if err != nil {
		return fmt.Errorf("failed to create archive structure: %w", err)
	}
	if err = writeVersionInfo(dataDir, metaDataDir); err != nil {
		return fmt.Errorf("failed to write version info: %w", err)
	}
	if err = writeBranchInfo(dataDir, metaDataDir); err != nil {
		return fmt.Errorf("failed to write branch info: %w", err)
	}
	if err = writeCanonicalFacts(dataDir, metaDataDir); err != nil {
		return fmt.Errorf("failed to write canonical facts: %w", err)
	}

	slog.Info("minimal collector completed successfully")
	return nil
}
