package operations

import (
	"fmt"

	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/subman"
)

// FeatureStatusResult represents the result of checking a feature's status.
type FeatureStatusResult struct {
	Enabled bool
	Err     error
}

// FeatureStatus checks the enabled status of a feature by calling the
// appropriate internal/* infrastructure function based on the feature type.
// This function provides the required layer boundary between cmd/ and internal/*
// as cmd/ cannot import internal/* directly per ADR-011 layering rules.
func FeatureStatus(opts FeatureOperationOptions) FeatureStatusResult {
	var enabled bool
	var err error

	switch opts.Feature {
	case Analytics:
		enabled, err = datacollection.InsightsClientIsRegistered()
	case Content:
		var client *subman.RHSMClient
		client, err = subman.NewRHSMClient()
		if err == nil {
			enabled, err = client.IsContentManagementEnabled()
		}
	case RemoteManagement:
		enabled, err = remotemanagement.AssertYggdrasilServiceState("active")
	default:
		err = fmt.Errorf("unknown feature: %s", opts.Feature)
	}

	return FeatureStatusResult{Enabled: enabled, Err: err}
}
