package features

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/redhatinsights/rhc/internal/conf"
	"github.com/redhatinsights/rhc/internal/datacollection"
	"github.com/redhatinsights/rhc/internal/remotemanagement"
	"github.com/redhatinsights/rhc/internal/rhsm"
	"github.com/urfave/cli/v2"
)

const RhcConnectFeaturesPreferencesPath = "/var/lib/rhc/rhc-connect-features-prefs.json"

// RhcFeature manages optional features of rhc.
type RhcFeature struct {
	// ID is an identifier of the feature.
	ID string
	// Description is human-readable description of the feature.
	Description string
	// WantEnabled
	WantEnabled bool
	// IsEnabledFunc is a callback function, and it returns true if the feature is enabled.
	IsEnabledFunc func() bool
	// IsEnabledInConf is a callback function, and it returns true if the feature is enabled in configuration.
	IsEnabledInConf func() *bool
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

// MapKnownFeatureIds is helper function, and it returns the map of IDs of known feature to the feature itself
func MapKnownFeatureIds() map[string]*RhcFeature {
	featureMap := map[string]*RhcFeature{}
	for _, feature := range KnownFeatures {
		featureMap[feature.ID] = feature
	}
	return featureMap
}

// ContentFeature allows enabling/disabling of content provided by Red Hat. It is
// typically a set of RPM repositories generated in /etc/yum.repos.d/redhat.repo
var ContentFeature = RhcFeature{
	ID:          "content",
	Requires:    []*RhcFeature{},
	WantEnabled: func() bool { return true }(),
	IsEnabledFunc: func() bool {
		slog.Debug("Checking if 'content' feature is enabled")
		if rhsm.IsRegistered() {
			contentEnabled, err := rhsm.IsContentManagementEnabled()
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to check if 'content' feature is enabled: %v", err))
				return false
			}
			return contentEnabled
		} else {
			// When the system is not registered, then return only preference from configuration
			return *conf.ConnectFeaturesPreferences.Content
		}
	},
	IsEnabledInConf: func() *bool {
		return conf.ConnectFeaturesPreferences.Content
	},
	Description: "Get access to RHEL content",
	EnableFunc: func(ctx *cli.Context) error {
		if rhsm.IsRegistered() {
			err := rhsm.SetContentManagement(true)
			if err != nil {
				return fmt.Errorf("failed to enable content management: %w", err)
			}
			slog.Debug("Content management enabled in rhsm.conf")
		} else {
			enabled := true
			conf.ConnectFeaturesPreferences.Content = &enabled
		}
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		if rhsm.IsRegistered() {
			err := rhsm.SetContentManagement(false)
			if err != nil {
				return fmt.Errorf("failed to disable content management: %w", err)
			}
			slog.Debug("Content management disabled in rhsm.conf")
		} else {
			disabled := false
			conf.ConnectFeaturesPreferences.Content = &disabled
		}
		return nil
	},
}

// AnalyticsFeature allows enabling/disabling of data collection for Red Hat Lightspeed
var AnalyticsFeature = RhcFeature{
	ID:          "analytics",
	Requires:    []*RhcFeature{&ContentFeature},
	WantEnabled: func() bool { return true }(),
	IsEnabledFunc: func() bool {
		slog.Debug("Checking if 'analytics' feature is enabled")
		if rhsm.IsRegistered() {
			analyticsEnabled, err := datacollection.InsightsClientIsRegistered()
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to check if 'analytics' feature is enabled: %v", err))
				return false
			}
			return analyticsEnabled
		} else {
			// When the system is not registered, then return only preference from configuration
			return *conf.ConnectFeaturesPreferences.Analytics
		}
	},
	IsEnabledInConf: func() *bool {
		return conf.ConnectFeaturesPreferences.Analytics
	},
	Description: "Enable data collection for Red Hat Lightspeed (formerly Insights)",
	EnableFunc: func(ctx *cli.Context) error {
		slog.Debug("Enabling 'analytics' feature...")
		if rhsm.IsRegistered() {
			err := datacollection.RegisterInsightsClient()
			if err != nil {
				return fmt.Errorf("failed to enable analytics: %w", err)
			}
			slog.Debug("The insights-client was registered successfully")
		} else {
			enabled := true
			conf.ConnectFeaturesPreferences.Analytics = &enabled
		}
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		slog.Debug("Disabling 'analytics' feature...")
		if rhsm.IsRegistered() {
			err := datacollection.UnregisterInsightsClient()
			if err != nil {
				return fmt.Errorf("failed to disable analytics: %w", err)
			}
			slog.Debug("The insights-client was unregistered successfully")
		} else {
			disabled := false
			conf.ConnectFeaturesPreferences.Analytics = &disabled
		}
		return nil
	},
}

