package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func beforeCollectorInfoAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	if ctx.Args().Len() != 1 {
		return fmt.Errorf("error: expected 1 argument of collector name, got %d", ctx.Args().Len())
	}
	return nil
}

func collectorInfoAction(ctx *cli.Context) (err error) {
	const notDefinedValue = "-"
	// TODO: Get this path from systemd
	const systemdDirectory = "/usr/lib/systemd/system/"

	collectorId := ctx.Args().First()

	fileName := collectorId + ".toml"
	filePath := filepath.Join(collectorDirName, fileName)
	collectorConfig, err := readCollectorConfig(filePath)
	collectorConfig.id = collectorId
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read TOML file %s: %v", fileName, err), 1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if !uiSettings.isMachineReadable {
		_, _ = fmt.Fprintf(w, "Name:\t%s\n", collectorConfig.Meta.Name)
		if collectorConfig.Meta.Feature != "" {
			_, _ = fmt.Fprintf(w, "Feature:\t%s\n\n", collectorConfig.Meta.Feature)
		} else {
			_, _ = fmt.Fprintf(w, "Feature:\t%s\n\n", notDefinedValue)
		}

		// TODO: Get last run from systemd
		_, _ = fmt.Fprintf(w, "Last run:\t%s\n", notDefinedValue)
		// TODO: Get next run from systemd
		_, _ = fmt.Fprintf(w, "Next run:\t%s\n\n", notDefinedValue)

		_, _ = fmt.Fprintf(w, "Config:\t%s\n", filePath)
		serviceFilePath := filepath.Join(systemdDirectory, collectorConfig.Systemd.Service)
		_, _ = fmt.Fprintf(w, "Service:\t%s\n", serviceFilePath)
		timerFilePath := filepath.Join(systemdDirectory, collectorConfig.Systemd.Timer)
		_, _ = fmt.Fprintf(w, "Timer:\t%s\n", timerFilePath)
		_ = w.Flush()
	} else {
		// TODO: implement JSON output containing all info (version, last run, next run, etc.)
		data, err := json.MarshalIndent(collectorConfig, "", "    ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	}
	return nil
}
