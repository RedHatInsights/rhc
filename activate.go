package main

import (
	"context"
	"fmt"
	systemd "github.com/redhatinsights/rhc/internal/systemd"
)

// activateService tries to enable and start the rhc-canonical-facts.timer,
// rhc-canonical-facts.service and yggdrasil.service.
func activateService() error {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	if err := conn.EnableUnit("rhc-canonical-facts.timer", true, false); err != nil {
		return fmt.Errorf("cannot enable rhc-canonical-facts.timer: %v", err)
	}

	// Start the canonical-facts service immediately, so the facts get generated
	// and written out before yggdrasil.service starts.
	if err := conn.StartUnit("rhc-canonical-facts.service", false); err != nil {
		return fmt.Errorf("cannot start rhc-canonical-facts.service: %v", err)
	}

	if err := conn.EnableUnit("yggdrasil.service", true, false); err != nil {
		return fmt.Errorf("cannot enable yggdrasil.service: %v", err)
	}

	if err := conn.Reload(); err != nil {
		return fmt.Errorf("cannot reload systemd: %v", err)
	}

	return nil
}

// isServiceInState returns true, when yggdrasil.service is in given state
func isServiceInState(wantedState string) (bool, error) {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return false, fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

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

// deactivateService tries to stop and disable the rhc-canonical-facts.timer,
// rhc-canonical-facts.service and yggdrasil.service.
func deactivateService() error {
	conn, err := systemd.NewConnectionContext(context.Background(), systemd.ConnectionTypeSystem)
	if err != nil {
		return fmt.Errorf("cannot connect to systemd: %v", err)
	}
	defer conn.Close()

	if err := conn.DisableUnit("rhc-canonical-facts.timer", true, false); err != nil {
		return fmt.Errorf("cannot disable rhc-canonical-facts.timer: %v", err)
	}

	if err := conn.DisableUnit("yggdrasil.service", true, false); err != nil {
		return fmt.Errorf("cannot disable yggdrasil.service: %v", err)
	}

	if err := conn.Reload(); err != nil {
		return fmt.Errorf("cannot reload systemd: %v", err)
	}

	return nil
}
