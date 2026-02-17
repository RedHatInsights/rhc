package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/redhatinsights/rhc/internal/conf"
	"github.com/redhatinsights/rhc/internal/features"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/ui"
)

// ConnectResult is structure holding information about results
// of connect command. The result could be printed in machine-readable format.
type ConnectResult struct {
	Hostname         string                   `json:"hostname"`
	HostnameError    string                   `json:"hostname_error,omitempty"`
	UID              int                      `json:"uid"`
	UIDError         string                   `json:"uid_error,omitempty"`
	RHSMConnected    bool                     `json:"rhsm_connected"`
	RHSMConnectError string                   `json:"rhsm_connect_error,omitempty"`
	Features         features.FeaturesResults `json:"features"`
	format           string
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
	if connectResult.RHSMConnectError != "" {
		errorMessages["rhsm"] = connectResult.RHSMConnectError
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
	slog.Info("Registering the system with Red Hat Subscription Management")
	returnedMsg, err := rhsm.RegisterRHSM(ctx, features.ContentFeatureInst.WantEnabled())
	if err != nil {
		connectResult.RHSMConnected = false
		connectResult.RHSMConnectError = fmt.Sprintf("cannot connect to Red Hat Subscription Management: %s", err)
		connectResult.Features.Content.Successful = features.BoolPtr(false)
		slog.Error(connectResult.RHSMConnectError)
		ui.Printf(
			"%s[%v] Cannot connect to Red Hat Subscription Management\n",
			ui.Indent.Small,
			ui.Icons.Error,
		)
		slog.Warn("Skipping generation of Red Hat repository file (RHSM registration failed)")
		ui.Printf(
			"%s[%v] Skipping generation of Red Hat repository file\n",
			ui.Indent.Medium,
			ui.Icons.Error,
		)
	} else {
		connectResult.RHSMConnected = true
		slog.Info(returnedMsg)
		ui.Printf("%s[%v] %s\n", ui.Indent.Small, ui.Icons.Ok, returnedMsg)
		if features.ContentFeatureInst.WantEnabled() {
			connectResult.Features.Content.Successful = features.BoolPtr(true)
			infoMsg := "Red Hat repository file generated"
			slog.Info(infoMsg)
			ui.Printf("%s[%v] Content ... %s\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
		} else {
			connectResult.Features.Content.Successful = features.BoolPtr(false)
			slog.Info("Red Hat repository file not generated (content feature disabled)")
			ui.Printf("%s[ ] Content ... Red Hat repository file not generated\n", ui.Indent.Medium)
		}
	}
}

// beforeConnectAction ensures that user has supplied correct CLI options
// and there is no conflict between them. When there is anything wrong,
// then this function will invoke cli.Exit() with an appropriate error
// message and error code. The exit codes are defined in the
// constants.go module
func beforeConnectAction(ctx *cli.Context) error {
	slog.Debug("Command 'rhc connect' started")

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

	if len(enabledFeatures) > 0 || len(disabledFeatures) > 0 {
		enabled := true
		conf.ConnectFeaturesPreferences.Content = &enabled
		conf.ConnectFeaturesPreferences.Analytics = &enabled
		conf.ConnectFeaturesPreferences.RemoteManagement = &enabled
	}

	// Consolidate the features values from the drop-in configuration file and CLI flags
	consolidatedEnabledFeatures, consolidatedDisabledFeatures, err := features.ConsolidateSelectedFeatures(
		&conf.ConnectFeaturesPreferences, enabledFeatures, disabledFeatures)
	if err != nil {
		return cli.Exit(err.Error(), ExitCodeUsage)
	}

	// Validate the selected features and their dependencies
	err = features.ValidateSelectedFeatures(&consolidatedEnabledFeatures, &consolidatedDisabledFeatures)
	if err != nil {
		return cli.Exit(err.Error(), ExitCodeUsage)
	}

	if !features.ContentFeatureInst.WantEnabled() && len(contentTemplates) > 0 {
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
		slog.Error(errMsg)
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
		slog.Error(fmt.Sprintf("Error retrieving system hostname: %v", err))
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
		if feature.WantEnabled() {
			if ui.IsOutputMachineReadable() {
				switch feature.ID() {
				case features.ContentFeatureID:
					connectResult.Features.Content.Enabled = features.BoolPtr(true)
				case features.AnalyticsFeatureID:
					connectResult.Features.Analytics.Enabled = features.BoolPtr(true)
				case features.ManagementFeatureID:
					connectResult.Features.RemoteManagement.Enabled = features.BoolPtr(true)
				}
			}
			featuresStr = append(featuresStr, "["+ui.Icons.Ok+"]"+feature.ID())
			slog.Info(fmt.Sprintf("Feature '%s' marked enabled", feature.ID()))
		} else {
			if ui.IsOutputMachineReadable() {
				switch feature.ID() {
				case features.ContentFeatureID:
					connectResult.Features.Content.Enabled = features.BoolPtr(false)
				case features.AnalyticsFeatureID:
					connectResult.Features.Analytics.Enabled = features.BoolPtr(false)
				case features.ManagementFeatureID:
					connectResult.Features.RemoteManagement.Enabled = features.BoolPtr(false)
				}
			}
			featuresStr = append(featuresStr, "[ ]"+feature.ID())
			slog.Info(fmt.Sprintf("Feature '%s' marked disabled", feature.ID()))
		}
	}
	featuresListStr := strings.Join(featuresStr, ", ")
	ui.Printf("Features preferences: %s\n", featuresListStr)
	enabledFeatures := ctx.StringSlice("enable-feature")
	disabledFeatures := ctx.StringSlice("disable-feature")
	if _, err := os.Stat(features.RhcConnectFeaturesPreferencesPath); !os.IsNotExist(err) {
		if len(enabledFeatures) > 0 || len(disabledFeatures) > 0 {
			ui.Printf(ui.ColorGrey + " * Feature preferences in the config file was overridden by command line options.\n" + ui.ColorReset)
		}
	}
	ui.Printf("\n")

	var start time.Time
	durations := make(map[string]time.Duration)

	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	connectResult.TryRegisterRHSM(ctx)
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	start = time.Now()
	connectResult.Features.TryRegisterInsightsClient(features.AnalyticsFeatureInst.WantEnabled())
	durations["insights"] = time.Since(start)

	/* 3. Start yggdrasil (rhcd) service */
	start = time.Now()
	reasons := features.ManagementFeatureInst.Reason()
	connectResult.Features.TryActivateServices(
		features.ManagementFeatureInst.WantEnabled(),
		&reasons,
	)
	durations[ServiceName] = time.Since(start)

	if connectResult.RHSMConnected {
		err = features.DeleteFeaturePreferencesFromFile(features.RhcConnectFeaturesPreferencesPath)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to delete feature preferences file: %v", err))
		} else {
			slog.Info(fmt.Sprintf("Feature preferences file: '%s' deleted after successful connection'",
				features.RhcConnectFeaturesPreferencesPath))
		}
		ui.Printf("\nSuccessfully connected to Red Hat!\n")
	}

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
