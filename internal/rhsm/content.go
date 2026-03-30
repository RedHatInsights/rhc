package rhsm

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"github.com/redhatinsights/rhc/internal/localization"
)

// IsContentManagementEnabled returns true if content management is enabled for the system in rhsm.conf
func IsContentManagementEnabled() (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}
	// godbus implements SystemBus as a singleton, do not call conn.Close()

	locale := localization.GetLocale()
	config := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	var contentEnabled string
	err = config.Call(
		"com.redhat.RHSM1.Config.Get",
		dbus.Flags(0),
		"rhsm.manage_repos",
		locale).Store(&contentEnabled)
	if err != nil {
		slog.Debug("Failed to get rhsm.manage_repos config", "error", err)
		return false, UnpackDBusError(err)
	}

	return contentEnabled == "1", nil
}

// SetContentManagement tries to enable or disable content management for the system in rhsm.conf
func SetContentManagement(enabled bool) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("cannot connect to system D-Bus: %w", err)
	}
	// godbus implements SystemBus as a singleton, do not call conn.Close()

	locale := localization.GetLocale()
	config := conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	enabledStr := "1"
	if !enabled {
		enabledStr = "0"
	}

	err = config.Call(
		"com.redhat.RHSM1.Config.Set",
		dbus.Flags(0),
		"rhsm.manage_repos",
		enabledStr,
		locale,
	).Err
	if err != nil {
		slog.Debug("Failed to set rhsm.manage_repos config", "error", err)
		return UnpackDBusError(err)
	}

	return nil
}
