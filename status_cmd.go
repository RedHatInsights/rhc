package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"time"

	"github.com/briandowns/spinner"
	systemd "github.com/coreos/go-systemd/v22/dbus"
)

// rhsmStatus tries to print status provided by RHSM D-Bus API. If we provide
// output in machine-readable format, then we only set files in SystemStatus
// structure and content of this structure will be printed later
func rhsmStatus(systemStatus *SystemStatus) error {

	uuid, err := getConsumerUUID()
	if err != nil {
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}
	if uuid == "" {
		systemStatus.returnCode += 1
		if uiSettings.isMachineReadable {
			systemStatus.RHSMConnected = false
		} else {
			fmt.Printf("%v Not connected to Red Hat Subscription Management\n", uiSettings.iconInfo)
		}
	} else {
		if uiSettings.isMachineReadable {
			systemStatus.RHSMConnected = true
		} else {
			fmt.Printf("%v Connected to Red Hat Subscription Management\n", uiSettings.iconOK)
		}
	}
	return nil
}

// insightStatus tries to print status of insights client
func insightStatus(systemStatus *SystemStatus) {
	var s *spinner.Spinner
	if uiSettings.isRich {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = " Checking Red Hat Insights..."
		s.Start()
	}
	isRegistered, err := insightsIsRegistered()
	if uiSettings.isRich {
		s.Stop()
	}
	if isRegistered {
		if uiSettings.isMachineReadable {
			systemStatus.InsightsConnected = true
		} else {
			fmt.Print(uiSettings.iconOK + " Connected to Red Hat Insights\n")
		}
	} else {
		systemStatus.returnCode += 1
		if err == nil {
			if uiSettings.isMachineReadable {
				systemStatus.InsightsConnected = false
			} else {
				fmt.Print(uiSettings.iconInfo + " Not connected to Red Hat Insights\n")
			}
		} else {
			if uiSettings.isMachineReadable {
				systemStatus.InsightsConnected = false
				systemStatus.InsightsError = err.Error()
			} else {
				fmt.Printf(uiSettings.iconError+" Cannot detect Red Hat Insights status: %v\n", err)
			}
		}
	}
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
			fmt.Printf(uiSettings.iconOK+" The %v service is active\n", ServiceName)
		}
	} else {
		systemStatus.returnCode += 1
		if uiSettings.isMachineReadable {
			systemStatus.YggdrasilRunning = false
		} else {
			fmt.Printf(uiSettings.iconInfo+" The %v service is inactive\n", ServiceName)
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
		return cli.Exit(err, 1)
	}

	/* 2. Get status of insights-client */
	insightStatus(&systemStatus)

	/* 3. Get status of yggdrasil (rhcd) service */
	err = serviceStatus(&systemStatus)
	if err != nil {
		return cli.Exit(err, 1)
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
