package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/subpop/go-log"
	"github.com/urfave/cli/v2"
)

// DisconnectResult is structure holding information about result of
// disconnect command. The result could be printed in machine-readable format.
type DisconnectResult struct {
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
	format                    string
}

// Error implement error interface for structure DisconnectResult
func (disconnectResult DisconnectResult) Error() string {
	var result string
	switch disconnectResult.format {
	case "json":
		data, err := json.MarshalIndent(disconnectResult, "", "    ")
		if err != nil {
			return err.Error()
		}
		result = string(data)
	case "":
		break
	default:
		result = "error: unsupported document format: " + disconnectResult.format
	}
	return result
}

// beforeDisconnectAction ensures the used has supplied a correct `--format` flag
func beforeDisconnectAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	return checkForUnknownArgs(ctx)
}

// disconnectAction tries to stop (yggdrasil) rhcd service, disconnect from Red Hat Insights,
// and finally it unregisters system from Red Hat Subscription Management
func disconnectAction(ctx *cli.Context) error {
	var disconnectResult DisconnectResult
	disconnectResult.format = ctx.String("format")

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot disconnect system"
		exitCode := 1
		if uiSettings.isMachineReadable {
			disconnectResult.UID = uid
			disconnectResult.UIDError = errMsg
			return cli.Exit(disconnectResult, exitCode)
		} else {
			return cli.Exit(fmt.Errorf("error: %s", errMsg), exitCode)
		}
	}

	hostname, err := os.Hostname()
	if uiSettings.isMachineReadable {
		disconnectResult.Hostname = hostname
	}
	if err != nil {
		exitCode := 1
		if uiSettings.isMachineReadable {
			disconnectResult.HostnameError = err.Error()
			return cli.Exit(disconnectResult, exitCode)
		} else {
			return cli.Exit(err, exitCode)
		}
	}

	interactivePrintf("Disconnecting %v from %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]LogMessage)

	/* 1. Deactivate yggdrasil (rhcd) service */
	start = time.Now()
	progressMessage := fmt.Sprintf(" Deactivating the %v service", ServiceName)
	err = showProgress(progressMessage, deactivateService, smallIndent)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot deactivate %s service: %v", ServiceName, err)
		errorMessages[ServiceName] = LogMessage{
			level:   log.LevelError,
			message: fmt.Errorf("%v", errMsg)}
		disconnectResult.YggdrasilStopped = false
		disconnectResult.YggdrasilStoppedError = errMsg
		interactivePrintf(" [%v] %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.YggdrasilStopped = true
		interactivePrintf(" [%v] Deactivated the %v service\n", uiSettings.iconOK, ServiceName)
	}
	durations[ServiceName] = time.Since(start)

	/* 2. Disconnect from Red Hat Insights */
	start = time.Now()
	err = showProgress(" Disconnecting from Red Hat Insights...", unregisterInsights, smallIndent)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot disconnect from Red Hat Insights: %v", err)
		errorMessages["insights"] = LogMessage{
			level:   log.LevelError,
			message: fmt.Errorf("%v", errMsg)}
		disconnectResult.InsightsDisconnected = false
		disconnectResult.InsightsDisconnectedError = errMsg
		interactivePrintf(" [%v] %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.InsightsDisconnected = true
		interactivePrintf(" [%v] Disconnected from Red Hat Insights\n", uiSettings.iconOK)
	}
	durations["insights"] = time.Since(start)

	/* 3. Unregister system from Red Hat Subscription Management */
	err = showProgress(
		" Disconnecting from Red Hat Subscription Management...",
		unregister,
		smallIndent,
	)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot disconnect from Red Hat Subscription Management: %v", err)
		errorMessages["rhsm"] = LogMessage{
			level:   log.LevelError,
			message: fmt.Errorf("%v", errMsg)}

		disconnectResult.RHSMDisconnected = false
		disconnectResult.RHSMDisconnectedError = errMsg
		interactivePrintf(" [%v] %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.RHSMDisconnected = true
		interactivePrintf(" [%v] Disconnected from Red Hat Subscription Management\n", uiSettings.iconOK)
	}
	durations["rhsm"] = time.Since(start)

	if !uiSettings.isMachineReadable {
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")
		showTimeDuration(durations)

		err = showErrorMessages("disconnect", errorMessages)
		if err != nil {
			return err
		}
	}

	return cli.Exit(disconnectResult, 0)
}
