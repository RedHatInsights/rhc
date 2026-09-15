package operations

import (
	"errors"
	"testing"

	"github.com/redhatinsights/rhc/internal/subman"
)

func TestSkipUnstarted(t *testing.T) {
	tests := []struct {
		name     string
		result   FeatureResult
		wantSkip bool
	}{
		{name: "not requested", result: FeatureResult{}, wantSkip: false},
		{name: "already successful", result: FeatureResult{Requested: true, Successful: true}, wantSkip: false},
		{name: "requested and unstarted", result: FeatureResult{Requested: true}, wantSkip: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result
			skipUnstarted(&got)
			if got.Skipped != tt.wantSkip {
				t.Errorf("Skipped = %v, want %v", got.Skipped, tt.wantSkip)
			}
		})
	}
}

func TestSkipRemoteManagement(t *testing.T) {
	report := ConnectReport{}
	skipRemoteManagement(&report, "analytics")

	if !report.RemoteManagement.Skipped {
		t.Error("Skipped = false, want true")
	}
	if report.RemoteManagement.SkipDependency != "analytics" {
		t.Errorf("SkipDependency = %q, want %q", report.RemoteManagement.SkipDependency, "analytics")
	}
	wantErr := "skipped: dependency 'analytics' failed"
	if report.RemoteManagement.Error != wantErr {
		t.Errorf("Error = %q, want %q", report.RemoteManagement.Error, wantErr)
	}
}

// passingConnectDeps returns dependencies where every side effect succeeds.
// Tests override single fields to exercise a specific failure.
func passingConnectDeps() connectDependencies {
	return connectDependencies{
		RegisterRHSM:           func(ConnectOptions) error { return nil },
		RegisterInsightsClient: func() error { return nil },
		ActivateYggdrasil:      func() error { return nil },
	}
}

func TestConnectRequestedFlags(t *testing.T) {
	opts := ConnectOptions{
		EnableContent:   true,
		EnableAnalytics: true,
	}

	report, err := connect(opts, passingConnectDeps())
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if !report.Content.Requested {
		t.Error("Content.Requested = false, want true")
	}
	if !report.Analytics.Requested {
		t.Error("Analytics.Requested = false, want true")
	}
	if report.RemoteManagement.Requested {
		t.Error("RemoteManagement.Requested = true, want false")
	}
}

func TestConnectRHSMFailureContinuesToAnalytics(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterRHSM = func(ConnectOptions) error {
		return errors.New("cannot connect to Red Hat Subscription Management: ERROR")
	}

	opts := ConnectOptions{
		EnableContent:          true,
		EnableAnalytics:        true,
		EnableRemoteManagement: true,
	}

	report, err := connect(opts, deps)
	if err != nil {
		t.Fatalf("connect() error = %v, want nil (RHSM failures are recorded)", err)
	}
	if report.RHSMConnected {
		t.Error("RHSMConnected = true, want false")
	}
	if report.RHSMError == "" {
		t.Error("RHSMError empty, want recorded failure")
	}
	if report.Content.Successful {
		t.Error("Content.Successful = true, want false")
	}
	if !report.Analytics.Successful {
		t.Error("Analytics.Successful = false, want true")
	}
	if report.RemoteManagement.SkipDependency != "content" {
		t.Errorf("SkipDependency = %q, want %q", report.RemoteManagement.SkipDependency, "content")
	}
}

func TestConnectOrganizationRequiredAborts(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterRHSM = func(ConnectOptions) error { return ErrOrganizationRequired }

	opts := ConnectOptions{
		EnableAnalytics:        true,
		EnableRemoteManagement: true,
	}

	report, err := connect(opts, deps)
	if !errors.Is(err, ErrOrganizationRequired) {
		t.Fatalf("connect() error = %v, want ErrOrganizationRequired", err)
	}
	if report.RHSMConnected {
		t.Error("RHSMConnected = true, want false")
	}
	if report.RHSMError != "" {
		t.Errorf("RHSMError = %q, want empty until the CLI records it", report.RHSMError)
	}
	if !report.Analytics.Skipped {
		t.Error("Analytics.Skipped = false, want true")
	}
	if !report.RemoteManagement.Skipped {
		t.Error("RemoteManagement.Skipped = false, want true")
	}
}

func TestConnectSkipsAnalyticsWhenDisabled(t *testing.T) {
	var analyticsCalls int
	deps := passingConnectDeps()
	deps.RegisterInsightsClient = func() error {
		analyticsCalls++
		return nil
	}

	report, err := connect(ConnectOptions{}, deps)
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if report.Analytics.Requested {
		t.Error("Analytics.Requested = true, want false")
	}
	if analyticsCalls != 0 {
		t.Errorf("RegisterInsightsClient calls = %d, want 0", analyticsCalls)
	}
	if _, ok := report.Durations["insights"]; ok {
		t.Error("Durations[insights] set, want unset when analytics is disabled")
	}
}

