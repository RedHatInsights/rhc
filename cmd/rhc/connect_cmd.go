package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
	errorMessages := make(map[string]LogMessage)
	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	var returnedMsg string
	returnedMsg, err = rhsm.RegisterRHSM(ctx, features.ContentFeature.Enabled)
	if err != nil {
		connectResult.RHSMConnected = false
		errorMessages["rhsm"] = LogMessage{
			level: slog.LevelError,
			message: fmt.Errorf("cannot connect to Red Hat Subscription Management: %w",
				err)}
		connectResult.RHSMConnectError = errorMessages["rhsm"].message.Error()
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
		ui.Printf("%s[%v] %v\n", ui.Indent.Small, ui.Icons.Ok, returnedMsg)
		if features.ContentFeature.Enabled {
			connectResult.Features.Content.Successful = true
			ui.Printf(
				"%s[%v] Content ... Red Hat repository file generated\n",
				ui.Indent.Medium,
				ui.Icons.Ok,
			)
		} else {
			connectResult.Features.Content.Successful = false
			ui.Printf("%s[ ] Content ... Red Hat repository file not generated\n", ui.Indent.Medium)
		}
	}
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	if features.AnalyticsFeature.Enabled {
		if errors, exist := errorMessages["rhsm"]; exist {
			if errors.level == slog.LevelError {
				ui.Printf(
					"%s[%v] Skipping connection to Red Hat Lightspeed (formerly Insights)\n",
					ui.Indent.Medium,
					ui.Icons.Error,
				)
			}
		} else {
			start = time.Now()
			err = ui.Spinner(datacollection.RegisterInsightsClient, ui.Indent.Medium, "Connecting to Red Hat Lightspeed (formerly Insights)...")
			if err != nil {
				connectResult.Features.Analytics.Successful = false
				errorMessages["insights"] = LogMessage{
					level:   slog.LevelError,
					message: fmt.Errorf("cannot connect to Red Hat Lightspeed (formerly Insights): %w", err)}
				connectResult.Features.Analytics.Error = errorMessages["insights"].message.Error()
				ui.Printf(
					"%s[%v] Analytics ... Cannot connect to Red Hat Lightspeed (formerly Insights)\n",
					ui.Indent.Medium,
					ui.Icons.Error,
				)
			} else {
				connectResult.Features.Analytics.Successful = true
				ui.Printf(
					"%s[%v] Analytics ... Connected to Red Hat Lightspeed (formerly Insights)\n",
					ui.Indent.Medium,
					ui.Icons.Ok,
				)
			}
			durations["insights"] = time.Since(start)
		}
	} else {
		connectResult.Features.Analytics.Successful = false
		ui.Printf("%s[ ] Analytics ... Connecting to Red Hat Lightspeed (formerly Insights) disabled\n", ui.Indent.Medium)
	}

	if features.ManagementFeature.Enabled {
		/* 3. Start yggdrasil (rhcd) service */
		if rhsmErrMsg, exist := errorMessages["rhsm"]; exist && rhsmErrMsg.level == slog.LevelError {
			connectResult.Features.RemoteManagement.Successful = false
			ui.Printf(
				"%s[%v] Skipping activation of %v service\n",
				ui.Indent.Medium,
				ui.Icons.Error,
				ServiceName,
			)
		} else {
			start = time.Now()
			progressMessage := fmt.Sprintf(" Activating the %v service", ServiceName)
			err = ui.Spinner(remotemanagement.ActivateServices, ui.Indent.Medium, progressMessage)
			if err != nil {
				connectResult.Features.RemoteManagement.Successful = false
				errorMessages[ServiceName] = LogMessage{
					level: slog.LevelError,
					message: fmt.Errorf("cannot activate %s service: %w",
						ServiceName, err)}

				connectResult.Features.RemoteManagement.Error = errorMessages[ServiceName].message.Error()
				ui.Printf(
					"%s[%v] Remote Management ... Cannot activate the %v service\n",
					ui.Indent.Medium,
					ui.Icons.Error,
					ServiceName,
				)
			} else {
				connectResult.Features.RemoteManagement.Successful = true
				ui.Printf(
					"%s[%v] Remote Management ... Activated the %v service\n",
					ui.Indent.Medium,
					ui.Icons.Ok,
					ServiceName,
				)
			}
			durations[ServiceName] = time.Since(start)
		}
	} else {
		connectResult.Features.RemoteManagement.Successful = false
		if features.ManagementFeature.Reason != "" {
			ui.Printf(
				"%s[ ] Management .... Starting %s service disabled (%s)\n",
				ui.Indent.Medium,
				ServiceName,
				features.ManagementFeature.Reason,
			)
		} else {
			ui.Printf(
				"%s[ ] Management .... Starting %s service disabled\n",
				ui.Indent.Medium,
				ServiceName,
			)
		}
	}

	ui.Printf("\nSuccessfully connected to Red Hat!\n")

	if !ui.IsOutputMachineReadable() {
		/* 5. Show footer message */
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")

		/* 6. Optionally display duration time of each sub-action */
		showTimeDuration(durations)
	}

	err = showErrorMessages("connect", errorMessages)
	if err != nil {
		return err
	}

	if ui.IsOutputMachineReadable() {
		fmt.Println(connectResult.Error())
	}

	return nil
}
