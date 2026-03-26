package main

import (
	"fmt"
	"log/slog"

	"github.com/redhatinsights/rhc/internal/collector"
	"github.com/redhatinsights/rhc/varlink/collectorapi"
	"github.com/redhatinsights/rhc/varlink/internalapi"
)

var (
	// Version is set at build time.
	Version = "dev"
)

// Backend implements the internal API backend.
type Backend struct{}

// NewBackend creates a new backend instance.
func NewBackend() *Backend {
	return &Backend{}
}

// Test implements the Test method of the internal API.
// Simply echoes back the input with a prefix.
func (b *Backend) Test(in *internalapi.TestIn) (*internalapi.TestOut, error) {
	output := fmt.Sprintf("Echo from rhc-server: %s", in.Input)
	return &internalapi.TestOut{Output: output}, nil
}

// List implements the List method of the collector API.
// Returns a list of all available collectors with full details.
func (b *Backend) List(_ *collectorapi.ListIn) (*collectorapi.ListOut, error) {
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
func (b *Backend) Info(in *collectorapi.InfoIn) (*collectorapi.InfoOut, error) {
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
