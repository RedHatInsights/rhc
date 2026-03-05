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
	"github.com/redhatinsights/rhc/internal/ui"
)

const RhcConnectFeaturesPreferencesPath = "/var/lib/rhc/rhc-connect-features-prefs.json"

// baseFeatureInterface defines the core methods required for managing base functionality of a feature.
type baseFeatureInterface interface {
	// ID returns the unique identifier of the feature.
	// Implemented in baseFeature.
	ID() string
	// Name returns the human-readable name of the feature.
	// Implemented in baseFeature.
	Name() string
	// Description returns the human-readable description of the feature.
	// Implemented in baseFeature.
	Description() string
	// SetWantEnabled sets the desired state of the feature to enabled or disabled based on the provided boolean value.
	// Implemented in baseFeature.
	SetWantEnabled(bool)
	// WantEnabled returns the desired state of the feature.
	// Implemented in baseFeature.
	WantEnabled() bool
	// SetReason sets the reason for enabling or disabling the feature.
	// Implemented in baseFeature.
	SetReason(string)
	// Reason returns the reason for enabling or disabling the feature.
	// Implemented in baseFeature.
	Reason() string
	// Requires returns the slice of features that this feature depends on.
	// Implemented in baseFeature.
	Requires() []RhcFeature
	// RequiredBy returns the slice of features that require this feature.
	RequiredBy() []RhcFeature
	// IsEnabled returns true if the feature is enabled, false otherwise.
	IsEnabled() bool
	// Enable enables the feature.
	Enable(featureResults *FeaturesResults) error
	// Disable disables the feature.
	Disable(featureResults *FeaturesResults) error
}

// setterFeatureInterface is the interface for enabling and disabling features.
// This interface should not expose any public method.
type setterFeatureInterface interface {
	// isEnabled checks if the feature is enabled. Specific for each feature.
	isEnabled() (bool, error)
	// enable enables the feature. Specific for each feature.
	enableFeature(featureResults *FeaturesResults) error
	// disable disables the feature. Specific for each feature.
	disableFeature(featureResults *FeaturesResults) error
}

// RhcFeature is the interface for optional features of rhc.
type RhcFeature interface {
	baseFeatureInterface
	setterFeatureInterface
}

// baseFeature implements the baseFeatureInterface, which is the same for all features.
type baseFeature struct {
	// id is an identifier of the feature.
	id string
	// name is the human-readable name of the feature.
	name string
	// description is the human-readable description of the feature.
	description string
	// wantEnabled contains information whether the user wants the feature enabled or not.
	wantEnabled bool
	// reason for disabling/enabling the feature
	reason string
	// self is a pointer to the feature itself, used for dependency resolution.
	self RhcFeature
	// requires is a slice of IDs of other features that are required for this feature.
	requires []RhcFeature
	// requiredBy is a slice of IDs of other features that require this feature.
	requiredBy []RhcFeature
}

func (feature *baseFeature) SetReason(reason string) {
	feature.reason = reason
}

func (feature *baseFeature) ID() string {
	return feature.id
}

func (feature *baseFeature) Name() string {
	return feature.name
}

func (feature *baseFeature) Description() string {
	return feature.description
}

func (feature *baseFeature) SetWantEnabled(wanted bool) {
	feature.wantEnabled = wanted
}

func (feature *baseFeature) WantEnabled() bool {
	return feature.wantEnabled
}

func (feature *baseFeature) Reason() string {
	return feature.reason
}

func (feature *baseFeature) Requires() []RhcFeature {
	return feature.requires
}

func (feature *baseFeature) RequiredBy() []RhcFeature {
	return feature.requiredBy
}

// confPref returns the feature preference from the configuration file, which
// is loaded during start of the program.
func (feature *baseFeature) confPref() *bool {
	switch feature.id {
	case ContentFeatureID:
		return conf.ConnectFeaturesPreferences.Content
	case AnalyticsFeatureID:
		return conf.ConnectFeaturesPreferences.Analytics
	case ManagementFeatureID:
		return conf.ConnectFeaturesPreferences.RemoteManagement
	default:
		return nil
	}
}

