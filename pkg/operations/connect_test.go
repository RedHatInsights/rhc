package operations

import (
	"errors"
	"testing"

	"github.com/redhatinsights/rhc/internal/subman"
)

func TestConnectOptionsRunStep(t *testing.T) {
	called := false
	fn := func() error {
		called = true
		return errors.New("from fn")
	}

	err := ConnectOptions{}.runStep(ConnectStepRHSM, fn)
	if !called {
		t.Error("runStep with nil OnStep did not call fn")
	}
	if err == nil || err.Error() != "from fn" {
		t.Errorf("runStep error = %v, want from fn", err)
	}

	called = false
	opts := ConnectOptions{
		OnStep: func(step ConnectStep, inner func() error) error {
			if step != ConnectStepAnalytics {
				t.Errorf("OnStep step = %q, want %q", step, ConnectStepAnalytics)
			}
			return errors.New("from OnStep")
		},
	}
	err = opts.runStep(ConnectStepAnalytics, fn)
	if called {
		t.Error("runStep with OnStep called fn directly")
	}
	if err == nil || err.Error() != "from OnStep" {
		t.Errorf("runStep error = %v, want from OnStep", err)
	}
}

func TestConnectOptionsAfterStep(t *testing.T) {
	ConnectOptions{}.afterStep(ConnectStepRHSM, ConnectReport{})

	called := false
	opts := ConnectOptions{
		AfterStep: func(step ConnectStep, report ConnectReport) {
			called = true
			if step != ConnectStepYggdrasil {
				t.Errorf("AfterStep step = %q, want %q", step, ConnectStepYggdrasil)
			}
		},
	}
	opts.afterStep(ConnectStepYggdrasil, ConnectReport{})
	if !called {
		t.Error("afterStep with AfterStep set was not called")
	}
}

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

func recordingHooks() (ConnectOptions, *[]ConnectStep, *[]ConnectStep) {
	var onSteps, afterSteps []ConnectStep
	opts := ConnectOptions{
		OnStep: func(step ConnectStep, fn func() error) error {
			onSteps = append(onSteps, step)
			return fn()
		},
		AfterStep: func(step ConnectStep, _ ConnectReport) {
			afterSteps = append(afterSteps, step)
		},
	}
	return opts, &onSteps, &afterSteps
}

