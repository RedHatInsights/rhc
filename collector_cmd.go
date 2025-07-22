package main

import "github.com/urfave/cli/v2"

func beforeCollectorAction(ctx *cli.Context) error {
	// TODO: Probably run something, but keep in mind that this command
	//       has subcommands. We will probably want to have JSON output
	return nil
}

func collectorAction(ctx *cli.Context) error {
	// TODO: Run something. Not defined in UX design document
	return nil
}
