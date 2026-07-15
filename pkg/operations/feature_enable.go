package operations

import (
	"fmt"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

// EnableFeatureResult represents the result of enabling a feature.
// It includes the target feature, status of the operation, nested results
// for dependencies, and any error that occurred.
type EnableFeatureResult struct {
	// Feature is the target feature that was requested to be enabled.
	Feature Feature
	// Status indicates the outcome: "enabled", "already-enabled", "failed", "dependency-failed"
	Status string
	// DependenciesEnabled contains the nested results for dependency features that were
	// processed during the cascade, in the order they were processed.
	DependenciesEnabled []EnableFeatureResult
	// Err contains any error that occurred during the enable operation.
	// If non-nil, the operation failed and Feature may not be enabled.
	Err error
}

// Status constants for EnableFeatureResult
const (
	EnableStatusEnabled          = "enabled"
	EnableStatusAlreadyEnabled   = "already-enabled"
	EnableStatusFailed           = "failed"
	EnableStatusDependencyFailed = "dependency-failed"
)

// EnableFeature enables a feature and its dependencies.
// It follows the dependency cascade: if a feature requires other features,
// those dependencies are enabled first, recursively.
//
// The dependency graph (explicit, compile-time verified):
//   - Analytics: no dependencies
//   - Content: no dependencies
//   - RemoteManagement: requires Content and Analytics
//
// If any dependency fails to enable, the operation stops immediately and returns
// Status="dependency-failed" with partial progress visible in DependenciesEnabled.
//
// The operation is idempotent: if the feature is already enabled, it returns
// immediately with Status="already-enabled" and no error.
func EnableFeature(opts FeatureOperationOptions) EnableFeatureResult {
	result := EnableFeatureResult{
		Feature:             opts.Feature,
		Status:              EnableStatusFailed,
		DependenciesEnabled: []EnableFeatureResult{},
		Err:                 nil,
	}

	// Check if already enabled
	status := FeatureStatus(opts)
	if status.Err != nil {
		result.Err = fmt.Errorf("checking feature status: %w", status.Err)
		result.Status = EnableStatusFailed
		return result
	}
	if status.Enabled {
		result.Status = EnableStatusAlreadyEnabled
		return result
	}

	// Enable dependencies first (explicit dependency graph)
	switch opts.Feature {
	case Analytics, Content:
		// No dependencies
	case RemoteManagement:
		// RemoteManagement requires Content and Analytics
		contentResult := EnableFeature(FeatureOperationOptions{Feature: Content})
		result.DependenciesEnabled = append(result.DependenciesEnabled, contentResult)
		if contentResult.Err != nil {
			result.Err = fmt.Errorf("enabling dependency %s: %w", Content, contentResult.Err)
			result.Status = EnableStatusDependencyFailed
			return result
		}

		analyticsResult := EnableFeature(FeatureOperationOptions{Feature: Analytics})
		result.DependenciesEnabled = append(result.DependenciesEnabled, analyticsResult)
		if analyticsResult.Err != nil {
			result.Err = fmt.Errorf("enabling dependency %s: %w", Analytics, analyticsResult.Err)
			result.Status = EnableStatusDependencyFailed
			return result
		}
	default:
		result.Err = fmt.Errorf("unknown feature: %s", opts.Feature)
		result.Status = EnableStatusFailed
		return result
	}

	// Enable the target feature
	var err error
	switch opts.Feature {
	case Analytics:
		err = datacollection.RegisterInsightsClient()
	case Content:
		var client *subman.RHSMClient
		client, err = subman.NewRHSMClient()
		if err == nil {
			err = client.SetContentManagement(true)
		}
	case RemoteManagement:
		err = remotemanagement.ActivateServices()
	default:
		err = fmt.Errorf("unknown feature: %s", opts.Feature)
	}

	if err != nil {
		result.Err = fmt.Errorf("enabling feature %s: %w", opts.Feature, err)
		result.Status = EnableStatusFailed
		return result
	}

	result.Status = EnableStatusEnabled
	return result
}
