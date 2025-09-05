package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v2"

	"github.com/briandowns/spinner"
	systemd "github.com/coreos/go-systemd/v22/dbus"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/localization"
)

// rhsmStatus tries to print status provided by RHSM D-Bus API. If we provide
// output in machine-readable format, then we only set files in SystemStatus
// structure and content of this structure will be printed later
func rhsmStatus(systemStatus *SystemStatus) error {

	uuid, err := getConsumerUUID()
	if err != nil {
		systemStatus.returnCode += 1
		systemStatus.RHSMError = err.Error()
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}
	if uuid == "" {
		systemStatus.returnCode += 1
		if uiSettings.isMachineReadable {
			systemStatus.RHSMConnected = false
		} else {
			interactivePrintf(
				"%s[ ] Not connected to Red Hat Subscription Management\n",
				smallIndent,
			)
		}
	} else {
		if uiSettings.isMachineReadable {
			systemStatus.RHSMConnected = true
		} else {
			interactivePrintf(
				"%s[%v] Connected to Red Hat Subscription Management\n",
				smallIndent,
				uiSettings.iconOK,
			)
		}
	}
	return nil
}

// isContentEnabled tries to read the configuration file rhsm.conf using D-Bus API
// and get the manage_repos option from the section [rhsm]. If the option is equal
// to "1", then content is managed by subscription-manager/RHSM
func isContentEnabled(systemStatus *SystemStatus) error {
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
		return unpackRHSMError(err)
	}

	uuid, err := getConsumerUUID()
	if err != nil {
		systemStatus.returnCode += 1
		systemStatus.ContentError = err.Error()
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}

	if contentEnabled == "1" && uuid != "" {
		if uiSettings.isMachineReadable {
			systemStatus.ContentEnabled = true
		} else {
			interactivePrintf(
				"%s[%v] Content ... Red Hat repository file generated\n",
				mediumIndent,
				uiSettings.iconOK,
			)
		}
	} else {
		if uiSettings.isMachineReadable {
			systemStatus.ContentEnabled = false
		} else {
			if uuid != "" {
				interactivePrintf(
					"%s[ ] Content ... Generating of Red Hat repository file disabled in rhsm.conf\n",
					mediumIndent,
				)
			} else {
				interactivePrintf(
					"%s[ ] Content ... Red Hat repository file not generated\n",
					mediumIndent,
				)
			}
		}
	}
	return nil
}

// insightStatus tries to print status of insights client
func insightStatus(systemStatus *SystemStatus) error {
	var s *spinner.Spinner
	if uiSettings.isRich {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Prefix = mediumIndent + "["
		s.Suffix = "] Checking Red Hat Insights..."
		s.Start()
	}
	isRegistered, err := datacollection.InsightsClientIsRegistered()
	if uiSettings.isRich {
		s.Stop()
	}
	if isRegistered {
		if uiSettings.isMachineReadable {
			systemStatus.InsightsConnected = true
		} else {
			interactivePrintf(
				"%s[%v] Analytics ... Connected to Red Hat Insights\n",
				mediumIndent,
				uiSettings.iconOK,
			)
		}
	} else {
		systemStatus.returnCode += 1
		if err == nil {
			if uiSettings.isMachineReadable {
				systemStatus.InsightsConnected = false
			} else {
				interactivePrintf(
					"%s[ ] Analytics ... Not connected to Red Hat Insights\n",
					mediumIndent,
				)
			}
		} else {
			if uiSettings.isMachineReadable {
				systemStatus.InsightsConnected = false
				systemStatus.InsightsError = err.Error()
			}
			return err
		}
	}
	return nil
}

// serviceStatus tries to print status of yggdrasil.service or rhcd.service
func serviceStatus(systemStatus *SystemStatus) error {
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
		if uiSettings.isMachineReadable {
			systemStatus.YggdrasilRunning = true
		} else {
			interactivePrintf(
				"%s[%v] Remote Management ... The %v service is active\n",
				mediumIndent,
				uiSettings.iconOK,
				ServiceName,
			)
		}
	} else {
		systemStatus.returnCode += 1
		loadState := properties["LoadState"]
		if loadState == "loaded" {
			if uiSettings.isMachineReadable {
				systemStatus.YggdrasilRunning = false
			} else {
				interactivePrintf(
					"%s[ ] Remote Management ... The %v service is inactive\n",
					mediumIndent,
					ServiceName,
				)
			}
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
						if uiSettings.isMachineReadable {
							systemStatus.YggdrasilRunning = false
							systemStatus.YggdrasilError = loadErrorString
						} else {
							interactivePrintf(
								"%s[%s] Remote Management ... %v\n",
								mediumIndent,
								uiSettings.iconError,
								loadErrorString,
							)
						}
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

	return checkForUnknownArgs(ctx)
}

// statusAction tries to print status of system. It means that it gives
// answer on following questions:
// 1. Is system registered to Red Hat Subscription Management?
// 2. Is system connected to Red Hat Insights?
// 3. Is yggdrasil.service (rhcd.service) running?
// Status can be printed as human-readable text or machine-readable JSON document.
// Format is influenced by --format json CLI option stored in CLI context
func statusAction(ctx *cli.Context) (err error) {
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
	if uiSettings.isMachineReadable {
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
		if uiSettings.isMachineReadable {
			systemStatus.HostnameError = err.Error()
		} else {
			return cli.Exit(err, 1)
		}
	}

	if uiSettings.isMachineReadable {
		systemStatus.SystemHostname = hostname
	} else {
		fmt.Printf("Connection status for %v:\n\n", hostname)
	}

	/* 1. Get Status of RHSM */
	err = rhsmStatus(&systemStatus)
	if err != nil {
		interactivePrintf(
			"%s[%s] Red Hat Subscription Management ... %s\n",
			smallIndent,
			uiSettings.iconError,
			err,
		)
	}

	/* 2. Is content enabled */
	err = isContentEnabled(&systemStatus)
	if err != nil {
		interactivePrintf(
			"%s[%s] Content ... %s\n",
			mediumIndent,
			uiSettings.iconError,
			err,
		)
	}

	/* 3. Get status of insights-client */
	err = insightStatus(&systemStatus)
	if err != nil {
		interactivePrintf(
			"%s[%v] Analytics ... Cannot detect Red Hat Insights status: %v\n",
			mediumIndent,
			uiSettings.iconError,
			err,
		)
	}

	/* 3. Get status of yggdrasil (rhcd) service */
	err = serviceStatus(&systemStatus)
	if err != nil {
		interactivePrintf(
			"%s[%s] Remote Management ... %s\n",
			mediumIndent,
			uiSettings.iconError,
			err,
		)
	}

	if !uiSettings.isMachineReadable {
		fmt.Printf("\nManage your connected systems: https://red.ht/connector\n")
	}

	// At the end check if all statuses are correct.
	// If not, return 1 exit code without any message.
	if systemStatus.returnCode != 0 {
		return cli.Exit("", 1)
	}

	return nil
}
