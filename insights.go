package main

import (
	"errors"
	"io/fs"
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

// insightsIsRegistered checks whether insights-client reports its
// status as registered or not. If the system is registered, `true` is
// returned, otherwise `false` is returned, and `error` is filled with
// an error value.
func insightsIsRegistered() (bool, error) {
	// While `insights-client --status` properly checks for registration status by
	// asking Inventory, its two modes (legacy v. non-legacy API) behave
	// differently (they return different texts with different exit codes) and
	// we can't rely on the output or exit codes.
	// The `.registered` file is always present on a registered system.
	err := exec.Command("/usr/bin/insights-client", "--status").Run()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			// If .unregistered exists, insights-client is confident
			// it is not registered. We can suppress the error,
			// we don't care why it returned non-zero exit code.
			_, err := os.Stat("/etc/insights-client/.unregistered")
			if err == nil {
				return false, nil
			}
		}
		return false, err
	}

	_, err = os.Stat("/etc/insights-client/.registered")
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
