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

// interceptConnect skips real RHSM/Insights/yggdrasil work by not calling fn.
func interceptConnect(onStepErr map[ConnectStep]error) (ConnectOptions, *[]ConnectStep, *[]ConnectStep) {
	var onSteps, afterSteps []ConnectStep
	opts := ConnectOptions{
		OnStep: func(step ConnectStep, fn func() error) error {
			onSteps = append(onSteps, step)
			if err, ok := onStepErr[step]; ok {
				return err
			}
			return nil
		},
		AfterStep: func(step ConnectStep, report ConnectReport) {
			afterSteps = append(afterSteps, step)
		},
	}
	return opts, &onSteps, &afterSteps
}

func TestConnectRequestedFlags(t *testing.T) {
	opts, _, _ := interceptConnect(nil)
	opts.EnableContent = true
	opts.EnableAnalytics = true

	report, err := Connect(opts)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
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

func TestConnectRHSMErrorSkipsUnstartedFeatures(t *testing.T) {
	want := errors.New("rhsm failed")
	opts, onSteps, afterSteps := interceptConnect(map[ConnectStep]error{
		ConnectStepRHSM: want,
	})
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

	report, err := Connect(opts)
	if !errors.Is(err, want) {
		t.Fatalf("Connect() error = %v, want %v", err, want)
	}
	if !report.Analytics.Skipped {
		t.Error("Analytics.Skipped = false, want true")
	}
	if !report.RemoteManagement.Skipped {
		t.Error("RemoteManagement.Skipped = false, want true")
	}
	if got := *onSteps; len(got) != 1 || got[0] != ConnectStepRHSM {
		t.Errorf("OnStep calls = %v, want [%s]", got, ConnectStepRHSM)
	}
	if len(*afterSteps) != 0 {
		t.Errorf("AfterStep calls = %v, want none after RHSM failure", *afterSteps)
	}
	if _, ok := report.Durations["rhsm"]; ok {
		t.Error("Durations[rhsm] set, want unset when RHSM step fails")
	}
}

func TestConnectSkipsAnalyticsWhenDisabled(t *testing.T) {
	opts, onSteps, afterSteps := interceptConnect(nil)

	report, err := Connect(opts)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	for _, step := range *onSteps {
		if step == ConnectStepAnalytics {
			t.Fatal("OnStep called for analytics when EnableAnalytics is false")
		}
	}
	if got := *afterSteps; len(got) != 3 ||
		got[0] != ConnectStepRHSM ||
		got[1] != ConnectStepAnalytics ||
		got[2] != ConnectStepYggdrasil {
		t.Errorf("AfterStep calls = %v, want [rhsm insights yggdrasil]", got)
	}
	if _, ok := report.Durations["insights"]; ok {
		t.Error("Durations[insights] set, want unset when analytics is disabled")
	}
}

func TestConnectSkipsRemoteManagementWhenContentUnsuccessful(t *testing.T) {
	opts, onSteps, afterSteps := interceptConnect(nil)
	opts.EnableContent = true
	opts.EnableAnalytics = true
	opts.EnableRemoteManagement = true

	report, err := Connect(opts)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
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

func TestConnectDurationsForCompletedSteps(t *testing.T) {
	opts, _, _ := interceptConnect(nil)
	opts.EnableAnalytics = true

	report, err := Connect(opts)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
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
	opts, onSteps, _ := interceptConnect(nil)
	opts.EnableRemoteManagement = false

	if _, err := Connect(opts); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	for _, step := range *onSteps {
		if step == ConnectStepYggdrasil {
			t.Fatal("OnStep called for yggdrasil when EnableRemoteManagement is false")
		}
	}
}

func TestConnectErrOrganizationRequiredMatchesSubman(t *testing.T) {
	if !errors.Is(ErrOrganizationRequired, subman.ErrOrganizationRequired) {
		t.Fatal("ErrOrganizationRequired is not subman.ErrOrganizationRequired")
	}
}
