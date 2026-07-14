package feature

import (
	"github.com/redhatinsights/rhc/internal/subman"
)

// Content implements IFeature.
type Content struct{}

func (c Content) ID() string {
	return "content"
}

func (c Content) Description() string {
	return "Red Hat content management"
}

func (c Content) Requires() []string {
	return []string{}
}

func (c Content) RequiredBy() []string {
	return []string{"remote-management"}
}

func (c Content) Enable() error {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return err
	}
	return client.SetContentManagement(true)
}

func (c Content) Disable() error {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return err
	}
	return client.SetContentManagement(false)
}

func (c Content) IsEnabled() (bool, error) {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return false, err
	}
	return client.IsContentManagementEnabled()
}
