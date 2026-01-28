package features

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/redhatinsights/rhc/internal/conf"
)

// Helper function to create a temporary TOML config file
func createTempFeaturesFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rhc-features.toml")
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
		tomlContent string
		want        conf.Features
	}{
		{
			description: "config with all features enabled",
			tomlContent: "features = { \"content\" = true, \"analytics\" = true, \"remote-management\" = true }",
			want: conf.Features{
				Content:    boolPtr(true),
				Analytics:  boolPtr(true),
				Management: boolPtr(true),
			},
		},
		{
			description: "config with all features disabled",
			tomlContent: "features = { \"content\" = false, \"analytics\" = false, \"remote-management\" = false }",
			want: conf.Features{
				Content:    boolPtr(false),
				Analytics:  boolPtr(false),
				Management: boolPtr(false),
			},
		},
		{
			description: "config with mixed feature states",
			tomlContent: "features = { \"content\" = true, \"analytics\" = false, \"remote-management\" = true }",
			want: conf.Features{
				Content:    boolPtr(true),
				Analytics:  boolPtr(false),
				Management: boolPtr(true),
			},
		},
		{
			description: "config with only content enabled",
			tomlContent: "features = { \"content\" = true }",
			want: conf.Features{
				Content:    boolPtr(true),
				Analytics:  nil,
				Management: nil,
			},
		},
		{
			description: "config with empty features section",
			tomlContent: `features = {}`,
			want: conf.Features{
				Content:    nil,
				Analytics:  nil,
				Management: nil,
			},
		},
		{
			description: "config with no features section",
			tomlContent: `cert-file = "/etc/pki/consumer/testing.pem"`,
			want: conf.Features{
				Content:    nil,
				Analytics:  nil,
				Management: nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			tmpFilePath := createTempFeaturesFile(t, test.tomlContent)
			confFeatures, err := GetFeaturesFromFile(tmpFilePath)
			if err != nil {
				t.Fatalf("failed to parse features config test file: %v", err)
			}

			if !boolPtrEqual(confFeatures.Content, test.want.Content) {
				t.Errorf("Content: got %v, want %v", ptrToString(confFeatures.Content), ptrToString(test.want.Content))
			}
			if !boolPtrEqual(confFeatures.Analytics, test.want.Analytics) {
				t.Errorf("Analytics: got %v, want %v", ptrToString(confFeatures.Analytics), ptrToString(test.want.Analytics))
			}
			if !boolPtrEqual(confFeatures.Management, test.want.Management) {
				t.Errorf("Management: got %v, want %v", ptrToString(confFeatures.Management), ptrToString(test.want.Management))
			}
		})
	}
}

