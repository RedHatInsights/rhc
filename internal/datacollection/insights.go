package datacollection

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

func RegisterInsightsClient() error {
	slog.Debug("Executing /usr/bin/insights-client --register")
	cmd := exec.Command("/usr/bin/insights-client", "--register")

	return cmd.Run()
}

func UnregisterInsightsClient() error {
	slog.Debug("Executing /usr/bin/insights-client --unregister")
	cmd := exec.Command("/usr/bin/insights-client", "--unregister")

	return cmd.Run()
}

// InsightsClientIsRegistered checks whether insights-client reports its
// status as registered or not. If the system is registered, `true` is
// returned, otherwise `false` is returned, and `error` is filled with
// an error value.
func InsightsClientIsRegistered() (bool, error) {
	var errBuffer bytes.Buffer
	slog.Debug("Executing /usr/bin/insights-client --status")
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
