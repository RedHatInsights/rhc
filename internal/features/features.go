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

// AnalyticsFeature allows to enable/disable collecting data for Red Hat Lightspeed
var AnalyticsFeature = RhcFeature{
	ID:          "analytics",
	Requires:    []*RhcFeature{&ContentFeature},
	Enabled:     func() bool { return true }(),
	Description: "Enable data collection for Red Hat Lightspeed (formerly Insights)",
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

// GetFeaturesFromFile loads features from a drop-in configuration file.
// TODO: When drop-in configuration is fully supported, remove or update this method
// to support loading features from multiple drop-in files.
func GetFeaturesFromFile(featuresFilePath string) (*conf.Features, error) {
	if _, err := os.Stat(featuresFilePath); err != nil {
		if os.IsNotExist(err) {
			slog.Debug(fmt.Sprintf("features config file not found: \"%s\"", featuresFilePath))
			return nil, nil
		}
		return nil, err
	}

	var tempConf conf.Conf
	configMetadata, err := toml.DecodeFile(featuresFilePath, &tempConf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse features from drop-in config file: \"%s\": %w", featuresFilePath, err)
	}

	// Check for invalid features in the config file
	// to alert the user of typos like "remote-managemennt"
	// instead of "remote-management"
	invalidFeatures := getUndecodedConfigKeys(configMetadata)
	if len(invalidFeatures) > 0 {
		slog.Warn(fmt.Sprintf("ignoring unknown feature(s) found in drop-in config file: %s", strings.Join(invalidFeatures, ", ")))
	}

	return &tempConf.Features, nil
}

// getUndecodedConfigKeys returns a list of config keys from the input config metadata
// that were not decoded into the destination value during the toml.DecodeFile call.
// This is useful to find typos and invalid feature keys in the config file.
func getUndecodedConfigKeys(configMetadata toml.MetaData) []string {
	configKeys := configMetadata.Undecoded()
	if len(configKeys) == 0 {
		return nil
	}

	var invalidFeatures []string
	for _, feature := range configKeys {
		featureKey := feature.String()
		invalidFeatures = append(invalidFeatures, featureKey)
	}

	return invalidFeatures
}

// ConsolidateSelectedFeatures gathers the features values from the drop-in
// configuration file and CLI flags to resolve dependencies between features.
// CLI flags always take precedence over config file values.
func ConsolidateSelectedFeatures(config *conf.Conf, enabledFeaturesIDs []string, disabledFeaturesIDs []string) (enabledFeatures []string, disabledFeatures []string, err error) {
	if config == nil {
		return nil, nil, fmt.Errorf("failed to consolidate selected features: config is nil")
	}

	featureStates := map[string]bool{}

	// First, load features from config file
	if config.Features.Content != nil {
		if *config.Features.Content {
			featureStates[ContentFeature.ID] = true
		} else {
			featureStates[ContentFeature.ID] = false
		}
	}
	if config.Features.Analytics != nil {
		if *config.Features.Analytics {
			featureStates[AnalyticsFeature.ID] = true
		} else {
			featureStates[AnalyticsFeature.ID] = false
		}
	}
	if config.Features.Management != nil {
		if *config.Features.Management {
			featureStates[ManagementFeature.ID] = true
		} else {
			featureStates[ManagementFeature.ID] = false
		}
	}

	// Then, if a feature is enabled from CLI flags, enable it in the featureStates map.
	// This is because the feature is explicitly enabled in CLI flags,
	// overriding the config file value. Similarly, the opposite
	// is done for disabled features from CLI flags.
	for _, feature := range enabledFeaturesIDs {
		featureStates[feature] = true
	}
	for _, feature := range disabledFeaturesIDs {
		featureStates[feature] = false
	}

	// Create a consolidated list of enabled and disabled features from the
	// map of config and CLI flags. At this point, we don't know if the combination
	// of enabled and disabled features is valid or not, so we need to check the validity
	// in the ValidateSelectedFeatures function.
	for feature, enabled := range featureStates {
		if enabled {
			enabledFeatures = append(enabledFeatures, feature)
		} else {
			disabledFeatures = append(disabledFeatures, feature)
		}
	}

	return enabledFeatures, disabledFeatures, nil
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
