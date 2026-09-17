package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/operations"
)

func TestFormatDisconnectReportHuman(t *testing.T) {
	tests := []struct {
		name      string
		report    operations.DisconnectReport
		wantSteps []string
	}{
		{
			name: "disconnected",
			report: operations.DisconnectReport{
				YggdrasilStopped:     true,
				InsightsDisconnected: true,
				RHSMDisconnected:     true,
			},
			wantSteps: []string{
				"[✓] Deactivated the yggdrasil service",
				"[✓] Disconnected from Red Hat Lightspeed (formerly Insights)",
				"[✓] Disconnected from Red Hat Subscription Management",
			},
		},
		{
			name: "already disconnected",
			report: operations.DisconnectReport{
				YggdrasilStopped:            true,
				YggdrasilAlreadyInactive:    true,
				InsightsDisconnected:        true,
				InsightsAlreadyDisconnected: true,
				RHSMDisconnected:            true,
				RHSMAlreadyDisconnected:     true,
			},
			wantSteps: []string{
				"[●] The yggdrasil service is already inactive",
				"[●] Already disconnected from Red Hat Lightspeed (formerly Insights)",
				"[●] Already disconnected from Red Hat Subscription Management",
			},
		},
		{
			name: "state checks failed silently",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.report.Hostname = "test-host"
			var err error
			out := captureStdout(t, func() {
				err = formatDisconnectReport(&tt.report, nil)
			})
			if err != nil {
				t.Fatalf("formatDisconnectReport() error = %v", err)
			}

			var gotSteps []string
			for line := range strings.SplitSeq(out, "\n") {
				if line = strings.TrimSpace(line); strings.HasPrefix(line, "[") {
					gotSteps = append(gotSteps, line)
				}
			}
			if !reflect.DeepEqual(gotSteps, tt.wantSteps) {
				t.Errorf("step lines = %q, want %q", gotSteps, tt.wantSteps)
			}
		})
	}
}

func TestFormatDisconnectReportStepErrors(t *testing.T) {
	report := &operations.DisconnectReport{
		Hostname:                  "test-host",
		YggdrasilStoppedError:     "remote management failed",
		InsightsDisconnectedError: "analytics failed",
		RHSMDisconnectedError:     "subscription management failed",
	}
	var err error
	out := captureStdout(t, func() {
		err = formatDisconnectReport(report, nil)
	})
	exitErr, ok := err.(cli.ExitCoder)
	if !ok {
		t.Fatalf("formatDisconnectReport() error = %v, want cli.ExitCoder", err)
	}
	if got := exitErr.ExitCode(); got != exitcode.Err {
		t.Errorf("exit code = %d, want %d", got, exitcode.Err)
	}

	steps, summary, found := strings.Cut(out, "The following errors were encountered during disconnect:")
	if !found {
		t.Fatalf("missing error summary in output: %q", out)
	}
	for _, message := range []string{
		report.YggdrasilStoppedError,
		report.InsightsDisconnectedError,
		report.RHSMDisconnectedError,
	} {
		if !strings.Contains(steps, message) || !strings.Contains(summary, message) {
			t.Errorf("error %q missing from step output or summary: %q", message, out)
		}
	}
}

func TestFormatDisconnectReportJSON(t *testing.T) {
	tests := []struct {
		name   string
		report operations.DisconnectReport
		want   map[string]any
	}{
		{
			name: "already disconnected omits internal fields and empty errors",
			report: operations.DisconnectReport{
				Hostname:                    "test-host",
				YggdrasilStopped:            true,
				YggdrasilAlreadyInactive:    true,
				InsightsDisconnected:        true,
				InsightsAlreadyDisconnected: true,
				RHSMDisconnected:            true,
				RHSMAlreadyDisconnected:     true,
				Durations:                   map[string]time.Duration{"rhsm": time.Second},
			},
			want: map[string]any{
				"hostname":              "test-host",
				"uid":                   float64(0),
				"yggdrasil_stopped":     true,
				"insights_disconnected": true,
				"rhsm_disconnected":     true,
			},
		},
		{
			name: "step errors retain successful exit and existing field names",
			report: operations.DisconnectReport{
				Hostname:                  "test-host",
				YggdrasilStoppedError:     "remote management failed",
				InsightsDisconnectedError: "analytics failed",
				RHSMDisconnectedError:     "subscription management failed",
			},
			want: map[string]any{
				"hostname":                    "test-host",
				"uid":                         float64(0),
				"yggdrasil_stopped":           false,
				"yggdrasil_stopped_error":     "remote management failed",
				"insights_disconnected":       false,
				"insights_disconnected_error": "analytics failed",
				"rhsm_disconnected":           false,
				"rhsm_disconnect_error":       "subscription management failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			out := captureStdout(t, func() {
				configureOutput(false, false, true)
				err = formatDisconnectReport(&tt.report, nil)
			})
			if err != nil {
				t.Fatalf("formatDisconnectReport() error = %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("invalid JSON output %q: %v", out, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JSON document = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestFormatDisconnectReportFatalErrors(t *testing.T) {
	tests := []struct {
		name     string
		report   operations.DisconnectReport
		message  string
		field    string
		wantCode int
	}{
		{
			name: "permission",
			report: operations.DisconnectReport{
				UID:      1000,
				UIDError: "non-root user cannot disconnect system",
			},
			message:  "non-root user cannot disconnect system",
			field:    "uid_error",
			wantCode: exitcode.NoPerm,
		},
		{
			name: "hostname",
			report: operations.DisconnectReport{
				HostnameError: "hostname unavailable",
			},
			message:  "hostname unavailable",
			field:    "hostname_error",
			wantCode: exitcode.Err,
		},
	}

	for _, tt := range tests {
		for _, format := range []string{"text", "json"} {
			t.Run(tt.name+"/"+format, func(t *testing.T) {
				var err error
				out := captureStdout(t, func() {
					configureOutput(false, false, format == "json")
					err = formatDisconnectReport(&tt.report, errors.New(tt.message))
				})
				if out != "" {
					t.Errorf("stdout = %q, want empty stdout for fatal errors", out)
				}
				exitErr, ok := err.(cli.ExitCoder)
				if !ok {
					t.Fatalf("formatDisconnectReport() error = %v, want cli.ExitCoder", err)
				}
				if got := exitErr.ExitCode(); got != tt.wantCode {
					t.Errorf("exit code = %d, want %d", got, tt.wantCode)
				}
				if format == "text" {
					if err.Error() != tt.message {
						t.Errorf("error = %q, want %q", err.Error(), tt.message)
					}
					return
				}
				var got map[string]any
				if jsonErr := json.Unmarshal([]byte(err.Error()), &got); jsonErr != nil {
					t.Fatalf("exit error contains invalid JSON %q: %v", err.Error(), jsonErr)
				}
				if got[tt.field] != tt.message {
					t.Errorf("JSON %s = %v, want %q", tt.field, got[tt.field], tt.message)
				}
			})
		}
	}
}
