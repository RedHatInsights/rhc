package main

import (
	"context"
	"fmt"

	systemd "github.com/coreos/go-systemd/v22/dbus"
)

// activateService tries to enable and start yggdrasil service.
// The service can be branded to rhcd on RHEL
func activateService() error {
    ctx := context.Background()
	conn, err := systemd.NewSystemConnectionContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	unitName := ServiceName + ".service"

	if _, _, err := conn.EnableUnitFilesContext(ctx, []string{unitName}, false, true); err != nil {
		return err
	}

	done := make(chan string)
	if _, err := conn.StartUnitContext(ctx, unitName, "replace", done); err != nil {
		return err
	}
	<-done
	properties, err := conn.GetUnitPropertiesContext(ctx, unitName)
	if err != nil {
		return err
	}
	activeState := properties["ActiveState"]
	if activeState.(string) != "active" {
		return fmt.Errorf("error: The unit %v failed to start. Run 'systemctl status %v' for more information", unitName, unitName)
	}

	return nil
}

// deactivateService tries to stop and disable yggdrasil service.
// The service can be branded to rhcd on RHEL
func deactivateService() error {
		// Use simple background context without anything extra
	ctx := context.Background()
	conn, err := systemd.NewSystemConnectionContext(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	unitName := ServiceName + ".service"

	done := make(chan string)
	if _, err := conn.StopUnitContext(ctx, unitName, "replace", done); err != nil {
		return err
	}
	<-done

	if _, err := conn.DisableUnitFilesContext(ctx, []string{unitName}, false); err != nil {
		return err
	}

	return nil
}
