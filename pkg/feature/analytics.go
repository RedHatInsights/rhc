package feature

import (
	"github.com/redhatinsights/rhc/internal/datacollection"
)

// Analytics implements IFeature.
type Analytics struct{}

func (a Analytics) ID() string {
	return "analytics"
}

func (a Analytics) Description() string {
	return "Red Hat Lightspeed data collection"
}

func (a Analytics) Requires() []string {
	return []string{}
}

func (a Analytics) RequiredBy() []string {
	return []string{"remote-management"}
}

func (a Analytics) Enable() error {
	return datacollection.RegisterInsightsClient()
}

func (a Analytics) Disable() error {
	return datacollection.UnregisterInsightsClient()
}

func (a Analytics) IsEnabled() (bool, error) {
	return datacollection.InsightsClientIsRegistered()
}
