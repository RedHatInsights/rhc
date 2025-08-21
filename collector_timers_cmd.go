package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"text/tabwriter"
	"time"
)

func beforeCollectorTimersAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorTimersAction(ctx *cli.Context) (err error) {
	collectors, err := readAllCollectors()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read collectors: %v", err), 1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if !uiSettings.isMachineReadable {
		_, _ = fmt.Fprintln(w, "ID\tLAST\tNEXT\t")
	}

	for _, collectorInfo := range collectors {
		var lastTimeStr, nextTimeStr string
		lastTime, err := readLastRun(&collectorInfo)
		if err != nil {
			lastTimeStr = notDefinedValue
		} else {
			lastTimeStr = lastTime.Format("Mon 2006-01-02 15:04 MST")
		}

		nextTime, err := getCollectorTimerNextTime(&collectorInfo)
		if err != nil {
			nextTimeStr = notDefinedValue
		} else {
			zeroTime := time.Unix(0, 0)
			if *nextTime == zeroTime {
				nextTimeStr = notDefinedValue
			} else {
				nextTimeStr = nextTime.Format("Mon 2006-01-02 15:04 MST")
			}
		}

		if !uiSettings.isMachineReadable {
			_, _ = fmt.Fprintf(w, "%s\t%v\t%v\t\n",
				collectorInfo.id, lastTimeStr, nextTimeStr)
		}
	}

	if !uiSettings.isMachineReadable {
		_ = w.Flush()
	}

	return nil
}
