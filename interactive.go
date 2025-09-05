package main

import (
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/conf"
	"github.com/redhatinsights/rhc/internal/ui"
)

// showProgress calls function and, when it is possible display spinner with
// some progress message.
func showProgress(
	progressMessage string,
	function func() error,
	prefixSpaces string,
) error {
	var s *spinner.Spinner
	if ui.IsOutputRich() {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = prefixSpaces + "["
		s.Suffix = "]" + progressMessage
		s.Start()
		// Stop spinner after running function
		defer func() { s.Stop() }()
	}
	return function()
}

// showTimeDuration shows table with duration of each sub-action
func showTimeDuration(durations map[string]time.Duration) {
	if conf.Config.LogLevel <= slog.LevelDebug {
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "STEP\tDURATION\t")
		for step, duration := range durations {
			_, _ = fmt.Fprintf(w, "%v\t%v\t\n", step, duration.Truncate(time.Millisecond))
		}
		_ = w.Flush()
	}
}

// showErrorMessages shows table with all error messages gathered during action
func showErrorMessages(action string, errorMessages map[string]LogMessage) error {
	if hasPriorityErrors(errorMessages, conf.Config.LogLevel) {
		if !ui.IsOutputMachineReadable() {
			fmt.Println()
			fmt.Printf("The following errors were encountered during %s:\n\n", action)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TYPE\tSTEP\tERROR\t")
			for step, logMsg := range errorMessages {
				if logMsg.level >= conf.Config.LogLevel {
					_, _ = fmt.Fprintf(w, "%v\t%v\t%v\n", logMsg.level, step, logMsg.message)
				}
			}
			_ = w.Flush()
			if hasPriorityErrors(errorMessages, slog.LevelError) {
				return cli.Exit("", 1)
			}
		}
	}
	return nil
}

// interactivePrintf is method for printing human-readable output. It suppresses output, when
// machine-readable format is used.
func interactivePrintf(format string, a ...interface{}) {
	if ui.IsOutputMachineReadable() {
		return
	}
	fmt.Printf(format, a...)
}
