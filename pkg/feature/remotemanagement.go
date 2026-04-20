package feature

import (
	"github.com/redhatinsights/rhc/internal/remotemanagement"
)

// RemoteManagement implements IFeature.
type RemoteManagement struct{}

func (r RemoteManagement) ID() string {
	return "remote-management"
}

func (r RemoteManagement) Description() string {
	return "Red Hat Lightspeed remote management"
}

func (r RemoteManagement) Requires() []string {
	return []string{"content", "analytics"}
}

func (r RemoteManagement) RequiredBy() []string {
	return []string{}
}

func (r RemoteManagement) Enable() error {
	return remotemanagement.ActivateServices()
}

func (r RemoteManagement) Disable() error {
	return remotemanagement.DeactivateServices()
}

func (r RemoteManagement) IsEnabled() (bool, error) {
	return remotemanagement.AssertYggdrasilServiceState("active")
}