// setConfPref set the feature preference in the configuration structure. The preference
// itself is written to file, when SaveFeaturePreferencesToFile() is called. This function
// also prints the message to the console, which is used for the user to see, that the
// preference was set.
func (feature *baseFeature) setConfPref(enabled bool) {
	if feature.confPref() != nil {
		*feature.confPref() = enabled
	}
	reason := feature.Reason()
	if enabled {
		if reason != "" {
			ui.Printf("%s[%s] %s ... Enabled in preference (%s)\n",
				ui.Indent.Medium, ui.Icons.Ok, feature.name, reason)
		} else {
			ui.Printf("%s[%s] %s ... Enabled in preference\n",
				ui.Indent.Medium, ui.Icons.Ok, feature.name)
		}
	} else {
		if reason != "" {
			ui.Printf("%s[ ] %s ... Disabled in preference (%s)\n",
				ui.Indent.Medium, feature.name, reason)
		} else {
			ui.Printf("%s[ ] %s ... Disabled in preference\n",
				ui.Indent.Medium, feature.name)
		}
	}
}

// IsEnabled returns true if the feature is enabled, false otherwise.
func (feature *baseFeature) IsEnabled() bool {
	slog.Debug(fmt.Sprintf("Checking if '%s' feature is enabled", feature.ID()))
	if rhsm.IsRegistered() {
		enabled, err := feature.self.isEnabled()
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to check if '%s' feature is enabled: %v", feature.ID(), err))
			return false
		}
		return enabled
	}
	// When the system is not registered, then return only preference from configuration
	return feature.confPref() != nil && *feature.confPref()
}

// Enable enables the feature. It first checks if all required features are enabled,
// and then enables the feature itself.
func (feature *baseFeature) Enable(featureResults *FeaturesResults) error {
	slog.Debug(fmt.Sprintf("Enabling '%s' feature", feature.ID()))
	// First, try to enable all required features. If any of them fails, then disable the feature.
	for _, reqFeature := range feature.Requires() {
		if !reqFeature.IsEnabled() {
			reqFeature.SetReason(fmt.Sprintf("required by '%s'", feature.ID()))
			err := reqFeature.Enable(featureResults)
			if err != nil {
				return fmt.Errorf("failed to enable required feature '%s': %w", reqFeature.ID(), err)
			}
			slog.Debug(fmt.Sprintf("The required feature '%s' enabled", reqFeature.ID()))
		}
	}
	// Then enable the feature itself
	if rhsm.IsRegistered() {
		err := feature.self.enableFeature(featureResults)
		if err != nil {
			return fmt.Errorf("failed to enable '%s': %w", feature.ID(), err)
		}
		slog.Debug(fmt.Sprintf("The '%s' enabled", feature.ID()))
		return nil
	}
	// When the system is not registered, then only set the preference in the configuration file
	feature.setConfPref(true)
	return nil
}

// Disable disables the feature. It first checks if all required features are disabled,
// and then disables the feature itself.
func (feature *baseFeature) Disable(featureResults *FeaturesResults) error {
	slog.Debug(fmt.Sprintf("Disabling '%s' feature", feature.ID()))
	// First, try to disable all required by features.
	for _, reqFeature := range feature.RequiredBy() {
		if reqFeature.IsEnabled() {
			reqFeature.SetReason(fmt.Sprintf("requires '%s'", feature.ID()))
			err := reqFeature.Disable(featureResults)
			if err != nil {
				return fmt.Errorf("failed to disable required by feature '%s': %w", reqFeature.ID(), err)
			}
			slog.Debug(fmt.Sprintf("The required by feature '%s' disabled", reqFeature.ID()))
		}
	}
	// Then disable the feature itself
	if rhsm.IsRegistered() {
		err := feature.self.disableFeature(featureResults)
		if err != nil {
			return fmt.Errorf("failed to disable '%s': %w", feature.ID(), err)
		}
		slog.Debug(fmt.Sprintf("The '%s' disabled", feature.ID()))
		return nil
	}
	// When the system is not registered, then only set the preference in the configuration file
	feature.setConfPref(false)
	return nil
}

// Content

type ContentFeature struct {
	baseFeature
}

func (feature *ContentFeature) isEnabled() (bool, error) {
	return rhsm.IsContentManagementEnabled()
}

func (feature *ContentFeature) enableFeature(featuresResults *FeaturesResults) error {
	featuresResults.TryEnableContent(true)
	if featuresResults.Content.Successful != nil && !*featuresResults.Content.Successful {
		return fmt.Errorf("failed to enable content: %v", featuresResults.Content.Error)
	}
	return nil
}

func (feature *ContentFeature) disableFeature(featureResults *FeaturesResults) error {
	featureResults.TryDisableContent()
	if featureResults.Content.Successful != nil && !*featureResults.Content.Successful {
		return fmt.Errorf("failed to disable content: %v", featureResults.Content.Error)
	}
	return nil
}

