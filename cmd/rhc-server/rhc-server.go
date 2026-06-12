package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jirihnidek/rhsm2"
	"github.com/redhatinsights/rhc/internal/collector"
	"github.com/redhatinsights/rhc/varlink/collectorapi"
	"github.com/redhatinsights/rhc/varlink/rhsmapi"
)

// CollectorBackend implements the collectorapi.Backend interface.
type CollectorBackend struct{}

// NewCollectorBackend creates a new CollectorBackend instance.
func NewCollectorBackend() *CollectorBackend {
	return &CollectorBackend{}
}

// List implements the List method of the collector API.
// Returns a list of all available collectors with full details.
func (b *CollectorBackend) List(_ *collectorapi.ListIn) (*collectorapi.ListOut, error) {
	// Get list of collector IDs
	collectorIDs, err := collector.GetCollectors()
	if err != nil {
		return nil, fmt.Errorf("failed to get collectors: %w", err)
	}

	// Build the list of collectors with full details
	collectors := make([]collectorapi.CollectorInfo, 0, len(collectorIDs))
	for _, id := range collectorIDs {
		info, err := buildCollectorInfo(id)
		if err != nil {
			slog.Warn("Failed to build collector info, skipping it", "id", id, "error", err)
			continue
		}
		collectors = append(collectors, *info)
	}

	return &collectorapi.ListOut{Collectors: collectors}, nil
}

// Info implements the Info method of the collector API.
// Returns detailed information about a specific collector including timing and configuration.
func (b *CollectorBackend) Info(in *collectorapi.InfoIn) (*collectorapi.InfoOut, error) {
	// Validate input parameter
	if _, err := collector.ValidateID(in.Id); err != nil {
		return nil, &collectorapi.InvalidParameterError{
			Parameter: "id",
		}
	}

	info, err := buildCollectorInfo(in.Id)
	if err != nil {
		return nil, &collectorapi.NoSuchCollectorError{
			Id: in.Id,
		}
	}

	return &collectorapi.InfoOut{Info: *info}, nil
}

// RHSMBackend implements the rhsmapi.Backend interface.
type RHSMBackend struct{}

// NewRHSMBackend creates a new RHSMBackend instance.
func NewRHSMBackend() *RHSMBackend {
	return &RHSMBackend{}
}

// Ping checks the status of the RHSM server.
func (b *RHSMBackend) Ping(in *rhsmapi.PingIn) (*rhsmapi.PingOut, error) {
	var rhsmServerStatus *rhsm2.RHSMStatus
	var err error
	if in.Metadata != nil {
		rhsmServerStatus, err = GetStatus(
			in.Metadata.UserAgent,
			in.Metadata.Locale,
			in.Metadata.CorrelationId,
		)
	} else {
		rhsmServerStatus, err = GetStatus(nil, nil, nil)
	}
	if err != nil {
		var typeClientErr *ClientError
		var typeServerErr *ServerError
		switch {
		case errors.As(err, &typeClientErr):
			return nil, &rhsmapi.InvalidClientConnectionError{Message: typeClientErr.Message}
		case errors.As(err, &typeServerErr):
			return nil, &rhsmapi.FailedServerResponseError{Message: typeServerErr.Message}
		default:
			slog.Error("Failed to get RHSM status", "error", err)
			return nil, err
		}
	}
	status, err := json.Marshal(rhsmServerStatus)
	if err != nil {
		return nil, &rhsmapi.FailedServerResponseError{Message: err.Error()}
	}
	return &rhsmapi.PingOut{Status: status}, nil
}

// IsRegistered checks if the system is registered with RHSM.
func (b *RHSMBackend) IsRegistered(in *rhsmapi.IsRegisteredIn) (*rhsmapi.IsRegisteredOut, error) {
	registered, err := IsSystemRegistered()
	if err != nil {
		// When it is not possible to determine registration status, then log the reason
		// and return false
		slog.Debug("Failed to determine registration status", "error", err)
		return &rhsmapi.IsRegisteredOut{Registered: false}, nil
	}
	return &rhsmapi.IsRegisteredOut{Registered: registered}, nil
}
