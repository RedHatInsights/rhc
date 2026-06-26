package subman

import "github.com/godbus/dbus/v5"

// Service defines the contract for subscription-manager D-Bus operations.
// The concrete implementation is [RHSMClient]. A mock implementation can be
// provided in unit tests.
type Service interface {
	// GetConsumerUUID returns the RHSM consumer UUID.
	// Returns [ErrNotRegistered] if the system is not currently registered.
	GetConsumerUUID() (string, error)

	// IsRegistered reports whether the system is registered with RHSM.
	IsRegistered() (bool, error)

	// IsContentManagementEnabled reports whether RHSM content management is
	// enabled in rhsm.conf (rhsm.manage_repos).
	IsContentManagementEnabled() (bool, error)

	// SetContentManagement enables or disables RHSM content management.
	SetContentManagement(enabled bool) error

	// Unregister removes the system's RHSM registration.
	Unregister() error

	// RegisterWithPassword registers the system using username/password credentials.
	// Returns [ErrOrganizationRequired] if the account belongs to multiple
	// organizations and none was specified; the caller should call
	// [Service.GetOrganizations] and retry with an explicit value.
	RegisterWithPassword(username, password, organization string, opts RegisterOptions) error

	// RegisterWithActivationKeys registers the system using activation keys.
	// Returns [ErrOrganizationRequired] if organization is empty.
	RegisterWithActivationKeys(organization string, activationKeys []string, opts RegisterOptions) error

	// GetOrganizations returns the organization keys available for the credentials.
	GetOrganizations(username, password string) ([]string, error)
}

// RHSMClient implements [Service] using D-Bus calls to subscription-manager.
type RHSMClient struct {
	conn *dbus.Conn
}

// NewRHSMClient creates a new RHSMClient backed by the system D-Bus.
// The returned client must not be closed by the caller; godbus manages
// the system bus connection as a process-wide singleton.
func NewRHSMClient() (*RHSMClient, error) {
	conn, err := bus()
	if err != nil {
		return nil, err
	}
	return &RHSMClient{conn: conn}, nil
}
