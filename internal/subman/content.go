package subman

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
	"github.com/redhatinsights/rhc/internal/localization"
)

// IsContentManagementEnabled reports whether content management is enabled for
// the system in rhsm.conf (rhsm.manage_repos).
func (c *RHSMClient) IsContentManagementEnabled() (bool, error) {
	slog.Debug("Checking content management status")

	locale := localization.GetLocale()
	config := c.conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")

	var value string
	err := config.Call(
		"com.redhat.RHSM1.Config.Get",
		dbus.Flags(0),
		"rhsm.manage_repos",
		locale,
	).Store(&value)
	if err != nil {
		return false, fmt.Errorf("reading content management setting: %w", newDbusError(err))
	}

	return value == "1", nil
}

// SetContentManagement enables or disables content management for the system
// in rhsm.conf (rhsm.manage_repos).
func (c *RHSMClient) SetContentManagement(enabled bool) error {
	slog.Debug("Setting content management", "enabled", enabled)

	locale := localization.GetLocale()
	config := c.conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")

	value := "0"
	if enabled {
		value = "1"
	}

	err := config.Call(
		"com.redhat.RHSM1.Config.Set",
		dbus.Flags(0),
		"rhsm.manage_repos",
		value,
		locale,
	).Err
	if err != nil {
		slog.Debug("Could not set rhsm.manage_repos", "error", err)
		return fmt.Errorf("setting content management: %w", newDbusError(err))
	}
	return nil
}
