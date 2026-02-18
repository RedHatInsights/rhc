package features

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/redhatinsights/rhc/internal/conf"
)

// Helper function to create a temporary TOML config file
func createTempFeaturesFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rhc-connect-features-prefs.json")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create temp features config file: %v", err)
	}
	return filePath
}

// TestGetFeaturesFromFile_ValidTOML tests parsing valid TOML configurations
func TestGetFeaturesFromFile(t *testing.T) {
	tests := []struct {
		description string
		jsonContent string
		want        conf.ConnectFeaturesPrefs
		wantErr     bool
	}{
		{
			description: "config with all features enabled",
			jsonContent: "{ \"content\": true, \"analytics\": true, \"remote-management\": true }",
			want: conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        boolPtr(true),
				RemoteManagement: boolPtr(true),
			},
			wantErr: false,
		},
		{
			description: "config with all features disabled",
			jsonContent: "{ \"content\": false, \"analytics\": false, \"remote-management\": false }",
			want: conf.ConnectFeaturesPrefs{
				Content:          boolPtr(false),
				Analytics:        boolPtr(false),
				RemoteManagement: boolPtr(false),
			},
			wantErr: false,
		},
		{
			description: "config with mixed feature states",
			jsonContent: "{ \"content\": true, \"analytics\": false, \"remote-management\": true }",
			want: conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        boolPtr(false),
				RemoteManagement: boolPtr(true),
			},
			wantErr: false,
		},
		{
			description: "config with only content enabled",
			jsonContent: "{ \"content\": true }",
			want: conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        nil,
				RemoteManagement: nil,
			},
			wantErr: false,
		},
		{
			description: "config with empty object",
			jsonContent: `{}`,
			want: conf.ConnectFeaturesPrefs{
				Content:          nil,
				Analytics:        nil,
				RemoteManagement: nil,
			},
			wantErr: false,
		},
		{
			description: "config with incompatible root object",
			jsonContent: `[]`,
			want: conf.ConnectFeaturesPrefs{
				Content:          nil,
				Analytics:        nil,
				RemoteManagement: nil,
			},
			wantErr: true,
		},
		{
			description: "corrupted JSON file",
			jsonContent: `{"content' = foo }`,
			want: conf.ConnectFeaturesPrefs{
				Content:          nil,
				Analytics:        nil,
				RemoteManagement: nil,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			tmpFilePath := createTempFeaturesFile(t, test.jsonContent)
			confFeatures, err := GetFeaturesFromFile(tmpFilePath)
			if err != nil {
				if !test.wantErr {
					t.Fatalf("failed to parse features config test file: %v", err)
				}
				return
			}

			if test.wantErr {
				t.Fatalf("expected error but got none")
			}

			if !boolPtrEqual(confFeatures.Content, test.want.Content) {
				t.Errorf("Content: got %v, want %v", ptrToString(confFeatures.Content), ptrToString(test.want.Content))
			}
			if !boolPtrEqual(confFeatures.Analytics, test.want.Analytics) {
				t.Errorf("Analytics: got %v, want %v", ptrToString(confFeatures.Analytics), ptrToString(test.want.Analytics))
			}
			if !boolPtrEqual(confFeatures.RemoteManagement, test.want.RemoteManagement) {
				t.Errorf("Management: got %v, want %v", ptrToString(confFeatures.RemoteManagement), ptrToString(test.want.RemoteManagement))
			}
		})
	}
}

