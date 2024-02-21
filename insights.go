package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
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
	var errBuffer bytes.Buffer
	cmd := exec.Command("/usr/bin/insights-client", "--status")
	cmd.Stderr = &errBuffer

	err := cmd.Run()

	if err != nil {
		// When the error is ExitError, then we know that insights-client only returned
		// some error code not equal to zero. We do not care about error number.
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			// When stderr is not empty, then we should return this as error
			// to be able to print this error in rhc output
			stdErr := errBuffer.String()
			if len(stdErr) == 0 {
				return false, nil
			} else {
				return false, fmt.Errorf("%s", strings.TrimSpace(stdErr))
			}
		} else {
			return false, err
		}
	}

	return cmd.ProcessState.Success(), err
}
