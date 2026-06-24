package subman

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/godbus/dbus/v5"
)

// ErrDBusUnavailable is returned when the system D-Bus daemon cannot be reached.
var ErrDBusUnavailable = errors.New("system D-Bus is not available")

// ErrAlreadyRegistered is returned when an operation requires the system to be
// unregistered, but it is already registered with RHSM.
var ErrAlreadyRegistered = errors.New("system is already registered with RHSM")

// ErrAlreadyUnregistered is returned when an operation requires the system to
// be registered, but it is already unregistered from RHSM.
var ErrAlreadyUnregistered = errors.New("system is already unregistered from RHSM")

// ErrOrganizationRequired is returned when an organization must be specified
// but was not.
var ErrOrganizationRequired = errors.New("organization is required")

// dbusError holds the structured error body returned by com.redhat.RHSM1 D-Bus methods.
type dbusError struct {
	Exception string `json:"exception"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

func (e dbusError) Error() string {
	return e.Message
}

// newDbusError translates a raw D-Bus error into a structured dbusError when
// the error originates from com.redhat.RHSM1. Returns the original error
// unchanged for all other cases or when the body cannot be parsed.
func newDbusError(err error) error {
	var e dbus.Error
	ok := errors.As(err, &e)
	if !ok {
		return err
	}
	if e.Name != "com.redhat.RHSM1.Error" {
		return err
	}

	var d dbusError
	if jsonErr := json.Unmarshal([]byte(e.Error()), &d); jsonErr != nil {
		slog.Debug("Failed to unmarshal D-Bus error body", "error", jsonErr)
		return err
	}
	return d
}
