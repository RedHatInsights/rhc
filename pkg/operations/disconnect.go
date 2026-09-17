package operations

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

// DisconnectReport describes the result of disconnecting the system from Red Hat.
type DisconnectReport struct {
	Hostname                  string `json:"hostname"`
	HostnameError             string `json:"hostname_error,omitempty"`
	UID                       int    `json:"uid"`
	UIDError                  string `json:"uid_error,omitempty"`
	RHSMDisconnected          bool   `json:"rhsm_disconnected"`
	RHSMDisconnectedError     string `json:"rhsm_disconnect_error,omitempty"`
	InsightsDisconnected      bool   `json:"insights_disconnected"`
	InsightsDisconnectedError string `json:"insights_disconnected_error,omitempty"`
	YggdrasilStopped          bool   `json:"yggdrasil_stopped"`
	YggdrasilStoppedError     string `json:"yggdrasil_stopped_error,omitempty"`

	YggdrasilAlreadyInactive    bool                     `json:"-"`
	InsightsAlreadyDisconnected bool                     `json:"-"`
	RHSMAlreadyDisconnected     bool                     `json:"-"`
	Durations                   map[string]time.Duration `json:"-"`
}

// Disconnect deactivates remote management, disconnects analytics, and unregisters
// the system from RHSM. It always returns a report. Only permission and hostname
// failures are returned as errors; action failures are recorded in the report.
func Disconnect() (*DisconnectReport, error) {
	report := &DisconnectReport{UID: os.Getuid()}
	if report.UID != 0 {
		report.UIDError = "non-root user cannot disconnect system"
		slog.Error(report.UIDError)
		return report, fmt.Errorf("%s", report.UIDError)
	}

	hostname, err := os.Hostname()
	report.Hostname = hostname
	if err != nil {
		report.HostnameError = err.Error()
		slog.Error("error retrieving system hostname", "err", err)
		return report, err
	}

	slog.Info(fmt.Sprintf("Disconnecting %v from Red Hat", hostname))
	report.Durations = make(map[string]time.Duration)

	// Preserve the existing behavior: state-check and client-construction errors
	// silently skip the affected step, while later steps still run.
	start := time.Now()
	deactivateRemoteManagement(report)
	report.Durations["yggdrasil"] = time.Since(start)

	start = time.Now()
	disconnectInsights(report)
	report.Durations["insights"] = time.Since(start)

	start = time.Now()
	disconnectRHSM(report)
	report.Durations["rhsm"] = time.Since(start)

	return report, nil
}

func deactivateRemoteManagement(report *DisconnectReport) {
	slog.Info("Deactivating the yggdrasil service")

	isInactive, err := remotemanagement.AssertYggdrasilServiceState("inactive")
	if err != nil {
		return
	}
	if isInactive {
		report.YggdrasilStopped = true
		report.YggdrasilAlreadyInactive = true
		slog.Info("The yggdrasil service is already inactive")
		return
	}

	if err := remotemanagement.DeactivateServices(); err != nil {
		report.YggdrasilStoppedError = fmt.Sprintf("Cannot deactivate yggdrasil service: %v", err)
		slog.Error(report.YggdrasilStoppedError)
		return
	}

	report.YggdrasilStopped = true
	slog.Info("Deactivated the yggdrasil service")
}

func disconnectInsights(report *DisconnectReport) {
	slog.Info("Disconnecting from Red Hat Lightspeed")

	isRegistered, err := datacollection.InsightsClientIsRegistered()
	if err != nil {
		return
	}
	if !isRegistered {
		report.InsightsDisconnected = true
		report.InsightsAlreadyDisconnected = true
		slog.Info("Already disconnected from Red Hat Lightspeed")
		return
	}

	if err := datacollection.UnregisterInsightsClient(); err != nil {
		report.InsightsDisconnectedError = fmt.Sprintf("Cannot disconnect from Red Hat Lightspeed (formerly Insights): %v", err)
		slog.Error(fmt.Sprintf("Cannot disconnect from Red Hat Lightspeed: %v", err))
		return
	}

	report.InsightsDisconnected = true
	slog.Debug("Disconnected from Red Hat Lightspeed")
}

func disconnectRHSM(report *DisconnectReport) {
	slog.Info("Unregistering system from Red Hat Subscription Management")

	client, err := subman.NewRHSMClient()
	if err != nil {
		return
	}
	isRegistered, err := client.IsRegistered()
	if err != nil {
		return
	}
	if !isRegistered {
		report.RHSMDisconnected = true
		report.RHSMAlreadyDisconnected = true
		slog.Info("Already disconnected from Red Hat Subscription Management")
		return
	}

	if err := client.Unregister(); err != nil {
		report.RHSMDisconnectedError = fmt.Sprintf("Cannot disconnect from Red Hat Subscription Management: %v", err)
		slog.Error(report.RHSMDisconnectedError)
		return
	}

	report.RHSMDisconnected = true
	slog.Debug("Disconnected from Red Hat Subscription Management")
}
