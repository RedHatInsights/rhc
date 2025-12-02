package main

import (
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/conf"
	"github.com/redhatinsights/rhc/internal/ui"
)

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
func showErrorMessages(action string, errorMessages map[string]string) error {
	if conf.Config.LogLevel > slog.LevelError || len(errorMessages) == 0 {
		return nil
	}
	if !ui.IsOutputMachineReadable() {
		fmt.Println()
		fmt.Printf("The following errors were encountered during %s:\n\n", action)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "STEP\tERROR\t")
		for step, errMsg := range errorMessages {
			_, _ = fmt.Fprintf(w, "%v\t%v\n", step, errMsg)
		}
		_ = w.Flush()
		// Direct users to the journal for full details
		fmt.Println()
		fmt.Println("Please see 'journalctl -t rhc' for full details.")
		if len(errorMessages) > 0 {
			return cli.Exit("", 1)
		}
	}
	return nil
}
