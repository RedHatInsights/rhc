package ui

import (
	"fmt"
	"os"
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
	Ok    string
	Info  string
	Error string
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

// ConfigureOutput sets up global state for communicating information to the user.
// 'rich' represents output's ability to display animations or colors,
// 'colored' represents user's preference to display colors, and requires 'rich' to be true,
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
		Ok:    "✓",
		Info:  "●",
		Error: "𐄂",
	}
	if rich && colored {
		Icons.Ok = colorGreen + Icons.Ok + colorReset
		Icons.Info = colorYellow + Icons.Info + colorReset
		Icons.Error = colorRed + Icons.Error + colorReset
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
