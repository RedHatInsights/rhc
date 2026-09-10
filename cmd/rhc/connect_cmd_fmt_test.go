package main

import (
	"testing"

	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/operations"
)

func TestFormatConnectJSON(t *testing.T) {
	report := operations.ConnectReport{
		Hostname:      "test-host",
		UID:           0,
		RHSMConnected: true,
		Content: operations.FeatureResult{
			Requested:  true,
			Successful: true,
			Enabled:    true,
		},
		Analytics: operations.FeatureResult{
			Requested:  true,
			Successful: true,
			Enabled:    true,
		},
		RemoteManagement: operations.FeatureResult{
			Requested:  true,
			Successful: true,
			Enabled:    true,
		},
	}

	got := captureStdout(t, func() {
		if err := formatConnectJSON(report); err != nil {
			t.Fatalf("formatConnectJSON() error = %v", err)
		}
	})

	want := `{
    "hostname": "test-host",
    "uid": 0,
    "rhsm_connected": true,
    "features": {
        "content": {
            "enabled": true,
            "successful": true
        },
        "analytics": {
            "enabled": true,
            "successful": true
        },
        "remote_management": {
            "enabled": true,
            "successful": true
        }
    }
}` + "\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestFormatConnectJSONPartialFailure(t *testing.T) {
	report := operations.ConnectReport{
		Hostname:      "test-host",
		RHSMConnected: true,
		RHSMError:     "",
		Content: operations.FeatureResult{
			Requested:  true,
			Successful: true,
			Enabled:    true,
		},
		Analytics: operations.FeatureResult{
			Requested:  true,
			Successful: false,
			Error:      "cannot connect to Red Hat Lightspeed (formerly Insights): boom",
			Enabled:    false,
		},
		RemoteManagement: operations.FeatureResult{
			Requested:  true,
			Successful: false,
			Skipped:    true,
			Error:      "skipped: dependency 'analytics' failed",
			Enabled:    false,
		},
	}

	got := captureStdout(t, func() {
		if err := formatConnectJSON(report); err != nil {
			t.Fatalf("formatConnectJSON() error = %v", err)
		}
	})

	want := `{
    "hostname": "test-host",
    "uid": 0,
    "rhsm_connected": true,
    "features": {
        "content": {
            "enabled": true,
            "successful": true
        },
        "analytics": {
            "enabled": false,
            "successful": false,
            "error": "cannot connect to Red Hat Lightspeed (formerly Insights): boom"
        },
        "remote_management": {
            "enabled": false,
            "successful": false,
            "error": "skipped: dependency 'analytics' failed",
            "skipped": true
        }
    }
}` + "\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestFormatConnectStepsReport(t *testing.T) {
	ui.ConfigureOutput(false, false, false)
	okIcon := ui.Icons.Ok
	errorIcon := ui.Icons.Error
	infoIcon := ui.Icons.Info
	warningIcon := ui.Icons.Warning

	tests := []struct {
		name   string
		report operations.ConnectReport
		want   string
	}{
		{
			name: "full success",
			report: operations.ConnectReport{
				RHSMConnected: true,
				Content:       operations.FeatureResult{Requested: true, Successful: true},
				Analytics:     operations.FeatureResult{Requested: true, Successful: true},
				RemoteManagement: operations.FeatureResult{
					Requested:  true,
					Successful: true,
				},
			},
			want: " [" + okIcon + "] Connected to Red Hat Subscription Management\n" +
				"  [" + okIcon + "] Content ... System has access to content\n" +
				"  [" + okIcon + "] Analytics ... Connected to Red Hat Lightspeed (formerly Insights)\n" +
				"  [" + okIcon + "] Remote Management ... Activated the yggdrasil service\n",
		},
		{
			name: "rhsm failure skips content",
			report: operations.ConnectReport{
				RHSMConnected: false,
				RHSMError:     "cannot connect",
				Analytics:     operations.FeatureResult{Requested: true},
				RemoteManagement: operations.FeatureResult{
					Requested:      true,
					Skipped:        true,
					SkipDependency: "content",
					Error:          "skipped: dependency 'content' failed",
				},
			},
			want: " [" + errorIcon + "] Cannot connect to Red Hat Subscription Management\n" +
				"  [" + errorIcon + "] Skipping generation of Red Hat repository file\n" +
				"  [" + errorIcon + "] Analytics ... Cannot connect to Red Hat Lightspeed (formerly Insights)\n" +
				"  [" + warningIcon + "] Remote Management ... Skipped (dependency 'content' failed)\n",
		},
		{
			name: "unrequested features skipped",
			report: operations.ConnectReport{
				RHSMConnected: true,
				Content:       operations.FeatureResult{Requested: true, Successful: true},
			},
			want: " [" + okIcon + "] Connected to Red Hat Subscription Management\n" +
				"  [" + okIcon + "] Content ... System has access to content\n" +
				"  [" + infoIcon + "] Analytics ... Skipped\n" +
				"  [" + infoIcon + "] Remote Management ... Skipped\n",
		},
		{
			name: "org required aborts before later steps",
			report: operations.ConnectReport{
				RHSMError: "no organization specified",
				Analytics: operations.FeatureResult{
					Requested: true,
					Skipped:   true,
				},
				RemoteManagement: operations.FeatureResult{
					Requested: true,
					Skipped:   true,
				},
			},
			want: " [" + errorIcon + "] Cannot connect to Red Hat Subscription Management\n" +
				"  [" + errorIcon + "] Skipping generation of Red Hat repository file\n" +
				"  [" + infoIcon + "] Analytics ... Skipped\n" +
				"  [" + infoIcon + "] Remote Management ... Skipped\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(t, func() { formatConnectStepsReport(tt.report) })
			if got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatConnectStepRHSM(t *testing.T) {
	ui.ConfigureOutput(false, false, false)
	okIcon := ui.Icons.Ok
	report := operations.ConnectReport{
		RHSMConnected: true,
		Content:       operations.FeatureResult{Requested: true, Successful: true},
	}
	got := captureStdout(t, func() {
		formatConnectStep(operations.ConnectStepRHSM, report)
	})
	want := " [" + okIcon + "] Connected to Red Hat Subscription Management\n" +
		"  [" + okIcon + "] Content ... System has access to content\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestConnectErrorMessages(t *testing.T) {
	report := operations.ConnectReport{
		RHSMError: "rhsm failed",
		Analytics: operations.FeatureResult{Error: "insights failed"},
		RemoteManagement: operations.FeatureResult{
			Skipped: true,
			Error:   "skipped: dependency 'analytics' failed",
		},
	}
	got := connectErrorMessages(report)
	if got["rhsm"] != "rhsm failed" {
		t.Errorf("rhsm error = %q", got["rhsm"])
	}
	if got["insights"] != "insights failed" {
		t.Errorf("insights error = %q", got["insights"])
	}
	if _, ok := got["yggdrasil"]; ok {
		t.Errorf("skipped remote management should not appear in error messages")
	}
}
