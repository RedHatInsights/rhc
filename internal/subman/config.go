package subman

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"github.com/redhatinsights/rhc/internal/localization"
)

// SetAllConfig writes multiple rhsm.conf keys in a single call through
// com.redhat.RHSM1.Config.SetAll. Keys use subscription-manager's
// "section.name" form, for example "server.hostname". Values are not written
// to the log. A nil or empty map is a no-op.
func (c *RHSMClient) SetAllConfig(settings map[string]string) error {
	if len(settings) == 0 {
		return nil
	}

	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	slog.Debug("Setting rhsm.conf keys", "keys", keys)

	locale := localization.GetLocale()
	config := c.conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	err := config.Call(
		"com.redhat.RHSM1.Config.SetAll",
		dbus.Flags(0),
		settings,
		locale,
	).Err
	if err != nil {
		slog.Debug("Could not set rhsm.conf keys", "keys", keys, "error", err)
		return fmt.Errorf("setting rhsm.conf keys: %w", newDbusError(err))
	}
	return nil
}
