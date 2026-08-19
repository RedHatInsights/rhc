package main

import (
	"io"
	"os"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/internal/ui"
	"github.com/redhatinsights/rhc/pkg/operations"
)

// captureStdout redirects stdout to a pipe, initializes plain UI output, and
// returns what fn printed. The function may reconfigure the UI while it runs.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer
	ui.ConfigureOutput(false, false, false)
	t.Cleanup(func() {
		os.Stdout = originalStdout
		ui.ConfigureOutput(true, true, false)
	})

	fn()
	_ = writer.Close()
	out, _ := io.ReadAll(reader)
	_ = reader.Close()
	return string(out)
}

func TestRunStatusActionJSON(t *testing.T) {
	statusCalls := 0
	getStatus := func() *operations.StatusReport {
		statusCalls++
		return &operations.StatusReport{
			Hostname:          "test-host",
			RHSMConnected:     true,
			ContentEnabled:    true,
			InsightsConnected: true,
			YggdrasilRunning:  true,
		}
	}

	var actionErr error
	got := captureStdout(t, func() {
		ui.ConfigureOutput(false, false, true)
		defer ui.ConfigureOutput(false, false, false)

		actionErr = runStatusAction(&cli.Command{Name: "status"}, getStatus)
	})

	if actionErr != nil {
		t.Fatalf("status command error = %v", actionErr)
	}
	if statusCalls != 1 {
		t.Errorf("status calls = %d, want 1", statusCalls)
	}
	want := `{
    "hostname": "test-host",
    "rhsm_connected": true,
    "content_enabled": true,
    "insights_connected": true,
    "yggdrasil_running": true
}` + "\n"
	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

// TestFormatStatusLines pins the exact human-readable line each formatter prints
// for its success, negative, and error states.
func TestFormatStatusLines(t *testing.T) {
	ui.ConfigureOutput(false, false, false)
	okIcon := ui.Icons.Ok
	errorIcon := ui.Icons.Error
	tests := []struct {
		name   string
		render func()
		want   string
	}{
		{
			name:   "rhsm connected",
			render: func() { formatRHSMStatus(operations.StatusReport{RHSMConnected: true}) },
			want:   " [" + okIcon + "] Connected to Red Hat Subscription Management\n",
		},
		{
			name:   "rhsm not connected",
			render: func() { formatRHSMStatus(operations.StatusReport{}) },
			want:   " [ ] Not connected to Red Hat Subscription Management\n",
		},
		{
			name:   "rhsm error",
			render: func() { formatRHSMStatus(operations.StatusReport{RHSMError: "boom"}) },
			want:   " [" + errorIcon + "] Red Hat Subscription Management ... unable to check registration status: boom\n",
		},
		{
			name:   "content enabled",
			render: func() { formatContentStatus(operations.StatusReport{ContentEnabled: true}) },
			want:   "  [" + okIcon + "] Content ... System has access to content\n",
		},
		{
			name:   "content disabled",
			render: func() { formatContentStatus(operations.StatusReport{}) },
			want:   "  [ ] Content ... System has no access to content\n",
		},
		{
			name:   "content error",
			render: func() { formatContentStatus(operations.StatusReport{ContentError: "boom"}) },
			want:   "  [" + errorIcon + "] Content ... unable to check content management: boom\n",
		},
		{
			name:   "insights connected",
			render: func() { formatInsightsStatus(operations.StatusReport{InsightsConnected: true}) },
			want:   "  [" + okIcon + "] Analytics ... Connected to Red Hat Lightspeed (formerly Insights)\n",
		},
		{
			name:   "insights not connected",
			render: func() { formatInsightsStatus(operations.StatusReport{}) },
			want:   "  [ ] Analytics ... Not connected to Red Hat Lightspeed (formerly Insights)\n",
		},
		{
			name:   "insights error",
			render: func() { formatInsightsStatus(operations.StatusReport{InsightsError: "boom"}) },
			want:   "  [" + errorIcon + "] Analytics ... Cannot detect Red Hat Lightspeed (formerly Insights) status: boom\n",
		},
		{
			name:   "yggdrasil active",
			render: func() { formatServiceStatus(operations.StatusReport{YggdrasilRunning: true}) },
			want:   "  [" + okIcon + "] Remote Management ... The yggdrasil service is active\n",
		},
		{
			name:   "yggdrasil not running",
			render: func() { formatServiceStatus(operations.StatusReport{}) },
			want:   "  [ ] Remote Management ... The yggdrasil service is not running\n",
		},
		{
			name: "yggdrasil error",
			render: func() {
				formatServiceStatus(operations.StatusReport{YggdrasilError: "The yggdrasil service is not available"})
			},
			want: "  [" + errorIcon + "] Remote Management ... The yggdrasil service is not available\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := captureStdout(t, tt.render)
			if got != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}
