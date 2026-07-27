package operations

import "github.com/redhatinsights/rhc/internal/subman"

// IsRegistered checks whether the system is registered with Red Hat
// Subscription Manager. It provides the layer boundary between cmd/
// and internal/subman per ADR-011 layering rules.
func IsRegistered() (bool, error) {
	client, err := subman.NewRHSMClient()
	if err != nil {
		return false, err
	}
	return client.IsRegistered()
}
