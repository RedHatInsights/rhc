package main

import (
	"context"
	"fmt"
	"github.com/redhatinsights/rhc/internal/systemd"
	"github.com/urfave/cli/v2"
	"path/filepath"
)

func beforeCollectorDisableAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	if ctx.Args().Len() != 1 {
		return fmt.Errorf("error: expected 1 argument of collector name, got %d", ctx.Args().Len())
	}
	return nil
}

// collectorDisableAction tries to disable collector timer
func collectorDisableAction(ctx *cli.Context) (err error) {
	collectorId := ctx.Args().First()

	fileName := collectorId + ".toml"
	collectorConfigfilePath := filepath.Join(collectorDirName, fileName)

	collectorConfig, err := readCollectorConfig(collectorConfigfilePath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read collector configuration file %s: %v", fileName, err), 1)
	}

	collectorTimer := collectorConfig.Systemd.Timer

	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	if err := conn.DisableUnit(collectorTimer, true, false); err != nil {
		return fmt.Errorf("cannot enable timer %s: %v", collectorTimer, err)
	}

	return nil
}
