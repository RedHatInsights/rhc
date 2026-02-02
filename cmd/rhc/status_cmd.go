package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/urfave/cli/v2"

	"github.com/briandowns/spinner"
	systemd "github.com/coreos/go-systemd/v22/dbus"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/localization"
	"github.com/redhatinsights/rhc/internal/ui"
)

// rhsmStatus tries to print status provided by RHSM D-Bus API. If we provide
// output in machine-readable format, then we only set files in SystemStatus
// structure and content of this structure will be printed later
func rhsmStatus(systemStatus *SystemStatus) error {
	slog.Info("Checking status of Red Hat Subscription Management")

	uuid, err := rhsm.GetConsumerUUID()
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

// isContentEnabled tries to read the configuration file rhsm.conf using D-Bus API
// and get the manage_repos option from the section [rhsm]. If the option is equal
// to "1", then content is managed by subscription-manager/RHSM
func isContentEnabled(systemStatus *SystemStatus) error {
	slog.Info("Checking content status")

	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}

	locale := localization.GetLocale()

	config := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")

	var contentEnabled string
	if err := config.Call(
		"com.redhat.RHSM1.Config.Get",
		0,
		"rhsm.manage_repos",
		locale).Store(&contentEnabled); err != nil {
		systemStatus.returnCode += 1
		systemStatus.ContentError = err.Error()
		return rhsm.UnpackDBusError(err)
	}

	uuid, err := rhsm.GetConsumerUUID()
	if err != nil {
		systemStatus.returnCode += 1
		systemStatus.ContentError = err.Error()
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}

	if contentEnabled == "1" && uuid != "" {
		systemStatus.ContentEnabled = true
		infoMsg := "Red Hat repository file generated"
		slog.Info(infoMsg)
		ui.Printf("%s[%v] Content ... %v\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
	} else {
		systemStatus.ContentEnabled = false
		if uuid != "" {
			infoMsg := "Generating of Red Hat repository file disabled in rhsm.conf"
			slog.Info(infoMsg)
			ui.Printf("%s[ ] Content ... %v\n", ui.Indent.Medium, infoMsg)
		} else {
			infoMsg := "Red Hat repository file not generated"
			slog.Info(infoMsg)
			ui.Printf("%s[ ] Content ... %v\n", ui.Indent.Medium, infoMsg)
		}
	}
	return nil
}

// insightStatus tries to print status of insights client
func insightStatus(systemStatus *SystemStatus) error {
	slog.Info("Checking status of Red Hat Lightspeed")

	var s *spinner.Spinner
	if ui.IsOutputRich() {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = ui.Indent.Medium + "["
		s.Suffix = "] Checking Red Hat Lightspeed (formerly Insights)..."
		s.Start()
	}
	isRegistered, err := datacollection.InsightsClientIsRegistered()
	if ui.IsOutputRich() {
		s.Stop()
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
	slog.Info(fmt.Sprintf("Checking status of %s service", ServiceName))

	ctx := context.Background()
	conn, err := systemd.NewSystemConnectionContext(ctx)
	if err != nil {
		systemStatus.YggdrasilRunning = false
		systemStatus.YggdrasilError = err.Error()
		return fmt.Errorf("unable to connect to systemd: %s", err)
	}
	defer conn.Close()
	unitName := ServiceName + ".service"
	properties, err := conn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		systemStatus.YggdrasilRunning = false
		systemStatus.YggdrasilError = err.Error()
		return fmt.Errorf("unable to get properties of %s: %s", unitName, err)
	}

	activeState := properties["ActiveState"]
	if activeState.(string) == "active" {
		systemStatus.YggdrasilRunning = true
		infoMsg := fmt.Sprintf("The %v service is active", ServiceName)
		slog.Info(infoMsg)
		ui.Printf("%s[%v] Remote Management ... %v\n", ui.Indent.Medium, ui.Icons.Ok, infoMsg)
	} else {
		systemStatus.returnCode += 1
		loadState := properties["LoadState"]
		if loadState == "loaded" {
			systemStatus.YggdrasilRunning = false
			warnMsg := fmt.Sprintf("The %v service is inactive but not loaded", ServiceName)
			slog.Warn(warnMsg)
			ui.Printf("%s[ ] Remote Management ... %v\n", ui.Indent.Medium, warnMsg)
		} else {
			loadError := properties["LoadError"]
			// This part of the systemd D-Bus API is a little bit tricky. It returns
			// an empty interface. It should contain a slice of two interfaces. The first
			// interface in the slice should be the string representing error ID
			// (e.g. "org.freedesktop.systemd1.NoSuchUnit"). The second interface should be
			// also string representing the human-readable error message.
			loadErrorType := reflect.TypeOf(loadError)
			// Check if the type is []interface{}
			if loadErrorType.Kind() == reflect.Slice && loadErrorType.Elem().Kind() == reflect.Interface {
				loadErrorSlice := loadError.([]interface{})
				if len(loadErrorSlice) >= 2 {
					// Check if the type of the second interface is string
					if reflect.TypeOf(loadErrorSlice[1]).Kind() == reflect.String {
						loadErrorString := loadErrorSlice[1].(string)
						systemStatus.YggdrasilRunning = false
						systemStatus.YggdrasilError = loadErrorString
						slog.Error(loadErrorString)
						ui.Printf(
							"%s[%s] Remote Management ... %v\n",
							ui.Indent.Medium,
							ui.Icons.Error,
							loadErrorString,
						)
					}
				}
			}
		}
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
func beforeStatusAction(ctx *cli.Context) error {
	err := setupFormatOption(ctx)
	if err != nil {
		return err
	}

	configureUI(ctx)

	return checkForUnknownArgs(ctx)
}

// statusAction tries to print status of system. It means that it gives
// answer on following questions:
// 1. Is system registered to Red Hat Subscription Management?
// 2. Is system connected to Red Hat Lightspeed?
// 3. Is yggdrasil.service (rhcd.service) running?
// Status can be printed as human-readable text or machine-readable JSON document.
// Format is influenced by --format json CLI option stored in CLI context
func statusAction(ctx *cli.Context) (err error) {
	logCommandStart(ctx)

	var systemStatus SystemStatus
	var machineReadablePrintFunc func(systemStatus *SystemStatus) error

	format := ctx.String("format")
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
					1)
			}
			// When any of status is not correct, then return 1 exit code
			if systemStatus.returnCode != 0 {
				err = cli.Exit("", 1)
			}
		}(&systemStatus)
	}

	hostname, err := os.Hostname()
	if err != nil {
		if ui.IsOutputMachineReadable() {
			systemStatus.HostnameError = err.Error()
		} else {
			return cli.Exit(err, 1)
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
	// If not, return 1 exit code without any message.
	if systemStatus.returnCode != 0 {
		return cli.Exit("", 1)
	}

	return nil
}