func TestGetUndecodedConfigKeys(t *testing.T) {
	tests := []struct {
		description             string
		tomlContent             string
		expectedInvalidFeatures []string
	}{
		{
			description:             "typo in feature key: contnet instead of content",
			tomlContent:             `features = { "contnet" = true }`,
			expectedInvalidFeatures: []string{"features.contnet"},
		},
		{
			description:             "typo in feature key: anlaytics instead of analytics",
			tomlContent:             `features = { "anlaytics" = true }`,
			expectedInvalidFeatures: []string{"features.anlaytics"},
		},
		{
			description:             "unknown feature key",
			tomlContent:             `features = { "key" = "value" }`,
			expectedInvalidFeatures: []string{"features.key"},
		},
		{
			description:             "mixed valid and invalid keys in features",
			tomlContent:             `features = { "content" = true, "typo" = false }`,
			expectedInvalidFeatures: []string{"features.typo"},
		},
		{
			description:             "valid single feature key should not error",
			tomlContent:             `features = { "remote-management" = true }`,
			expectedInvalidFeatures: []string{},
		},
		{
			description:             "valid multiple feature keys should not error",
			tomlContent:             `features = { "content" = true, "analytics" = false, "remote-management" = true }`,
			expectedInvalidFeatures: []string{},
		},
		{
			description:             "empty features section should not error",
			tomlContent:             `features = {}`,
			expectedInvalidFeatures: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			tmpFilePath := createTempFeaturesFile(t, test.tomlContent)
			var tempConf conf.Conf
			configMetadata, err := toml.DecodeFile(tmpFilePath, &tempConf)
			if err != nil {
				t.Fatalf("failed to decode features config test file: %v", err)
			}

			invalidFeatures := getUndecodedConfigKeys(configMetadata)

			slices.Sort(invalidFeatures)
			slices.Sort(test.expectedInvalidFeatures)
			if !slices.Equal(invalidFeatures, test.expectedInvalidFeatures) {
				t.Errorf("invalid features mismatch: got %v, want %v", invalidFeatures, test.expectedInvalidFeatures)
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
		config      *conf.Conf
		input       inputFeatures
		want        wantFeatures
		wantError   error
	}{
		{
			description: "no config and no CLI features provided",
			config:      nil,
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    boolPtr(true),
					Analytics:  boolPtr(true),
					Management: boolPtr(true),
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    boolPtr(false),
					Analytics:  boolPtr(false),
					Management: boolPtr(false),
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    boolPtr(true),
					Analytics:  nil,
					Management: nil,
				},
			},
			input: inputFeatures{
				enabledFeatures:  []string{"analytics"},
				disabledFeatures: []string{},
			},
			want: wantFeatures{
				enabledFeatures:  []string{"content", "analytics"},
				disabledFeatures: []string{},
			},
		},
		{
			description: "CLI overrides config - enabled cli flag overrides disabled config option",
			config: &conf.Conf{
				Features: conf.Features{
					Content:    boolPtr(false),
					Analytics:  boolPtr(false),
					Management: boolPtr(false),
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    boolPtr(true),
					Analytics:  boolPtr(true),
					Management: boolPtr(true),
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    nil,
					Analytics:  nil,
					Management: nil,
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    boolPtr(true),
					Analytics:  boolPtr(true),
					Management: nil,
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    nil,
					Analytics:  nil,
					Management: nil,
				},
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
			config: &conf.Conf{
				Features: conf.Features{
					Content:    nil,
					Analytics:  nil,
					Management: nil,
				},
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
			resultEnabledFeatures, resultDisabledFeatures, err := ConsolidateSelectedFeatures(test.config, test.input.enabledFeatures, test.input.disabledFeatures)

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
					t.Errorf("enabled features mismatch: got %v, want %v", test.input.enabledFeatures, test.want.enabledFeatures)
				}

				slices.Sort(resultDisabledFeatures)
				slices.Sort(test.want.disabledFeatures)
				if !slices.Equal(resultDisabledFeatures, test.want.disabledFeatures) {
					t.Errorf("disabled features mismatch: got %v, want %v", test.input.disabledFeatures, test.want.disabledFeatures)
				}
			}
		})
	}
}

