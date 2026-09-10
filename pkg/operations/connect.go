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

type ConnectStep string

const (
	ConnectStepRHSM      ConnectStep = "rhsm"
	ConnectStepAnalytics ConnectStep = "insights"
	ConnectStepYggdrasil ConnectStep = "yggdrasil"
)

var ErrOrganizationRequired = subman.ErrOrganizationRequired

type ConnectOptions struct {
	Username               string
	Password               string
	Organization           string
	ActivationKeys         []string
	ContentTemplates       []string
	EnableContent          bool
	EnableAnalytics        bool
	EnableRemoteManagement bool
	OnStep                 func(step ConnectStep, fn func() error) error
	AfterStep              func(step ConnectStep, report ConnectReport)
}

type FeatureResult struct {
	Requested      bool
	Successful     bool
	Skipped        bool
	SkipDependency string
	Error          string
	Enabled        bool
}

type ConnectReport struct {
	Hostname         string
	HostnameError    string
	UID              int
	UIDError         string
	RHSMConnected    bool
	RHSMError        string
	Content          FeatureResult
	Analytics        FeatureResult
	RemoteManagement FeatureResult
	Durations        map[string]time.Duration
}

func (opts ConnectOptions) runStep(step ConnectStep, fn func() error) error {
	if opts.OnStep == nil {
		return fn()
	}
	return opts.OnStep(step, fn)
}

func (opts ConnectOptions) afterStep(step ConnectStep, report ConnectReport) {
	if opts.AfterStep != nil {
		opts.AfterStep(step, report)
	}
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

func GetOrganizations(username, password string) ([]string, error) {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return nil, err
	}
	return client.GetOrganizations(username, password)
}

func Connect(opts ConnectOptions) (ConnectReport, error) {
	return connect(opts, defaultConnectDependencies())
}

func connect(opts ConnectOptions, connectDeps connectDependencies) (ConnectReport, error) {
	report := ConnectReport{
		Content:          FeatureResult{Requested: opts.EnableContent},
		Analytics:        FeatureResult{Requested: opts.EnableAnalytics},
		RemoteManagement: FeatureResult{Requested: opts.EnableRemoteManagement},
	}
	report.Durations = make(map[string]time.Duration)

	start := time.Now()
	err := opts.runStep(
		ConnectStepRHSM,
		func() error {
			return registerRHSM(opts, &report, connectDeps.RegisterRHSM)
		},
	)
	if err != nil {
		skipUnstarted(&report.Analytics)
		skipUnstarted(&report.RemoteManagement)
		return report, err
	}
	report.Durations["rhsm"] = time.Since(start)
	opts.afterStep(ConnectStepRHSM, report)

	if opts.EnableAnalytics {
		start = time.Now()
		_ = opts.runStep(
			ConnectStepAnalytics,
			func() error {
				slog.Info("Connecting to Red Hat Lightspeed")
				if err := connectDeps.RegisterInsightsClient(); err != nil {
					report.Analytics.Error = fmt.Sprintf(
						"cannot connect to Red Hat Lightspeed (formerly Insights): %v", err)
					slog.Error("cannot connect to Red Hat Lightspeed", "err", err)
				} else {
					report.Analytics.Successful = true
				}
				return nil
			},
		)
		report.Durations["insights"] = time.Since(start)
	}
	opts.afterStep(ConnectStepAnalytics, report)

	if !opts.EnableRemoteManagement {
		opts.afterStep(ConnectStepYggdrasil, report)
		return report, nil
	}

	switch {
	case !report.Content.Successful:
		skipRemoteManagement(&report, "content")
	case !report.Analytics.Successful:
		skipRemoteManagement(&report, "analytics")
	default:
		start = time.Now()
		_ = opts.runStep(
			ConnectStepYggdrasil,
			func() error {
				slog.Info("Activating yggdrasil service")
				if err := connectDeps.ActivateYggdrasil(); err != nil {
					report.RemoteManagement.Error = fmt.Sprintf(
						"cannot activate the yggdrasil service: %v", err)
					slog.Error(report.RemoteManagement.Error)
				} else {
					report.RemoteManagement.Successful = true
				}
				return nil
			},
		)
		report.Durations["yggdrasil"] = time.Since(start)
	}
	opts.afterStep(ConnectStepYggdrasil, report)
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

func skipUnstarted(result *FeatureResult) {
	if result.Requested && !result.Successful {
		result.Skipped = true
	}
}
