package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/subpop/go-log"

	"github.com/urfave/cli/v2"
	"golang.org/x/sys/unix"
)

// isTerminal returns true if the file descriptor is terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
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

func ConfigPath() (string, error) {
	// default config file path in `/etc/rhc/config.toml`
	filePath := filepath.Join("/etc", LongName, "config.toml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	return filePath, nil
}

// hasPriorityErrors checks if the errorMessage map has any error
// with a higher priority than the logLevel configure.
func hasPriorityErrors(errorMessages map[string]LogMessage, level log.Level) bool {
	for _, logMsg := range errorMessages {
		if logMsg.level <= level {
			return true
		}
	}
	return false
}

// getLocale tries to get current locale
func getLocale() string {
	// FIXME: Locale should be detected in more reliable way. We are going to support
	//        localization in better way. Maybe we could use following go module
	//        https://github.com/Xuanwo/go-locale. Maybe some other will be better.
	locale := os.Getenv("LANG")
	return locale
}

// checkForUnknownArgs returns an error if any unknown arguments are present.
func checkForUnknownArgs(ctx *cli.Context) error {
	if ctx.Args().Len() != 0 {
		return fmt.Errorf("error: unknown option(s): %s",
			strings.Join(ctx.Args().Slice(), " "))
	}
	return nil
}

// setupFormatOption ensures the user has supplied a correct `--format` flag
// and set values in uiSettings, when JSON format is used.
func setupFormatOption(ctx *cli.Context) error {
	// This is run after the `app.Before()` has been run,
	// the uiSettings is already set up for us to modify.
	format := ctx.String("format")
	switch format {
	case "":
		return nil
	case "json":
		uiSettings.isMachineReadable = true
		uiSettings.isRich = false
		return nil
	default:
		err := fmt.Errorf(
			"unsupported format: %s (supported formats: %s)",
			format,
			`"json"`,
		)
		return cli.Exit(err, 1)
	}
}
