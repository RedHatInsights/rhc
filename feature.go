package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/subpop/go-log"
	"github.com/urfave/cli/v2"
)

// FeaturesStatePath points to a filesystem location where the state file is stored.
var FeaturesStatePath = "/var/lib/rhc/features.json"

// RequiredFeatures contains a list of features that cannot be disabled.
var RequiredFeatures = []Feature{IdentityFeature}

// DefaultFeatures contains a list of features that are enabled by default.
var DefaultFeatures = []Feature{IdentityFeature, ContentFeature, AnalyticsFeature}

type FeaturesState struct {
	Enabled []string `json:"enabled"`
}

func (s *FeaturesState) IsEnabled(featureID string) bool {
	for _, feature := range s.Enabled {
		if feature == featureID {
			return true
		}
	}
	return false
}

func (s *FeaturesState) IsDefault(featureID string) bool {
	for _, defaultFeature := range DefaultFeatures {
		if defaultFeature.ID == featureID {
			return true
		}
	}
	return false
}

func GetFeaturesState() *FeaturesState {
	content, err := os.ReadFile(FeaturesStatePath)
	if err != nil {
		var state = &FeaturesState{Enabled: []string{}}
		for _, defaultFeature := range DefaultFeatures {
			state.Enabled = append(state.Enabled, defaultFeature.ID)
		}
		if err = state.Save(); err != nil {
			log.Errorf("could not create features cache: %v", err)
			panic(err)
		}
		return state
	}

	var state = FeaturesState{
		Enabled: []string{},
	}

	if err = json.Unmarshal(content, &state); err != nil {
		panic(err)
	}
	return &state
}

func (s *FeaturesState) Save() error {
	content, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(FeaturesStatePath, content, 0644)
}

// KnownFeatures is a sorted list of features, ordered from least to the most dependent.
var KnownFeatures = []*Feature{
	&IdentityFeature,
	&ContentFeature,
	&AnalyticsFeature,
	&ManagementFeature,
	&MalwareFeature,
	&ComplianceFeature,
}

func GetFeature(featureID string) (*Feature, bool) {
	for _, feature := range KnownFeatures {
		if feature.ID == featureID {
			return feature, true
		}
	}
	return nil, false
}

// Feature manages optional features of rhc.
//
// ID is an identifier of the feature.
// Description is human-readable description of the feature.
// Requires is a list of IDs of other features that are required for this feature. Feature
// dependencies are not resolved.
// EnableFunc is called when the feature should transition into enabled state.
// DisableFunc is called when the feature should transition into disabled state.
type Feature struct {
	ID          string
	Description string
	Requires    []*Feature
	EnableFunc  func(ctx *cli.Context) error
	DisableFunc func(ctx *cli.Context) error
}

func (f *Feature) String() string {
	return fmt.Sprintf("Feature{ID:%s}", f.ID)
}

var IdentityFeature = Feature{
	ID:          "identity",
	Requires:    []*Feature{},
	Description: "Identify a RHEL system",
	EnableFunc: func(ctx *cli.Context) error {
		log.Debug("'identity' is currently a meta feature implicitly requiring 'content'")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		log.Debug("'identity' is currently a meta feature implicitly requiring 'content'")
		return nil
	},
}

var ContentFeature = Feature{
	ID:          "content",
	Requires:    []*Feature{&IdentityFeature},
	Description: "Get access to RHEL content",
	EnableFunc: func(ctx *cli.Context) error {
		log.Debug("'content' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		log.Debug("'content' feature not implemented")
		return nil
	},
}

var ManagementFeature = Feature{
	ID:          "management",
	Requires:    []*Feature{&IdentityFeature, &ContentFeature, &AnalyticsFeature},
	Description: "Remote management",
	EnableFunc: func(ctx *cli.Context) error {
		log.Debug("'management' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		log.Debug("'management' feature not implemented")
		return nil
	},
}

var MalwareFeature = Feature{
	ID:          "malware",
	Requires:    []*Feature{&IdentityFeature, &AnalyticsFeature},
	Description: "Malware analytics",
	EnableFunc: func(ctx *cli.Context) error {
		log.Debug("'malware' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		log.Debug("'malware' feature not implemented")
		return nil
	},
}

var ComplianceFeature = Feature{
	ID:          "compliance",
	Requires:    []*Feature{&IdentityFeature, &AnalyticsFeature},
	Description: "Compliance analytics and management",
	EnableFunc: func(ctx *cli.Context) error {
		log.Debug("'compliance' feature not implemented")
		return nil
	},
	DisableFunc: func(ctx *cli.Context) error {
		log.Debug("'compliance' feature not implemented")
		return nil
	},
}
