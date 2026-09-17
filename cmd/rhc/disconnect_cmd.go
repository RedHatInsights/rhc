package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/operations"
)

// disconnectJSONDocument is the machine-readable representation of a disconnect
// report. It keeps the presentation data separate from operations.DisconnectReport.
type disconnectJSONDocument struct {
	Hostname                  string `json:"hostname"`
	HostnameError             string `json:"hostname_error,omitempty"`
	UID                       int    `json:"uid"`
	UIDError                  string `json:"uid_error,omitempty"`
	RHSMDisconnected          bool   `json:"rhsm_disconnected"`
	RHSMDisconnectedError     string `json:"rhsm_disconnect_error,omitempty"`
	InsightsDisconnected      bool   `json:"insights_disconnected"`
	InsightsDisconnectedError string `json:"insights_disconnected_error,omitempty"`
	YggdrasilStopped          bool   `json:"yggdrasil_stopped"`
	YggdrasilStoppedError     string `json:"yggdrasil_stopped_error,omitempty"`
}

func disconnectJSONDocumentFrom(report *operations.DisconnectReport) disconnectJSONDocument {
	return disconnectJSONDocument{
		Hostname:                  report.Hostname,
		HostnameError:             report.HostnameError,
		UID:                       report.UID,
		UIDError:                  report.UIDError,
		RHSMDisconnected:          report.RHSMDisconnected,
		RHSMDisconnectedError:     report.RHSMDisconnectedError,
		InsightsDisconnected:      report.InsightsDisconnected,
		InsightsDisconnectedError: report.InsightsDisconnectedError,
		YggdrasilStopped:          report.YggdrasilStopped,
		YggdrasilStoppedError:     report.YggdrasilStoppedError,
	}
}

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

	hostname, err := os.Hostname()
	if err != nil {
		slog.Error("error retrieving system hostname", "err", err)
		return formatDisconnectReport(&operations.DisconnectReport{HostnameError: err.Error()}, err)
	}

	slog.Info("Disconnecting from Red Hat", "hostname", hostname)
	printf("Disconnecting %v from Red Hat.\nThis might take a few seconds.\n\n", hostname)

	var report *operations.DisconnectReport
	err = withSpinner(func() error {
		var err error
		report, err = operations.Disconnect()
		return err
	}, indent.Small, "Disconnecting from Red Hat...")
	report.Hostname = hostname

	return formatDisconnectReport(report, err)
}

// formatDisconnectReport prints the report and selects the CLI exit code.
func formatDisconnectReport(report *operations.DisconnectReport, err error) error {
	if err != nil {
		code := exitcode.Err
		if report.UIDError != "" {
			code = exitcode.NoPerm
		}
		if isOutputMachineReadable() {
			data, marshalErr := json.MarshalIndent(disconnectJSONDocumentFrom(report), "", "    ")
			if marshalErr != nil {
				return cli.Exit(marshalErr, code)
			}
			return cli.Exit(string(data), code)
		}
		return cli.Exit(err, code)
	}

	if isOutputMachineReadable() {
		if err := printJSON(disconnectJSONDocumentFrom(report)); err != nil {
			return cli.Exit(
				fmt.Errorf("unable to print disconnect as json document: %s", err.Error()),
				exitcode.IOErr)
		}
		// Preserve the successful JSON exit code even when individual steps fail.
		return nil
	}

	errorMessages := make(map[string]string)
	switch {
	case report.YggdrasilStoppedError != "":
		errorMessages["yggdrasil"] = report.YggdrasilStoppedError
		printf(" [%v] %v\n", icons.Error, report.YggdrasilStoppedError)
	case report.YggdrasilAlreadyInactive:
		printf(" [%v] %v\n", icons.Info, "The yggdrasil service is already inactive")
	case report.YggdrasilStopped:
		printf(" [%v] %v\n", icons.Ok, "Deactivated the yggdrasil service")
	}

	switch {
	case report.InsightsDisconnectedError != "":
		errorMessages["insights"] = report.InsightsDisconnectedError
		printf(" [%v] %v\n", icons.Error, report.InsightsDisconnectedError)
	case report.InsightsAlreadyDisconnected:
		printf(" [%v] %v\n", icons.Info, "Already disconnected from Red Hat Lightspeed (formerly Insights)")
	case report.InsightsDisconnected:
		printf(" [%v] %v\n", icons.Ok, "Disconnected from Red Hat Lightspeed (formerly Insights)")
	}

	switch {
	case report.RHSMDisconnectedError != "":
		errorMessages["rhsm"] = report.RHSMDisconnectedError
		printf(" [%v] %v\n", icons.Error, report.RHSMDisconnectedError)
	case report.RHSMAlreadyDisconnected:
		printf(" [%v] %v\n", icons.Info, "Already disconnected from Red Hat Subscription Management")
	case report.RHSMDisconnected:
		printf(" [%v] %v\n", icons.Ok, "Disconnected from Red Hat Subscription Management")
	}

	showTimeDuration(report.Durations)
	return showErrorMessages("disconnect", errorMessages)
}
