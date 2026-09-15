package operations

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

// ErrOrganizationRequired indicates that password registration needs an
// explicit organization before it can continue.
var ErrOrganizationRequired = subman.ErrOrganizationRequired

// ConnectOptions contains validated values for a connection attempt.
type ConnectOptions struct {
	Username               string
	Password               string
	Organization           string
	ActivationKeys         []string
	ContentTemplates       []string
	EnableContent          bool
	EnableAnalytics        bool
	EnableRemoteManagement bool
}

// ConnectFeatureResult describes the requested action and resulting state of
// one connect feature.
type ConnectFeatureResult struct {
	Requested      bool
	Successful     bool
	Skipped        bool
	SkipDependency string
	Error          string
	Enabled        bool
}

// ConnectReport contains connection outcome data without presentation or
// exit-code behavior.
type ConnectReport struct {
	Hostname         string
	HostnameError    string
	UID              int
	UIDError         string
	RHSMConnected    bool
	RHSMError        string
	Content          ConnectFeatureResult
	Analytics        ConnectFeatureResult
	RemoteManagement ConnectFeatureResult
	Durations        map[string]time.Duration
}

type connectDependencies struct {
	RegisterRHSM           func(opts ConnectOptions) error
	RegisterInsightsClient func() error
	ActivateYggdrasil      func() error
}

func defaultConnectDependencies() connectDependencies {
	return connectDependencies{
		RegisterRHSM:           defaultRegisterRHSM,
		RegisterInsightsClient: datacollection.RegisterInsightsClient,
		ActivateYggdrasil:      remotemanagement.ActivateServices,
	}
}

func defaultRegisterRHSM(opts ConnectOptions) error {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return fmt.Errorf("cannot connect to subscription-manager: %s", err)
	}
	regOpts := subman.RegisterOptions{
		EnvironmentNames: opts.ContentTemplates,
		EnableContent:    opts.EnableContent,
	}
	if len(opts.ActivationKeys) > 0 {
		err = client.RegisterWithActivationKeys(opts.Organization, opts.ActivationKeys, regOpts)
	} else {
		err = client.RegisterWithPassword(opts.Username, opts.Password, opts.Organization, regOpts)
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrOrganizationRequired) {
		return err
	}
	return fmt.Errorf("cannot connect to Red Hat Subscription Management: %s", err)
}

// GetOrganizations returns the organizations available to the supplied account.
func GetOrganizations(username, password string) ([]string, error) {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return nil, err
	}
	return client.GetOrganizations(username, password)
}

// Connect registers the system and enables the requested dependent features.
func Connect(opts ConnectOptions) (ConnectReport, error) {
	return connect(opts, defaultConnectDependencies())
}

func connect(opts ConnectOptions, connectDeps connectDependencies) (ConnectReport, error) {
	report := ConnectReport{
		Content:          ConnectFeatureResult{Requested: opts.EnableContent},
		Analytics:        ConnectFeatureResult{Requested: opts.EnableAnalytics},
		RemoteManagement: ConnectFeatureResult{Requested: opts.EnableRemoteManagement},
	}
	report.Durations = make(map[string]time.Duration)

	start := time.Now()
	err := registerRHSM(opts, &report, connectDeps.RegisterRHSM)
	if err != nil {
		skipUnstarted(&report.Analytics)
		skipUnstarted(&report.RemoteManagement)
		return report, err
	}
	report.Durations["rhsm"] = time.Since(start)

	if opts.EnableAnalytics {
		start = time.Now()
		slog.Info("Connecting to Red Hat Lightspeed")
		if err := connectDeps.RegisterInsightsClient(); err != nil {
			report.Analytics.Error = fmt.Sprintf(
				"cannot connect to Red Hat Lightspeed (formerly Insights): %v", err)
			slog.Error("cannot connect to Red Hat Lightspeed", "err", err)
		} else {
			report.Analytics.Successful = true
		}
		report.Durations["insights"] = time.Since(start)
	}

	if !opts.EnableRemoteManagement {
		return report, nil
	}

	switch {
	case !report.Content.Successful:
		skipRemoteManagement(&report, "content")
	case !report.Analytics.Successful:
		skipRemoteManagement(&report, "analytics")
	default:
		start = time.Now()
		slog.Info("Activating yggdrasil service")
		if err := connectDeps.ActivateYggdrasil(); err != nil {
			report.RemoteManagement.Error = fmt.Sprintf(
				"cannot activate the yggdrasil service: %v", err)
			slog.Error(report.RemoteManagement.Error)
		} else {
			report.RemoteManagement.Successful = true
		}
		report.Durations["yggdrasil"] = time.Since(start)
	}
	return report, nil
}

func registerRHSM(opts ConnectOptions, report *ConnectReport, register func(opts ConnectOptions) error) error {
	slog.Info("Registering the system with Red Hat Subscription Management")
	err := register(opts)
	if errors.Is(err, ErrOrganizationRequired) {
		return ErrOrganizationRequired
	}

	if err != nil {
		report.RHSMError = err.Error()
		slog.Error(report.RHSMError)
		return nil
	}
	report.RHSMConnected = true
	report.Content.Successful = opts.EnableContent

	if report.Content.Successful {
		slog.Info("System has access to content")
	} else {
		slog.Info("System has no access to content")
	}

	return nil
}

func skipRemoteManagement(report *ConnectReport, dependency string) {
	report.RemoteManagement.Skipped = true
	report.RemoteManagement.SkipDependency = dependency
	report.RemoteManagement.Error = fmt.Sprintf("skipped: dependency '%s' failed", dependency)
	slog.Warn("Skipping remote-management (dependency failed)", "dependency", dependency)
}

func skipUnstarted(result *ConnectFeatureResult) {
	if result.Requested && !result.Successful {
		result.Skipped = true
	}
}
