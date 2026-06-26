package subman

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/redhatinsights/rhc/internal/localization"
)

// RegisterOptions groups the options common to the registration methods.
type RegisterOptions struct {
	// EnvironmentNames is the list of content template names to associate with the host.
	EnvironmentNames []string
	// EnableContent controls whether RHSM content management (manage_repos)
	// is enabled after registration.
	EnableContent bool
}

// buildOptions converts RegisterOptions into the D-Bus options map expected by
// the RHSM registration methods.
func buildOptions(opts RegisterOptions) map[string]string {
	options := make(map[string]string)
	if len(opts.EnvironmentNames) != 0 {
		options["environment_type"] = "content-template"
		options["environment_names"] = strings.Join(opts.EnvironmentNames, ",")
	}
	options["enable_content"] = strconv.FormatBool(opts.EnableContent)
	return options
}

// GetConsumerUUID returns the RHSM consumer UUID.
// Returns [ErrNotRegistered] if the system is not currently registered.
func (c *RHSMClient) GetConsumerUUID() (string, error) {
	slog.Debug("Getting consumer UUID")
	var uuid string
	locale := localization.GetLocale()
	err := c.conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Consumer").Call(
		"com.redhat.RHSM1.Consumer.GetUuid",
		dbus.Flags(0),
		locale,
	).Store(&uuid)
	if err != nil {
		return "", fmt.Errorf("getting consumer UUID: %w", newDbusError(err))
	}
	if uuid == "" {
		return "", ErrNotRegistered
	}
	return uuid, nil
}

// IsRegistered reports whether the system is currently registered with RHSM.
func (c *RHSMClient) IsRegistered() (bool, error) {
	slog.Debug("Checking if system is registered to Red Hat Subscription Management")
	_, err := c.GetConsumerUUID()
	if errors.Is(err, ErrNotRegistered) {
		slog.Debug("Consumer UUID is not set, system is not registered")
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not determine registration status: %w", err)
	}
	slog.Debug("Consumer UUID is set, system is registered")
	return true, nil
}

// GetOrganizations returns the list of organization names available for the
// given username and password.
func (c *RHSMClient) GetOrganizations(username, password string) ([]string, error) {
	slog.Debug("Retrieving available organizations")

	var organizations []string
	getOrganizations := func(privConn *dbus.Conn, locale string) error {
		slog.Debug("Calling method com.redhat.RHSM1.Register.GetOrgs")
		var raw string
		if err := privConn.Object(
			"com.redhat.RHSM1",
			"/com/redhat/RHSM1/Register").Call(
			"com.redhat.RHSM1.Register.GetOrgs",
			dbus.Flags(0),
			username,
			password,
			map[string]string{},
			locale,
		).Store(&raw); err != nil {
			return fmt.Errorf("retrieving available organizations: %w", newDbusError(err))
		}

		var parseErr error
		organizations, parseErr = unpackOrganizations(raw)
		if parseErr != nil {
			return fmt.Errorf("parsing available organizations: %w", parseErr)
		}

		return nil
	}

	if err := withPrivateRegisterSocket(c.conn, getOrganizations); err != nil {
		return nil, err
	}

	return organizations, nil
}

// RegisterWithPassword registers the system using username/password credentials.
//
// If the account belongs to multiple organizations, and an empty string has been
// passed in, [ErrOrganizationRequired] is returned; the caller should call
// [RHSMClient.GetOrganizations] to retrieve the available organization names,
// prompt the user, and retry with an explicit value.
func (c *RHSMClient) RegisterWithPassword(username, password, organization string, opts RegisterOptions) error {
	slog.Debug("Registering system with username and password")

	registerWithPassword := func(privConn *dbus.Conn, locale string) error {
		options := buildOptions(opts)
		slog.Debug("Calling method com.redhat.RHSM1.Register.Register")
		if err := privConn.Object(
			"com.redhat.RHSM1",
			"/com/redhat/RHSM1/Register").Call(
			"com.redhat.RHSM1.Register.Register",
			dbus.Flags(0),
			organization,
			username,
			password,
			options,
			map[string]string{},
			locale,
		).Err; err != nil {
			unpacked := newDbusError(err)
			var d dbusError
			if errors.As(unpacked, &d) && d.Exception == "OrgNotSpecifiedException" {
				return ErrOrganizationRequired
			}

			return fmt.Errorf("registering with RHSM: %w", unpacked)
		}

		return nil
	}

	return withPrivateRegisterSocket(c.conn, registerWithPassword)
}

// RegisterWithActivationKeys registers the system using activation keys.
//
// Returns [ErrOrganizationRequired] if organization is empty.
func (c *RHSMClient) RegisterWithActivationKeys(organization string, activationKeys []string, opts RegisterOptions) error {
	slog.Debug("Registering system with activation keys")
	if organization == "" {
		return ErrOrganizationRequired
	}

	registerWithActivationKeys := func(privConn *dbus.Conn, locale string) error {
		options := buildOptions(opts)
		slog.Debug("Calling method com.redhat.RHSM1.Register.RegisterWithActivationKeys")
		if err := privConn.Object(
			"com.redhat.RHSM1",
			"/com/redhat/RHSM1/Register").Call(
			"com.redhat.RHSM1.Register.RegisterWithActivationKeys",
			dbus.Flags(0),
			organization,
			activationKeys,
			options,
			map[string]string{},
			locale,
		).Err; err != nil {
			return fmt.Errorf("registering with RHSM: %w", newDbusError(err))
		}
		return nil
	}

	return withPrivateRegisterSocket(c.conn, registerWithActivationKeys)
}

// Unregister removes the system's RHSM registration.
func (c *RHSMClient) Unregister() error {
	slog.Debug("Unregistering system from Red Hat Subscription Management")
	slog.Debug("Calling method com.redhat.RHSM1.Unregister.Unregister")
	locale := localization.GetLocale()
	if err := c.conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Unregister").Call(
		"com.redhat.RHSM1.Unregister.Unregister",
		dbus.Flags(0),
		map[string]string{}, // reserved for future use
		locale,
	).Err; err != nil {
		return fmt.Errorf("unregistering from RHSM: %w", newDbusError(err))
	}

	return nil
}
