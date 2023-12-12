package main

import (
	"context"
	"fmt"
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
		if uiSettings.isMachineReadable {
			systemStatus.RHSMConnected = false
		} else {
			fmt.Printf(uiSettings.iconInfo + " Not connected to Red Hat Subscription Management\n")
		}
	} else {
		if uiSettings.isMachineReadable {
			systemStatus.RHSMConnected = true
		} else {
			fmt.Printf(uiSettings.iconOK + " Connected to Red Hat Subscription Management\n")
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
		if err == nil {
			if uiSettings.isMachineReadable {
				systemStatus.InsightsConnected = false
			} else {
				fmt.Print(uiSettings.iconInfo + " Not connected to Red Hat Insights\n")
			}
		} else {
			if uiSettings.isMachineReadable {
				systemStatus.InsightsConnected = false
				systemStatus.InsightsError = err
			} else {
				fmt.Printf(uiSettings.iconError+" Cannot execute insights-client: %v\n", err)
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
		systemStatus.YggdrasilError = err
		return fmt.Errorf("unable to connect to systemd: %s", err)
	}
	defer conn.Close()
	unitName := ServiceName + ".service"
	properties, err := conn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		systemStatus.YggdrasilRunning = false
		systemStatus.YggdrasilError = err
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
		if uiSettings.isMachineReadable {
			systemStatus.YggdrasilRunning = false
		} else {
			fmt.Printf(uiSettings.iconInfo+" The %v service is inactive\n", ServiceName)
		}
	}
	return nil
}
