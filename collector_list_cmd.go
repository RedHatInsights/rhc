package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

func beforeCollectorListAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	return checkForUnknownArgs(ctx)
}

// collectorListAction tries to display all installed rhc collectors
func collectorListAction(ctx *cli.Context) (err error) {
	files, err := os.ReadDir(collectorDirName)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read directory %s: %v", collectorDirName, err), 1)
	}

	var collectors []CollectorInfo
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if !uiSettings.isMachineReadable {
		_, _ = fmt.Fprintln(w, "ID\tNAME\t")
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".toml" {
			slog.Debug(fmt.Sprintf("file '%s' is not a TOML file, skipping", file.Name()))
			continue
		}

		var collectorInfo *CollectorInfo
		filePath := filepath.Join(collectorDirName, file.Name())

		collectorInfo, err = readCollectorConfig(filePath)
		if err != nil {
			slog.Warn(fmt.Sprintf("failed to read TOML file %s: %v\n", file.Name(), err))
			continue
		}

		collectorInfo.id, _ = strings.CutSuffix(file.Name(), ".toml")

		if uiSettings.isMachineReadable {
			collectors = append(collectors, *collectorInfo)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%v\t\n", collectorInfo.id, collectorInfo.Meta.Name)
		}

	}

	if uiSettings.isMachineReadable {
		data, err := json.MarshalIndent(collectors, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		_ = w.Flush()
	}

	return nil
}
