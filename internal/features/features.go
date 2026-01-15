package features

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/redhatinsights/rhc/internal/conf"
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
	Requires:    []*RhcFeature{&ContentFeature},
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

// GetFeaturesFromFiles loads features from a drop-in configuration file.
// TODO: When drop-in configuration is fully supported, remove or update this method 
// to support loading features from multiple drop-in files.
func GetFeaturesFromFiles(featuresFilePath string) (*conf.Features, error) {
	if _, err := os.Stat(featuresFilePath); err != nil {
		slog.Debug(fmt.Sprintf("features config file not found: \"%s\"", featuresFilePath))
		return nil, nil
	}

	var tempConf conf.Conf
	if _, err := toml.DecodeFile(featuresFilePath, &tempConf); err != nil {
		return nil, fmt.Errorf("failed to parse features from drop-in config file: \"%s\": %w", featuresFilePath, err)
	}
	
	return &tempConf.Features, nil
}

// ConsolidateSelectedFeatures gathers the features values from the drop-in 
// configuration file and CLI flags to resolve dependencies between features.
// CLI flags always take precedence over config file values.
func ConsolidateSelectedFeatures(config *conf.Conf, enabledFeaturesIDs *[]string, disabledFeaturesIDs *[]string) error {
	if config == nil {
		return fmt.Errorf("failed to consolidate selected features: config is nil")
	}
	
	enabledFeatureSet := map[string]bool{}
	disabledFeatureSet := map[string]bool{}

	// First, load features from config file
	if config.Features.Content != nil {
		if *config.Features.Content {
			enabledFeatureSet[ContentFeature.ID] = true
		} else {
			disabledFeatureSet[ContentFeature.ID] = true
		}
	}
	if config.Features.Analytics != nil {
		if *config.Features.Analytics {
			enabledFeatureSet[AnalyticsFeature.ID] = true
		} else {
			disabledFeatureSet[AnalyticsFeature.ID] = true
		}
	}
	if config.Features.Management != nil {
		if *config.Features.Management {
			enabledFeatureSet[ManagementFeature.ID] = true
		} else {
			disabledFeatureSet[ManagementFeature.ID] = true
		}
	}

	// Then, if a feature is enabled from CLI flags, remove it from disabled set 
	// and add it to enabled set. This is because the feature is explicitly enabled 
	// in CLI flags, overriding the config file value. Similarly, the opposite 
	// is done for disabled features from CLI flags.
	for _, feature := range *enabledFeaturesIDs {
		delete(disabledFeatureSet, feature)
		enabledFeatureSet[feature] = true
	}
	for _, feature := range *disabledFeaturesIDs {
		delete(enabledFeatureSet, feature)
		disabledFeatureSet[feature] = true
	}

	// Create a consolidated list of enabled and disabled features from the
	// sets of config and CLI flags. At this point, we don't know if the combination
	// of enabled and disabled features is valid or not, so we need to check the validity 
	// in the ValidateSelectedFeatures function.
	*enabledFeaturesIDs = []string{}
	*disabledFeaturesIDs = []string{}
	for feature := range enabledFeatureSet {
		*enabledFeaturesIDs = append(*enabledFeaturesIDs, feature)
	}
	for feature := range disabledFeatureSet {
		*disabledFeaturesIDs = append(*disabledFeaturesIDs, feature)
	}

	return nil
}

// ValidateSelectedFeatures checks the validity of selected enabled and disabled features and handles
// the dependency resolution between features.
func ValidateSelectedFeatures(enabledFeaturesIDs *[]string, disabledFeaturesIDs *[]string) error {
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
