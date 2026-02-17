package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/sys/unix"
)

const (
	ColorGreen  = "\x1b[0032m"
	ColorYellow = "\x1b[0033m"
	ColorRed    = "\x1b[0031m"
	ColorGrey   = "\x1b[0090m"
	DefaultText = "\x1b[0039m"
	ColorReset  = "\x1b[0000m"
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
	Ok       string
	Info     string
	Error    string
	Enabled  string
	Disabled string
}

var Icons icons
var isOutputRich bool
var isColored bool
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

// ConfigureOutput sets up global state for communicating information to the user.
// 'rich' represents output's ability to display animations or colors,
// 'colored' represents user's preference to display colors, and requires 'rich' to be true,
// 'machine' is true when the output is formatted as JSON or similar machine-readable format.
func ConfigureOutput(rich bool, colored bool, machine bool) {
	if machine {
		isOutputMachineReadable = true
		isOutputRich = false
		isColored = false
	}
	if rich {
		isOutputRich = true
	} else {
		isOutputRich = false
	}
	if colored {
		isColored = true
	} else {
		isColored = false
	}

	Icons = icons{
		Ok:       "✓",
		Info:     "●",
		Error:    "𐄂",
		Enabled:  "✓",
		Disabled: "x",
	}
	if rich && colored {
		Icons.Ok = ColorGreen + Icons.Ok + ColorReset
		Icons.Info = ColorYellow + Icons.Info + ColorReset
		Icons.Error = ColorRed + Icons.Error + ColorReset
		Icons.Enabled = ColorGreen + Icons.Enabled + ColorReset
		Icons.Disabled = ColorRed + Icons.Disabled + ColorReset
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

// IsOutputColored returns true when the output should be displayed with colors.
func IsOutputColored() bool {
	return isColored
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

// Spinner calls a function and displays a spinner with explanatory message.
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
