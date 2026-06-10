package main

import (
	"log/slog"
	"os"

	"github.com/redhatinsights/rhc/pkg/exitcode"
)

func main() {
	if len(os.Args) < 2 {
		slog.Error("usage: data-collector OUTPUT_DIRECTORY")
		os.Exit(exitcode.Usage)
	}
	outputDir := os.Args[1]

	if err := runDataCollection(outputDir); err != nil {
		slog.Error("data collector failed", "error", err)
		os.Exit(exitcode.Err)
	}
}