func TestConnectSkipsRemoteManagementWhenContentUnsuccessful(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterRHSM = func(ConnectOptions) error {
		return errors.New("cannot connect to Red Hat Subscription Management: ERROR")
	}
	var yggdrasilCalls int
	deps.ActivateYggdrasil = func() error {
		yggdrasilCalls++
		return nil
	}

	opts := ConnectOptions{
		EnableContent:          true,
		EnableAnalytics:        true,
		EnableRemoteManagement: true,
	}

	report, err := connect(opts, deps)
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if report.RemoteManagement.Successful {
		t.Error("RemoteManagement.Successful = true, want false")
	}
	if !report.RemoteManagement.Skipped {
		t.Error("RemoteManagement.Skipped = false, want true")
	}
	if report.RemoteManagement.SkipDependency != "content" {
		t.Errorf("SkipDependency = %q, want %q", report.RemoteManagement.SkipDependency, "content")
	}
	wantErr := "skipped: dependency 'content' failed"
	if report.RemoteManagement.Error != wantErr {
		t.Errorf("Error = %q, want %q", report.RemoteManagement.Error, wantErr)
	}
	if yggdrasilCalls != 0 {
		t.Errorf("ActivateYggdrasil calls = %d, want 0", yggdrasilCalls)
	}
	if _, ok := report.Durations["yggdrasil"]; ok {
		t.Error("Durations[yggdrasil] set, want unset when remote management is skipped")
	}
}

func TestConnectInsightsFailureSkipsYggdrasil(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterInsightsClient = func() error { return errors.New("insights ERROR") }
	var yggdrasilCalls int
	deps.ActivateYggdrasil = func() error {
		yggdrasilCalls++
		return nil
	}

	opts := ConnectOptions{
		EnableContent:          true,
		EnableAnalytics:        true,
		EnableRemoteManagement: true,
	}

	report, err := connect(opts, deps)
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if !report.RHSMConnected || !report.Content.Successful {
		t.Fatal("RHSM and content should succeed")
	}
	if report.Analytics.Successful {
		t.Error("Analytics.Successful = true, want false")
	}
	if report.Analytics.Error == "" {
		t.Error("Analytics.Error empty, want recorded failure")
	}
	if report.RemoteManagement.SkipDependency != "analytics" {
		t.Errorf("SkipDependency = %q, want %q", report.RemoteManagement.SkipDependency, "analytics")
	}
	if yggdrasilCalls != 0 {
		t.Errorf("ActivateYggdrasil calls = %d, want 0", yggdrasilCalls)
	}
}

func TestConnectFullSuccess(t *testing.T) {
	opts := ConnectOptions{
		EnableContent:          true,
		EnableAnalytics:        true,
		EnableRemoteManagement: true,
	}

	report, err := connect(opts, passingConnectDeps())
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if !report.RHSMConnected || !report.Content.Successful ||
		!report.Analytics.Successful || !report.RemoteManagement.Successful {
		t.Fatalf("report = %+v", report)
	}
}

func TestConnectDurationsForCompletedSteps(t *testing.T) {
	opts := ConnectOptions{EnableAnalytics: true}

	report, err := connect(opts, passingConnectDeps())
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	for _, key := range []string{"rhsm", "insights"} {
		if d, ok := report.Durations[key]; !ok {
			t.Errorf("Durations[%q] missing", key)
		} else if d < 0 {
			t.Errorf("Durations[%q] = %v, want >= 0", key, d)
		}
	}
}

func TestConnectDoesNotActivateYggdrasilWhenNotRequested(t *testing.T) {
	var yggdrasilCalls int
	deps := passingConnectDeps()
	deps.ActivateYggdrasil = func() error {
		yggdrasilCalls++
		return nil
	}

	if _, err := connect(ConnectOptions{}, deps); err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if yggdrasilCalls != 0 {
		t.Errorf("ActivateYggdrasil calls = %d, want 0", yggdrasilCalls)
	}
}

func TestConnectActivationKeysPassedThrough(t *testing.T) {
	var got ConnectOptions
	deps := passingConnectDeps()
	deps.RegisterRHSM = func(opts ConnectOptions) error {
		got = opts
		return nil
	}

	_, err := connect(ConnectOptions{
		Organization:   "org",
		ActivationKeys: []string{"k1"},
		EnableContent:  true,
	}, deps)
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if got.Organization != "org" || len(got.ActivationKeys) != 1 || got.ActivationKeys[0] != "k1" {
		t.Fatalf("RegisterRHSM got org=%q keys=%v", got.Organization, got.ActivationKeys)
	}
}

func TestConnectErrOrganizationRequiredMatchesSubman(t *testing.T) {
	if !errors.Is(ErrOrganizationRequired, subman.ErrOrganizationRequired) {
		t.Fatal("ErrOrganizationRequired is not subman.ErrOrganizationRequired")
	}
}
