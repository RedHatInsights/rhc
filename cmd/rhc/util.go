package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"

	"github.com/redhatinsights/rhc/pkg/exitcode"
)

// isShellCompletion returns true when the process was invoked for shell completion.
func isShellCompletion() bool {
	for _, arg := range os.Args {
		if arg == "--generate-shell-completion" {
			return true
		}
	}
	return false
}

// isTerminal returns true if the file descriptor is terminal.
func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}

// BashCompleteCommand prints all visible flag options for the given command,
// and then recursively calls itself on each subcommand.
func BashCompleteCommand(cmd *cli.Command, w io.Writer) {
	for _, name := range cmd.Names() {
		_, _ = fmt.Fprintf(w, "%v\n", name)
	}

	PrintFlagNames(cmd.VisibleFlags(), w)

	for _, command := range cmd.Commands {
		BashCompleteCommand(command, w)
	}
}

// PrintFlagNames prints the long and short names of each flag in the slice.
func PrintFlagNames(flags []cli.Flag, w io.Writer) {
	for _, flag := range flags {
		for _, name := range flag.Names() {
			if len(name) > 1 {
				_, _ = fmt.Fprintf(w, "--%v\n", name)
			} else {
				_, _ = fmt.Fprintf(w, "-%v\n", name)
			}
		}
	}
}

// BashComplete prints all commands, subcommands and flags to the application
// writer.
func ShellComplete(ctx context.Context, cmd *cli.Command) {
	for _, command := range cmd.Root().Commands {
		BashCompleteCommand(command, cmd.Root().Writer)

		// global flags
		PrintFlagNames(cmd.Root().Flags, cmd.Root().Writer)
	}
}

func ConfigPath() (string, error) {
	// default config file path in `/etc/rhc/config.toml`
	filePath := filepath.Join("/etc", "rhc", "config.toml")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil
	}

	return filePath, nil
}

// checkForUnknownArgs returns an error if any unknown arguments are present.
func checkForUnknownArgs(cmd *cli.Command) error {
	if cmd.Args().Len() != 0 {
		return fmt.Errorf("unknown option(s): %s",
			strings.Join(cmd.Args().Slice(), " "))
	}
	return nil
}

// checkFormatFlag ensures the user has supplied a correct `--format` flag.
func checkFormatFlag(cmd *cli.Command) error {
	format := cmd.String("format")
	switch format {
	case "", "json":
		return nil
	default:
		err := fmt.Errorf(
			"unsupported format: %s (supported formats: %s)",
			format,
			`"json"`,
		)
		return cli.Exit(err, exitcode.DataErr)
	}
}

// getFullCommandName uses ctx.Lineage() to reconstruct the full command name including parent commands,
// excluding flags and arguments
func getFullCommandName(cmd *cli.Command) string {
	var commandParts []string
	for _, c := range cmd.Lineage() {
		if c != nil {
			commandParts = append([]string{c.Name}, commandParts...)
		}
	}

	return strings.Join(commandParts, " ")
}

// logCommandStart logs the start of a command execution. This should be called at the beginning
// of each command's Action function to ensure the full command name (including all subcommands)
// is properly logged.
func logCommandStart(cmd *cli.Command) {
	fullCommandName := getFullCommandName(cmd)
	slog.Info(fmt.Sprintf("Command '%s' started", fullCommandName))
}

// validateCollectorCommand performs common validation for collector commands.
func validateCollectorCommand(cmd *cli.Command, requiresCollectorID, requiresFormat bool) error {
	if requiresFormat {
		if err := checkFormatFlag(cmd); err != nil {
			return err
		}
	}
	if requiresCollectorID && cmd.Args().Len() == 0 {
		if isShellCompletion() {
			return nil
		}
		commandName := getFullCommandName(cmd)
		return cli.Exit(fmt.Sprintf("%s requires a collector ID", commandName), exitcode.Usage)
	}
	configureUI(cmd)
	return nil
}
