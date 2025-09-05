package ui

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
)

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
