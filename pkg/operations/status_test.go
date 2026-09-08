package operations

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/redhatinsights/rhc/internal/remotemanagement"
)

func checkError(err error) func() (bool, error) {
	return func() (bool, error) { return false, err }
}

// passingDeps returns dependencies where every check succeeds. Tests override
// single fields to exercise a specific failure.
func passingDeps() statusDependencies {
	return statusDependencies{
		Hostname:                   func() (string, error) { return "test-host", nil },
		RHSMIsRegistered:           func() (bool, error) { return true, nil },
		ContentIsEnabled:           func() (bool, error) { return true, nil },
		InsightsClientIsRegistered: func() (bool, error) { return true, nil },
		GetUnitState: func(string) (*remotemanagement.UnitState, error) {
			return &remotemanagement.UnitState{ActiveState: "active", LoadState: "loaded"}, nil
		},
	}
}

func TestGetStatusAllConnected(t *testing.T) {
	report := getStatus(passingDeps())

	if report.Hostname != "test-host" {
		t.Errorf("Hostname = %q, want %q", report.Hostname, "test-host")
	}
	if !report.RHSMConnected {
		t.Error("RHSMConnected = false, want true")
	}
	if !report.ContentEnabled {
		t.Error("ContentEnabled = false, want true")
	}
	if !report.InsightsConnected {
		t.Error("InsightsConnected = false, want true")
	}
	if !report.YggdrasilRunning {
		t.Error("YggdrasilRunning = false, want true")
	}
	if report.HasFailures() {
		t.Errorf("HasFailures() = true, want false (failedChecks = %d)", report.failedChecks)
	}
}

// Independent RHSM and content check errors are both recorded and counted.
func TestGetStatusRHSMAndContentErrors(t *testing.T) {
	deps := passingDeps()
	deps.RHSMIsRegistered = checkError(errors.New("rhsm boom"))
	deps.ContentIsEnabled = checkError(errors.New("content boom"))

	report := getStatus(deps)

	if report.RHSMError != "rhsm boom" {
		t.Errorf("RHSMError = %q, want %q", report.RHSMError, "rhsm boom")
	}
	if report.ContentError != "content boom" {
		t.Errorf("ContentError = %q, want %q", report.ContentError, "content boom")
	}
	if report.failedChecks != 2 {
		t.Errorf("failedChecks = %d, want 2", report.failedChecks)
	}
	if !report.InsightsConnected {
		t.Error("InsightsConnected = false, want true")
	}
	if !report.YggdrasilRunning {
		t.Error("YggdrasilRunning = false, want true")
	}
}

func TestGetStatusRequiresRHSMConnectionForContent(t *testing.T) {
	deps := passingDeps()
	deps.RHSMIsRegistered = func() (bool, error) { return false, nil }
	deps.ContentIsEnabled = func() (bool, error) { return true, nil }

	report := getStatus(deps)

	if report.RHSMConnected {
		t.Error("RHSMConnected = true, want false")
	}
	if report.ContentEnabled {
		t.Error("ContentEnabled = true, want false")
	}
	if report.failedChecks != 1 {
		t.Errorf("failedChecks = %d, want 1", report.failedChecks)
	}
}

func TestGetStatusYggdrasilUnavailable(t *testing.T) {
	deps := passingDeps()
	deps.GetUnitState = func(string) (*remotemanagement.UnitState, error) {
		return &remotemanagement.UnitState{ActiveState: "inactive", LoadState: "not-found"}, nil
	}

	report := getStatus(deps)

	if report.YggdrasilRunning {
		t.Error("YggdrasilRunning = true, want false")
	}
	wantError := "The yggdrasil service is not available"
	if report.YggdrasilError != wantError {
		t.Errorf("YggdrasilError = %q, want %q", report.YggdrasilError, wantError)
	}
	if report.failedChecks != 1 {
		t.Errorf("failedChecks = %d, want 1", report.failedChecks)
	}
}

func TestGetStatusCountsEachFailure(t *testing.T) {
	deps := passingDeps()
	deps.RHSMIsRegistered = func() (bool, error) { return false, nil }
	deps.ContentIsEnabled = func() (bool, error) { return false, nil }
	deps.InsightsClientIsRegistered = func() (bool, error) { return false, nil }
	deps.GetUnitState = func(string) (*remotemanagement.UnitState, error) {
		return &remotemanagement.UnitState{ActiveState: "inactive", LoadState: "loaded"}, nil
	}

	report := getStatus(deps)

	// RHSM not registered, insights not registered, yggdrasil loaded-but-inactive.
	if report.failedChecks != 3 {
		t.Errorf("failedChecks = %d, want 3", report.failedChecks)
	}
	if !report.HasFailures() {
		t.Error("HasFailures() = false, want true")
	}
}

func TestHasFailures(t *testing.T) {
	if (&StatusReport{}).HasFailures() {
		t.Error("empty report should report no failures")
	}
	if !(&StatusReport{failedChecks: 1}).HasFailures() {
		t.Error("failedChecks = 1 should report a failure")
	}
}

func TestStatusReportJSONContract(t *testing.T) {
	report := &StatusReport{Hostname: "host", failedChecks: 5}

	data, err := json.MarshalIndent(report, "", "    ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{
    "hostname": "host",
    "rhsm_connected": false,
    "content_enabled": false,
    "insights_connected": false,
    "yggdrasil_running": false
}`
	if got := string(data); got != want {
		t.Errorf("JSON document =\n%s\nwant:\n%s", got, want)
	}
}
