package main

import (
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
	cmd := exec.Command("/usr/bin/insights-client", "--status")

	err := cmd.Run()

	if err != nil {
		// When the error is ExitError, then we know that insights-client only returned
		// some error code not equal to zero. We do not care about error number.
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		} else {
			return false, err
		}
	}

	return cmd.ProcessState.Success(), err
}
