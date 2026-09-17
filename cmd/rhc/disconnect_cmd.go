package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/operations"
)

// beforeDisconnectAction ensures the user has supplied a correct `--format` flag.
func beforeDisconnectAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}

	configureUI(cmd)

	return ctx, checkForUnknownArgs(cmd)
}

// disconnectAction disconnects the system and formats the resulting report.
func disconnectAction(_ context.Context, cmd *cli.Command) error {
	logCommandStart(cmd)

	var report *operations.DisconnectReport
	err := ui.Spinner(func() error {
		var err error
		report, err = operations.Disconnect()
		return err
	}, ui.Indent.Small, "Disconnecting from Red Hat...")

	return formatDisconnectReport(report, err)
}

// formatDisconnectReport prints the report and selects the CLI exit code.
func formatDisconnectReport(report *operations.DisconnectReport, err error) error {
	if err != nil {
		code := exitcode.Err
		if report.UIDError != "" {
			code = exitcode.NoPerm
		}
		if ui.IsOutputMachineReadable() {
			data, marshalErr := json.MarshalIndent(report, "", "    ")
			if marshalErr != nil {
				return cli.Exit(marshalErr, code)
			}
			return cli.Exit(string(data), code)
		}
		return cli.Exit(err, code)
	}

	if ui.IsOutputMachineReadable() {
		if err := ui.PrintJSON(report); err != nil {
			return cli.Exit(
				fmt.Errorf("unable to print disconnect as json document: %s", err.Error()),
				exitcode.IOErr)
		}
		// Preserve the successful JSON exit code even when individual steps fail.
		return nil
	}

	ui.Printf("Disconnecting %v from Red Hat.\nThis might take a few seconds.\n\n", report.Hostname)

	errorMessages := make(map[string]string)
	switch {
	case report.YggdrasilStoppedError != "":
		errorMessages["yggdrasil"] = report.YggdrasilStoppedError
		ui.Printf(" [%v] %v\n", ui.Icons.Error, report.YggdrasilStoppedError)
	case report.YggdrasilAlreadyInactive:
		ui.Printf(" [%v] %v\n", ui.Icons.Info, "The yggdrasil service is already inactive")
	case report.YggdrasilStopped:
		ui.Printf(" [%v] %v\n", ui.Icons.Ok, "Deactivated the yggdrasil service")
	}

	switch {
	case report.InsightsDisconnectedError != "":
		errorMessages["insights"] = report.InsightsDisconnectedError
		ui.Printf(" [%v] %v\n", ui.Icons.Error, report.InsightsDisconnectedError)
	case report.InsightsAlreadyDisconnected:
		ui.Printf(" [%v] %v\n", ui.Icons.Info, "Already disconnected from Red Hat Lightspeed (formerly Insights)")
	case report.InsightsDisconnected:
		ui.Printf(" [%v] %v\n", ui.Icons.Ok, "Disconnected from Red Hat Lightspeed (formerly Insights)")
	}

	switch {
	case report.RHSMDisconnectedError != "":
		errorMessages["rhsm"] = report.RHSMDisconnectedError
		ui.Printf(" [%v] %v\n", ui.Icons.Error, report.RHSMDisconnectedError)
	case report.RHSMAlreadyDisconnected:
		ui.Printf(" [%v] %v\n", ui.Icons.Info, "Already disconnected from Red Hat Subscription Management")
	case report.RHSMDisconnected:
		ui.Printf(" [%v] %v\n", ui.Icons.Ok, "Disconnected from Red Hat Subscription Management")
	}

	showTimeDuration(report.Durations)
	return showErrorMessages("disconnect", errorMessages)
}