func TestConnectRequestedFlags(t *testing.T) {
	opts, _, _ := recordingHooks()
	opts.EnableContent = true
	opts.EnableAnalytics = true

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

func TestConnectOnStepErrorAbortsLaterSteps(t *testing.T) {
	want := errors.New("rhsm spinner failed")
	var onSteps, afterSteps []ConnectStep
	opts := ConnectOptions{
		EnableAnalytics:        true,
		EnableRemoteManagement: true,
		OnStep: func(step ConnectStep, fn func() error) error {
			onSteps = append(onSteps, step)
			if step == ConnectStepRHSM {
				return want
			}
			return fn()
		},
		AfterStep: func(step ConnectStep, _ ConnectReport) {
			afterSteps = append(afterSteps, step)
		},
	}

	report, err := connect(opts, passingConnectDeps())
	if !errors.Is(err, want) {
		t.Fatalf("connect() error = %v, want %v", err, want)
	}
	if !report.Analytics.Skipped {
		t.Error("Analytics.Skipped = false, want true")
	}
	if !report.RemoteManagement.Skipped {
		t.Error("RemoteManagement.Skipped = false, want true")
	}
	if len(onSteps) != 1 || onSteps[0] != ConnectStepRHSM {
		t.Errorf("OnStep calls = %v, want [%s]", onSteps, ConnectStepRHSM)
	}
	if len(afterSteps) != 0 {
		t.Errorf("AfterStep calls = %v, want none after OnStep failure", afterSteps)
	}
	if _, ok := report.Durations["rhsm"]; ok {
		t.Error("Durations[rhsm] set, want unset when the RHSM OnStep fails")
	}
}

func TestConnectRHSMFailureContinuesToAnalytics(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterRHSM = func(ConnectOptions) error {
		return errors.New("cannot connect to Red Hat Subscription Management: ERROR")
	}

	opts, onSteps, afterSteps := recordingHooks()
	opts.EnableContent = true
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

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

	wantOn := []ConnectStep{ConnectStepRHSM, ConnectStepAnalytics}
	if got := *onSteps; !equalSteps(got, wantOn) {
		t.Errorf("OnStep calls = %v, want %v", got, wantOn)
	}
	wantAfter := []ConnectStep{ConnectStepRHSM, ConnectStepAnalytics, ConnectStepYggdrasil}
	if got := *afterSteps; !equalSteps(got, wantAfter) {
		t.Errorf("AfterStep calls = %v, want %v", got, wantAfter)
	}
}

func TestConnectOrganizationRequiredAborts(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterRHSM = func(ConnectOptions) error { return ErrOrganizationRequired }

	opts, onSteps, afterSteps := recordingHooks()
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

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
	if got := *onSteps; !equalSteps(got, []ConnectStep{ConnectStepRHSM}) {
		t.Errorf("OnStep calls = %v, want [%s]", got, ConnectStepRHSM)
	}
	if len(*afterSteps) != 0 {
		t.Errorf("AfterStep calls = %v, want none", *afterSteps)
	}
}

func TestConnectSkipsAnalyticsWhenDisabled(t *testing.T) {
	opts, onSteps, afterSteps := recordingHooks()

	report, err := connect(opts, passingConnectDeps())
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if report.Analytics.Requested {
		t.Error("Analytics.Requested = true, want false")
	}
	for _, step := range *onSteps {
		if step == ConnectStepAnalytics {
			t.Fatal("OnStep called for analytics when EnableAnalytics is false")
		}
	}
	if got := *afterSteps; !equalSteps(got, []ConnectStep{
		ConnectStepRHSM, ConnectStepAnalytics, ConnectStepYggdrasil,
	}) {
		t.Errorf("AfterStep calls = %v, want [rhsm insights yggdrasil]", got)
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

	opts, onSteps, afterSteps := recordingHooks()
	opts.EnableContent = true
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

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

	for _, step := range *onSteps {
		if step == ConnectStepYggdrasil {
			t.Fatal("OnStep called for yggdrasil when content was unsuccessful")
		}
	}
	if got := *afterSteps; len(got) != 3 || got[2] != ConnectStepYggdrasil {
		t.Errorf("AfterStep calls = %v, want yggdrasil last", got)
	}
	if _, ok := report.Durations["yggdrasil"]; ok {
		t.Error("Durations[yggdrasil] set, want unset when remote management is skipped")
	}
}

func TestConnectInsightsFailureSkipsYggdrasil(t *testing.T) {
	deps := passingConnectDeps()
	deps.RegisterInsightsClient = func() error { return errors.New("insights ERROR") }

	opts, onSteps, _ := recordingHooks()
	opts.EnableContent = true
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

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
	for _, step := range *onSteps {
		if step == ConnectStepYggdrasil {
			t.Fatal("OnStep called for yggdrasil when analytics failed")
		}
	}
}

func TestConnectFullSuccess(t *testing.T) {
	opts, onSteps, afterSteps := recordingHooks()
	opts.EnableContent = true
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

	report, err := connect(opts, passingConnectDeps())
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if !report.RHSMConnected || !report.Content.Successful ||
		!report.Analytics.Successful || !report.RemoteManagement.Successful {
		t.Fatalf("report = %+v", report)
	}
	want := []ConnectStep{ConnectStepRHSM, ConnectStepAnalytics, ConnectStepYggdrasil}
	if got := *onSteps; !equalSteps(got, want) {
		t.Errorf("OnStep calls = %v, want %v", got, want)
	}
	if got := *afterSteps; !equalSteps(got, want) {
		t.Errorf("AfterStep calls = %v, want %v", got, want)
	}
}

func TestConnectDurationsForCompletedSteps(t *testing.T) {
	opts, _, _ := recordingHooks()
	opts.EnableAnalytics = true

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

func TestConnectDoesNotCallYggdrasilOnStepWhenNotRequested(t *testing.T) {
	var yggdrasilCalls int
	deps := passingConnectDeps()
	deps.ActivateYggdrasil = func() error {
		yggdrasilCalls++
		return nil
	}

	opts, onSteps, _ := recordingHooks()
	opts.EnableRemoteManagement = false

	if _, err := connect(opts, deps); err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if yggdrasilCalls != 0 {
		t.Errorf("ActivateYggdrasil calls = %d, want 0", yggdrasilCalls)
	}
	for _, step := range *onSteps {
		if step == ConnectStepYggdrasil {
			t.Fatal("OnStep called for yggdrasil when EnableRemoteManagement is false")
		}
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

func equalSteps(got, want []ConnectStep) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
