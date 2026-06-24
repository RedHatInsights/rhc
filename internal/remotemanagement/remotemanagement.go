package remotemanagement

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/redhatinsights/rhc/internal/systemd"
)

// ActivateServices tries to enable and start the rhc-canonical-facts.timer,
// rhc-canonical-facts.service and yggdrasil.service (in this order).
// Error is returned as soon as one of the calls to systemd fails.
func ActivateServices() error {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	slog.Debug("Enabling rhc-canonical-facts.timer")
	if err := conn.EnableUnit("rhc-canonical-facts.timer", true, false); err != nil {
		return fmt.Errorf("cannot enable rhc-canonical-facts.timer: %v", err)
	}

	// Start the canonical-facts service immediately, so the facts get generated
	// and written out before yggdrasil.service starts.
	slog.Debug("Starting rhc-canonical-facts.service")
	if err := conn.StartUnit("rhc-canonical-facts.service", false); err != nil {
		return fmt.Errorf("cannot start rhc-canonical-facts.service: %v", err)
	}

	slog.Debug("Enabling yggdrasil.service")
	if err := conn.EnableUnit("yggdrasil.service", true, false); err != nil {
		return fmt.Errorf("cannot enable yggdrasil.service: %v", err)
	}

	slog.Debug("Reloading systemd")
	if err := conn.Reload(); err != nil {
		return fmt.Errorf("cannot reload systemd: %v", err)
	}

	return nil
}

// UnitState holds the state of a systemd unit as reported by systemd.
type UnitState struct {
	// ActiveState is the systemd ActiveState property value (e.g. "active", "inactive").
	ActiveState string
	// LoadState is the systemd LoadState property value (e.g. "loaded", "not-found").
	LoadState string
	// LoadError is the human-readable error message from the systemd LoadError
	// property. It is non-empty only when the unit failed to load.
	LoadError string
}

// GetUnitState returns the current state of a systemd unit.
func GetUnitState(name string) (*UnitState, error) {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	props, err := conn.GetUnitProperties(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get properties of %s: %v", name, err)
	}

	result := &UnitState{}
	result.ActiveState, _ = props["ActiveState"].(string)
	result.LoadState, _ = props["LoadState"].(string)

	if result.ActiveState != "active" && result.LoadState != "loaded" {
		// This part of the systemd D-Bus API returns two objects, one is a slice
		// representing the error ID ("org.freedesktop.systemd1.NoSuchUnit"), the
		// other a string representing a human-readable error message.
		// systemd returns LoadError as []interface{}{errorID string, errorMessage string}.
		raw := props["LoadError"]
		t := reflect.TypeOf(raw)
		if t != nil && t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Interface {
			slice := raw.([]interface{})
			if len(slice) >= 2 {
				if msg, ok := slice[1].(string); ok {
					result.LoadError = msg
				}
			}
		}
	}

	return result, nil
}

// AssertYggdrasilServiceState returns true, when yggdrasil.service is in given state
func AssertYggdrasilServiceState(wantedState string) (bool, error) {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return false, fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	slog.Debug("retrieving yggdrasil.service unit state")
	state, err := conn.GetUnitState("yggdrasil.service")
	if err != nil {
		return false, fmt.Errorf("cannot get unit state: %v", err)
	}
	if state == wantedState {
		return true, nil
	} else {
		return false, nil
	}
}

// DeactivateServices tries to stop and disable the rhc-canonical-facts.timer,
// rhc-canonical-facts.service and yggdrasil.service (in this order).
// Error is returned as soon as one of the calls to systemd fails.
func DeactivateServices() error {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	slog.Debug("Disabling rhc-canonical-facts.service")
	if err := conn.DisableUnit("rhc-canonical-facts.timer", true, false); err != nil {
		return fmt.Errorf("cannot disable rhc-canonical-facts.timer: %v", err)
	}

	slog.Debug("Disabling yggdrasil.service")
	if err := conn.DisableUnit("yggdrasil.service", true, false); err != nil {
		return fmt.Errorf("cannot disable yggdrasil.service: %v", err)
	}

	slog.Debug("Reloading systemd")
	if err := conn.Reload(); err != nil {
		return fmt.Errorf("cannot reload systemd: %v", err)
	}

	return nil
}
