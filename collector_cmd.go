package main

import (
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// Run

func beforeCollectorRunAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorRunAction(ctx *cli.Context) (err error) {

	return nil
}

// Info

func beforeCollectorInfoAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorInfoAction(ctx *cli.Context) (err error) {

	return nil
}

// List

func beforeCollectorListAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	return checkForUnknownArgs(ctx)
}

const (
	collectorDirName = "/usr/lib/rhc/collector.d"
)

type CollectorInfo struct {
	Id   string `json:"id"`
	Info struct {
		Name    string `json:"name" toml:"name"`
		Feature string `json:"feature,omitempty" toml:"feature,omitempty"`
	} `json:"meta" toml:"meta"`
	Exec struct {
		Command         string `json:"command" toml:"command"`
		UploaderCommand string `json:"uploader" toml:"uploader"`
		ContentType     string `json:"content_type" toml:"content_type"`
	} `json:"exec" toml:"exec"`
	Systemd struct {
		Service string `json:"service" toml:"service"`
		Timer   string `json:"timer" toml:"timer"`
	} `json:"systemd" toml:"systemd"`
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

		collectorInfo.Id, _ = strings.CutSuffix(file.Name(), ".toml")

		if uiSettings.isMachineReadable {
			collectors = append(collectors, *collectorInfo)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%v\t\n", collectorInfo.Id, collectorInfo.Info.Name)
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

// Timers

func beforeCollectorTimersAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorTimersAction(ctx *cli.Context) (err error) {
	return nil
}

// Enable

func beforeCollectorEnableAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorEnableAction(ctx *cli.Context) (err error) {
	return nil
}

// Disable

func beforeCollectorDisableAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorDisableAction(ctx *cli.Context) (err error) {
	return nil
}