// ManagementFeature allows to enable/disable remote management of the host
// using yggdrasil service and various workers
var ManagementFeature = RhcFeature{
	ID:          "remote-management",
	Requires:    []*RhcFeature{&ContentFeature, &AnalyticsFeature},
	WantEnabled: func() bool { return true }(),
	IsEnabledFunc: func() bool {
		slog.Debug("Checking if 'remote-management' feature is enabled")
		if rhsm.IsRegistered() {
			analyticsEnabled, err := remotemanagement.AssertYggdrasilServiceState("active")
			if err != nil {
				slog.Warn(fmt.Sprintf("Failed to check if 'remote-management' feature is enabled: %v", err))
				return false
			}
			return analyticsEnabled
		} else {
			// When the system is not registered, then return only preference from configuration
			return *conf.ConnectFeaturesPreferences.RemoteManagement
		}

	},
	IsEnabledInConf: func() *bool {
		return conf.ConnectFeaturesPreferences.RemoteManagement
	},
	Description: "Remote management",
	EnableFunc: func(ctx *cli.Context) error {
		slog.Debug("enabling 'remote-management' feature...")
		if rhsm.IsRegistered() {
			err := remotemanagement.ActivateServices()
			if err != nil {
				return fmt.Errorf("failed to enable remote-management: %w", err)
			}
			slog.Debug("The yggdrasil.service was activated successfully")
		} else {
			enabled := true
			conf.ConnectFeaturesPreferences.RemoteManagement = &enabled
		}
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		slog.Debug("Disabling 'remote-management' feature...")
		if rhsm.IsRegistered() {
			err := remotemanagement.DeactivateServices()
			if err != nil {
				return fmt.Errorf("failed to disable remote-management: %w", err)
			}
			slog.Debug("The yggdrasil.service was deactivated successfully")
		} else {
			disabled := false
			conf.ConnectFeaturesPreferences.RemoteManagement = &disabled
		}
		return nil
	},
}

// SaveFeaturePreferencesToFile saves features preferences to the "preference" file.
// It is typically /var/lib/rhc/rhc-connect-features-prefs.json
func SaveFeaturePreferencesToFile(
	featuresFilePath string,
	featPrefs conf.ConnectFeaturesPrefs,
) error {
	dirPath := filepath.Dir(featuresFilePath)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		slog.Debug(fmt.Sprintf("Creating directory %s", dirPath))
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}
	}

	content, err := json.MarshalIndent(featPrefs, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal features preferences: %w", err)
	}
	err = os.WriteFile(featuresFilePath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write features preferences to file: %w", err)
	}
	return nil
}

func DeleteFeaturePreferencesFromFile(featuresFilePath string) error {
	return os.Remove(featuresFilePath)
}

// GetFeaturesFromFile loads features from the "preference" file.
// It is typically /var/lib/rhc/rhc-connect-features-prefs.json
func GetFeaturesFromFile(featuresFilePath string) (*conf.ConnectFeaturesPrefs, error) {
	if _, err := os.Stat(featuresFilePath); err != nil {
		if os.IsNotExist(err) {
			slog.Info(fmt.Sprintf("features config file not found: '%s'", featuresFilePath))
			return nil, nil
		}
		return nil, err
	}

	featuresPrefContent, err := os.ReadFile(featuresFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read features from file: '%s': %w",
			featuresFilePath, err)
	}

	var featPrefs conf.ConnectFeaturesPrefs
	err = json.Unmarshal(featuresPrefContent, &featPrefs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse features from file: '%s': %w",
			featuresFilePath, err)
	}

	slog.Debug(fmt.Sprintf("Loaded features from preference file: %v", featuresFilePath))
	return &featPrefs, nil
}

// ConsolidateSelectedFeatures gathers the features values from the drop-in
// configuration file and CLI flags to resolve dependencies between features.
// CLI flags always take precedence over config file values.
func ConsolidateSelectedFeatures(
	connectFeatPrefs *conf.ConnectFeaturesPrefs,
	enabledFeaturesIDs []string,
	disabledFeaturesIDs []string,
) (enabledFeatures []string, disabledFeatures []string, err error) {
	if connectFeatPrefs == nil {
		return nil, nil, fmt.Errorf("failed to consolidate selected features: config is nil")
	}

	featureStates := map[string]bool{}

	// First, load features from config file
	if connectFeatPrefs.Content != nil {
		if *connectFeatPrefs.Content {
			featureStates[ContentFeature.ID] = true
		} else {
			featureStates[ContentFeature.ID] = false
		}
	} else {
		featureStates[ContentFeature.ID] = true
	}
	if connectFeatPrefs.Analytics != nil {
		if *connectFeatPrefs.Analytics {
			featureStates[AnalyticsFeature.ID] = true
		} else {
			featureStates[AnalyticsFeature.ID] = false
		}
	} else {
		featureStates[AnalyticsFeature.ID] = true
	}
	if connectFeatPrefs.RemoteManagement != nil {
		if *connectFeatPrefs.RemoteManagement {
			featureStates[ManagementFeature.ID] = true
		} else {
			featureStates[ManagementFeature.ID] = false
		}
	} else {
		featureStates[ManagementFeature.ID] = true
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
	// First, check disabled features: check only the correctness of IDs
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
		disabledFeature.WantEnabled = false
	}

	// Then check enabled features, and it is more tricky because:
	// 1) you cannot enable a feature, which was already disabled
	// 2) you cannot enable a feature, which depends on the disabled feature
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
		enabledFeature.WantEnabled = true
	}

	for _, feature := range KnownFeatures {
		for _, requiredFeature := range feature.Requires {
			if !requiredFeature.WantEnabled {
				feature.WantEnabled = false
				feature.Reason = fmt.Sprintf("required feature \"%s\" is disabled", requiredFeature.ID)
			}
		}
	}

	return nil
}
