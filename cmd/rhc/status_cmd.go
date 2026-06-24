package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/exitcode"
)

// rhsmStatus tries to print status provided by RHSM D-Bus API. If we provide
// output in machine-readable format, then we only set files in SystemStatus
// structure and content of this structure will be printed later
func rhsmStatus(systemStatus *SystemStatus) error {
	slog.Info("Checking status of Red Hat Subscription Management")

	uuid, err := subman.GetConsumerUUID()
	if err != nil {
		systemStatus.returnCode += 1
		systemStatus.RHSMError = err.Error()
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}
	if uuid == "" {
		systemStatus.returnCode += 1
		systemStatus.RHSMConnected = false
		infoMsg := "Not connected to Red Hat Subscription Management"
		slog.Info(infoMsg)
		ui.Printf("%s[ ] %v\n", ui.Indent.Small, infoMsg)
	} else {
		systemStatus.RHSMConnected = true
		infoMsg := "Connected to Red Hat Subscription Management"
		slog.Info(infoMsg)
		ui.Printf("%s[%v] %v\n", ui.Indent.Small, ui.Icons.Ok, infoMsg)
	}
	return nil
}

// isContentEnabled checks whether the system is registered and the content management
// is enabled. Both conditions must be true for content access to be available.
func isContentEnabled(systemStatus *SystemStatus) error {
	slog.Info("Checking content status")

	contentEnabled, err := subman.IsContentManagementEnabled()
	if err != nil {
		systemStatus.returnCode += 1
		systemStatus.ContentError = err.Error()
		return fmt.Errorf("unable to check content management: %w", err)
	}

	registered, err := subman.IsRegistered()
	if err != nil {
		systemStatus.returnCode += 1
		systemStatus.ContentError = err.Error()
		return fmt.Errorf("unable to check registration status: %s", err)
	}

	if contentEnabled && registered {
		systemStatus.ContentEnabled = true
		infoMsg := "System has access to content"
		slog.Info(infoMsg)
		ui.Printf("%s[%v] Content ... %v\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
	} else {
		systemStatus.ContentEnabled = false
		infoMsg := "System has no access to content"
		slog.Info(infoMsg)
		ui.Printf("%s[ ] Content ... %v\n", ui.Indent.Medium, infoMsg)
	}
	return nil
}

// insightStatus tries to print status of insights client
func insightStatus(systemStatus *SystemStatus) error {
	slog.Info("Checking status of Red Hat Lightspeed")

	var isRegistered bool
	var err error
	spinErr := ui.Spinner(func() error {
		isRegistered, err = datacollection.InsightsClientIsRegistered()
		return nil
	}, ui.Indent.Medium, "Checking Red Hat Lightspeed (formerly Insights)...")
	if spinErr != nil {
		return spinErr
	}

	if isRegistered {
		systemStatus.InsightsConnected = true
		slog.Info("Connected to Red Hat Lightspeed")
		ui.Printf("%s[%v] Analytics ... Connected to Red Hat Lightspeed (formerly Insights)\n", ui.Indent.Medium, ui.Icons.Ok)
	} else {
		systemStatus.returnCode += 1
		if err == nil {
			systemStatus.InsightsConnected = false
			slog.Info("Not connected to Red Hat Lightspeed")
			ui.Printf("%s[ ] Analytics ... Not connected to Red Hat Lightspeed (formerly Insights)\n", ui.Indent.Medium)
		} else {
			systemStatus.InsightsConnected = false
			systemStatus.InsightsError = err.Error()
			return err
		}
	}
	return nil
}

// serviceStatus tries to print status of yggdrasil.service or rhcd.service
func serviceStatus(systemStatus *SystemStatus) error {
	slog.Info("Checking status of yggdrasil service")

	state, err := remotemanagement.GetUnitState("yggdrasil.service")
	if err != nil {
		systemStatus.YggdrasilRunning = false
		systemStatus.YggdrasilError = err.Error()
		return err
	}

	if state.ActiveState == "active" {
		systemStatus.YggdrasilRunning = true
		infoMsg := "The yggdrasil service is active"
		slog.Info(infoMsg)
		ui.Printf("%s[%v] Remote Management ... %v\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
	} else if state.LoadState == "loaded" {
		systemStatus.returnCode += 1
		systemStatus.YggdrasilRunning = false
		warnMsg := "The yggdrasil service is not running"
		slog.Warn(warnMsg)
		ui.Printf("%s[ ] Remote Management ... %v\n", ui.Indent.Medium, warnMsg)
	} else {
		systemStatus.returnCode += 1
		systemStatus.YggdrasilRunning = false
		errMsg := "The yggdrasil service is not available"
		systemStatus.YggdrasilError = errMsg
		if state.LoadError != "" {
			slog.Error(errMsg, "reason", state.LoadError)
		} else {
			slog.Error(errMsg)
		}
		ui.Printf("%s[%s] Remote Management ... %v\n", ui.Indent.Medium, ui.Icons.Error, errMsg)
	}
	return nil
}

// SystemStatus is structure holding information about system status
// When more file format is supported, then add more tags for fields
// like xml:"hostname"
type SystemStatus struct {
	SystemHostname    string `json:"hostname"`
	HostnameError     string `json:"hostname_error,omitempty"`
	RHSMConnected     bool   `json:"rhsm_connected"`
	RHSMError         string `json:"rhsm_error,omitempty"`
	ContentEnabled    bool   `json:"content_enabled"`
	ContentError      string `json:"content_error,omitempty"`
	InsightsConnected bool   `json:"insights_connected"`
	InsightsError     string `json:"insights_error,omitempty"`
	YggdrasilRunning  bool   `json:"yggdrasil_running"`
	YggdrasilError    string `json:"yggdrasil_error,omitempty"`
	returnCode        int
}

// printJSONStatus tries to print the system status as JSON to stdout.
// When marshaling of systemStatus fails, then error is returned
func printJSONStatus(systemStatus *SystemStatus) error {
	data, err := json.MarshalIndent(systemStatus, "", "    ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// beforeStatusAction ensures the user has supplied a correct `--format` flag.
func beforeStatusAction(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	err := checkFormatFlag(cmd)
	if err != nil {
		return ctx, err
	}

	configureUI(cmd)

	return ctx, checkForUnknownArgs(cmd)
}

// statusAction tries to print status of system. It means that it gives
// answer on following questions:
// 1. Is system registered to Red Hat Subscription Management?
// 2. Is system connected to Red Hat Lightspeed?
// 3. Is yggdrasil.service (rhcd.service) running?
// Status can be printed as human-readable text or machine-readable JSON document.
// Format is influenced by --format json CLI option stored in CLI context
func statusAction(ctx context.Context, cmd *cli.Command) (err error) {
	logCommandStart(cmd)

	var systemStatus SystemStatus
	var machineReadablePrintFunc func(systemStatus *SystemStatus) error

	format := cmd.String("format")
	switch format {
	case "json":
		machineReadablePrintFunc = printJSONStatus
	default:
		break
	}

	// When printing of status is requested, then print machine-readable file format
	// at the end of this function
	if ui.IsOutputMachineReadable() {
		defer func(systemStatus *SystemStatus) {
			err = machineReadablePrintFunc(systemStatus)
			// When it was not possible to print status to machine-readable format, then
			// change returned error to CLI exit error to be able to set exit code to
			// a non-zero value
			if err != nil {
				err = cli.Exit(
					fmt.Errorf("unable to print status as %s document: %s", format, err.Error()),
					exitcode.IOErr)
			}
			// When any of status is not correct, then return exitcode.Err exit code
			if systemStatus.returnCode != 0 {
				err = cli.Exit("", exitcode.Err)
			}
		}(&systemStatus)
	}

	hostname, err := os.Hostname()
	if err != nil {
		if ui.IsOutputMachineReadable() {
			systemStatus.HostnameError = err.Error()
		} else {
			return cli.Exit(err, exitcode.Err)
		}
	}

	systemStatus.SystemHostname = hostname
	ui.Printf("Connection status for %v:\n\n", hostname)
	slog.Info("Checking system connection status")

	/* 1. Get Status of RHSM */
	err = rhsmStatus(&systemStatus)
	if err != nil {
		slog.Error(fmt.Sprintf("Cannot detect Red Hat Subscription Management status: %v", err))
		ui.Printf(
			"%s[%s] Red Hat Subscription Management ... %s\n",
			ui.Indent.Small,
			ui.Icons.Error,
			err,
		)
	}

	/* 2. Is content enabled */
	err = isContentEnabled(&systemStatus)
	if err != nil {
		slog.Error(fmt.Sprintf("Cannot detect content management status: %v", err))
		ui.Printf(
			"%s[%s] Content ... %s\n",
			ui.Indent.Medium,
			ui.Icons.Error,
			err,
		)
	}

	/* 3. Get status of insights-client */
	err = insightStatus(&systemStatus)
	if err != nil {
		slog.Error(fmt.Sprintf("Cannot detect Red Hat Lightspeed status: %v", err))
		ui.Printf("%s[%v] Analytics ... Cannot detect Red Hat Lightspeed (formerly Insights) status: %v\n",
			ui.Indent.Medium,
			ui.Icons.Error,
			err,
		)
	}

	/* 3. Get status of yggdrasil (rhcd) service */
	err = serviceStatus(&systemStatus)
	if err != nil {
		ui.Printf(
			"%s[%s] Remote Management ... %s\n",
			ui.Indent.Medium,
			ui.Icons.Error,
			err,
		)
	}

	ui.Printf("\nManage your connected systems: https://red.ht/connector\n")

	// At the end check if all statuses are correct.
	// If not, return exitcode.Err exit code without any message.
	if systemStatus.returnCode != 0 {
		return cli.Exit("", exitcode.Err)
	}

	return nil
}
