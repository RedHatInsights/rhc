package main

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/urfave/cli/v2"
)

// RhcFeature manages optional features of rhc.
type RhcFeature struct {
	// ID is an identifier of the feature.
	ID string
	// Description is human-readable description of the feature.
	Description string
	// Enabled represents the state of feature
	Enabled bool
	// Reason for disabling feature
	Reason string
	// Requires is a list of IDs of other features that are required for this feature. RhcFeature
	// dependencies are not resolved.
	Requires []*RhcFeature
	// EnableFunc is callback function, and it is called when the feature should transition
	// into enabled state.
	EnableFunc func(ctx *cli.Context) error
	// DisableFunc is also callback function, and it is called when the feature should transition
	// into disabled state.
	DisableFunc func(ctx *cli.Context) error
}

func (f *RhcFeature) String() string {
	return fmt.Sprintf("Feature{ID:%s}", f.ID)
}

// KnownFeatures is a sorted list of features, ordered from least to the most dependent.
var KnownFeatures = []*RhcFeature{
	&ContentFeature,
	&AnalyticsFeature,
	&ManagementFeature,
}

// listKnownFeatureIds is helper function, and it returns the list of IDs of known feature
func listKnownFeatureIds() []string {
	var ids []string
	for _, feature := range KnownFeatures {
		ids = append(ids, feature.ID)
	}
	return ids
}

// ContentFeature allows to enable/disable content provided by Red Hat. It is
// typically set of RPM repositories generated in /etc/yum.repos.d/redhat.repo
var ContentFeature = RhcFeature{
	ID:          "content",
	Requires:    []*RhcFeature{},
	Enabled:     func() bool { return true }(),
	Description: "Get access to RHEL content",
	EnableFunc: func(ctx *cli.Context) error {
		slog.Debug("enabling 'content' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		slog.Debug("disabling 'content' feature not implemented")
		return nil
	},
}

// AnalyticsFeature allows to enable/disable collecting data for Red Hat Insights
var AnalyticsFeature = RhcFeature{
	ID:          "analytics",
	Requires:    []*RhcFeature{},
	Enabled:     func() bool { return true }(),
	Description: "Enable data collection for Red Hat Insights",
	EnableFunc: func(ctx *cli.Context) error {
		slog.Debug("enabling 'analytics' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		slog.Debug("disabling 'analytics' feature not implemented")
		return nil
	},
}

// ManagementFeature allows to enable/disable remote management of the host
// using yggdrasil service and various workers
var ManagementFeature = RhcFeature{
	ID:          "remote-management",
	Requires:    []*RhcFeature{&ContentFeature, &AnalyticsFeature},
	Enabled:     func() bool { return true }(),
	Description: "Remote management",
	EnableFunc: func(ctx *cli.Context) error {
		slog.Debug("enabling 'management' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		slog.Debug("disabling 'management' feature not implemented")
		return nil
	},
}

// checkFeatureInput checks input of enabled and disabled features
func checkFeatureInput(enabledFeaturesIDs *[]string, disabledFeaturesIDs *[]string) error {
	// First check disabled features: check only correctness of IDs
	for _, featureId := range *disabledFeaturesIDs {
		isKnown := false
		var disabledFeature *RhcFeature = nil
		for _, rhcFeature := range KnownFeatures {
			if featureId == rhcFeature.ID {
				disabledFeature = rhcFeature
				isKnown = true
				break
			}
		}
		if !isKnown {
			supportedIds := listKnownFeatureIds()
			hint := strings.Join(supportedIds, ",")
			return fmt.Errorf("cannot disable feature \"%s\": no such feature exists (%s)", featureId, hint)
		}
		disabledFeature.Enabled = false
	}

	// Then check enabled features, and it is more tricky, because:
	// 1) you cannot enable feature, which was already disabled
	// 2) you cannot enable feature, which depends on disabled feature
	for _, featureId := range *enabledFeaturesIDs {
		isKnown := false
		var enabledFeature *RhcFeature = nil
		for _, rhcFeature := range KnownFeatures {
			if featureId == rhcFeature.ID {
				enabledFeature = rhcFeature
				isKnown = true
				break
			}
		}
		if !isKnown {
			supportedIds := listKnownFeatureIds()
			hint := strings.Join(supportedIds, ",")
			return fmt.Errorf("cannot enable feature \"%s\": no such feature exists (%s)", featureId, hint)
		}
		for _, disabledFeatureId := range *disabledFeaturesIDs {
			if featureId == disabledFeatureId {
				return fmt.Errorf("cannot enable feature: \"%s\": feature \"%s\" explicitly disabled",
					featureId, disabledFeatureId)
			}
			for _, requiredFeature := range enabledFeature.Requires {
				if requiredFeature.ID == disabledFeatureId {
					return fmt.Errorf("cannot enable feature: \"%s\": required feature \"%s\" explicitly disabled",
						enabledFeature.ID, disabledFeatureId)
				}
			}
		}
		enabledFeature.Enabled = true
	}

	for _, feature := range KnownFeatures {
		for _, requiredFeature := range feature.Requires {
			if !requiredFeature.Enabled {
				feature.Enabled = false
				feature.Reason = fmt.Sprintf("required feature \"%s\" is disabled", requiredFeature.ID)
			}
		}
	}

	return nil
}
