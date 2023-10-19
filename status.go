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
func rhsmStatus(systemStatus *SystemStatus, uiPreferences *UserInterfacePreferences) error {

	uuid, err := getConsumerUUID()
	if err != nil {
		return fmt.Errorf("unable to get consumer UUID: %s", err)
	}
	if uuid == "" {
		if uiPreferences.MachineReadable {
			systemStatus.RHSMConnected = false
		} else {
			fmt.Printf(uiPreferences.DisconnectedPrefix + " Not connected to Red Hat Subscription Management\n")
		}
	} else {
		if uiPreferences.MachineReadable {
			systemStatus.RHSMConnected = true
		} else {
			fmt.Printf(uiPreferences.ConnectedPrefix + " Connected to Red Hat Subscription Management\n")
		}
	}
	return nil
}

// insightStatus tries to print status of insights client
func insightStatus(systemStatus *SystemStatus, uiPreferences *UserInterfacePreferences) {
	var s *spinner.Spinner
	if uiPreferences.IsColorful {
		s = spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = " Checking Red Hat Insights..."
		s.Start()
	}
	isRegistered, err := insightsIsRegistered()
	if uiPreferences.IsColorful {
		s.Stop()
	}
	if isRegistered {
		if uiPreferences.MachineReadable {
			systemStatus.InsightsConnected = true
		} else {
			fmt.Print(uiPreferences.ConnectedPrefix + " Connected to Red Hat Insights\n")
		}
	} else {
		if err == nil {
			if uiPreferences.MachineReadable {
				systemStatus.InsightsConnected = false
			} else {
				fmt.Print(uiPreferences.DisconnectedPrefix + " Not connected to Red Hat Insights\n")
			}
		} else {
			if uiPreferences.MachineReadable {
				systemStatus.InsightsConnected = false
				systemStatus.InsightsError = err
			} else {
				fmt.Printf(uiPreferences.ErrorPrefix+" Cannot execute insights-client: %v\n", err)
			}
		}
	}
}

// serviceStatus tries to print status of yggdrasil.service or rhcd.service
func serviceStatus(systemStatus *SystemStatus, uiPreferences *UserInterfacePreferences) error {
	ctx := context.Background()
	conn, err := systemd.NewSystemConnectionContext(ctx)
	if err != nil {
		systemStatus.RHCDRunning = false
		systemStatus.RHCDError = err
		return fmt.Errorf("unable to connect to systemd: %s", err)
	}
	defer conn.Close()
	unitName := ShortName + "d.service"
	properties, err := conn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		systemStatus.RHCDRunning = false
		systemStatus.RHCDError = err
		return fmt.Errorf("unable to get properties of %s: %s", unitName, err)
	}
	activeState := properties["ActiveState"]
	if activeState.(string) == "active" {
		if uiPreferences.MachineReadable {
			systemStatus.RHCDRunning = true
		} else {
			fmt.Printf(uiPreferences.ConnectedPrefix+" The %v daemon is active\n", BrandName)
		}
	} else {
		if uiPreferences.MachineReadable {
			systemStatus.RHCDRunning = false
		} else {
			fmt.Printf(uiPreferences.DisconnectedPrefix+" The %v daemon is inactive\n", BrandName)
		}
	}
	return nil
}
