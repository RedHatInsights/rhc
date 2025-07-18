package main

import "github.com/urfave/cli/v2"

func beforeCollectorDisableAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorDisableAction(ctx *cli.Context) (err error) {
	return nil
}