func TestConsolidateSelectedFeatures(t *testing.T) {
	type inputFeatures struct {
		enabledFeatures  []string
		disabledFeatures []string
	}
	type wantFeatures struct {
		enabledFeatures  []string
		disabledFeatures []string
	}
	tests := []struct {
		description string
		featPrefs   *conf.ConnectFeaturesPrefs
		input       inputFeatures
		want        wantFeatures
		wantError   error
	}{
		{
			description: "no config and no CLI features provided",
			featPrefs:   nil,
			input: inputFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{},
			},
			wantError: fmt.Errorf("failed to consolidate selected features: config is nil"),
		},
		{
			description: "config with all features enabled, no CLI features",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        boolPtr(true),
				RemoteManagement: boolPtr(true),
			},
			input: inputFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics", "remote-management"},
				disabledFeatures: []string{},
			},
		},
		{
			description: "config with all features disabled, no CLI features",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          boolPtr(false),
				Analytics:        boolPtr(false),
				RemoteManagement: boolPtr(false),
			},
			input: inputFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{"content", "analytics", "remote-management"},
			},
		},
		{
			description: "config with content feature enabled, CLI enables analytics",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        nil, // Default value is true
				RemoteManagement: nil, // Default value is true
			},
			input: inputFeatures{
				enabledFeatures:  []string{"analytics"},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics", "remote-management"},
				disabledFeatures: []string{},
			},
		},
		{
			description: "CLI overrides config - enabled cli flag overrides disabled config option",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          boolPtr(false),
				Analytics:        boolPtr(false),
				RemoteManagement: boolPtr(false),
			},
			input: inputFeatures{
				enabledFeatures:  []string{"content"},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content"},
				disabledFeatures: []string{"analytics", "remote-management"},
			},
		},
		{
			description: "CLI overrides config - disable cli flag overrides enabled config option",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        boolPtr(true),
				RemoteManagement: boolPtr(true),
			},
			input: inputFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{"remote-management"},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics"},
				disabledFeatures: []string{"remote-management"},
			},
		},
		{
			description: "config without any feature flags provided, with CLI flags",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          nil,
				Analytics:        nil,
				RemoteManagement: nil,
			},
			input: inputFeatures{
				enabledFeatures:  []string{"content", "analytics"},
				disabledFeatures: []string{"remote-management"},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics"},
				disabledFeatures: []string{"remote-management"},
			},
		},
		{
			description: "config with partial enabled features, and CLI disable partial features",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          boolPtr(true),
				Analytics:        boolPtr(true),
				RemoteManagement: nil,
			},
			input: inputFeatures{
				enabledFeatures:  []string{"content", "analytics"},
				disabledFeatures: []string{"remote-management"},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics"},
				disabledFeatures: []string{"remote-management"},
			},
		},
		{
			description: "all config features nil, all CLI features enabled",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          nil,
				Analytics:        nil,
				RemoteManagement: nil,
			},
			input: inputFeatures{
				enabledFeatures:  []string{"content", "analytics", "remote-management"},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics", "remote-management"},
				disabledFeatures: []string{},
			},
		},
		{
			description: "all config features nil, all CLI features disabled",
			featPrefs: &conf.ConnectFeaturesPrefs{
				Content:          nil,
				Analytics:        nil,
				RemoteManagement: nil,
			},
			input: inputFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{"content", "analytics", "remote-management"},
			},
			want: wantFeatures{
				enabledFeatures:  []string{},
				disabledFeatures: []string{"content", "analytics", "remote-management"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			resultEnabledFeatures, resultDisabledFeatures, err := ConsolidateSelectedFeatures(
				test.featPrefs, test.input.enabledFeatures, test.input.disabledFeatures)

			if test.wantError != nil {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				slices.Sort(resultEnabledFeatures)
				slices.Sort(test.want.enabledFeatures)
				if !slices.Equal(resultEnabledFeatures, test.want.enabledFeatures) {
					t.Errorf("enabled features mismatch: got %v, want %v",
						resultEnabledFeatures, test.want.enabledFeatures)
				}

				slices.Sort(resultDisabledFeatures)
				slices.Sort(test.want.disabledFeatures)
				if !slices.Equal(resultDisabledFeatures, test.want.disabledFeatures) {
					t.Errorf("disabled features mismatch: got %v, want %v",
						resultDisabledFeatures, test.want.disabledFeatures)
				}
			}
		})
	}
}

