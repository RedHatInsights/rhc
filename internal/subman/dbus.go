package subman

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
	"github.com/redhatinsights/rhc/internal/localization"
)

// bus returns the shared system D-Bus connection.
// godbus implements SystemBus as a process-wide singleton; the returned
// connection must never be closed by callers.
func bus() (*dbus.Conn, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDBusUnavailable, err)
	}
	return conn, nil
}

// unpackOrganizations unmarshals the JSON list of organizations returned by the D-Bus
// GetOrgs method into a plain slice of organization key strings.
func unpackOrganizations(s string) ([]string, error) {
	// The D-Bus object contains multiple keys, but we only care about the organization names.
	var orgs []struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(s), &orgs); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(orgs))
	for _, o := range orgs {
		keys = append(keys, o.Key)
	}

	return keys, nil
}

// withPrivateRegisterSocket opens the private RHSM registration socket, authenticates,
// and calls fn with the live connection and the resolved locale string.
// It ensures the socket is stopped and closed on return regardless of outcome.
// fn must not retain the connection after it returns.
func withPrivateRegisterSocket(conn *dbus.Conn, fn func(*dbus.Conn, string) error) error {
	locale := localization.GetLocale()
	registerServer := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/RegisterServer")

	slog.Debug("Opening private D-Bus UNIX socket")
	var socketURI string
	err := registerServer.Call(
		"com.redhat.RHSM1.RegisterServer.Start",
		dbus.Flags(0),
		locale,
	).Store(&socketURI)
	if err != nil {
		return fmt.Errorf("starting RHSM register server: %w", newDbusError(err))
	}
	defer func() {
		slog.Debug("Closing private UNIX socket", "socket", socketURI)
		registerServer.Call("com.redhat.RHSM1.RegisterServer.Stop", dbus.FlagNoReplyExpected, locale)
	}()

	slog.Debug("Connecting to private D-Bus UNIX socket", "socket", socketURI)
	privConn, err := dbus.Dial(socketURI)
	if err != nil {
		return fmt.Errorf("connecting to private D-Bus socket: %w", err)
	}
	defer func() {
		slog.Debug("Closing connection to private D-Bus UNIX socket", "socket", socketURI)
		if closeErr := privConn.Close(); closeErr != nil {
			slog.Debug("Unable to close private D-Bus socket", "socket", socketURI, "err", closeErr)
		}
	}()

	if err = privConn.Auth(nil); err != nil {
		return fmt.Errorf("authenticating private D-Bus connection: %w", err)
	}

	return fn(privConn, locale)
}
