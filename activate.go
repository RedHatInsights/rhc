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
