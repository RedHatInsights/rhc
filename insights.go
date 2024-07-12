package main

import (
	"os"
	"os/exec"
)

func registerInsights() error {
	cmd := exec.Command("/usr/bin/insights-client", "--register")

	return cmd.Run()
}

func unregisterInsights() error {
	cmd := exec.Command("/usr/bin/insights-client", "--unregister")

	return cmd.Run()
}

func insightsIsRegistered() (bool, error) {
	// While `insights-client --status` properly checks for registration status by
	// asking Inventory, its two modes (legacy v. non-legacy API) behave
	// differently (they return different texts with different exit codes) and
	// we can't rely on the output or exit codes.
	// The `.registered` file is always present on a registered system.
	err := exec.Command("/usr/bin/insights-client", "--status").Run()
	if err != nil {
		return false, err
	}

	_, err = os.Stat("/etc/insights-client/.registered")
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
