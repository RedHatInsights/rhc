package operations

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

type disconnectRHSMClient interface {
	IsRegistered() (bool, error)
	Unregister() error
}

// disconnectDependencies keeps infrastructure calls replaceable in tests.
type disconnectDependencies struct {
	UID                         func() int
	AssertYggdrasilServiceState func(string) (bool, error)
	DeactivateServices          func() error
	InsightsClientIsRegistered  func() (bool, error)
	UnregisterInsightsClient    func() error
	NewRHSMClient               func() (disconnectRHSMClient, error)
}

func defaultDisconnectDependencies() disconnectDependencies {
	return disconnectDependencies{
		UID:                         os.Getuid,
		AssertYggdrasilServiceState: remotemanagement.AssertYggdrasilServiceState,
		DeactivateServices:          remotemanagement.DeactivateServices,
		InsightsClientIsRegistered:  datacollection.InsightsClientIsRegistered,
		UnregisterInsightsClient:    datacollection.UnregisterInsightsClient,
		NewRHSMClient: func() (disconnectRHSMClient, error) {
			return subman.NewRHSMClient()
		},
	}
}

// DisconnectReport describes the result of disconnecting the system from Red Hat.
type DisconnectReport struct {
	Hostname                  string
	HostnameError             string
	UID                       int
	UIDError                  string
	RHSMDisconnected          bool
	RHSMDisconnectedError     string
	InsightsDisconnected      bool
	InsightsDisconnectedError string
	YggdrasilStopped          bool
	YggdrasilStoppedError     string

	YggdrasilAlreadyInactive    bool
	InsightsAlreadyDisconnected bool
	RHSMAlreadyDisconnected     bool
	Durations                   map[string]time.Duration
}

// Disconnect deactivates remote management, disconnects analytics, and unregisters
// the system from RHSM. It always returns a report. Only a permission failure is
// returned as an error; action failures are recorded in the report.
func Disconnect() (*DisconnectReport, error) {
	return disconnect(defaultDisconnectDependencies())
}

func disconnect(deps disconnectDependencies) (*DisconnectReport, error) {
	report := &DisconnectReport{UID: deps.UID()}
	if report.UID != 0 {
		report.UIDError = "non-root user cannot disconnect system"
		slog.Error(report.UIDError)
		return report, errors.New(report.UIDError)
	}

	report.Durations = make(map[string]time.Duration)

	// Preserve the existing behavior: state-check and client-construction errors
	// skip the affected step, while later steps still run.
	start := time.Now()
	deactivateRemoteManagement(report, deps)
	report.Durations["yggdrasil"] = time.Since(start)

	start = time.Now()
	disconnectInsights(report, deps)
	report.Durations["insights"] = time.Since(start)

	start = time.Now()
	disconnectRHSM(report, deps)
	report.Durations["rhsm"] = time.Since(start)

	return report, nil
}

func deactivateRemoteManagement(report *DisconnectReport, deps disconnectDependencies) {
	slog.Info("Deactivating the yggdrasil service")

	isInactive, err := deps.AssertYggdrasilServiceState("inactive")
	if err != nil {
		slog.Debug("cannot check the yggdrasil service state", "error", err)
		return
	}
	if isInactive {
		report.YggdrasilStopped = true
		report.YggdrasilAlreadyInactive = true
		slog.Info("The yggdrasil service is already inactive")
		return
	}

	if err := deps.DeactivateServices(); err != nil {
		report.YggdrasilStoppedError = fmt.Sprintf("Cannot deactivate yggdrasil service: %v", err)
		slog.Error(report.YggdrasilStoppedError)
		return
	}

	report.YggdrasilStopped = true
	slog.Info("Deactivated the yggdrasil service")
}

func disconnectInsights(report *DisconnectReport, deps disconnectDependencies) {
	slog.Info("Disconnecting from Red Hat Lightspeed")

	isRegistered, err := deps.InsightsClientIsRegistered()
	if err != nil {
		slog.Debug("cannot check the Red Hat Lightspeed registration state", "error", err)
		return
	}
	if !isRegistered {
		report.InsightsDisconnected = true
		report.InsightsAlreadyDisconnected = true
		slog.Info("Already disconnected from Red Hat Lightspeed")
		return
	}

	if err := deps.UnregisterInsightsClient(); err != nil {
		report.InsightsDisconnectedError = fmt.Sprintf("Cannot disconnect from Red Hat Lightspeed (formerly Insights): %v", err)
		slog.Error(fmt.Sprintf("Cannot disconnect from Red Hat Lightspeed: %v", err))
		return
	}

	report.InsightsDisconnected = true
	slog.Debug("Disconnected from Red Hat Lightspeed")
}

func disconnectRHSM(report *DisconnectReport, deps disconnectDependencies) {
	slog.Info("Unregistering system from Red Hat Subscription Management")

	client, err := deps.NewRHSMClient()
	if err != nil {
		slog.Debug("cannot create the Red Hat Subscription Management client", "error", err)
		return
	}
	isRegistered, err := client.IsRegistered()
	if err != nil {
		slog.Debug("cannot check the Red Hat Subscription Management registration state", "error", err)
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
