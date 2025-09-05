package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
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

	// When machine is already connected, then return error
	uuid, err := getConsumerUUID()
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

	// When machine-readable format is used, then additional requirements have to be met.
	// User has to provide username & password or at least one activation key and organization,
	// because no interaction with user is possible in this case.
	if uiSettings.isMachineReadable {
		if (username == "" || password == "") && (len(activationKeys) == 0 || organization == "") {
			exitErr := cli.Exit(
				"--username/--password or --organization/--activation-key are required when a machine-readable format is used",
				ExitCodeUsage,
			)
			return exitErr
		}
	}

	err = checkFeatureInput(&enabledFeatures, &disabledFeatures)
	if err != nil {
		return cli.Exit(err.Error(), ExitCodeUsage)
	}

	if !ContentFeature.Enabled && len(contentTemplates) > 0 {
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

	var featuresStr []string
	for _, feature := range KnownFeatures {
		if feature.Enabled {
			if uiSettings.isMachineReadable {
				switch feature.ID {
				case "content":
					connectResult.Features.Content.Enabled = true
				case "analytics":
					connectResult.Features.Analytics.Enabled = true
				case "remote-management":
					connectResult.Features.RemoteManagement.Enabled = true
				}
			}
			featuresStr = append(featuresStr, "["+symbolOK+"]"+feature.ID)
		} else {
			if uiSettings.isMachineReadable {
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
	interactivePrintf("Features preferences: %s\n\n", featuresListStr)

	var start time.Time
	durations := make(map[string]time.Duration)
	errorMessages := make(map[string]LogMessage)
	/* 1. Register to RHSM, because we need to get consumer certificate. This blocks following action */
	start = time.Now()
	var returnedMsg string
	returnedMsg, err = registerRHSM(ctx, ContentFeature.Enabled)
	if err != nil {
		connectResult.RHSMConnected = false
		errorMessages["rhsm"] = LogMessage{
			level: slog.LevelError,
			message: fmt.Errorf("cannot connect to Red Hat Subscription Management: %w",
				err)}
		if uiSettings.isMachineReadable {
			connectResult.RHSMConnectError = errorMessages["rhsm"].message.Error()
			connectResult.Features.Content.Successful = false
		} else {
			fmt.Printf(
				"%s[%v] Cannot connect to Red Hat Subscription Management\n",
				smallIndent,
				uiSettings.iconError,
			)
			fmt.Printf(
				"%s[%v] Skipping generation of Red Hat repository file\n",
				mediumIndent,
				uiSettings.iconError,
			)
		}
	} else {
		connectResult.RHSMConnected = true
		interactivePrintf("%s[%v] %v\n", smallIndent, uiSettings.iconOK, returnedMsg)
		if ContentFeature.Enabled {
			if uiSettings.isMachineReadable {
				connectResult.Features.Content.Successful = true
			}
			interactivePrintf(
				"%s[%v] Content ... Red Hat repository file generated\n",
				mediumIndent,
				uiSettings.iconOK,
			)
		} else {
			if uiSettings.isMachineReadable {
				connectResult.Features.Content.Successful = false
			}
			interactivePrintf("%s[ ] Content ... Red Hat repository file not generated\n", mediumIndent)
		}
	}
	durations["rhsm"] = time.Since(start)

	/* 2. Register insights-client */
	if AnalyticsFeature.Enabled {
		if errors, exist := errorMessages["rhsm"]; exist {
			if errors.level == slog.LevelError {
				interactivePrintf(
					"%s[%v] Skipping connection to Red Hat Insights\n",
					mediumIndent,
					uiSettings.iconError,
				)
			}
		} else {
			start = time.Now()
			err = showProgress(" Connecting to Red Hat Insights...", datacollection.RegisterInsightsClient, mediumIndent)
			if err != nil {
				connectResult.Features.Analytics.Successful = false
				errorMessages["insights"] = LogMessage{
					level:   slog.LevelError,
					message: fmt.Errorf("cannot connect to Red Hat Insights: %w", err)}
				if uiSettings.isMachineReadable {
					connectResult.Features.Analytics.Error = errorMessages["insights"].message.Error()
				} else {
					fmt.Printf(
						"%s[%v] Analytics ... Cannot connect to Red Hat Insights\n",
						mediumIndent,
						uiSettings.iconError,
					)
				}
			} else {
				connectResult.Features.Analytics.Successful = true
				interactivePrintf(
					"%s[%v] Analytics ... Connected to Red Hat Insights\n",
					mediumIndent,
					uiSettings.iconOK,
				)
			}
			durations["insights"] = time.Since(start)
		}
	} else {
		if uiSettings.isMachineReadable {
			connectResult.Features.Analytics.Successful = false
		}
		interactivePrintf("%s[ ] Analytics ... Connecting to Red Hat Insights disabled\n", mediumIndent)
	}

	if ManagementFeature.Enabled {
		/* 3. Start yggdrasil (rhcd) service */
		if rhsmErrMsg, exist := errorMessages["rhsm"]; exist && rhsmErrMsg.level == slog.LevelError {
			connectResult.Features.RemoteManagement.Successful = false
			interactivePrintf(
				"%s[%v] Skipping activation of %v service\n",
				mediumIndent,
				uiSettings.iconError,
				ServiceName,
			)
		} else {
			start = time.Now()
			progressMessage := fmt.Sprintf(" Activating the %v service", ServiceName)
			err = showProgress(progressMessage, remotemanagement.ActivateServices, mediumIndent)
			if err != nil {
				connectResult.Features.RemoteManagement.Successful = false
				errorMessages[ServiceName] = LogMessage{
					level: slog.LevelError,
					message: fmt.Errorf("cannot activate %s service: %w",
						ServiceName, err)}
				if uiSettings.isMachineReadable {
					connectResult.Features.RemoteManagement.Error = errorMessages[ServiceName].message.Error()
				} else {
					interactivePrintf(
						"%s[%v] Remote Management ... Cannot activate the %v service\n",
						mediumIndent,
						uiSettings.iconError,
						ServiceName,
					)
				}
			} else {
				connectResult.Features.RemoteManagement.Successful = true
				interactivePrintf(
					"%s[%v] Remote Management ... Activated the %v service\n",
					mediumIndent,
					uiSettings.iconOK,
					ServiceName,
				)
			}
			durations[ServiceName] = time.Since(start)
		}
	} else {
		if uiSettings.isMachineReadable {
			connectResult.Features.RemoteManagement.Successful = false
		}
		if ManagementFeature.Reason != "" {
			interactivePrintf(
				"%s[ ] Management .... Starting %s service disabled (%s)\n",
				mediumIndent,
				ServiceName,
				ManagementFeature.Reason,
			)
		} else {
			interactivePrintf(
				"%s[ ] Management .... Starting %s service disabled\n",
				mediumIndent,
				ServiceName,
			)
		}
	}

	interactivePrintf("\nSuccessfully connected to Red Hat!\n")

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

	if uiSettings.isMachineReadable {
		fmt.Println(connectResult.Error())
	}

	return nil
}
