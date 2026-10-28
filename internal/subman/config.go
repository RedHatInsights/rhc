package subman

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"github.com/redhatinsights/rhc/internal/localization"
)

// SetConfig writes one rhsm.conf key through com.redhat.RHSM1.Config.Set.
// key is subscription-manager's "section.name" form, for example
// "server.hostname". Values are not written to the log.
func (c *RHSMClient) SetConfig(key, value string) error {
	slog.Debug("Setting rhsm.conf key", "key", key)

	locale := localization.GetLocale()
	config := c.conn.Object("com.redhat.RHSM1", "/com/redhat/RHSM1/Config")
	err := config.Call(
		"com.redhat.RHSM1.Config.Set",
		dbus.Flags(0),
		key,
		value,
		locale,
	).Err
	if err != nil {
		slog.Debug("Could not set rhsm.conf key", "key", key, "error", err)
		return fmt.Errorf("setting %s: %w", key, newDbusError(err))
	}
	return nil
}