// Analytics

type AnalyticsFeature struct {
	baseFeature
}

func (feature *AnalyticsFeature) isEnabled() (bool, error) {
	return datacollection.InsightsClientIsRegistered()
}

func (feature *AnalyticsFeature) enableFeature(featuresResults *FeaturesResults) error {
	reasons := feature.Reason()
	featuresResults.TryRegisterInsightsClient(true, &reasons)
	if featuresResults.Analytics.Successful != nil && !*featuresResults.Analytics.Successful {
		return fmt.Errorf("failed to register insights client: %v", featuresResults.Analytics.Error)
	}
	return nil
}

func (feature *AnalyticsFeature) disableFeature(featuresResults *FeaturesResults) error {
	reasons := feature.Reason()
	featuresResults.TryUnRegisterInsightsClient(&reasons)
	if featuresResults.Analytics.Successful != nil && !*featuresResults.Analytics.Successful {
		return fmt.Errorf("failed to unregister insights client: %v", featuresResults.Analytics.Error)
	}
	return nil
}

// Remote Management

type ManagementFeature struct {
	baseFeature
}

func (feature *ManagementFeature) isEnabled() (bool, error) {
	return remotemanagement.AssertYggdrasilServiceState("active")
}

func (feature *ManagementFeature) enableFeature(featureResults *FeaturesResults) error {
	featureResults.TryActivateServices(true, nil)
	if featureResults.RemoteManagement.Successful != nil && !*featureResults.RemoteManagement.Successful {
		return fmt.Errorf("failed to activate services: %v", featureResults.RemoteManagement.Error)
	}
	return nil
}

func (feature *ManagementFeature) disableFeature(featureResults *FeaturesResults) error {
	reasons := feature.Reason()
	featureResults.TryDeactivateServices(&reasons)
	if featureResults.RemoteManagement.Successful != nil && !*featureResults.RemoteManagement.Successful {
		return fmt.Errorf("failed to deactivate services: %v", featureResults.RemoteManagement.Error)
	}
	return nil
}

const (
	ContentFeatureID    = "content"
	AnalyticsFeatureID  = "analytics"
	ManagementFeatureID = "remote-management"
)

type FeatureManager struct {
	ContentFeature    *ContentFeature
	AnalyticsFeature  *AnalyticsFeature
	ManagementFeature *ManagementFeature
	featureMap        map[string]RhcFeature
	featureList       []RhcFeature
}

// NewFeatureManager is a factory function for the FeatureManager. It creates the feature manager
// and initializes all required fields and dependencies between features.
func NewFeatureManager() *FeatureManager {
	var contentFeature = ContentFeature{
		baseFeature: baseFeature{
			id:          ContentFeatureID,
			name:        "Content",
			requires:    []RhcFeature{},
			wantEnabled: true,
			description: "Access to package repositories",
		},
	}
	contentFeature.self = &contentFeature

	var analyticsFeature = AnalyticsFeature{
		baseFeature: baseFeature{
			id:          AnalyticsFeatureID,
			name:        "Analytics",
			requires:    []RhcFeature{},
			wantEnabled: true,
			description: "Red Hat Lightspeed data collection",
		},
	}
	analyticsFeature.self = &analyticsFeature

	var managementFeature = ManagementFeature{
		baseFeature: baseFeature{
			id:          ManagementFeatureID,
			name:        "Remote Management",
			requires:    []RhcFeature{&contentFeature, &analyticsFeature},
			wantEnabled: true,
			description: "Red Hat Lightspeed remote management",
		},
	}
	managementFeature.self = &managementFeature

	// FIXME: this is a temporary solution. We should implement automatic dependency graph and
	//        fill `requiredBy` field from `requires` of other features
	contentFeature.requiredBy = []RhcFeature{&managementFeature}
	analyticsFeature.requiredBy = []RhcFeature{&managementFeature}
	managementFeature.requiredBy = []RhcFeature{}

	featMap := map[string]RhcFeature{
		ContentFeatureID:    &contentFeature,
		AnalyticsFeatureID:  &analyticsFeature,
		ManagementFeatureID: &managementFeature,
	}
	featList := []RhcFeature{
		&contentFeature,
		&analyticsFeature,
		&managementFeature,
	}

	fm := &FeatureManager{
		ContentFeature:    &contentFeature,
		AnalyticsFeature:  &analyticsFeature,
		ManagementFeature: &managementFeature,
		featureMap:        featMap,
		featureList:       featList,
	}

	return fm
}

