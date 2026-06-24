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
	return subman.SetContentManagement(true)
}

func (c Content) Disable() error {
	return subman.SetContentManagement(false)
}

func (c Content) IsEnabled() (bool, error) {
	return subman.IsContentManagementEnabled()
}
