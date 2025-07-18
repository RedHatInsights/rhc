package main

import "github.com/urfave/cli/v2"

func beforeCollectorEnableAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorEnableAction(ctx *cli.Context) (err error) {
	return nil
}
