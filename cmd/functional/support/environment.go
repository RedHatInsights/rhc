package support

import (
	"fmt"
	"os"
)

// ValidTargets lists the allowed values for TARGET.
var ValidTargets = map[string]bool{
	"hosted":    true,
	"satellite": true,
	"local":     true,
}

// cleanEnvironment defines the complete set of environment variables that are
// set for every test run. No other variables from the caller's environment are
// inherited - this guarantees reproducibility and prevents proxy/RHSM state
// from leaking into tests.
var cleanEnvironment = map[string]string{
	"PATH":   "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"HOME":   "/root",
	"LANG":   "C.UTF-8",
	"LC_ALL": "C.UTF-8",
	"TERM":   "xterm",
}

// ResetEnvironment clears the current process environment and re-applies the
// known-good set. No proxy variables, no RHSM variables, no inherited state.
func ResetEnvironment() {
	os.Clearenv()
	for k, v := range cleanEnvironment {
		if err := os.Setenv(k, v); err != nil {
			panic(fmt.Sprintf("ResetEnvironment: os.Setenv(%q): %v", k, err))
		}
	}
}

// ValidateConfig reads CONF and returns an error if it is not set or is not
// an existing directory. Called once at suite startup.
func ValidateConfig() (string, error) {
	config := os.Getenv("CONF")
	if config == "" {
		return "", fmt.Errorf("CONF is not set; must be an existing directory path")
	}
	info, err := os.Stat(config)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("CONF=%q is not an existing directory", config)
	}
	return config, nil
}

// ValidateTarget reads TARGET and returns an error if it is not set
// or not one of the known valid values. Called once at suite startup.
func ValidateTarget() (string, error) {
	target := os.Getenv("TARGET")
	if target == "" {
		return "", fmt.Errorf(
			"TARGET is not set; must be one of: hosted, satellite, local",
		)
	}
	if !ValidTargets[target] {
		return "", fmt.Errorf(
			"TARGET=%q is not valid; must be one of: hosted, satellite, local",
			target,
		)
	}
	return target, nil
}
