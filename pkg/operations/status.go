package operations

import (
	"log/slog"
	"os"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

// statusDependencies holds the infrastructure calls used to gather status.
// Tests replace these functions with fakes.
type statusDependencies struct {
	Hostname                   func() (string, error)
	RHSMIsRegistered           func() (bool, error)
	ContentIsEnabled           func() (bool, error)
	InsightsClientIsRegistered func() (bool, error)
	GetUnitState               func(string) (*remotemanagement.UnitState, error)
}

// defaultStatusDependencies returns the dependencies backed by the real
// infrastructure calls.
func defaultStatusDependencies() statusDependencies {
	return statusDependencies{
		Hostname:                   os.Hostname,
		RHSMIsRegistered:           IsRegistered,
		ContentIsEnabled:           contentIsEnabled,
		InsightsClientIsRegistered: datacollection.InsightsClientIsRegistered,
		GetUnitState:               remotemanagement.GetUnitState,
	}
}

func contentIsEnabled() (bool, error) {
	service, err := subman.NewRHSMClient()
	if err != nil {
		return false, err
	}
	return service.IsContentManagementEnabled()
}

// StatusReport describes the system's connection status. Its fields are the
// machine-readable form; failedChecks is unexported and so is not serialized.
type StatusReport struct {
	Hostname          string `json:"hostname"`
	HostnameError     string `json:"hostname_error,omitempty"`
	RHSMConnected     bool   `json:"rhsm_connected"`
	RHSMError         string `json:"rhsm_error,omitempty"`
	ContentEnabled    bool   `json:"content_enabled"`
	ContentError      string `json:"content_error,omitempty"`
	InsightsConnected bool   `json:"insights_connected"`
	InsightsError     string `json:"insights_error,omitempty"`
	YggdrasilRunning  bool   `json:"yggdrasil_running"`
	YggdrasilError    string `json:"yggdrasil_error,omitempty"`
	// failedChecks counts checks that failed in a way that should make the
	// command exit non-zero.
	failedChecks int
}

// HasFailures reports whether any check failed in a way that should result in a
// non-zero exit code.
func (r *StatusReport) HasFailures() bool {
	return r.failedChecks != 0
}

// GetStatus gathers the system's connection status using the real
// infrastructure dependencies.
func GetStatus() *StatusReport {
	return getStatus(defaultStatusDependencies())
}

// getStatus gathers the system's connection status using the given dependencies.
// Per-check failures are recorded in the report rather than returned as an error.
func getStatus(dependencies statusDependencies) *StatusReport {
	slog.Info("Checking system connection status")

	report := &StatusReport{}

	collectHostname(report, dependencies.Hostname)

	collectRHSMStatus(report, dependencies.RHSMIsRegistered)
	collectContentStatus(report, dependencies.ContentIsEnabled)
	collectInsightsStatus(report, dependencies.InsightsClientIsRegistered)
	collectYggdrasilStatus(report, dependencies.GetUnitState)

	return report
}

func collectHostname(report *StatusReport, hostname func() (string, error)) {
	name, err := hostname()
	report.Hostname = name
	if err != nil {
		report.HostnameError = err.Error()
	}
}

func collectRHSMStatus(report *StatusReport, isRegistered func() (bool, error)) {
	slog.Info("Checking status of Red Hat Subscription Management")

	registered, err := isRegistered()
	if err != nil {
		report.failedChecks++
		report.RHSMError = err.Error()
		slog.Error("Cannot detect Red Hat Subscription Management status", "error", err)
		return
	}
	if !registered {
		report.failedChecks++
		report.RHSMConnected = false
		slog.Info("Not connected to Red Hat Subscription Management")
	} else {
		report.RHSMConnected = true
		slog.Info("Connected to Red Hat Subscription Management")
	}
}

func collectContentStatus(report *StatusReport, isEnabled func() (bool, error)) {
	slog.Info("Checking content status")

	contentEnabled, err := isEnabled()
	if err != nil {
		report.failedChecks++
		report.ContentError = err.Error()
		slog.Error("Cannot detect content management status", "error", err)
		return
	}

	if contentEnabled && report.RHSMConnected {
		report.ContentEnabled = true
		slog.Info("System has access to content")
	} else {
		report.ContentEnabled = false
		slog.Info("System has no access to content")
	}
}

func collectInsightsStatus(report *StatusReport, insightsClientIsRegistered func() (bool, error)) {
	slog.Info("Checking status of Red Hat Lightspeed")

	isRegistered, err := insightsClientIsRegistered()

	if isRegistered {
		report.InsightsConnected = true
		slog.Info("Connected to Red Hat Lightspeed")
	} else {
		report.failedChecks++
		if err == nil {
			report.InsightsConnected = false
			slog.Info("Not connected to Red Hat Lightspeed")
		} else {
			report.InsightsConnected = false
			report.InsightsError = err.Error()
			slog.Error("Cannot detect Red Hat Lightspeed status", "error", err)
		}
	}
}

func collectYggdrasilStatus(
	report *StatusReport,
	getUnitState func(string) (*remotemanagement.UnitState, error),
) {
	slog.Info("Checking status of yggdrasil service")

	state, err := getUnitState("yggdrasil.service")
	if err != nil {
		report.YggdrasilRunning = false
		report.YggdrasilError = err.Error()
		return
	}

	if state.ActiveState == "active" {
		report.YggdrasilRunning = true
		slog.Info("The yggdrasil service is active")
	} else if state.LoadState == "loaded" {
		report.failedChecks++
		report.YggdrasilRunning = false
		slog.Warn("The yggdrasil service is not running")
	} else {
		report.failedChecks++
		report.YggdrasilRunning = false
		errMsg := "The yggdrasil service is not available"
		report.YggdrasilError = errMsg
		if state.LoadError != "" {
			slog.Error(errMsg, "reason", state.LoadError)
		} else {
			slog.Error(errMsg)
		}
	}
}
