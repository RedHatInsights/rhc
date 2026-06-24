package subman

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/godbus/dbus/v5"
)

// DBusError is used for parsing JSON document returned by D-Bus methods.
type DBusError struct {
	Exception string `json:"exception"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// Error returns textual representation of DBusError. This implements all necessary
// methods for the error interface. Thus, DBusError can be handled as a regular error.
func (dbusError DBusError) Error() string {
	return fmt.Sprintf("%v: %v", dbusError.Severity, dbusError.Message)
}

// UnpackDBusError tries to unpack a JSON document (part of the error) into the structure DBusError.
// When it is not possible to parse an error into structure, then a corresponding or original error
// is returned. When it is possible to parse error into structure, then DBusError is returned
func UnpackDBusError(err error) error {
	var e dbus.Error
	ok := errors.As(err, &e)
	if !ok {
		return err
	}
	if e.Name != "com.redhat.RHSM1.Error" {
		return err
	}

	var dbusError DBusError
	if jsonErr := json.Unmarshal([]byte(e.Error()), &dbusError); jsonErr != nil {
		slog.Debug("Failed to unmarshal D-Bus error body", "error", jsonErr)
		return err
	}
	return dbusError
}
