package operations

import (
	"fmt"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

// DisableFeatureResult represents the result of disabling a feature.
// It includes the target feature, status of the operation, nested results
// for dependents, and any error that occurred.
type DisableFeatureResult struct {
	// Feature is the target feature that was requested to be disabled.
	Feature Feature
	// Status indicates the outcome: "disabled", "already-disabled", "failed", "dependent-failed"
	Status string
	// DependentsDisabled contains the nested results for dependent features that were
	// disabled during the cascade, in the order they were disabled.
	DependentsDisabled []DisableFeatureResult
	// Err contains any error that occurred during the disable operation.
	// If non-nil, the operation failed and Feature may not be disabled.
	Err error
}

// Status constants for DisableFeatureResult
const (
	DisableStatusDisabled        = "disabled"
	DisableStatusAlreadyDisabled = "already-disabled"
	DisableStatusFailed          = "failed"
	DisableStatusDependentFailed = "dependent-failed"
)

// DisableFeature disables a feature and its dependents.
// It follows the dependent cascade: if other features depend on this feature,
// those dependents are disabled first, recursively.
//
// The dependency graph (explicit, compile-time verified):
//   - Analytics: required by RemoteManagement
//   - Content: required by RemoteManagement
//   - RemoteManagement: no dependents
//
// If any dependent fails to disable, the operation stops immediately and returns
// Status="dependent-failed" with partial progress visible in DependentsDisabled.
//
// The operation is convergent: dependents are always verified and disabled even
// if the target feature is already disabled, repairing inconsistent states where
// the target is stopped but its dependents are still running. If the target is
// already disabled and no dependents need teardown, it returns
// Status="already-disabled" and no error.
func DisableFeature(opts FeatureOperationOptions) DisableFeatureResult {
	result := DisableFeatureResult{
		Feature:            opts.Feature,
		Status:             DisableStatusFailed,
		DependentsDisabled: []DisableFeatureResult{},
		Err:                nil,
	}

	// Check if already disabled
	status := FeatureStatus(opts)
	if status.Err != nil {
		result.Err = fmt.Errorf("checking feature status: %w", status.Err)
		result.Status = DisableStatusFailed
		return result
	}
	targetAlreadyDisabled := !status.Enabled

	// Disable dependents first (explicit dependency graph reversed).
	// This runs even when the target is already disabled, so that
	// re-running disable converges to a consistent state when
	// dependents are still running (e.g. after a partial disconnect).
	switch opts.Feature {
	case Analytics, Content:
		// Analytics and Content are required by RemoteManagement
		// Check if RemoteManagement is enabled and disable it first
		rmStatus := FeatureStatus(FeatureOperationOptions{Feature: RemoteManagement})
		if rmStatus.Err != nil {
			result.Err = fmt.Errorf("checking dependent %s status: %w", RemoteManagement, rmStatus.Err)
			result.Status = DisableStatusFailed
			return result
		}
		if rmStatus.Enabled {
			rmResult := DisableFeature(FeatureOperationOptions{Feature: RemoteManagement})
			result.DependentsDisabled = append(result.DependentsDisabled, rmResult)
			if rmResult.Err != nil {
				result.Err = fmt.Errorf("disabling dependent %s: %w", RemoteManagement, rmResult.Err)
				result.Status = DisableStatusDependentFailed
				return result
			}
		}
	case RemoteManagement:
		// No dependents
	default:
		result.Err = fmt.Errorf("unknown feature: %s", opts.Feature)
		result.Status = DisableStatusFailed
		return result
	}

	if targetAlreadyDisabled {
		result.Status = DisableStatusAlreadyDisabled
		return result
	}

	// Disable the target feature
	var err error
	switch opts.Feature {
	case Analytics:
		err = datacollection.UnregisterInsightsClient()
	case Content:
		var client *subman.RHSMClient
		client, err = subman.NewRHSMClient()
		if err == nil {
			err = client.SetContentManagement(false)
		}
	case RemoteManagement:
		err = remotemanagement.DeactivateServices()
	default:
		err = fmt.Errorf("unknown feature: %s", opts.Feature)
	}

	if err != nil {
		result.Err = fmt.Errorf("disabling feature %s: %w", opts.Feature, err)
		result.Status = DisableStatusFailed
		return result
	}

	result.Status = DisableStatusDisabled
	return result
}
