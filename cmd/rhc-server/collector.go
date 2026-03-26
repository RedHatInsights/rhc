package main

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/redhatinsights/rhc/internal/collector"
	"github.com/redhatinsights/rhc/internal/systemd"
	"github.com/redhatinsights/rhc/varlink/collectorapi"
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
	setLastRunTime(info)
	setNextRunTime(info)

	return info, nil
}

// buildBasicCollectorInfo constructs the basic CollectorInfo fields from config.
func buildBasicCollectorInfo(id string, config collector.Config) *collectorapi.CollectorInfo {
	info := &collectorapi.CollectorInfo{
		Id:          id,
		Name:        config.Name,
		ConfigPath:  filepath.Join(collector.ConfigDir, id+".toml"),
		ServiceName: fmt.Sprintf("rhc-collector-%s.service", id),
		TimerName:   fmt.Sprintf("rhc-collector-%s.timer", id),
	}

	// Set feature field (nil for non-analytics features, shown as "-")
	if config.IsAnalyticsFeature {
		feature := "analytics"
		info.Feature = &feature
	}

	return info
}

// setLastRunTime sets the last run time from the timer cache if available.
func setLastRunTime(info *collectorapi.CollectorInfo) {
	timer, err := collector.ReadTimerCache(info.Id)
	if err == nil && !timer.LastFinished.IsZero() {
		lastRun := int(timer.LastFinished.Unix())
		info.LastRun = &lastRun
	} else if err != nil {
		slog.Debug("Failed to read timer cache", "id", info.Id, "error", err)
	}
}

// setNextRunTime sets the next run time from systemd timer if available.
func setNextRunTime(info *collectorapi.CollectorInfo) {
	timerInfo, err := systemd.GetTimerInfo(info.TimerName)
	if err == nil && timerInfo.Next > 0 {
		// Convert from microseconds to seconds
		nextRun := int(timerInfo.Next / 1_000_000)
		info.NextRun = &nextRun
	} else if err != nil {
		slog.Debug("Failed to get timer info from systemd", "timer", info.TimerName, "error", err)
	}
}
