package operations

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

type fakeDisconnectRHSMClient struct {
	isRegistered func() (bool, error)
	unregister   func() error
}

func (client *fakeDisconnectRHSMClient) IsRegistered() (bool, error) {
	return client.isRegistered()
}

func (client *fakeDisconnectRHSMClient) Unregister() error {
	return client.unregister()
}

func disconnectTestDependencies(t *testing.T, calls *[]string) (disconnectDependencies, *fakeDisconnectRHSMClient) {
	t.Helper()
	client := &fakeDisconnectRHSMClient{
		isRegistered: func() (bool, error) {
			*calls = append(*calls, "check rhsm")
			return true, nil
		},
		unregister: func() error {
			*calls = append(*calls, "unregister rhsm")
			return nil
		},
	}
	return disconnectDependencies{
		UID: func() int { return 0 },
		AssertYggdrasilServiceState: func(state string) (bool, error) {
			if state != "inactive" {
				t.Errorf("requested service state = %q, want inactive", state)
			}
			*calls = append(*calls, "check yggdrasil")
			return false, nil
		},
		DeactivateServices: func() error {
			*calls = append(*calls, "deactivate yggdrasil")
			return nil
		},
		InsightsClientIsRegistered: func() (bool, error) {
			*calls = append(*calls, "check insights")
			return true, nil
		},
		UnregisterInsightsClient: func() error {
			*calls = append(*calls, "unregister insights")
			return nil
		},
		NewRHSMClient: func() (disconnectRHSMClient, error) {
			*calls = append(*calls, "create rhsm client")
			return client, nil
		},
	}, client
}

func TestDisconnectSuccessfulOrder(t *testing.T) {
	var calls []string
	deps, _ := disconnectTestDependencies(t, &calls)
	report, err := disconnect(deps)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{
		"check yggdrasil", "deactivate yggdrasil",
		"check insights", "unregister insights",
		"create rhsm client", "check rhsm", "unregister rhsm",
	}
	if !slices.Equal(calls, wantCalls) {
		t.Errorf("calls = %v, want %v", calls, wantCalls)
	}
	if report.UID != 0 {
		t.Errorf("unexpected system identity: %+v", report)
	}
	if !report.YggdrasilStopped || !report.InsightsDisconnected || !report.RHSMDisconnected {
		t.Errorf("all services should be disconnected: %+v", report)
	}
	if report.YggdrasilAlreadyInactive || report.InsightsAlreadyDisconnected || report.RHSMAlreadyDisconnected {
		t.Errorf("newly disconnected services reported as already disconnected: %+v", report)
	}
	if report.YggdrasilStoppedError != "" || report.InsightsDisconnectedError != "" || report.RHSMDisconnectedError != "" {
		t.Errorf("successful actions should not report errors: %+v", report)
	}
}

func TestDisconnectAlreadyDisconnected(t *testing.T) {
	var calls []string
	deps, client := disconnectTestDependencies(t, &calls)
	deps.AssertYggdrasilServiceState = func(string) (bool, error) { return true, nil }
	deps.InsightsClientIsRegistered = func() (bool, error) { return false, nil }
	client.isRegistered = func() (bool, error) { return false, nil }

	report, err := disconnect(deps)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"create rhsm client"}) {
		t.Errorf("already disconnected services must skip actions, calls = %v", calls)
	}
	if !report.YggdrasilStopped || !report.InsightsDisconnected || !report.RHSMDisconnected {
		t.Errorf("already disconnected services should count as successful: %+v", report)
	}
	if !report.YggdrasilAlreadyInactive || !report.InsightsAlreadyDisconnected || !report.RHSMAlreadyDisconnected {
		t.Errorf("missing already disconnected flags: %+v", report)
	}
}

func TestDisconnectStateFailuresSkipOnlyAffectedAction(t *testing.T) {
	for _, step := range []string{"yggdrasil", "insights", "rhsm", "rhsm client"} {
		t.Run(step, func(t *testing.T) {
			var calls []string
			deps, client := disconnectTestDependencies(t, &calls)
			checkErr := errors.New("state unavailable")
			wantSuccess := [3]bool{true, true, true}
			switch step {
			case "yggdrasil":
				deps.AssertYggdrasilServiceState = func(string) (bool, error) { return false, checkErr }
				wantSuccess[0] = false
			case "insights":
				deps.InsightsClientIsRegistered = func() (bool, error) { return false, checkErr }
				wantSuccess[1] = false
			case "rhsm":
				client.isRegistered = func() (bool, error) { return false, checkErr }
				wantSuccess[2] = false
			case "rhsm client":
				deps.NewRHSMClient = func() (disconnectRHSMClient, error) { return nil, checkErr }
				wantSuccess[2] = false
			}

			report, err := disconnect(deps)
			if err != nil {
				t.Fatalf("state-check failure should remain nonfatal: %v", err)
			}
			for i, action := range []string{"deactivate yggdrasil", "unregister insights", "unregister rhsm"} {
				if called := slices.Contains(calls, action); called != wantSuccess[i] {
					t.Errorf("%s called = %v, want %v", action, called, wantSuccess[i])
				}
			}
			gotSuccess := [3]bool{report.YggdrasilStopped, report.InsightsDisconnected, report.RHSMDisconnected}
			if gotSuccess != wantSuccess {
				t.Errorf("successful steps = %v, want %v", gotSuccess, wantSuccess)
			}
			if report.YggdrasilStoppedError != "" || report.InsightsDisconnectedError != "" || report.RHSMDisconnectedError != "" {
				t.Errorf("state-check failures should remain silent: %+v", report)
			}
		})
	}
}

func TestDisconnectActionFailuresContinue(t *testing.T) {
	var calls []string
	deps, client := disconnectTestDependencies(t, &calls)
	deps.DeactivateServices = func() error { return errors.New("yggdrasil failed") }
	deps.UnregisterInsightsClient = func() error { return errors.New("insights failed") }
	client.unregister = func() error { return errors.New("rhsm failed") }

	report, err := disconnect(deps)
	if err != nil {
		t.Fatalf("action failures belong in the report: %v", err)
	}
	// All three errors show that every action ran despite earlier failures.
	if !strings.Contains(report.YggdrasilStoppedError, "yggdrasil failed") ||
		!strings.Contains(report.InsightsDisconnectedError, "insights failed") ||
		!strings.Contains(report.RHSMDisconnectedError, "rhsm failed") {
		t.Errorf("missing action failures: %+v", report)
	}
	if report.YggdrasilStopped || report.InsightsDisconnected || report.RHSMDisconnected {
		t.Errorf("failed actions should not count as successful: %+v", report)
	}
}

func TestDisconnectNonRootStopsBeforeServices(t *testing.T) {
	// Unset dependencies must never be called after a permission failure.
	report, err := disconnect(disconnectDependencies{UID: func() int { return 1000 }})
	wantError := "non-root user cannot disconnect system"
	if err == nil || err.Error() != wantError {
		t.Errorf("error = %v, want %q", err, wantError)
	}
	if report == nil || report.UID != 1000 || report.UIDError != wantError {
		t.Errorf("missing permission failure details: %+v", report)
	}
}
