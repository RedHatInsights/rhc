package main

import (
	"log/slog"
	"path/filepath"

	"github.com/redhatinsights/rhc/internal/collector"
	"github.com/redhatinsights/rhc/internal/systemd"
	collectorapi "github.com/redhatinsights/rhc/varlink/collector"
)

// buildCollectorInfo constructs a CollectorInfo struct for the given collector ID.
// Returns the collector information or an error if the collector is not found or invalid.
func buildCollectorInfo(id string) (*collectorapi.CollectorInfo, error) {
	// Validate collector ID and get config
	config, err := collector.GetConfig(id)
	if err != nil {
		return nil, err
	}

	// Build basic collector info with paths and names
	info := buildBasicCollectorInfo(id, config)

	// Enrich with timing data (last run and next run)
	setLast_runTime(info)
	setNext_runTime(info)

	return info, nil
}

// buildBasicCollectorInfo constructs the basic CollectorInfo fields from config.
func buildBasicCollectorInfo(id string, config collector.Config) *collectorapi.CollectorInfo {
	info := &collectorapi.CollectorInfo{
		Id:           id,
		Name:         config.Name,
		Config_path:  filepath.Join(collector.ConfigDir, id+".toml"),
		Service_name: config.Service,
		Timer_name:   config.Timer,
	}

	// Set feature field (nil for non-analytics features, shown as "-")
	if config.IsAnalyticsFeature {
		feature := "analytics"
		info.Feature = &feature
	}

	return info
}

// setLast_runTime sets the last run time from the timer cache if available.
func setLast_runTime(info *collectorapi.CollectorInfo) {
	timer, err := collector.ReadTimerCache(info.Id)
	if err == nil && !timer.LastFinished.IsZero() {
		lastRun := int64(timer.LastFinished.Unix())
		info.Last_run = &lastRun
	} else if err != nil {
		slog.Debug("Failed to read timer cache", "id", info.Id, "error", err)
	}
}

// setNext_runTime sets the next run time from systemd timer if available.
func setNext_runTime(info *collectorapi.CollectorInfo) {
	timerInfo, err := systemd.GetTimerInfo(info.Timer_name)
	if err == nil && timerInfo.Next > 0 {
		// Convert from microseconds to seconds
		nextRun := int64(timerInfo.Next / 1_000_000)
		info.Next_run = &nextRun
	} else if err != nil {
		slog.Debug("Failed to get timer info from systemd", "timer", info.Timer_name, "error", err)
	}
}
