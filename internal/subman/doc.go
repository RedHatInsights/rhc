// Package subman provides a D-Bus client adapter for subscription-manager.
//
// All operations communicate with com.redhat.RHSM1 over the system bus.
// The underlying godbus library manages the system bus connection as
// a process-wide singleton; functions in this package must never close
// the connection they obtain from the bus.
//
// # Identity and registration
//
//   - [IsRegistered] reports whether the system is currently registered with RHSM.
//   - [RegisterWithPassword] registers the system using a username and password.
//     If the account belongs to multiple organizations, and none has been passed
//     in, [ErrOrganizationRequired] is returned; the caller should call
//     [GetOrganizations] to obtain the list, prompt the user, and retry with an
//     explicit organization.
//   - [GetOrganizations] returns the organization keys available for a username
//     and password. Use this after [RegisterWithPassword] returns
//     [ErrOrganizationRequired].
//   - [RegisterWithActivationKeys] registers the system using activation keys.
//     An organization must always be specified for this method.
//   - [Unregister] removes the system's RHSM registration.
//
// # Content management
//
//   - [IsContentManagementEnabled] reports whether RHSM content management is
//     enabled in rhsm.conf (rhsm.manage_repos).
//   - [SetContentManagement] enables or disables RHSM content management.
//
// # Registration options
//
// Both registration methods accept a [RegisterOptions] value that groups the
// options shared by all registration flows (environment names and content
// enablement). The EnvironmentNames field corresponds to the --content-template
// CLI flag.
package subman
