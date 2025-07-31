package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"log/slog"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"
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

		// Try to get the collector version from version command
		version, err := runVersionCommand(collectorConfig)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get collector version: %v", err))
			_, _ = fmt.Fprintf(w, "Version:\t%s\n", notDefinedValue)
		} else {
			_, _ = fmt.Fprintf(w, "Version:\t%s\n", *version)
		}

		if collectorConfig.Meta.Feature != "" {
			_, _ = fmt.Fprintf(w, "Feature:\t%s\n\n", collectorConfig.Meta.Feature)
		} else {
			_, _ = fmt.Fprintf(w, "Feature:\t%s\n\n", notDefinedValue)
		}

		// Try to get the last run from the cache file
		lastTime, err := readLastRun(collectorConfig)
		if err != nil {
			_, _ = fmt.Fprintf(w, "Last run:\t%s\n", notDefinedValue)
		} else {
			lastRunStr := lastTime.Format("Mon 2006-01-02 15:04 MST")
			_, _ = fmt.Fprintf(w, "Last run:\t%s\n", lastRunStr)
		}

		// Try to get the next run from the systemd D-Bus API
		nextTime, err := getCollectorTimerNextTime(collectorConfig)
		if err != nil {
			_, _ = fmt.Fprintf(w, "Next run:\t%s\n\n", notDefinedValue)
		} else {
			zeroTime := time.Unix(0, 0)
			if *nextTime == zeroTime {
				_, _ = fmt.Fprintf(w, "Next run:\t%s\n\n", notDefinedValue)
			} else {
				nowTime := time.Now()
				delay := nextTime.Sub(nowTime)
				nextTimeStr := nextTime.Format("Mon 2006-01-02 15:04 MST")
				_, _ = fmt.Fprintf(w, "Next run:\t%s (in %s)\n\n",
					nextTimeStr, delay.Round(time.Second).String())
			}
		}

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
