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

	defer func() {
		conn.DisableUnit(unitName, true, true)
	}()

	t.Run("enable_unit", func(t *testing.T) {
		err := conn.EnableUnit(unitName, false, true)
		if err != nil {
			t.Fatalf("EnableUnit failed: %v", err)
		}
	})

	t.Run("get_unit_state", func(t *testing.T) {
		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Fatalf("GetUnitState failed: %v", err)
		}
		if state == "" {
			t.Error("expected non-empty state")
		}
	})

	t.Run("start_unit", func(t *testing.T) {
		err := conn.StartUnit(unitName, true)
		if err != nil {
			t.Fatalf("StartUnit failed: %v", err)
		}

		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Fatalf("GetUnitState failed: %v", err)
		}
		if state != "active" {
			t.Errorf("expected unit to be active, got %q", state)
		}
	})

	t.Run("stop_unit", func(t *testing.T) {
		err := conn.StopUnit(unitName, true)
		if err != nil {
			t.Fatalf("StopUnit failed: %v", err)
		}

		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Fatalf("GetUnitState failed: %v", err)
		}
		if state == "active" {
			t.Errorf("expected unit to be inactive after stop, got %q", state)
		}
	})

	t.Run("disable_unit", func(t *testing.T) {
		err := conn.DisableUnit(unitName, false, true)
		if err != nil {
			t.Fatalf("DisableUnit failed: %v", err)
		}
	})
}

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
		conn.DisableUnit(unitName, false, true)
	}()

	t.Run("enable_invalid_unit", func(t *testing.T) {
		err := conn.EnableUnit(unitName, false, true)
		if err != nil {
			t.Logf("EnableUnit failed as expected: %v", err)
		}
	})

	t.Run("start_invalid_unit", func(t *testing.T) {
		err := conn.StartUnit(unitName, false)
		if err == nil {
			t.Error("expected error when starting invalid unit")
		}
	})

	t.Run("get_state_invalid_unit", func(t *testing.T) {
		state, err := conn.GetUnitState(unitName)
		if err != nil {
			t.Logf("GetUnitState failed for invalid unit: %v", err)
		} else {
			t.Logf("GetUnitState returned: %q", state)
		}
	})
}

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

	t.Run("get_state_nonexistent", func(t *testing.T) {
		state, err := conn.GetUnitState(unitName)
		if err == nil {
			t.Logf("GetUnitState for non-existent unit returned: %q", state)
		} else {
			t.Logf("GetUnitState failed for non-existent unit: %v", err)
		}
	})

	t.Run("enable_nonexistent", func(t *testing.T) {
		err := conn.EnableUnit(unitName, false, true)
		if err == nil {
			t.Error("expected error when enabling non-existent unit")
		}
	})

	t.Run("start_nonexistent", func(t *testing.T) {
		err := conn.StartUnit(unitName, false)
		if err == nil {
			t.Error("expected error when starting non-existent unit")
		}
	})

	t.Run("stop_nonexistent", func(t *testing.T) {
		err := conn.StopUnit(unitName, false)
		if err == nil {
			t.Error("expected error when stopping non-existent unit")
		}
	})

	t.Run("disable_nonexistent", func(t *testing.T) {
		err := conn.DisableUnit(unitName, false, true)
		if err == nil {
			t.Error("expected error when disabling non-existent unit")
		}
	})
}
