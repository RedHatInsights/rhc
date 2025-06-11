package main

import (
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
)

const (
	colorGreen  = "\u001B[32m"
	colorYellow = "\u001B[33m"
	colorRed    = "\u001B[31m"
	colorReset  = "\u001B[0m"
)

const smallIndent = " "
const mediumIndent = "  "

// userInterfaceSettings manages standard output preference.
// It tracks colors, icons and machine-readable output (e.g. json).
//
// It is instantiated via uiSettings by calling configureUISettings.
type userInterfaceSettings struct {
	// isMachineReadable describes the machine-readable mode (e.g., `--format json`)
	isMachineReadable bool
	// isRich describes the ability to display colors and animations
	isRich    bool
	iconOK    string
	iconInfo  string
	iconError string
}

// uiSettings is an instance that keeps actual data of output preference.
//
// It is managed by calling the configureUISettings method.
var uiSettings = userInterfaceSettings{}

const symbolOK string = "✓"
const symbolInfo string = "●"
const symbolError string = "𐄂"

// configureUISettings is called by the CLI library when it loads up.
// It sets up the uiSettings object.
func configureUISettings(ctx *cli.Context) {
	if ctx.Bool("no-color") {
		uiSettings = userInterfaceSettings{
			isRich:            false,
			isMachineReadable: false,
			iconOK:            symbolOK,
			iconInfo:          symbolInfo,
			iconError:         symbolError,
		}
	} else {
		uiSettings = userInterfaceSettings{
			isRich:            true,
			isMachineReadable: false,
			iconOK:            colorGreen + symbolOK + colorReset,
			iconInfo:          colorYellow + symbolInfo + colorReset,
			iconError:         colorRed + symbolError + colorReset,
		}
	}
}

// showProgress calls function and, when it is possible display spinner with
// some progress message.
func showProgress(
	progressMessage string,
	function func() error,
	prefixSpaces string,
) error {
	var s *spinner.Spinner
	if uiSettings.isRich {
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
	if config.LogLevel <= slog.LevelDebug {
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
	if hasPriorityErrors(errorMessages, config.LogLevel) {
		if !uiSettings.isMachineReadable {
			fmt.Println()
			fmt.Printf("The following errors were encountered during %s:\n\n", action)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "TYPE\tSTEP\tERROR\t")
			for step, logMsg := range errorMessages {
				if logMsg.level >= config.LogLevel {
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
	if !uiSettings.isMachineReadable {
		fmt.Printf(format, a...)
	}
}
