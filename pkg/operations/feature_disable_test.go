package operations

import (
	"strings"
	"testing"
)

// TestDisableRemoteManagement tests disabling RemoteManagement when it has no enabled dependents.
func TestDisableRemoteManagement(t *testing.T) {
	result := DisableFeature(FeatureOperationOptions{Feature: RemoteManagement})
	if len(result.DependentsDisabled) != 0 {
		t.Errorf("DependentsDisabled length = %d, want 0 for RemoteManagement",
			len(result.DependentsDisabled))
	}
	if result.Feature != RemoteManagement {
		t.Errorf("Feature = %v, want %v", result.Feature, RemoteManagement)
	}

	validStatuses := []string{DisableStatusDisabled, DisableStatusAlreadyDisabled, DisableStatusFailed}
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
	if result.Status == DisableStatusAlreadyDisabled && result.Err != nil {
		t.Errorf("Status=%q but Err=%v, expected nil error", DisableStatusAlreadyDisabled, result.Err)
	}
}

// TestDisableAnalytics tests disabling Analytics when RemoteManagement depends on it.
func TestDisableAnalytics(t *testing.T) {
	rmStatus := FeatureStatus(FeatureOperationOptions{Feature: RemoteManagement})
	if rmStatus.Err != nil {
		t.Skipf("Cannot determine RemoteManagement status: %v", rmStatus.Err)
	}

	result := DisableFeature(FeatureOperationOptions{Feature: Analytics})
	if result.Feature != Analytics {
		t.Errorf("Feature = %v, want %v", result.Feature, Analytics)
	}
	if rmStatus.Enabled {
		if result.Err != nil {
			if result.Status != DisableStatusFailed && result.Status != DisableStatusDependentFailed {
				t.Errorf("expected status to be failed or dependent-failed when error occurred, got %q", result.Status)
			}
		} else {
			found := false
			for _, dep := range result.DependentsDisabled {
				if dep.Feature == RemoteManagement {
					found = true
					break
				}
			}
			if !found && result.Status != DisableStatusAlreadyDisabled {
				t.Errorf("RemoteManagement should be in DependentsDisabled when it was enabled, got: %v",
					result.DependentsDisabled)
			}
		}
	}
}

// TestDisableContent tests disabling Content when RemoteManagement depends on it.
func TestDisableContent(t *testing.T) {
	rmStatus := FeatureStatus(FeatureOperationOptions{Feature: RemoteManagement})
	if rmStatus.Err != nil {
		t.Skipf("Cannot determine RemoteManagement status: %v", rmStatus.Err)
	}

	result := DisableFeature(FeatureOperationOptions{Feature: Content})
	if result.Feature != Content {
		t.Errorf("Feature = %v, want %v", result.Feature, Content)
	}
	if rmStatus.Enabled {
		if result.Err != nil {
			if result.Status != DisableStatusFailed && result.Status != DisableStatusDependentFailed {
				t.Errorf("expected status to be failed or dependent-failed when error occurred, got %q", result.Status)
			}
		} else {
			found := false
			for _, dep := range result.DependentsDisabled {
				if dep.Feature == RemoteManagement {
					found = true
					break
				}
			}
			if !found && result.Status != DisableStatusAlreadyDisabled {
				t.Errorf("RemoteManagement should be in DependentsDisabled when it was enabled, got: %v",
					result.DependentsDisabled)
			}
		}
	}
}

