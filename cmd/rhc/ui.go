package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/sys/unix"
)

const (
	colorGreen  = "\u001B[32m"
	colorYellow = "\u001B[33m"
	colorRed    = "\u001B[31m"
	colorReset  = "\u001B[0m"
)

var indent = indentSet{
	Small:  " ",
	Medium: "  ",
}

type indentSet struct {
	Small  string
	Medium string
}

type iconSet struct {
	Ok      string
	Info    string
	Error   string
	Warning string
}

var icons iconSet
var animationsEnabled bool
var outputMachineReadable bool

func init() {
	// Default to colored and animated terminal experience
	configureOutput(true, true, false)
}

// isInteractive returns true if the standard output is a terminal.
func isInteractive() bool {
	return isTerminal(os.Stdout.Fd())
}

// isTerminal returns true if the file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// configureOutput sets up a global state for communicating information to the user.
// 'animated' enables transient animations such as spinners,
// 'colored' enables ANSI colors,
// 'machineReadable' is true for JSON or similar machine-readable formats.
func configureOutput(animated bool, colored bool, machineReadable bool) {
	animationsEnabled = animated
	outputMachineReadable = machineReadable

	icons = iconSet{
		Ok:      "✓",
		Info:    "●",
		Warning: "!",
		Error:   "𐄂",
	}
	if colored {
		icons.Ok = colorGreen + icons.Ok + colorReset
		icons.Info = colorYellow + icons.Info + colorReset
		icons.Error = colorRed + icons.Error + colorReset
		icons.Warning = colorRed + icons.Warning + colorReset
	}
}

// isOutputMachineReadable returns true when the output should be formatted as
// JSON or similar machine-readable format.
func isOutputMachineReadable() bool {
	return outputMachineReadable
}

// areAnimationsEnabled returns true when transient output animations are enabled.
func areAnimationsEnabled() bool {
	return animationsEnabled
}

// printf acts as a no-op if the output is machine-readable.
// Otherwise, passes the input to fmt.Printf.
func printf(
	format string,
	a ...interface{},
) {
	if isOutputMachineReadable() {
		return
	}
	fmt.Printf(format, a...)
}

// withSpinner calls a function and displays a spinner with an explanatory message.
// The spinner is not displayed when animations are disabled.
func withSpinner(
	function func() error,
	prefix string,
	message string,
) error {
	var s *spinner.Spinner
	if areAnimationsEnabled() {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = prefix + "["
		s.Suffix = "]" + " " + message
		s.Start()
		// Stop the spinner when the function exits.
		defer func() { s.Stop() }()
	}
	return function()
}

// printJSON prints the given data as JSON to stdout.
// When marshaling of data fails, then error is returned.
func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// printTable prints data in a table format using tabwriter.
// headers are the column headers, rows contain the data for each row.
func printTable(headers []string, rows [][]string) {
	if isOutputMachineReadable() {
		return
	}

	if len(rows) == 0 {
		fmt.Println("No data is available to print.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func(w *tabwriter.Writer) {
		err := w.Flush()
		if err != nil {
			slog.Debug("Unable to flush tabwriter", "error", err)
			return
		}
	}(w)

	for i, header := range headers {
		if i == len(headers)-1 {
			_, _ = fmt.Fprint(w, header)
		} else {
			_, _ = fmt.Fprint(w, header+"\t")
		}
	}
	_, _ = fmt.Fprintln(w)

	for _, row := range rows {
		for i, cell := range row {
			if i == len(row)-1 {
				_, _ = fmt.Fprint(w, cell)
			} else {
				_, _ = fmt.Fprint(w, cell+"\t")
			}
		}
		_, _ = fmt.Fprintln(w)
	}
}
