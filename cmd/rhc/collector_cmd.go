package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/emersion/go-varlink"
	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/collector"
	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/varlink/collectorapi"
)

const rhcServerSocket = "/run/rhc/com.redhat.rhc"

// newCollectorClient creates a varlink client for the collector API.
func newCollectorClient() (*collectorapi.Client, func(), error) {
	conn, err := net.Dial("unix", rhcServerSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to rhc-server: %w", err)
	}
	varlinkClient := varlink.NewClient(conn)
	client := &collectorapi.Client{Client: varlinkClient}
	cleanup := func() {
		err = varlinkClient.Close()
		if err != nil {
			slog.Debug("failed to close varlink client", "error", err)
			return
		}
	}
	return client, cleanup, nil
}

// beforeCollectorInfoAction validates the collector info command arguments and configuration.
func beforeCollectorInfoAction(ctx *cli.Context) error {
	return validateCollectorCommand(ctx, true, true)
}

// collectorInfoAction retrieves and displays information for a specific collector.
func collectorInfoAction(ctx *cli.Context) error {
	logCommandStart(ctx)
	client, cleanup, err := newCollectorClient()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to connect to rhc-server: %v", err), exitcode.Unavailable)
	}
	defer cleanup()
	response, err := client.Info(&collectorapi.InfoIn{Id: ctx.Args().First()})
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to get collector info: %v", err), exitcode.Err)
	}
	ui.PrintCollectorInfo(&response.Info)
	return nil
}

// beforeCollectorListAction validates the collector list command configuration.
func beforeCollectorListAction(ctx *cli.Context) error {
	return validateCollectorCommand(ctx, false, true)
}

// collectorListAction retrieves and displays a list of all available collectors.
func collectorListAction(ctx *cli.Context) error {
	logCommandStart(ctx)

	client, cleanup, err := newCollectorClient()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to connect to rhc-server: %v", err), exitcode.Unavailable)
	}
	defer cleanup()

	response, err := client.List(&collectorapi.ListIn{})
	if err != nil {
		slog.Debug("failed to list collectors", "error", err)
		return cli.Exit("No data collectors available.", exitcode.Err)
	}

	if ui.IsOutputMachineReadable() {
		if len(response.Collectors) == 0 {
			fmt.Println("{}")
			return nil
		}
		jsonData, err := json.Marshal(response.Collectors)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to marshal collectors: %v", err), exitcode.Err)
		}
		fmt.Println(string(jsonData))
		return nil
	}

	var rows [][]string
	for _, info := range response.Collectors {
		rows = append(rows, []string{info.Id, info.Name})
	}

	headers := []string{"ID", "NAME"}
	ui.PrintTable(headers, rows)
	return nil
}

// beforeCollectorTimersAction validates the collector timers command configuration.
func beforeCollectorTimersAction(ctx *cli.Context) error {
	return validateCollectorCommand(ctx, false, true)
}

// collectorTimersAction retrieves and displays timer information for all collectors.
func collectorTimersAction(ctx *cli.Context) error {
	logCommandStart(ctx)

	client, cleanup, err := newCollectorClient()
	if err != nil {
		return cli.Exit(fmt.Sprintf("failed to connect to rhc-server: %v", err), exitcode.Unavailable)
	}
	defer cleanup()

	response, err := client.List(&collectorapi.ListIn{})
	if err != nil {
		slog.Debug("failed to list collectors for timers", "error", err)
		return cli.Exit("No data collectors available.", exitcode.Err)
	}

	var infos []*collectorapi.CollectorInfo
	for i := range response.Collectors {
		infos = append(infos, &response.Collectors[i])
	}

	ui.PrintCollectorTimers(infos)
	return nil
}

// beforeCollectorEnableAction validates the collector enable command arguments.
func beforeCollectorEnableAction(ctx *cli.Context) error {
	return validateCollectorCommand(ctx, true, false)
}

// collectorEnableAction enables a collector timer and optionally triggers immediate collection.
func collectorEnableAction(ctx *cli.Context) error {
	logCommandStart(ctx)
	collectorId := ctx.Args().First()
	nowFlag := ctx.Bool("now")
	conn, timerName, err := collector.ValidateCollectorAndConnect(collectorId)
	if err != nil {
		return cli.Exit(fmt.Sprintf("%v", err), exitcode.Err)
	}
	defer conn.Close()

	err = conn.EnableUnit(timerName, true, false)
	if err != nil {
		if strings.Contains(fmt.Sprintf("%v", err), "does not exist") {
			return cli.Exit(fmt.Sprintf("timer unit %s does not exist, collector systemd units need to be installed first", timerName), exitcode.OSFile)
		}
		return cli.Exit(fmt.Sprintf("failed to enable timer %s: %v", timerName, err), exitcode.OSFile)
	}

	if nowFlag {
		serviceName := strings.Replace(timerName, ".timer", ".service", 1)
		err = conn.StartUnit(serviceName, false)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to start service %s: %v", serviceName, err), exitcode.OSFile)
		}
		ui.Printf("Enabled timer %s and triggered immediate collection.\n", timerName)
	} else {
		ui.Printf("Enabled timer %s.\n", timerName)
	}
	return nil
}

// beforeCollectorDisableAction validates the collector disable command arguments.
func beforeCollectorDisableAction(ctx *cli.Context) error {
	return validateCollectorCommand(ctx, true, false)
}

// collectorDisableAction disables a collector timer and optionally stops immediate collection.
func collectorDisableAction(ctx *cli.Context) error {
	logCommandStart(ctx)
	collectorId := ctx.Args().First()
	nowFlag := ctx.Bool("now")
	conn, timerName, err := collector.ValidateCollectorAndConnect(collectorId)
	if err != nil {
		return cli.Exit(fmt.Sprintf("%v", err), exitcode.Err)
	}
	defer conn.Close()

	if nowFlag {
		serviceName := strings.Replace(timerName, ".timer", ".service", 1)
		err = conn.StopUnit(serviceName, false)
		if err != nil {
			return cli.Exit(fmt.Sprintf("failed to stop service %s: %v", serviceName, err), exitcode.OSFile)
		}
	}

	err = conn.DisableUnit(timerName, true, false)
	if err != nil {
		if strings.Contains(fmt.Sprintf("%v", err), "does not exist") {
			return cli.Exit(fmt.Sprintf("timer unit %s does not exist. Collector systemd units need to be installed first.", timerName), exitcode.OSFile)
		}
		return cli.Exit(fmt.Sprintf("failed to disable timer %s: %v", timerName, err), exitcode.OSFile)
	}

	if nowFlag {
		ui.Printf("Disabled timer %s and stopped collection immediately.\n", timerName)
	} else {
		ui.Printf("Disabled timer %s.\n", timerName)
	}
	return nil
}
