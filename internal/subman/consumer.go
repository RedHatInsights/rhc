package subman

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jirihnidek/rhsm2"
)

// GetEnvironments returns environments assigned to the registered consumer.
// There is no D-Bus method for consumer environments; this uses the Candlepin
// REST API via rhsm2 (GET consumers/{uuid}).
func GetEnvironments() ([]Environment, error) {
	slog.Debug("Retrieving consumer environments")

	uuid, err := GetConsumerUUID()
	if err != nil {
		slog.Debug("Failed to get consumer UUID for environments lookup", "error", err)
		return nil, err
	}
	if uuid == "" {
		return nil, fmt.Errorf("system is not registered")
	}

	appName := RHSMClientAppName
	rhsmClient, err := rhsm2.GetRHSMClient(&appName, nil)
	if err != nil {
		slog.Debug("Failed to create RHSM client for environments lookup", "error", err)
		return nil, err
	}

	consumer, err := rhsmClient.GetConsumer(nil)
	if err != nil {
		slog.Debug("Failed to get consumer for environments lookup", "error", err)
		return nil, err
	}

	if consumer.Environments == nil {
		slog.Debug("Consumer has no environments assigned")
		return []Environment{}, nil
	}

	data, err := json.Marshal(consumer.Environments)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal environments: %w", err)
	}

	var environments []Environment
	if err := json.Unmarshal(data, &environments); err != nil {
		return nil, fmt.Errorf("unable to unmarshal environments: %w", err)
	}

	return environments, nil
}