func TestValidateSelectedFeatures(t *testing.T) {
	// Reset feature states before each test
	resetFeatures := func() {
		ContentFeature.Enabled = true
		AnalyticsFeature.Enabled = true
		ManagementFeature.Enabled = true
	}

	tests := []struct {
		description      string
		enabledFeatures  []string
		disabledFeatures []string
		expectError      bool
		errorContains    string
		validateState    func(*testing.T)
	}{
		{
			description:      "validate selected features where all features enabled",
			enabledFeatures:  []string{"content", "analytics", "remote-management"},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				if !ContentFeature.Enabled {
					t.Error("ContentFeature should be enabled")
				}
				if !AnalyticsFeature.Enabled {
					t.Error("AnalyticsFeature should be enabled")
				}
				if !ManagementFeature.Enabled {
					t.Error("ManagementFeature should be enabled")
				}
			},
		},
		{
			description:      "validate selected features where all features disabled",
			enabledFeatures:  []string{},
			disabledFeatures: []string{"content", "analytics", "remote-management"},
			expectError:      false,
			validateState: func(t *testing.T) {
				if ContentFeature.Enabled {
					t.Error("ContentFeature should be disabled")
				}
				if AnalyticsFeature.Enabled {
					t.Error("AnalyticsFeature should be disabled")
				}
				if ManagementFeature.Enabled {
					t.Error("ManagementFeature should be disabled")
				}
			},
		},
		{
			description:      "validate selected features where unknown feature in enabled list",
			enabledFeatures:  []string{"unknown-feature"},
			disabledFeatures: []string{},
			expectError:      true,
			errorContains:    "no such feature exists",
		},
		{
			description:      "validate selected features where unknown feature in disabled list",
			enabledFeatures:  []string{},
			disabledFeatures: []string{"unknown-feature"},
			expectError:      true,
			errorContains:    "no such feature exists",
		},
		{
			description:      "validate selected features where feature in both enabled and disabled",
			enabledFeatures:  []string{"content"},
			disabledFeatures: []string{"content"},
			expectError:      true,
			errorContains:    "explicitly disabled",
		},
		{
			description:      "validate selected features where management requires content - content disabled",
			enabledFeatures:  []string{"remote-management"},
			disabledFeatures: []string{"content"},
			expectError:      true,
			errorContains:    "required feature",
		},
		{
			description:      "validate selected features where management requires analytics - analytics disabled",
			enabledFeatures:  []string{"remote-management"},
			disabledFeatures: []string{"analytics"},
			expectError:      true,
			errorContains:    "required feature",
		},
		{
			description:      "validate selected features where management enabled with dependencies",
			enabledFeatures:  []string{"content", "analytics", "remote-management"},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				if !ManagementFeature.Enabled {
					t.Error("ManagementFeature should be enabled when dependencies are met")
				}
			},
		},
		{
			description:      "validate selected features where disable dependency affects dependent feature",
			enabledFeatures:  []string{},
			disabledFeatures: []string{"content"},
			expectError:      false,
			validateState: func(t *testing.T) {
				if ManagementFeature.Enabled {
					t.Error("ManagementFeature should be disabled when Content is disabled")
				}
				if ManagementFeature.Reason == "" {
					t.Error("ManagementFeature.Reason should be set")
				}
			},
		},
		{
			description:      "validate selected features where enable only content - no dependencies",
			enabledFeatures:  []string{"content"},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				if !ContentFeature.Enabled {
					t.Error("ContentFeature should be enabled")
				}
			},
		},
		{
			description:      "validate selected features where enable analytics and content - valid dependencies",
			enabledFeatures:  []string{"content", "analytics"},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				if !ContentFeature.Enabled {
					t.Error("ContentFeature should be enabled")
				}
				if !AnalyticsFeature.Enabled {
					t.Error("AnalyticsFeature should be enabled")
				}
			},
		},
		{
			description:      "validate selected features where analytics requires content - content not explicitly enabled",
			enabledFeatures:  []string{"analytics"},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				if !AnalyticsFeature.Enabled {
					t.Error("AnalyticsFeature should be enabled")
				}
				// Content should remain in its default state (not explicitly set)
			},
		},
		{
			description:      "validate selected features where disable content - analytics should be affected",
			enabledFeatures:  []string{},
			disabledFeatures: []string{"content"},
			expectError:      false,
			validateState: func(t *testing.T) {
				if ContentFeature.Enabled {
					t.Error("ContentFeature should be disabled")
				}
				if AnalyticsFeature.Enabled {
					t.Error("AnalyticsFeature should be disabled due to content being disabled")
				}
				if AnalyticsFeature.Reason == "" {
					t.Error("AnalyticsFeature.Reason should be set")
				}
			},
		},
		{
			description:      "validate selected features where disable analytics only - management should be affected",
			enabledFeatures:  []string{},
			disabledFeatures: []string{"analytics"},
			expectError:      false,
			validateState: func(t *testing.T) {
				if AnalyticsFeature.Enabled {
					t.Error("AnalyticsFeature should be disabled")
				}
				if ManagementFeature.Enabled {
					t.Error("ManagementFeature should be disabled due to analytics being disabled")
				}
				if ManagementFeature.Reason == "" {
					t.Error("ManagementFeature.Reason should be set")
				}
			},
		},
		{
			description:      "validate selected features where multiple unknown features in enabled list",
			enabledFeatures:  []string{"feature1", "feature2"},
			disabledFeatures: []string{},
			expectError:      true,
			errorContains:    "no such feature exists",
		},
		{
			description:      "validate selected features where multiple unknown features in disabled list",
			enabledFeatures:  []string{},
			disabledFeatures: []string{"feature1", "feature2"},
			expectError:      true,
			errorContains:    "no such feature exists",
		},
		{
			description:      "validate selected features where enable analytics without content - content disabled",
			enabledFeatures:  []string{"analytics"},
			disabledFeatures: []string{"content"},
			expectError:      true,
			errorContains:    "required feature",
		},
		{
			description:      "validate selected features where enable management with content but analytics disabled",
			enabledFeatures:  []string{"content", "remote-management"},
			disabledFeatures: []string{"analytics"},
			expectError:      true,
			errorContains:    "required feature",
		},
		{
			description:      "validate selected features where disable all explicitly and try to enable management",
			enabledFeatures:  []string{"remote-management"},
			disabledFeatures: []string{"content", "analytics", "remote-management"},
			expectError:      true,
			errorContains:    "explicitly disabled",
		},
		{
			description:      "validate selected features where empty enabled and disabled lists",
			enabledFeatures:  []string{},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				// All features should remain in their default state
			},
		},
		{
			description:      "validate selected features where case sensitive - wrong case for feature name",
			enabledFeatures:  []string{"Content"},
			disabledFeatures: []string{},
			expectError:      true,
			errorContains:    "no such feature exists",
		},
		{
			description:      "validate selected features where enable content twice in list",
			enabledFeatures:  []string{"content", "content"},
			disabledFeatures: []string{},
			expectError:      false,
			validateState: func(t *testing.T) {
				if !ContentFeature.Enabled {
					t.Error("ContentFeature should be enabled")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			resetFeatures()

			err := ValidateSelectedFeatures(&test.enabledFeatures, &test.disabledFeatures)

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
