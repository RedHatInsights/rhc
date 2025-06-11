// Package systemd provides a structured API for interacting with systemd
// services. It builds on top of the go-systemd package's D-Bus API, abstracting
// away some of the quirks that exist due to the D-Bus bindings.
package systemd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	systemd "github.com/coreos/go-systemd/v22/dbus"
)

type ConnectionType int

const (
	ConnectionTypeSystem ConnectionType = iota
	ConnectionTypeUser
)

type Conn struct {
	ctx  context.Context
	conn *systemd.Conn
}

// NewConnectionContext creates a new connection to the given systemd service.
func NewConnectionContext(ctx context.Context, connectionType ConnectionType) (*Conn, error) {
	var conn *systemd.Conn
	var err error
	if connectionType == ConnectionTypeSystem {
		conn, err = systemd.NewSystemConnectionContext(ctx)
	} else {
		conn, err = systemd.NewUserConnectionContext(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot establish connection to systemd: %v", err)
	}

	return &Conn{
		ctx:  ctx,
		conn: conn,
	}, nil
}

func (c *Conn) Close() {
	c.conn.Close()
}

// Reload instructs systemd to scan for and reload unit files.
func (c *Conn) Reload() error {
	return c.conn.ReloadContext(c.ctx)
}

// EnableUnit enables the named unit. If activate is true, it also starts the
// unit. If runtime is true, the unit is enabled for the runtime only (/run). If
// false, it is enabled persistently (/etc).
func (c *Conn) EnableUnit(name string, activate bool, runtime bool) error {
	if _, _, err := c.conn.EnableUnitFilesContext(c.ctx, []string{name}, runtime, true); err != nil {
		return fmt.Errorf("cannot enable unit %v: %v", name, err)
	}

	if activate {
		if err := c.StartUnit(name, true); err != nil {
			return fmt.Errorf("cannot start unit %v: %v", name, err)
		}
	}

	return nil
}

// StartUnit starts the named unit. If wait is true, the method waits until the
// unit state becomes "active".
func (c *Conn) StartUnit(name string, wait bool) error {
	jobComplete := make(chan string)
	_, err := c.conn.StartUnitContext(c.ctx, name, "replace", jobComplete)
	if err != nil {
		return fmt.Errorf("cannot start unit %v: %v", name, err)
	}
	result := <-jobComplete
	switch result {
	case "done":
		// The job successfully started, break to proceed
		break
	default:
		return fmt.Errorf("failed to start unit with reason: %v", result)
	}

	if wait {
		if err := c.waitForState(name, "active", 1*time.Second); err != nil {
			return fmt.Errorf("timed out waiting for state 'active': %v", err)
		}
	}

	return nil
}

// DisableUnit disables the named unit. If deactivate is true, it also stops the
// unit. If runtime is true, the unit is disabled for the runtime only (/run).
// If false, it is disabled persistently (/etc).
func (c *Conn) DisableUnit(name string, deactivate bool, runtime bool) error {
	if _, err := c.conn.DisableUnitFilesContext(c.ctx, []string{name}, runtime); err != nil {
		return fmt.Errorf("cannot disable unit %v: %v", name, err)
	}

	if deactivate {
		if err := c.StopUnit(name, true); err != nil {
			return fmt.Errorf("cannot stop unit %v: %v", name, err)
		}
	}

	return nil
}

// StopUnit stops the named unit. If wait is true, the method waits until the
// unit state becomes "inactive".
func (c *Conn) StopUnit(name string, wait bool) error {
	jobComplete := make(chan string)
	_, err := c.conn.StopUnitContext(c.ctx, name, "replace", jobComplete)
	if err != nil {
		return fmt.Errorf("cannot stop unit %v: %v", name, err)
	}
	result := <-jobComplete
	switch result {
	case "done":
		break
	default:
		return fmt.Errorf("failed to stop unit with reason: %v", result)
	}

	if wait {
		if err := c.waitForState(name, "inactive", 5*time.Second); err != nil {
			return fmt.Errorf("timed out waiting for state 'inactive': %v", err)
		}
	}

	return nil
}

// GetUnitState checks the given unit's "ActiveState" property.
func (c *Conn) GetUnitState(name string) (string, error) {
	prop, err := c.conn.GetUnitPropertyContext(c.ctx, name, "ActiveState")
	if err != nil {
		return "", fmt.Errorf("cannot get unit property 'ActiveState': %v", err)
	}
	var state string
	if err := prop.Value.Store(&state); err != nil {
		return "", fmt.Errorf("cannot store property %v (%v) to string: %v", prop.Name, prop.Value.String(), err)
	}
	return state, nil
}

// waitForState checks the unit state, waiting until it matches the given state,
// or the timeout occurs.
func (c *Conn) waitForState(unit string, wantState string, timeout time.Duration) error {
	after := time.After(timeout)
	for {
		select {
		case <-after:
			return fmt.Errorf("timed out waiting %v for unit state '%v'", timeout, wantState)
		default:
			state, err := c.GetUnitState(unit)
			if err != nil {
				return fmt.Errorf("cannot get unit state: %v", err)
			}
			if state == wantState {
				return nil
			} else {
				slog.Debug("got unit state", "state", state)
			}
		}
	}
}
