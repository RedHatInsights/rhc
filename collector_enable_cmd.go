package main

import (
	"context"
	"fmt"
	"github.com/redhatinsights/rhc/internal/systemd"
	"github.com/urfave/cli/v2"
	"path/filepath"
)

func beforeCollectorEnableAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	if ctx.Args().Len() != 1 {
		return fmt.Errorf("error: expected 1 argument of collector name, got %d", ctx.Args().Len())
	}
	return nil
}

// collectorEnableAction tries to enable given collector timer and start
func collectorEnableAction(ctx *cli.Context) (err error) {
	collectorId := ctx.Args().First()
	startNow := ctx.Bool("now")

	fileName := collectorId + ".toml"
	collectorConfigfilePath := filepath.Join(collectorConfigDirPath, fileName)

	collectorConfig, err := readCollectorConfig(collectorConfigfilePath)
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to read collector configuration file %s: %v", fileName, err), 1)
	}

	collectorTimer := collectorConfig.Systemd.Timer
	collectorService := collectorConfig.Systemd.Service

	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	if err := conn.EnableUnit(collectorTimer, true, false); err != nil {
		return fmt.Errorf("cannot enable timer %s: %v", collectorTimer, err)
	}

	if startNow {
		if err := conn.StartUnit(collectorService, false); err != nil {
			return fmt.Errorf("cannot start service %s: %v", collectorService, err)
		}
	}

	return nil
}
