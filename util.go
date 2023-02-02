package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// isTerminal returns true if the file descriptor is terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// stdIsRedirected returns true if the std* is redirected out of the terminal.
// This can be used to check if an rhc command is being redirected to a file.
func stdIsRedirected(std *os.File) (bool, error) {
	out, err := std.Stat()
	if err != nil {
		return false, err
	}
	return (out.Mode() & os.ModeCharDevice) != os.ModeCharDevice, nil
}

// readInput is used to gather user input from the terminal and checks if
// rhc is being used in a non-interactive context to ensure the terminal is not blocked.
func readInput(prompt string, isSensitiveInput bool) (string, error) {
	stdoutRedirected, err := stdIsRedirected(os.Stdout)
	if err != nil {
		return "", err
	}
	if stdoutRedirected {
		errMsg := fmt.Errorf("non-interactive usage of rhc does not support interactive user input")
		return "", errMsg
	}
	if prompt != "" {
		fmt.Print(prompt)
	}
	if isSensitiveInput {
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	scanner := bufio.NewScanner(os.Stdin)
	_ = scanner.Scan()
	return scanner.Text(), nil
}

// BashCompleteCommand prints all visible flag options for the given command,
// and then recursively calls itself on each subcommand.
func BashCompleteCommand(cmd *cli.Command, w io.Writer) {
	for _, name := range cmd.Names() {
		fmt.Fprintf(w, "%v\n", name)
	}

	PrintFlagNames(cmd.VisibleFlags(), w)

	for _, command := range cmd.Subcommands {
		BashCompleteCommand(command, w)
	}
}

// PrintFlagNames prints the long and short names of each flag in the slice.
func PrintFlagNames(flags []cli.Flag, w io.Writer) {
	for _, flag := range flags {
		for _, name := range flag.Names() {
			if len(name) > 1 {
				fmt.Fprintf(w, "--%v\n", name)
			} else {
				fmt.Fprintf(w, "-%v\n", name)
			}
		}
	}
}

// BashComplete prints all commands, subcommands and flags to the application
// writer.
func BashComplete(c *cli.Context) {
	for _, command := range c.App.VisibleCommands() {
		BashCompleteCommand(command, c.App.Writer)

		// global flags
		PrintFlagNames(c.App.VisibleFlags(), c.App.Writer)
	}
}
