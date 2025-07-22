package main

import (
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
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
	collectors, err := readAllCollectors()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read collectors: %v", err), 1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if !uiSettings.isMachineReadable {
		_, _ = fmt.Fprintln(w, "ID\tNAME\t")
	}

	if !uiSettings.isMachineReadable {
		for _, collectorInfo := range collectors {
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
