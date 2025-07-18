package main

import "github.com/urfave/cli/v2"

func beforeCollectorTimersAction(ctx *cli.Context) error {
	return checkForUnknownArgs(ctx)
}

func collectorTimersAction(ctx *cli.Context) (err error) {
	return nil
}
