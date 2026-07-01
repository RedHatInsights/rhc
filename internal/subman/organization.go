package subman

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"github.com/redhatinsights/rhc/internal/localization"
)

// GetOrganization returns the organization for the registered consumer via D-Bus.
func GetOrganization() (*Organization, error) {
	slog.Debug("Retrieving consumer organization")

	uuid, err := GetConsumerUUID()
	if err != nil {
		slog.Debug("Failed to get consumer UUID for organization lookup", "error", err)
		return nil, err
	}
	if uuid == "" {
		return nil, fmt.Errorf("system is not registered")
	}

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}
	// godbus implements SystemBus as a singleton, do not call conn.Close()

	locale := localization.GetLocale()

	var orgJSON string
	err = conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Consumer").Call(
		"com.redhat.RHSM1.Consumer.GetOrg",
		dbus.Flags(0),
		locale).Store(&orgJSON)
	if err != nil {
		slog.Debug("Failed to get consumer organization from D-Bus", "error", err)
		return nil, UnpackDBusError(err)
	}

	var org Organization
	if err := json.Unmarshal([]byte(orgJSON), &org); err != nil {
		slog.Debug("Failed to unmarshal organization JSON from D-Bus", "error", err)
		return nil, fmt.Errorf("unable to unmarshal organization: %w", err)
	}

	slog.Debug("Retrieved consumer organization", "key", org.Key)
	return &org, nil
}