// TestDisableResultStructure tests the structure of DisableFeatureResult.
func TestDisableResultStructure(t *testing.T) {
	tests := []struct {
		name    string
		feature Feature
	}{
		{"Analytics", Analytics},
		{"Content", Content},
		{"RemoteManagement", RemoteManagement},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DisableFeature(FeatureOperationOptions{Feature: tt.feature})
			if result.Feature != tt.feature {
				t.Errorf("Feature = %v, want %v", result.Feature, tt.feature)
			}
			if result.DependentsDisabled == nil {
				t.Error("DependentsDisabled should never be nil, should be empty slice")
			}
			if result.Status == "" {
				t.Error("Status should not be empty")
			}
			if result.Status == DisableStatusAlreadyDisabled && result.Err != nil {
				t.Errorf("Status=%q but Err=%v, should be mutually exclusive",
					DisableStatusAlreadyDisabled, result.Err)
			}
			if result.Err != nil {
				if result.Status != DisableStatusFailed && result.Status != DisableStatusDependentFailed {
					t.Errorf("expected status to be failed when Err is non-nil, got %q", result.Status)
				}
			}
		})
	}
}

// TestDisableUnknownFeature tests error handling for invalid feature.
func TestDisableUnknownFeature(t *testing.T) {
	invalidFeature := Feature(999)

	result := DisableFeature(FeatureOperationOptions{Feature: invalidFeature})
	if result.Feature != invalidFeature {
		t.Errorf("Feature = %v, want %v", result.Feature, invalidFeature)
	}
	if result.Err == nil {
		t.Error("Err = nil, want error for unknown feature")
	}
	if !strings.Contains(result.Err.Error(), "unknown feature") {
		t.Errorf("Err = %v, want error message containing 'unknown feature'", result.Err)
	}
	if result.Status != DisableStatusFailed {
		t.Errorf("Status = %q, want %q for unknown feature", result.Status, DisableStatusFailed)
	}
}

// TestDisableAnalyticsAlwaysProcessesDependents verifies the convergent
// behavior: disabling Analytics always checks whether RemoteManagement (its
// dependent) is still running, even when Analytics itself is already disabled.
// This repairs inconsistent states where a dependency was disabled but
// RemoteManagement was not torn down (e.g. after a partial disconnect).
func TestDisableAnalyticsAlwaysProcessesDependents(t *testing.T) {
	rmStatus := FeatureStatus(FeatureOperationOptions{Feature: RemoteManagement})
	if rmStatus.Err != nil {
		t.Skipf("Cannot determine RemoteManagement status: %v", rmStatus.Err)
	}

	result := DisableFeature(FeatureOperationOptions{Feature: Analytics})
	if result.Feature != Analytics {
		t.Fatalf("Feature = %v, want %v", result.Feature, Analytics)
	}

	if rmStatus.Enabled {
		// RemoteManagement was running — disable should have attempted to
		// tear it down regardless of Analytics' own state
		found := false
		for _, dep := range result.DependentsDisabled {
			if dep.Feature == RemoteManagement {
				found = true
				break
			}
		}
		if !found && result.Status != DisableStatusFailed && result.Status != DisableStatusDependentFailed {
			t.Errorf("RemoteManagement was enabled but not in DependentsDisabled "+
				"(status=%q, err=%v)", result.Status, result.Err)
		}
	}
}

// TestDisableIdempotency tests that calling DisableFeature multiple times is safe.
func TestDisableIdempotency(t *testing.T) {
	feature := RemoteManagement

	// First call
	result1 := DisableFeature(FeatureOperationOptions{Feature: feature})
	// Second call (should be idempotent)
	result2 := DisableFeature(FeatureOperationOptions{Feature: feature})

	// If the first call succeeded or indicated already disabled,
	// the second call should indicate already disabled
	if result1.Err == nil || result1.Status == DisableStatusAlreadyDisabled {
		if result2.Status != DisableStatusAlreadyDisabled {
			t.Errorf("Second call Status = %q, want %q after successful first call",
				result2.Status, DisableStatusAlreadyDisabled)
		}
		if result2.Err != nil {
			t.Errorf("Second call Err = %v, want nil after successful first call", result2.Err)
		}
	}

	if result1.Feature != result2.Feature {
		t.Errorf("Feature mismatch: first=%v, second=%v", result1.Feature, result2.Feature)
	}
}
