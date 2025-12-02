package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/redhatinsights/rhc/internal/features"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/ui"
)

type FeatureResult struct {
	Enabled    bool   `json:"enabled"`
	Successful bool   `json:"successful"`
	Error      string `json:"error,omitempty"`
}

// ConnectResult is structure holding information about results
// of connect command. The result could be printed in machine-readable format.
type ConnectResult struct {
	Hostname         string `json:"hostname"`
	HostnameError    string `json:"hostname_error,omitempty"`
	UID              int    `json:"uid"`
	UIDError         string `json:"uid_error,omitempty"`
	RHSMConnected    bool   `json:"rhsm_connected"`
	RHSMConnectError string `json:"rhsm_connect_error,omitempty"`
	Features         struct {
		Content          FeatureResult `json:"content"`
		Analytics        FeatureResult `json:"analytics"`
		RemoteManagement FeatureResult `json:"remote_management"`
	} `json:"features"`
	format string
}

// Error implement error interface for structure ConnectResult
func (connectResult *ConnectResult) Error() string {
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

func (connectResult *ConnectResult) errorMessages() map[string]string {
	errorMessages := make(map[string]string)
	if connectResult.Features.Content.Error != "" {
		errorMessages["rhsm"] = connectResult.Features.Content.Error
	}
	if connectResult.Features.Analytics.Error != "" {
		errorMessages["insights"] = connectResult.Features.Analytics.Error
	}
	if connectResult.Features.RemoteManagement.Error != "" {
		errorMessages[ServiceName] = connectResult.Features.RemoteManagement.Error
	}
	return errorMessages
}

// TryRegisterRHSM will attempt to register the system with Red Hat Subscription Management.
// If this fails, then both RHSMConnected and Features.Content.Successful will be set to false, and the error message
// will be stored in RHSMConnectError.
func (connectResult *ConnectResult) TryRegisterRHSM(ctx *cli.Context) {
	returnedMsg, err := rhsm.RegisterRHSM(ctx, features.ContentFeature.Enabled)
	if err != nil {
		connectResult.RHSMConnected = false
		connectResult.RHSMConnectError = fmt.Sprintf("cannot connect to Red Hat Subscription Management: %s", err)
		connectResult.Features.Content.Successful = false
		ui.Printf(
			"%s[%v] Cannot connect to Red Hat Subscription Management\n",
			ui.Indent.Small,
			ui.Icons.Error,
		)
		ui.Printf(
			"%s[%v] Skipping generation of Red Hat repository file\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
	} else {
		connectResult.RHSMConnected = true
		ui.Printf("%s[%v] %s\n", ui.Indent.Small, ui.Icons.Ok, returnedMsg)
		if features.ContentFeature.Enabled {
			connectResult.Features.Content.Successful = true
			infoMsg := "Red Hat repository file generated"
			ui.Printf("%s[%v] Content ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
		} else {
			connectResult.Features.Content.Successful = false
			ui.Printf("%s[ ] Content ... Red Hat repository file not generated\n", ui.Indent.Medium)
		}
	}
}

// TryRegisterInsightsClient will attempt to register the system with Red Hat Lightspeed.
// If this fails, then Features.Analytics.Successful will be set to false, and the
// error message will be stored in Features.Analytics.Error.
func (connectResult *ConnectResult) TryRegisterInsightsClient() {
	if !features.AnalyticsFeature.Enabled {
		connectResult.Features.Analytics.Successful = false
		ui.Printf("%s[ ] Analytics ... Connecting to Red Hat Lightspeed (formerly Insights) disabled\n", ui.Indent.Medium)
		return
	}

	if connectResult.RHSMConnectError != "" {
		ui.Printf(
			"%s[%v] Skipping connection to Red Hat Lightspeed (formerly Insights)\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	err := ui.Spinner(datacollection.RegisterInsightsClient, ui.Indent.Medium, "Connecting to Red Hat Lightspeed (formerly Insights)...")
	if err != nil {
		connectResult.Features.Analytics.Successful = false
		connectResult.Features.Analytics.Error = fmt.Sprintf("cannot connect to Red Hat Lightspeed (formerly Insights): %v", err)
		ui.Printf(
			"%s[%v] Analytics ... Cannot connect to Red Hat Lightspeed (formerly Insights)\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
		return
	}

	connectResult.Features.Analytics.Successful = true
	infoMsg := "Connected to Red Hat Lightspeed (formerly Insights)"
	ui.Printf("%s[%v] Analytics ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
}

// TryActivateServices will attempt to activate the yggdrasil service.
// If this fails, then Features.RemoteManagement.Successful will be set to false, and the
// error message will be stored in Features.RemoteManagement.Error.
func (connectResult *ConnectResult) TryActivateServices() {
	if !features.ManagementFeature.Enabled {
		connectResult.Features.RemoteManagement.Successful = false
		if features.ManagementFeature.Reason != "" {
			infoMsg := fmt.Sprintf("Starting %s service disabled (%s)", ServiceName, features.ManagementFeature.Reason)
			ui.Printf("%s[ ] Management .... %s\n", ui.Indent.Medium, infoMsg)
		} else {
			infoMsg := fmt.Sprintf("Starting %s service disabled", ServiceName)
			ui.Printf("%s[ ] Management .... %s\n", ui.Indent.Medium, infoMsg)
		}
		return
	}

	if connectResult.RHSMConnectError != "" {
		connectResult.Features.RemoteManagement.Successful = false
		ui.Printf(
			"%s[%v] Skipping activation of %v service\n",
			ui.Indent.Medium,
			ui.Icons.Error,
			ServiceName,
		)
		return
	}

	progressMessage := fmt.Sprintf(" Activating the %v service", ServiceName)
	err := ui.Spinner(remotemanagement.ActivateServices, ui.Indent.Medium, progressMessage)
	if err != nil {
		connectResult.Features.RemoteManagement.Successful = false
		connectResult.Features.RemoteManagement.Error = fmt.Sprintf("cannot activate the %s service: %v", ServiceName, err)
		ui.Printf(
			"%s[%v] Remote Management ... Cannot activate the %v service\n",
			ui.Indent.Medium,
			ui.Icons.Error,
			ServiceName,
		)
		return
	}

	connectResult.Features.RemoteManagement.Successful = true
	infoMsg := fmt.Sprintf("Activated the %s service", ServiceName)
	ui.Printf("%s[%v] Remote Management ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
}

// beforeConnectAction ensures that user has supplied correct CLI options
// and there is no conflict between them. When there is anything wrong,
// then this function will invoke cli.Exit() with an appropriate error
// message and error code. The exit codes are defined in the
// constants.go module
func beforeConnectAction(ctx *cli.Context) error {
	// First check if machine-readable format is used
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	// When machine is already connected, then return error
	uuid, err := rhsm.GetConsumerUUID()
	if err != nil {
		return cli.Exit(
			fmt.Sprintf("unable to get consumer UUID: %s", err),
			ExitCodeSoftware,
		)
	}
	if uuid != "" {
		return cli.Exit("this system is already connected", ExitCodeUsage)
	}

	username := ctx.String("username")
	password := ctx.String("password")
	organization := ctx.String("organization")
	activationKeys := ctx.StringSlice("activation-key")
	enabledFeatures := ctx.StringSlice("enable-feature")
	disabledFeatures := ctx.StringSlice("disable-feature")
	contentTemplates := ctx.StringSlice("content-template")

	if len(activationKeys) > 0 {
		if username != "" {
			exitErr := cli.Exit(
				"--username and --activation-key can not be used together",
				ExitCodeUsage,
			)
			return exitErr

		}
		if password != "" {
			exitErr := cli.Exit(
				"--password and --activation-key can not be used together",
				ExitCodeUsage,
			)
			return exitErr

		}
		if organization == "" {
			exitErr := cli.Exit(
				"--organization is required, when --activation-key is used",
				ExitCodeUsage,
			)
			return exitErr
		}
	}

	// Exit if username/password or activation key/organization haven't been provided,
	// and we cannot ask interactively.
	if !ui.IsInteractive() {
		if (username == "" || password == "") && (len(activationKeys) == 0 || organization == "") {
			exitErr := cli.Exit(
				"--username/--password or --organization/--activation-key are required when a machine-readable format is used",
				ExitCodeUsage,
			)
			return exitErr
		}
	}

	err = features.CheckFeatureInput(&enabledFeatures, &disabledFeatures)
	if err != nil {
		return cli.Exit(err.Error(), ExitCodeUsage)
	}

	if !features.ContentFeature.Enabled && len(contentTemplates) > 0 {
		return cli.Exit(
			"'--content-template' can not be used together with '--disable-feature content'",
			ExitCodeUsage,
		)
	}

	err = checkForUnknownArgs(ctx)
	if err != nil {
		return cli.Exit(err.Error(), ExitCodeUsage)
	}

	return nil
}

// connectAction tries to register system against Red Hat Subscription Management,
// gather the profile information that the system will configure
// connect system to Red Hat Lightspeed, and it also tries to start rhcd service
func connectAction(ctx *cli.Context) error {
	var connectResult ConnectResult
	connectResult.format = ctx.String("format")

	uid := os.Getuid()
	if uid != 0 {
		errMsg := "non-root user cannot connect system"
		exitCode := 1
		if ui.IsOutputMachineReadable() {
			connectResult.UID = uid
			connectResult.UIDError = errMsg
			return cli.Exit(connectResult, exitCode)
		} else {
			return cli.Exit(fmt.Errorf("error: %s", errMsg), exitCode)
		}
	}

	hostname, err := os.Hostname()
	if ui.IsOutputMachineReadable() {
		connectResult.Hostname = hostname
	}
	if err != nil {
		exitCode := 1
		if ui.IsOutputMachineReadable() {
			connectResult.HostnameError = err.Error()
			return cli.Exit(connectResult, exitCode)
		} else {
			return cli.Exit(err, exitCode)
		}
	}

	ui.Printf("Connecting %v to %v.\nThis might take a few seconds.\n\n", hostname, Provider)

	var featuresStr []string
	for _, feature := range features.KnownFeatures {
		if feature.Enabled {
			if ui.IsOutputMachineReadable() {
				switch feature.ID {
				case "content":
					connectResult.Features.Content.Enabled = true
				case "analytics":
					connectResult.Features.Analytics.Enabled = true
				case "remote-management":
					connectResult.Features.RemoteManagement.Enabled = true
				}
			}
			featuresStr = append(featuresStr, "["+ui.Icons.Ok+"]"+feature.ID)
		} else {
			if ui.IsOutputMachineReadable() {
				switch feature.ID {
				case "content":
					connectResult.Features.Content.Enabled = false
				case "analytics":
					connectResult.Features.Analytics.Enabled = false
				case "remote-management":
					connectResult.Features.RemoteManagement.Enabled = false
				}
			}
			featuresStr = append(featuresStr, "[ ]"+feature.ID)
		}
	}
	featuresListStr := strings.Join(featuresStr, ", ")
	ui.Printf("Features preferences: %s\n\n", featuresListStr)

	var start time.Time
	durations := make(map[string]time.Duration)

	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	connectResult.TryRegisterRHSM(ctx)
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	start = time.Now()
	connectResult.TryRegisterInsightsClient()
	durations["insights"] = time.Since(start)

	/* 3. Start yggdrasil (rhcd) service */
	start = time.Now()
	connectResult.TryActivateServices()
	durations[ServiceName] = time.Since(start)

	ui.Printf("\nSuccessfully connected to Red Hat!\n")

	if !ui.IsOutputMachineReadable() {
		/* 5. Show footer message */
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")

		/* 6. Optionally display duration time of each sub-action */
		showTimeDuration(durations)
	}

	err = showErrorMessages("connect", connectResult.errorMessages())
	if err != nil {
		return err
	}

	if ui.IsOutputMachineReadable() {
		fmt.Println(connectResult.Error())
	}

	return nil
}
