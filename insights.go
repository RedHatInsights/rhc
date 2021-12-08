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

func insightsIsRegistered() bool {
	cmd := exec.Command("/usr/bin/insights-client", "--status")

	_ = cmd.Run()

	return cmd.ProcessState.Success()
}