func TestValidateSelectedFeatures(t *testing.T) {
	// Reset feature states before each test
	resetFeatures := func() {
		ContentFeature.WantEnabled = true
		AnalyticsFeature.WantEnabled = true
		ManagementFeature.WantEnabled = true
	}

	tests := []struct {
		description          string
		wantEnabledFeatures  []string
		wantDisabledFeatures []string
		expectError          bool
		errorContains        string
		validateState        func(*testing.T)
	}{
		{
			description:          "validate selected features where all features enabled",
			wantEnabledFeatures:  []string{"content", "analytics", "remote-management"},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				if !ContentFeature.WantEnabled {
					t.Error("ContentFeature should be enabled")
				}
				if !AnalyticsFeature.WantEnabled {
					t.Error("AnalyticsFeature should be enabled")
				}
				if !ManagementFeature.WantEnabled {
					t.Error("ManagementFeature should be enabled")
				}
			},
		},
		{
			description:          "validate selected features where all features disabled",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{"content", "analytics", "remote-management"},
			expectError:          false,
			validateState: func(t *testing.T) {
				if ContentFeature.WantEnabled {
					t.Error("ContentFeature should be disabled")
				}
				if AnalyticsFeature.WantEnabled {
					t.Error("AnalyticsFeature should be disabled")
				}
				if ManagementFeature.WantEnabled {
					t.Error("ManagementFeature should be disabled")
				}
			},
		},
		{
			description:          "validate selected features where unknown feature in enabled list",
			wantEnabledFeatures:  []string{"unknown-feature"},
			wantDisabledFeatures: []string{},
			expectError:          true,
			errorContains:        "no such feature exists",
		},
		{
			description:          "validate selected features where unknown feature in disabled list",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{"unknown-feature"},
			expectError:          true,
			errorContains:        "no such feature exists",
		},
		{
			description:          "validate selected features where feature in both enabled and disabled",
			wantEnabledFeatures:  []string{"content"},
			wantDisabledFeatures: []string{"content"},
			expectError:          true,
			errorContains:        "explicitly disabled",
		},
		{
			description:          "validate selected features where management requires content - content disabled",
			wantEnabledFeatures:  []string{"remote-management"},
			wantDisabledFeatures: []string{"content"},
			expectError:          true,
			errorContains:        "required feature",
		},
		{
			description:          "validate selected features where management requires analytics - analytics disabled",
			wantEnabledFeatures:  []string{"remote-management"},
			wantDisabledFeatures: []string{"analytics"},
			expectError:          true,
			errorContains:        "required feature",
		},
		{
			description:          "validate selected features where management enabled with dependencies",
			wantEnabledFeatures:  []string{"content", "analytics", "remote-management"},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				if !ManagementFeature.WantEnabled {
					t.Error("ManagementFeature should be enabled when dependencies are met")
				}
			},
		},
		{
			description:          "validate selected features where disable dependency affects dependent feature",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{"content"},
			expectError:          false,
			validateState: func(t *testing.T) {
				if ManagementFeature.WantEnabled {
					t.Error("ManagementFeature should be disabled when Content is disabled")
				}
				if ManagementFeature.Reason == "" {
					t.Error("ManagementFeature.Reason should be set")
				}
			},
		},
		{
			description:          "validate selected features where enable only content - no dependencies",
			wantEnabledFeatures:  []string{"content"},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				if !ContentFeature.WantEnabled {
					t.Error("ContentFeature should be enabled")
				}
			},
		},
		{
			description:          "validate selected features where enable analytics and content - valid dependencies",
			wantEnabledFeatures:  []string{"content", "analytics"},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				if !ContentFeature.WantEnabled {
					t.Error("ContentFeature should be enabled")
				}
				if !AnalyticsFeature.WantEnabled {
					t.Error("AnalyticsFeature should be enabled")
				}
			},
		},
		{
			description:          "validate selected features where analytics requires content - content not explicitly enabled",
			wantEnabledFeatures:  []string{"analytics"},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				if !AnalyticsFeature.WantEnabled {
					t.Error("AnalyticsFeature should be enabled")
				}
				// Content should remain in its default state (not explicitly set)
			},
		},
		{
			description:          "validate selected features where disable content - analytics should be affected",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{"content"},
			expectError:          false,
			validateState: func(t *testing.T) {
				if ContentFeature.WantEnabled {
					t.Error("ContentFeature should be disabled")
				}
				if AnalyticsFeature.WantEnabled {
					t.Error("AnalyticsFeature should be disabled due to content being disabled")
				}
				if AnalyticsFeature.Reason == "" {
					t.Error("AnalyticsFeature.Reason should be set")
				}
			},
		},
		{
			description:          "validate selected features where disable analytics only - management should be affected",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{"analytics"},
			expectError:          false,
			validateState: func(t *testing.T) {
				if AnalyticsFeature.WantEnabled {
					t.Error("AnalyticsFeature should be disabled")
				}
				if ManagementFeature.WantEnabled {
					t.Error("ManagementFeature should be disabled due to analytics being disabled")
				}
				if ManagementFeature.Reason == "" {
					t.Error("ManagementFeature.Reason should be set")
				}
			},
		},
		{
			description:          "validate selected features where multiple unknown features in enabled list",
			wantEnabledFeatures:  []string{"feature1", "feature2"},
			wantDisabledFeatures: []string{},
			expectError:          true,
			errorContains:        "no such feature exists",
		},
		{
			description:          "validate selected features where multiple unknown features in disabled list",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{"feature1", "feature2"},
			expectError:          true,
			errorContains:        "no such feature exists",
		},
		{
			description:          "validate selected features where enable analytics without content - content disabled",
			wantEnabledFeatures:  []string{"analytics"},
			wantDisabledFeatures: []string{"content"},
			expectError:          true,
			errorContains:        "required feature",
		},
		{
			description:          "validate selected features where enable management with content but analytics disabled",
			wantEnabledFeatures:  []string{"content", "remote-management"},
			wantDisabledFeatures: []string{"analytics"},
			expectError:          true,
			errorContains:        "required feature",
		},
		{
			description:          "validate selected features where disable all explicitly and try to enable management",
			wantEnabledFeatures:  []string{"remote-management"},
			wantDisabledFeatures: []string{"content", "analytics", "remote-management"},
			expectError:          true,
			errorContains:        "explicitly disabled",
		},
		{
			description:          "validate selected features where empty enabled and disabled lists",
			wantEnabledFeatures:  []string{},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				// All features should remain in their default state
			},
		},
		{
			description:          "validate selected features where case sensitive - wrong case for feature name",
			wantEnabledFeatures:  []string{"Content"},
			wantDisabledFeatures: []string{},
			expectError:          true,
			errorContains:        "no such feature exists",
		},
		{
			description:          "validate selected features where enable content twice in list",
			wantEnabledFeatures:  []string{"content", "content"},
			wantDisabledFeatures: []string{},
			expectError:          false,
			validateState: func(t *testing.T) {
				if !ContentFeature.WantEnabled {
					t.Error("ContentFeature should be enabled")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			resetFeatures()

			err := ValidateSelectedFeatures(&test.wantEnabledFeatures, &test.wantDisabledFeatures)

			if test.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !test.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !test.expectError && test.validateState != nil {
				test.validateState(t)
			}
		})
	}
}

// boolPtr is a test helper function that converts a bool
// to a bool pointer
func boolPtr(b bool) *bool {
	return &b
}

// boolPtrEqual is a test helper function that compares
// two bool pointers for equality
func boolPtrEqual(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// ptrToString is a test helper function that converts a bool
// pointer to a string for error messages
func ptrToString(b *bool) string {
	if b == nil {
		return "nil"
	}
	if *b {
		return "true"
	}
	return "false"
}
