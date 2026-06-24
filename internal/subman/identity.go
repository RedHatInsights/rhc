package subman

import (
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"

	"github.com/redhatinsights/rhc/internal/localization"
)

// GetConsumerUUID returns the consumer UUID from RHSM D-Bus API.
// An empty string means the system is not registered.
func GetConsumerUUID() (string, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return "", err
	}
	// godbus implements SystemBus as a singleton, do not call conn.Close()

	locale := localization.GetLocale()

	var uuid string
	err = conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Consumer").Call(
		"com.redhat.RHSM1.Consumer.GetUuid",
		dbus.Flags(0),
		locale).Store(&uuid)
	if err != nil {
		return "", UnpackDBusError(err)
	}

	return uuid, nil
}

// IsRegistered returns true when the system is registered with RHSM.
func IsRegistered() (bool, error) {
	slog.Debug("Checking if system is registered to Red Hat Subscription Management")

	uuid, err := GetConsumerUUID()
	if err != nil {
		return false, err
	}
	if uuid != "" {
		slog.Debug("Consumer UUID is set, system is registered")
		return true, nil
	}

	slog.Debug("Consumer UUID is not set, system is not registered")
	return false, nil
}

// Unregister removes the system registration from RHSM.
func Unregister() error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	// godbus implements SystemBus as a singleton, do not call conn.Close()

	uuid, err := GetConsumerUUID()
	if err != nil {
		return err
	}
	if uuid == "" {
		return fmt.Errorf("warning: the system is already unregistered")
	}

	locale := localization.GetLocale()

	slog.Debug("Calling method com.redhat.RHSM1.Unregister.Unregister")
	err = conn.Object(
		"com.redhat.RHSM1",
		"/com/redhat/RHSM1/Unregister").Call(
		"com.redhat.RHSM1.Unregister.Unregister",
		dbus.Flags(0),
		map[string]string{},
		locale).Err
	if err != nil {
		return UnpackDBusError(err)
	}

	return nil
}
