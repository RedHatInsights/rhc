package operations

import "fmt"

// Feature represents a system feature that can be enabled or disabled.
type Feature int

const (
	// Analytics represents Red Hat Lightspeed data collection.
	Analytics Feature = iota
	// Content represents Red Hat content management.
	Content
	// RemoteManagement represents Red Hat Lightspeed remote management.
	RemoteManagement
)

// FeatureOperationOptions contains options for feature operations.
// Currently holds only the Feature identifier, but can be extended
// in the future to include additional operation-specific parameters.
type FeatureOperationOptions struct {
	Feature Feature
}

// String returns the string representation of the feature.
// The returned string matches the feature ID used in commands and configuration.
func (f Feature) String() string {
	switch f {
	case Analytics:
		return "analytics"
	case Content:
		return "content"
	case RemoteManagement:
		return "remote-management"
	default:
		return fmt.Sprintf("unknown(%d)", int(f))
	}
}

// ParseFeature converts a string feature name to a Feature enum value.
// Returns an error if the string does not match any known feature.
func ParseFeature(s string) (Feature, error) {
	switch s {
	case "analytics":
		return Analytics, nil
	case "content":
		return Content, nil
	case "remote-management":
		return RemoteManagement, nil
	default:
		return 0, fmt.Errorf("unknown feature: %s", s)
	}
}