func (fm *FeatureManager) List() []RhcFeature {
	return fm.featureList
}

func (fm *FeatureManager) Map() map[string]RhcFeature {
	return fm.featureMap
}

var FeatureMgr = NewFeatureManager()

// ListKnownFeatureIds is the helper function, and it returns the list of IDs of known features.
// It can be used for the case, when you want to display the list of features in the help message
func ListKnownFeatureIds() []string {
	var ids []string
	for _, feature := range FeatureMgr.List() {
		ids = append(ids, feature.ID())
	}
	return ids
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

// DeleteFeaturePreferencesFromFile deletes the features preferences file.
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

// TODO: Refactor ConsolidateSelectedFeatures to use a more efficient algorithm for feature consolidation
//       that would use benefits of the dependency graph.

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
			featureStates[ContentFeatureID] = true
		} else {
			featureStates[ContentFeatureID] = false
		}
	} else {
		featureStates[ContentFeatureID] = true
	}
	if connectFeatPrefs.Analytics != nil {
		if *connectFeatPrefs.Analytics {
			featureStates[AnalyticsFeatureID] = true
		} else {
			featureStates[AnalyticsFeatureID] = false
		}
	} else {
		featureStates[AnalyticsFeatureID] = true
	}
	if connectFeatPrefs.RemoteManagement != nil {
		if *connectFeatPrefs.RemoteManagement {
			featureStates[ManagementFeatureID] = true
		} else {
			featureStates[ManagementFeatureID] = false
		}
	} else {
		featureStates[ManagementFeatureID] = true
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

// TODO: Refactor ValidateSelectedFeatures to use a more efficient algorithm for feature validation
//       that would use benefits of the dependency graph.

// ValidateSelectedFeatures checks the validity of selected enabled and disabled features and handles
// the dependency resolution between features.
func ValidateSelectedFeatures(enabledFeaturesIDs *[]string, disabledFeaturesIDs *[]string) error {
	// First, check disabled features: check only the correctness of IDs
	for _, featureId := range *disabledFeaturesIDs {
		isKnown := false
		for _, rhcFeature := range FeatureMgr.List() {
			if featureId == rhcFeature.ID() {
				rhcFeature.SetWantEnabled(false)
				slog.Debug(fmt.Sprintf("Disabling feature \"%s\"", featureId))
				isKnown = true
				break
			}
		}
		if !isKnown {
			supportedIds := ListKnownFeatureIds()
			hint := strings.Join(supportedIds, ",")
			return fmt.Errorf("cannot disable feature \"%s\": no such feature exists (%s)", featureId, hint)
		}
	}

	// Then check enabled features, and it is more tricky because:
	// 1) you cannot enable a feature, which was already disabled
	// 2) you cannot enable a feature, which depends on the disabled feature
	for _, featureId := range *enabledFeaturesIDs {
		isKnown := false
		var enabledFeature *RhcFeature = nil
		for _, rhcFeature := range FeatureMgr.List() {
			if featureId == rhcFeature.ID() {
				enabledFeature = &rhcFeature
				isKnown = true
				break
			}
		}
		if !isKnown {
			supportedIds := ListKnownFeatureIds()
			hint := strings.Join(supportedIds, ",")
			return fmt.Errorf("cannot enable feature \"%s\": no such feature exists (%s)", featureId, hint)
		}
		for _, disabledFeatureId := range *disabledFeaturesIDs {
			if featureId == disabledFeatureId {
				return fmt.Errorf("cannot enable feature: \"%s\": feature \"%s\" explicitly disabled",
					featureId, disabledFeatureId)
			}
			for _, requiredFeature := range (*enabledFeature).Requires() {
				if requiredFeature.ID() == disabledFeatureId {
					return fmt.Errorf("cannot enable feature: \"%s\": required feature \"%s\" explicitly disabled",
						(*enabledFeature).ID(), disabledFeatureId)
				}
			}
		}
		(*enabledFeature).SetWantEnabled(true)
	}

	for _, feature := range FeatureMgr.List() {
		for _, requiredFeature := range feature.Requires() {
			if !requiredFeature.WantEnabled() {
				feature.SetWantEnabled(false)
				feature.SetReason(fmt.Sprintf("required feature \"%s\" is disabled", requiredFeature.ID()))
			}
		}
	}

	return nil
}
