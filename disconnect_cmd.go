package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
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

// disconnectService tries to stop yggdrasil.service, when it hasn't
// been already stopped.
func disconnectService(disconnectResult *DisconnectResult, errorMessages *map[string]LogMessage) error {
	// First check if the service hasn't been already stopped
	isInactive, err := remotemanagement.AssertYggdrasilServiceState("inactive")
	if err != nil {
		return err
	}
	if isInactive {
		infoMsg := fmt.Sprintf("The %s service is already inactive", ServiceName)
		disconnectResult.YggdrasilStopped = true
		interactivePrintf(" [%v] %v\n", uiSettings.iconInfo, infoMsg)
		return nil
	}
	// When the service is not inactive, then try to get this service to this state
	progressMessage := fmt.Sprintf(" Deactivating the %v service", ServiceName)
	err = showProgress(progressMessage, remotemanagement.DeactivateServices, smallIndent)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot deactivate %s service: %v", ServiceName, err)
		(*errorMessages)[ServiceName] = LogMessage{
			level:   slog.LevelError,
			message: fmt.Errorf("%v", errMsg)}
		disconnectResult.YggdrasilStopped = false
		disconnectResult.YggdrasilStoppedError = errMsg
		interactivePrintf(" [%v] %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.YggdrasilStopped = true
		interactivePrintf(" [%v] Deactivated the %v service\n", uiSettings.iconOK, ServiceName)
	}
	return nil
}

// disconnectInsightsClient tries to unregister insights-client if the client hasn't been
// already unregistered
func disconnectInsightsClient(disconnectResult *DisconnectResult, errorMessages *map[string]LogMessage) error {
	isRegistered, err := datacollection.InsightsClientIsRegistered()
	if err != nil {
		return err
	}
	if !isRegistered {
		infoMsg := "Already disconnected from Red Hat Insights"
		disconnectResult.InsightsDisconnected = true
		interactivePrintf(" [%v] %v\n", uiSettings.iconInfo, infoMsg)
		return nil
	}
	err = showProgress(" Disconnecting from Red Hat Insights...", datacollection.UnregisterInsightsClient, smallIndent)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot disconnect from Red Hat Insights: %v", err)
		(*errorMessages)["insights"] = LogMessage{
			level:   slog.LevelError,
			message: fmt.Errorf("%v", errMsg)}
		disconnectResult.InsightsDisconnected = false
		disconnectResult.InsightsDisconnectedError = errMsg
		interactivePrintf(" [%v] %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.InsightsDisconnected = true
		interactivePrintf(" [%v] Disconnected from Red Hat Insights\n", uiSettings.iconOK)
	}
	return nil
}

// disconnectRHSM tries to unregister system from RHSM if the client hasn't been already
// unregistered from RHSM
func disconnectRHSM(disconnectResult *DisconnectResult, errorMessages *map[string]LogMessage) error {
	isRegistered, err := isRHSMRegistered()
	if err != nil {
		return err
	}
	if !isRegistered {
		infoMsg := "Already disconnected from Red Hat Subscription Management"
		disconnectResult.InsightsDisconnected = true
		interactivePrintf(" [%v] %v\n", uiSettings.iconInfo, infoMsg)
		return nil
	}
	err = showProgress(
		" Disconnecting from Red Hat Subscription Management...",
		unregister,
		smallIndent,
	)
	if err != nil {
		errMsg := fmt.Sprintf("Cannot disconnect from Red Hat Subscription Management: %v", err)
		(*errorMessages)["rhsm"] = LogMessage{
			level:   slog.LevelError,
			message: fmt.Errorf("%v", errMsg)}

		disconnectResult.RHSMDisconnected = false
		disconnectResult.RHSMDisconnectedError = errMsg
		interactivePrintf(" [%v] %v\n", uiSettings.iconError, errMsg)
	} else {
		disconnectResult.RHSMDisconnected = true
		interactivePrintf(" [%v] Disconnected from Red Hat Subscription Management\n", uiSettings.iconOK)
	}
	return nil
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
	_ = disconnectService(&disconnectResult, &errorMessages)
	durations[ServiceName] = time.Since(start)

	/* 2. Disconnect from Red Hat Insights */
	start = time.Now()
	_ = disconnectInsightsClient(&disconnectResult, &errorMessages)
	durations["insights"] = time.Since(start)

	/* 3. Unregister system from Red Hat Subscription Management */
	start = time.Now()
	_ = disconnectRHSM(&disconnectResult, &errorMessages)
	durations["rhsm"] = time.Since(start)

	if !uiSettings.isMachineReadable {
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")
		showTimeDuration(durations)

		err = showErrorMessages("disconnect", errorMessages)
		if err != nil {
			return err
		}
	}

	if uiSettings.isMachineReadable {
		fmt.Println(disconnectResult.Error())
	}

	return nil
}
