package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// setupMockOSRelease creates a temporary os-release file for testing.
// Returns a cleanup function that should be called with defer.
func setupMockOSRelease(t *testing.T, id, version string) func() {
	t.Helper()

	// Save original values
	originalPath := osReleasePath

	// Reset cache for this test (sync.Once cannot be copied, so we create a new one)
	cachedOSVersion = ""
	cacheOnce = sync.Once{}

	// Create temporary file
	tmpDir := t.TempDir()
	mockFile := filepath.Join(tmpDir, "os-release")

	content := "ID=" + id + "\nVERSION_ID=" + version + "\n"
	if err := os.WriteFile(mockFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create mock os-release: %v", err)
	}

	// Override package variable
	osReleasePath = mockFile

	// Return cleanup function
	return func() {
		osReleasePath = originalPath
		// Reset cache again for subsequent tests
		cachedOSVersion = ""
		cacheOnce = sync.Once{}
	}
}

func TestGetUserAgent(t *testing.T) {
	// Setup mock RHEL environment for all tests
	cleanup := setupMockOSRelease(t, "rhel", "9.3")
	defer cleanup()

	tests := []struct {
		name        string
		component   string
		version     string
		triggeredBy string
		contains    []string
	}{
		{
			name:        "full user agent with all fields",
			component:   "rhc-collector",
			version:     "1.0.0",
			triggeredBy: "testing.collector",
			contains: []string{
				"rhc-collector/1.0.0",
				"triggered-by: testing.collector",
				"rhel/9.3",
			},
		},
		{
			name:        "user agent with empty triggered-by",
			component:   "rhc-collector",
			version:     "1.0.0",
			triggeredBy: "",
			contains: []string{
				"rhc-collector/1.0.0",
				"triggered-by: ",
				"rhel/9.3",
			},
		},
		{
			name:        "rhc-server component",
			component:   "rhc-server",
			version:     "1.2.3",
			triggeredBy: "sample.api.method",
			contains: []string{
				"rhc-server/1.2.3",
				"triggered-by: sample.api.method",
				"rhel/9.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userAgent := GetUserAgent(tt.component, tt.version, tt.triggeredBy)

			if userAgent == "" {
				t.Error("GetUserAgent() returned empty string")
				return
			}

			for _, expected := range tt.contains {
				if !strings.Contains(userAgent, expected) {
					t.Errorf("GetUserAgent() = %q, want to contain %q", userAgent, expected)
				}
			}
		})
	}
}
