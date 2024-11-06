package main

import (
	"encoding/json"
	"fmt"
	"github.com/subpop/go-log"
	"github.com/urfave/cli/v2"
	"os"
	"time"
)

// ConnectResult is structure holding information about results
// of connect command. The result could be printed in machine-readable format.
type ConnectResult struct {
	Hostname              string `json:"hostname"`
	HostnameError         string `json:"hostname_error,omitempty"`
	UID                   int    `json:"uid"`
	UIDError              string `json:"uid_error,omitempty"`
	RHSMConnected         bool   `json:"rhsm_connected"`
	RHSMConnectError      string `json:"rhsm_connect_error,omitempty"`
	InsightsConnected     bool   `json:"insights_connected"`
	InsightsError         string `json:"insights_connect_error,omitempty"`
	YggdrasilStarted      bool   `json:"yggdrasil_started"`
	YggdrasilStartedError string `json:"yggdrasil_started_error,omitempty"`
	format                string
}

// Error implement error interface for structure ConnectResult
func (connectResult ConnectResult) Error() string {
	var result string
	switch connectResult.format {
	case "json":
		data, err := json.MarshalIndent(connectResult, "", "    ")
		if err != nil {
			return err.Error()
		}
		result = string(data)
	case "":
		break
	default:
		result = "error: unsupported document format: " + connectResult.format
	}
	return result
}

// beforeConnectAction ensures that user has supplied a correct CLI options
// and there is no conflict between provided options
func beforeConnectAction(ctx *cli.Context) error {
	// First check if machine-readable format is used
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	username := ctx.String("username")
	password := ctx.String("password")
	organization := ctx.String("organization")
	activationKeys := ctx.StringSlice("activation-key")

	if len(activationKeys) > 0 {
		if username != "" {
			return fmt.Errorf("--username and --activation-key can not be used together")
		}
		if organization == "" {
			return fmt.Errorf("--organization is required, when --activation-key is used")
		}
	}

	// When machine-readable format is used, then additional requirements have to be met
	if uiSettings.isMachineReadable {
		if username == "" || password == "" {
			return fmt.Errorf("--username/--password or --organization/--activation-key are required when a machine-readable format is used")
		}
	}

	return checkForUnknownArgs(ctx)
}

// connectAction tries to register system against Red Hat Subscription Management,
// gather the profile information that the system will configure
// connect system to Red Hat Insights, and it also tries to start rhcd service
func connectAction(ctx *cli.Context) error {
	var connectResult ConnectResult
	connectResult.format = ctx.String("format")

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot connect system"
		exitCode := 1
		if uiSettings.isMachineReadable {
			connectResult.UID = uid
			connectResult.UIDError = errMsg
			return cli.Exit(connectResult, exitCode)
		} else {
			return cli.Exit(fmt.Errorf("error: %s", errMsg), exitCode)
		}
	}

	hostname, err := os.Hostname()
	if uiSettings.isMachineReadable {
		connectResult.Hostname = hostname
	}
	if err != nil {
		exitCode := 1
		if uiSettings.isMachineReadable {
			connectResult.HostnameError = err.Error()
			return cli.Exit(connectResult, exitCode)
		} else {
			return cli.Exit(err, exitCode)
		}
	}

	interactivePrintf("Connecting %v to %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]LogMessage)
	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	var returnedMsg string
	returnedMsg, err = registerRHSM(ctx)
	if err != nil {
		connectResult.RHSMConnected = false
		errorMessages["rhsm"] = LogMessage{
			level: log.LevelError,
			message: fmt.Errorf("cannot connect to Red Hat Subscription Management: %w",
				err)}
		if uiSettings.isMachineReadable {
			connectResult.RHSMConnectError = errorMessages["rhsm"].message.Error()
		} else {
			fmt.Printf(
				"%v Cannot connect to Red Hat Subscription Management\n",
				uiSettings.iconError,
			)
		}
	} else {
		connectResult.RHSMConnected = true
		interactivePrintf("%v %v\n", uiSettings.iconOK, returnedMsg)
	}
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	if errors, exist := errorMessages["rhsm"]; exist {
		if errors.level == log.LevelError {
			interactivePrintf(
				"%v Skipping connection to Red Hat Insights\n",
				uiSettings.iconError,
			)
		}
	} else {
		start = time.Now()
		err = showProgress(" Connecting to Red Hat Insights...", registerInsights)
		if err != nil {
			connectResult.InsightsConnected = false
			errorMessages["insights"] = LogMessage{
				level:   log.LevelError,
				message: fmt.Errorf("cannot connect to Red Hat Insights: %w", err)}
			if uiSettings.isMachineReadable {
				connectResult.InsightsError = errorMessages["insights"].message.Error()
			} else {
				fmt.Printf("%v Cannot connect to Red Hat Insights\n", uiSettings.iconError)
			}
		} else {
			connectResult.InsightsConnected = true
			interactivePrintf("%v Connected to Red Hat Insights\n", uiSettings.iconOK)
		}
		durations["insights"] = time.Since(start)
	}

	/* 3. Start yggdrasil (rhcd) service */
	if rhsmErrMsg, exist := errorMessages["rhsm"]; exist && rhsmErrMsg.level == log.LevelError {
		connectResult.YggdrasilStarted = false
		interactivePrintf(
			"%v Skipping activation of %v service\n",
			uiSettings.iconError,
			ServiceName,
		)
	} else {
		start = time.Now()
		progressMessage := fmt.Sprintf(" Activating the %v service", ServiceName)
		err = showProgress(progressMessage, activateService)
		if err != nil {
			connectResult.YggdrasilStarted = false
			errorMessages[ServiceName] = LogMessage{
				level: log.LevelError,
				message: fmt.Errorf("cannot activate %s service: %w",
					ServiceName, err)}
			if uiSettings.isMachineReadable {
				connectResult.YggdrasilStartedError = errorMessages[ServiceName].message.Error()
			} else {
				fmt.Printf("%v Cannot activate the %v service\n", uiSettings.iconError, ServiceName)
			}
		} else {
			connectResult.YggdrasilStarted = true
			interactivePrintf("%v Activated the %v service\n", uiSettings.iconOK, ServiceName)
		}
		durations[ServiceName] = time.Since(start)
		interactivePrintf("\nSuccessfully connected to Red Hat!\n")
	}

	if !uiSettings.isMachineReadable {
		/* 5. Show footer message */
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")

		/* 6. Optionally display duration time of each sub-action */
		showTimeDuration(durations)
	}

	err = showErrorMessages("connect", errorMessages)
	if err != nil {
		return err
	}

	return cli.Exit(connectResult, 0)
}
