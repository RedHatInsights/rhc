package feature

import (
	"fmt"
)

type IFeature interface {
	ID() string
	Description() string
	// Requires returns a list of all feature IDs the feature depends on
	Requires() []string
	// RequiredBy returns a list of all feature IDs that require this feature
	RequiredBy() []string

	// Enable enables a feature. Acts as a no-op if it is already enabled.
	Enable() error
	// Disable disables a feature. Acts as a no-op if it is already disabled.
	Disable() error
	// IsEnabled returns true if the feature is enabled, false otherwise.
	// Returns an error if the feature misbehaves.
	IsEnabled() (bool, error)
}

// Individual features self-register here in their init()s
var registered = []IFeature{
	Content{},
	Analytics{},
	RemoteManagement{},
}

func All() []IFeature {
	return registered
}

func Get(id string) (IFeature, error) {
	for _, f := range registered {
		if f.ID() == id {
			return f, nil
		}
	}
	return nil, fmt.Errorf("feature %q not found", id)
}

func MustGet(id string) IFeature {
	f, err := Get(id)
	if err != nil {
		panic(err)
	}
	return f
}
