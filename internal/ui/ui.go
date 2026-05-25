package ui

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

var Indent = indent{
	Small:  " ",
	Medium: "  ",
}

type indent struct {
	Small  string
	Medium string
}

type icons struct {
	Ok      string
	Info    string
	Error   string
	Warning string
}

var Icons icons
var isOutputRich bool
var isOutputMachineReadable bool

func init() {
	// Default to colored and animated terminal experience
	ConfigureOutput(true, true, false)
}

// IsInteractive returns true if the standard output is a terminal.
func IsInteractive() bool {
	return isTerminal(os.Stdout.Fd())
}

// isTerminal returns true if the file descriptor is a terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// ConfigureOutput sets up a global state for communicating information to the user.
// 'rich' represents the output's ability to display animations or colors,
// 'colored' represents the user's preference to display colors, and requires 'rich' to be true,
// 'machine' is true when the output is formatted as JSON or similar machine-readable format.
func ConfigureOutput(rich bool, colored bool, machine bool) {
	if machine {
		isOutputMachineReadable = true
		isOutputRich = false
	}
	if rich {
		isOutputRich = true
	}

	Icons = icons{
		Ok:      "✓",
		Info:    "●",
		Warning: "!",
		Error:   "𐄂",
	}
	if rich && colored {
		Icons.Ok = colorGreen + Icons.Ok + colorReset
		Icons.Info = colorYellow + Icons.Info + colorReset
		Icons.Error = colorRed + Icons.Error + colorReset
		Icons.Warning = colorRed + Icons.Warning + colorReset
	}
}

// IsOutputMachineReadable returns true when the output should be formatted as
// JSON or similar machine-readable format.
func IsOutputMachineReadable() bool {
	return isOutputMachineReadable
}

// IsOutputRich returns true when the output should be displayed in a terminal
// supporting animations and colors.
func IsOutputRich() bool {
	return isOutputRich
}

// Printf acts as a no-op if the output is machine-readable.
// Otherwise, passes the input to fmt.Printf.
func Printf(
	format string,
	a ...interface{},
) {
	if IsOutputMachineReadable() {
		return
	}
	fmt.Printf(format, a...)
}

// Spinner calls a function and displays a spinner with an explanatory message.
// The spinner is not displayed if the output isn't a rich terminal.
func Spinner(
	function func() error,
	prefix string,
	message string,
) error {
	var s *spinner.Spinner
	if IsOutputRich() {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = prefix + "["
		s.Suffix = "]" + " " + message
		s.Start()
		// Stop the spinner when the function exits.
		defer func() { s.Stop() }()
	}
	return function()
}

// PrintJSON prints the given data as JSON to stdout.
// When marshaling of data fails, then error is returned.
func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintTable prints data in a table format using tabwriter.
// headers are the column headers, rows contain the data for each row.
func PrintTable(headers []string, rows [][]string) {
	if IsOutputMachineReadable() {
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
