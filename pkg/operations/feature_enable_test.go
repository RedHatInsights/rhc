package operations

import (
	"strings"
	"testing"
)

// TestEnableAnalytics tests enabling the Analytics feature with no dependencies.
func TestEnableAnalytics(t *testing.T) {
	result := EnableFeature(FeatureOperationOptions{Feature: Analytics})
	if len(result.DependenciesEnabled) != 0 {
		t.Errorf("expected 0 dependencies enabled, got %d: %v",
			len(result.DependenciesEnabled), result.DependenciesEnabled)
	}
	if result.Feature != Analytics {
		t.Errorf("expected feature to be Analytics, got %v", result.Feature)
	}

	validStatuses := []string{EnableStatusEnabled, EnableStatusAlreadyEnabled, EnableStatusFailed}
	statusValid := false
	for _, s := range validStatuses {
		if result.Status == s {
			statusValid = true
			break
		}
	}
	if !statusValid {
		t.Errorf("expected status to be one of %v, got %q", validStatuses, result.Status)
	}
}

// TestEnableContent tests enabling the Content feature with no dependencies.
func TestEnableContent(t *testing.T) {
	result := EnableFeature(FeatureOperationOptions{Feature: Content})
	if len(result.DependenciesEnabled) != 0 {
		t.Errorf("expected 0 dependencies enabled, got %d: %v",
			len(result.DependenciesEnabled), result.DependenciesEnabled)
	}
	if result.Feature != Content {
		t.Errorf("expected feature to be Content, got %v", result.Feature)
	}

	validStatuses := []string{EnableStatusEnabled, EnableStatusAlreadyEnabled, EnableStatusFailed}
	statusValid := false
	for _, s := range validStatuses {
		if result.Status == s {
			statusValid = true
			break
		}
	}
	if !statusValid {
		t.Errorf("expected status to be one of %v, got %q", validStatuses, result.Status)
	}
}

// TestEnableRemoteManagement tests enabling RemoteManagement which requires both Content and Analytics dependencies.
func TestEnableRemoteManagement(t *testing.T) {
	result := EnableFeature(FeatureOperationOptions{Feature: RemoteManagement})
	if result.Feature != RemoteManagement {
		t.Errorf("expected feature to be RemoteManagement, got %v", result.Feature)
	}

	if len(result.DependenciesEnabled) == 2 {
		if result.DependenciesEnabled[0].Feature != Content {
			t.Errorf("expected first dependency to be Content, got %v", result.DependenciesEnabled[0].Feature)
		}
		if result.DependenciesEnabled[1].Feature != Analytics {
			t.Errorf("expected second dependency to be Analytics, got %v", result.DependenciesEnabled[1].Feature)
		}
	}

	if result.Err != nil {
		if result.Status != EnableStatusFailed && result.Status != EnableStatusDependencyFailed {
			t.Errorf("expected status to be %q or %q when error occurred, got %q",
				EnableStatusFailed, EnableStatusDependencyFailed, result.Status)
		}
	}
}

// TestEnableResultStructure tests the structure of EnableFeatureResult.
func TestEnableResultStructure(t *testing.T) {
	features := []Feature{Analytics, Content, RemoteManagement}

	for _, feature := range features {
		t.Run(feature.String(), func(t *testing.T) {
			result := EnableFeature(FeatureOperationOptions{Feature: feature})
			if result.Feature != feature {
				t.Errorf("expected Feature=%v, got %v", feature, result.Feature)
			}
			if result.Status == "" {
				t.Error("Status should not be empty")
			}
			if result.DependenciesEnabled == nil {
				t.Error("DependenciesEnabled should not be nil")
			}

			if result.Err != nil {
				// When error occurs, status should be "failed" or "dependency-failed"
				if result.Status != EnableStatusFailed && result.Status != EnableStatusDependencyFailed {
					t.Errorf("expected status to be %q or %q when Err is non-nil, got %q",
						EnableStatusFailed, EnableStatusDependencyFailed, result.Status)
				}
			}

			if result.Status == EnableStatusAlreadyEnabled {
				// When already enabled, should have no error
				if result.Err != nil {
					t.Errorf("expected no error when Status=%q, got: %v", EnableStatusAlreadyEnabled, result.Err)
				}
			}
		})
	}
}

// TestEnableUnknownFeature tests handling of an unknown/invalid feature type.
func TestEnableUnknownFeature(t *testing.T) {
	invalidFeature := Feature(999)

	result := EnableFeature(FeatureOperationOptions{Feature: invalidFeature})
	if result.Err == nil {
		t.Error("expected error for unknown feature, got nil")
	}
	if result.Err != nil && !strings.Contains(result.Err.Error(), "unknown feature") {
		t.Errorf("expected error to mention 'unknown feature', got: %v", result.Err)
	}
	if result.Status != EnableStatusFailed {
		t.Errorf("expected status to be %q, got %q", EnableStatusFailed, result.Status)
	}
	if result.Feature != invalidFeature {
		t.Errorf("expected feature to be %v, got %v", invalidFeature, result.Feature)
	}
	if len(result.DependenciesEnabled) != 0 {
		t.Errorf("expected no dependencies enabled for invalid feature, got: %v",
			result.DependenciesEnabled)
	}
}
