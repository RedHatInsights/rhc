package systemd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewConnectionContext(t *testing.T) {
	if _, has := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !has {
		t.Skip("DBUS_SESSION_BUS_ADDRESS undefined")
	}

	tests := []struct {
		name           string
		connectionType ConnectionType
		expectError    bool
	}{
		{
			name:           "user_connection",
			connectionType: ConnectionTypeUser,
			expectError:    false,
		},
		{
			name:           "system_connection",
			connectionType: ConnectionTypeSystem,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewConnectionContext(context.Background(), tt.connectionType)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if tt.connectionType == ConnectionTypeSystem && err != nil {
				t.Skipf("System connection failed: %v", err)
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if conn == nil {
				t.Error("expected non-nil connection")
				return
			}

			conn.Close()
		})
	}
}

func TestReload(t *testing.T) {
	if _, has := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !has {
		t.Skip("DBUS_SESSION_BUS_ADDRESS undefined")
	}

	conn, err := NewConnectionContext(context.Background(), ConnectionTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	err = conn.Reload()
	if err != nil {
		t.Logf("Reload failed: %v", err)
	}
}

// TestUnitOperationsValid tests systemd unit lifecycle operations (enable, start, stop, disable)
// on a valid test unit file
func TestUnitOperationsValid(t *testing.T) {
	if _, has := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !has {
		t.Skip("DBUS_SESSION_BUS_ADDRESS undefined")
	}

	conn, err := NewConnectionContext(context.Background(), ConnectionTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	unitFile := "testdata/simple.service"
	abs, err := filepath.Abs(unitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.conn.LinkUnitFilesContext(conn.ctx, []string{abs}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	unitName := filepath.Base(unitFile)

	t.Run("enable_unit", func(t *testing.T) {
		err = conn.EnableUnit(unitName, false, true)
		if err != nil {
			t.Fatalf("unexpected error when enabling valid unit: %v", err)
		}
	})

	t.Run("get_unit_state", func(t *testing.T) {
		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Fatalf("unexpected error when getting unit state: %v", err)
		}
		if state != "inactive" {
			t.Errorf("expected unit to be inactive, got %q", state)
		}
	})

	t.Run("start_unit", func(t *testing.T) {
		err = conn.StartUnit(unitName, true)
		if err != nil {
			t.Fatalf("StartUnit failed: %v", err)
		}

		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Fatalf("unexpected error when getting unit state: %v", err)
		}
		if state != "active" {
			t.Errorf("expected unit to be active, got %q", state)
		}
	})

	t.Run("stop_unit", func(t *testing.T) {
		err := conn.StopUnit(unitName, true)
		if err != nil {
			t.Fatalf("unexpected error when stopping unit: %v", err)
		}

		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Fatalf("unexpected error when getting unit state: %v", err)
		}
		if state != "inactive" {
			t.Errorf("expected unit to be inactive after stop, got %q", state)
		}
	})

	t.Run("disable_unit", func(t *testing.T) {
		err = conn.DisableUnit(unitName, false, true)
		if err != nil {
			t.Fatalf("unexpected error disabling unit: %v", err)
		}
	})
}

// TestUnitOperationsInvalid tests systemd unit lifecycle operations (enable, start, stop, disable)
// on a unit file with malformed syntax.
func TestUnitOperationsInvalid(t *testing.T) {
	if _, has := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !has {
		t.Skip("DBUS_SESSION_BUS_ADDRESS undefined")
	}

	conn, err := NewConnectionContext(context.Background(), ConnectionTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	unitFile := "testdata/invalid.service"
	abs, err := filepath.Abs(unitFile)
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.conn.LinkUnitFilesContext(conn.ctx, []string{abs}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	unitName := filepath.Base(unitFile)

	defer func() {
		err = conn.DisableUnit(unitName, false, true)
		if err != nil {
			t.Fatalf("unexpected error disabling unit: %v", err)
		}
	}()

	// systemd will not return an error when enabling an invalid unit
	t.Run("enable_invalid_unit", func(t *testing.T) {
		err = conn.EnableUnit(unitName, false, true)
		if err != nil {
			t.Errorf("unexpected error when enabling unit: %v", err)
		}
	})

	t.Run("start_invalid_unit", func(t *testing.T) {
		err = conn.StartUnit(unitName, false)
		if err == nil {
			t.Error("expected error when starting invalid unit")
		}
	})

	// systemd will return an "inactive" unit state for invalid/non-existent units
	t.Run("get_invalid_unit_state", func(t *testing.T) {
		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Errorf("unexpected error getting invalid unit state: %v", err)
		}
		if state != "inactive" {
			t.Errorf("expected invalid unit to be inactive, got %q", state)
		}
	})
}

// TestUnitOperationsNonExistent tests systemd unit lifecycle operations (enable, start, stop, disable)
// on a unit file that doesn't exist on the filesystem.
func TestUnitOperationsNonExistent(t *testing.T) {
	if _, has := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !has {
		t.Skip("DBUS_SESSION_BUS_ADDRESS undefined")
	}

	conn, err := NewConnectionContext(context.Background(), ConnectionTypeUser)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	unitName := "nonexistent.service"

	// systemd will return an "inactive" unit state for invalid/non-existent units
	t.Run("get_nonexistent_unit_state", func(t *testing.T) {
		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Errorf("unexpected error when getting unit state: %v", err)
		}
		if state != "inactive" {
			t.Errorf("expected non-existent unit to be inactive, got %q", state)
		}
	})

	t.Run("enable_nonexistent_unit", func(t *testing.T) {
		err = conn.EnableUnit(unitName, false, true)
		if err == nil {
			t.Error("expected error when enabling non-existent unit")
		}
	})

	t.Run("start_nonexistent_unit", func(t *testing.T) {
		err = conn.StartUnit(unitName, false)
		if err == nil {
			t.Error("expected error when starting non-existent unit")
		}
	})

	t.Run("stop_nonexistent_unit", func(t *testing.T) {
		err = conn.StopUnit(unitName, false)
		if err == nil {
			t.Error("expected error when stopping non-existent unit")
		}
	})

	t.Run("disable_nonexistent_unit", func(t *testing.T) {
		err = conn.DisableUnit(unitName, false, true)
		if err == nil {
			t.Error("expected error when disabling non-existent unit")
		}
	})
}
