package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/operations"
)

// beforeStatusAction ensures the user has supplied a correct `--format` flag.
func beforeStatusAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}

	configureUI(cmd)

	return ctx, checkForUnknownArgs(cmd)
}

// statusAction prints the status of the system: whether it is registered to Red
// Hat Subscription Management, connected to Red Hat Lightspeed, and whether
// yggdrasil.service (rhcd.service) is running. The output is human-readable text
// or a machine-readable JSON document, selected by the --format CLI option.
func statusAction(_ context.Context, cmd *cli.Command) error {
	return runStatusAction(cmd, operations.GetStatus)
}

func runStatusAction(cmd *cli.Command, getStatus func() *operations.StatusReport) error {
	logCommandStart(cmd)

	var report *operations.StatusReport
	_ = withSpinner(func() error {
		report = getStatus()
		return nil
	}, indent.Small, "Querying status...")

	if isOutputMachineReadable() {
		printErr := printJSON(report)
		// A failing check takes precedence over a printing error.
		if report.HasFailures() {
			return cli.Exit("", exitcode.Err)
		}
		if printErr != nil {
			return cli.Exit(
				fmt.Errorf("unable to print status as %s document: %s", cmd.String("format"), printErr.Error()),
				exitcode.IOErr)
		}
		return nil
	}

	if report.HostnameError != "" {
		return cli.Exit(report.HostnameError, exitcode.Err)
	}

	printf("Connection status for %v:\n\n", report.Hostname)
	formatRHSMStatus(*report)
	formatContentStatus(*report)
	formatInsightsStatus(*report)
	formatServiceStatus(*report)

	printf("\nManage your connected systems: https://red.ht/connector\n")

	if report.HasFailures() {
		return cli.Exit("", exitcode.Err)
	}

	return nil
}

// formatRHSMStatus prints the human-readable Red Hat Subscription Management line.
func formatRHSMStatus(report operations.StatusReport) {
	switch {
	case report.RHSMError != "":
		printf(
			"%s[%s] Red Hat Subscription Management ... %s\n",
			indent.Small,
			icons.Error,
			"unable to check registration status: "+report.RHSMError,
		)
	case report.RHSMConnected:
		printf("%s[%v] %v\n", indent.Small, icons.Ok, "Connected to Red Hat Subscription Management")
	default:
		printf("%s[ ] %v\n", indent.Small, "Not connected to Red Hat Subscription Management")
	}
}

// formatContentStatus prints the human-readable content access line.
func formatContentStatus(report operations.StatusReport) {
	switch {
	case report.ContentError != "":
		printf(
			"%s[%s] Content ... %s\n",
			indent.Medium,
			icons.Error,
			"unable to check content management: "+report.ContentError,
		)
	case report.ContentEnabled:
		printf("%s[%v] Content ... %v\n", indent.Medium, icons.Ok, "System has access to content")
	default:
		printf("%s[ ] Content ... %v\n", indent.Medium, "System has no access to content")
	}
}

// formatInsightsStatus prints the human-readable Red Hat Lightspeed line.
func formatInsightsStatus(report operations.StatusReport) {
	switch {
	case report.InsightsConnected:
		printf("%s[%v] Analytics ... Connected to Red Hat Lightspeed (formerly Insights)\n", indent.Medium, icons.Ok)
	case report.InsightsError != "":
		printf("%s[%v] Analytics ... Cannot detect Red Hat Lightspeed (formerly Insights) status: %v\n", indent.Medium, icons.Error, report.InsightsError)
	default:
		printf("%s[ ] Analytics ... Not connected to Red Hat Lightspeed (formerly Insights)\n", indent.Medium)
	}
}

// formatServiceStatus prints the human-readable remote management (yggdrasil) line.
func formatServiceStatus(report operations.StatusReport) {
	switch {
	case report.YggdrasilError != "":
		printf("%s[%s] Remote Management ... %s\n", indent.Medium, icons.Error, report.YggdrasilError)
	case report.YggdrasilRunning:
		printf("%s[%v] Remote Management ... %v\n", indent.Medium, icons.Ok, "The yggdrasil service is active")
	default:
		printf("%s[ ] Remote Management ... %v\n", indent.Medium, "The yggdrasil service is not running")
	}
}
