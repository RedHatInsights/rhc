// Package subman provides a D-Bus client adapter for subscription-manager.
//
// # Architecture overview
//
// All operations communicate with the com.redhat.RHSM1 D-Bus service. The
// package is structured around two connection types with different lifetimes
// and purposes:
//
//  1. The system bus connection — a process-wide singleton managed by godbus.
//     Callers must never close it. Used for every operation except registration.
//
//  2. The private registration socket — a temporary UNIX socket opened only
//     for the duration of a registration call. RHSM requires credentials to be
//     sent over this private channel rather than the shared system bus.
//
// # Private registration socket flow
//
// Registration (password or activation-key) uses a two-step connection
// sequence that differs from all other operations:
//
//  1. Call RegisterServer.Start on the system bus — returns a private UNIX
//     socket address (e.g. "unix:path=/run/dbus-XXXXXX").
//
//  2. Dial that address, authenticate (Auth), and call the actual Register
//     or RegisterWithActivationKeys method on the private connection.
//
//  3. Call RegisterServer.Stop on the system bus when done (deferred).
//
// This flow is encapsulated in withPrivateRegisterSocket, which dials and
// authenticates the private connection directly and closes it on return.
//
// # Service interface and RHSMClient
//
// [Service] defines the full contract for subscription-manager operations.
// [RHSMClient] is the concrete implementation backed by *dbus.Conn:
//
//	type RHSMClient struct {
//	    conn *dbus.Conn  // system bus (process-wide singleton)
//	}
//
// Construct a client with [NewRHSMClient]:
//
//	client, err := subman.NewRHSMClient()
//	if err != nil {
//	    // system D-Bus daemon is not reachable
//	}
package subman
