package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/subpop/go-log"
)

var StatePath = "/var/lib/rhc/connected"

func IsConnected() bool {
	_, err := os.Stat(StatePath)
	return err == nil
}

func SetConnected(state bool) {
	if state {
		timestamp := time.Now().Format("2006-01-02T15:04:05")
		err := os.WriteFile(StatePath, []byte(timestamp), 0664)
		if err != nil {
			log.Errorf("error writing state file: %v", err)
		}
	} else {
		err := os.Remove(StatePath)
		if err != nil {
			log.Errorf("error removing state file: %v", err)
		}
	}
}

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
