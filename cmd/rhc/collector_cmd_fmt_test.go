package main

import (
	"strings"
	"testing"
	"time"

	"github.com/redhatinsights/rhc/varlink/collectorapi"
)

// TestPrintCollectorInfoUploadStatus verifies that the human-readable collector
// info output reports the upload status next to the collection run time, so that
// a successful collection is distinguishable from a successful upload.
func TestPrintCollectorInfoUploadStatus(t *testing.T) {
	lastRun := int(time.Now().Add(-2 * time.Minute).Unix())
	lastUploaded := int(time.Now().Add(-time.Minute).Unix())
	lastUploadedStamp := time.Unix(int64(lastUploaded), 0).Format(timeFormat)
	failedCode := 1

	tests := []struct {
		description    string
		uploadExitCode *int
		lastUploaded   *int
		want           []string
		unwanted       []string
	}{
		{
			description:    "successful upload",
			lastUploaded:   &lastUploaded,
			uploadExitCode: new(int),
			want:           []string{"Last upload:   " + lastUploadedStamp, "- success"},
			unwanted:       []string{"failed"},
		},
		{
			description:    "failed upload",
			lastUploaded:   &lastUploaded,
			uploadExitCode: &failedCode,
			want:           []string{"Last upload:   failed at " + lastUploadedStamp},
			unwanted:       []string{"- success"},
		},
		{
			description: "upload never recorded",
			want:        []string{"Last upload:   -"},
			unwanted:    []string{"success", "failed"},
		},
	}
	// The fields printed below the timing block are aligned with it.
	aligned := map[string]string{
		"Config:":  "/usr/lib/rhc/collectors/com.redhat.minimal.toml",
		"Service:": "rhc-collector-com.redhat.minimal.service",
		"Timer:":   "rhc-collector-com.redhat.minimal.timer",
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			info := &collectorapi.CollectorInfo{
				Id:             "com.redhat.minimal",
				Name:           "Minimal Host Inventory Collector",
				LastRun:        &lastRun,
				LastUploaded:   test.lastUploaded,
				UploadExitCode: test.uploadExitCode,
				ConfigPath:     "/usr/lib/rhc/collectors/com.redhat.minimal.toml",
				ServiceName:    "rhc-collector-com.redhat.minimal.service",
				TimerName:      "rhc-collector-com.redhat.minimal.timer",
			}

			got := captureStdout(t, func() { printCollectorInfo(info) })

			for _, want := range test.want {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q:\n%s", want, got)
				}
			}
			for _, unwanted := range test.unwanted {
				if strings.Contains(got, unwanted) {
					t.Errorf("output unexpectedly contains %q:\n%s", unwanted, got)
				}
			}
			for label, value := range aligned {
				want := label + strings.Repeat(" ", len("Last upload:   ")-len(label)) + value
				if !strings.Contains(got, want) {
					t.Errorf("output does not align %q with the timing block:\n%s", label, got)
				}
			}
		})
	}
}
