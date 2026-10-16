package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redhatinsights/rhc/varlink/collectorapi"
)

const timeFormat = "Mon 2006-01-02 15:04 MST"

// collectorListJSON is the JSON representation of a single collector entry
// in "rhc collector list --format json" output.
type collectorListJSON struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

// collectorTimersJSON is the JSON representation of a single collector
// entry in "rhc collector timers --format json" output.
type collectorTimersJSON struct {
	Id      string `json:"id"`
	LastRun *int   `json:"last_run,omitempty"`
	NextRun *int   `json:"next_run,omitempty"`
}

// collectorInfoJSON is the JSON representation of "rhc collector info
// --format json" output. It omits "id": the ID is already the CLI argument
// the caller supplied, and it is not shown in the human-readable output
// either.
type collectorInfoJSON struct {
	Name        string  `json:"name"`
	Feature     *string `json:"feature,omitempty"`
	LastRun     *int    `json:"last_run,omitempty"`
	NextRun     *int    `json:"next_run,omitempty"`
	ConfigPath  string  `json:"config_path"`
	ServiceName string  `json:"service_name"`
	TimerName   string  `json:"timer_name"`
}

// formatRelativeTime converts a duration into a human-readable relative time string.
func formatRelativeTime(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 24 {
		days := hours / 24
		if hours%24 > 0 {
			return fmt.Sprintf("%dd %dh", days, hours%24)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	seconds := int(d.Seconds())
	return fmt.Sprintf("%ds", seconds)
}

// printMachineReadable marshals data to JSON and prints it to stdout.
func printMachineReadable(data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal data to JSON", "error", err)
		return
	}
	fmt.Println(string(jsonData))
}

// PrintCollectorInfo formats CollectorInfo for output.
// For human-readable output, it prints formatted text.
// For machine-readable output, it returns JSON.
func printCollectorInfo(info *collectorapi.CollectorInfo) {
	if isOutputMachineReadable() {
		printMachineReadable(collectorInfoJSON{
			Name:        info.Name,
			Feature:     info.Feature,
			LastRun:     info.LastRun,
			NextRun:     info.NextRun,
			ConfigPath:  info.ConfigPath,
			ServiceName: info.ServiceName,
			TimerName:   info.TimerName,
		})
		return
	}

	fmt.Printf("Name:      %s\n", info.Name)
	feature := "-"
	if info.Feature != nil {
		feature = *info.Feature
	}
	fmt.Printf("Feature:   %s\n\n", feature)

	if info.LastRun != nil {
		lastRunTime := time.Unix(int64(*info.LastRun), 0)
		relativeTime := formatRelativeTime(time.Since(lastRunTime))
		fmt.Printf("Last run:  %s (%s ago)\n", lastRunTime.Format(timeFormat), relativeTime)
	} else {
		fmt.Printf("Last run:  -\n")
	}
	if info.NextRun != nil {
		nextRunTime := time.Unix(int64(*info.NextRun), 0)
		relativeTime := formatRelativeTime(time.Until(nextRunTime))
		fmt.Printf("Next run:  %s (%s)\n\n", nextRunTime.Format(timeFormat), relativeTime)
	} else {
		fmt.Printf("Next run:  -\n\n")
	}

	fmt.Printf("Config:   %s\n", info.ConfigPath)
	fmt.Printf("Service:  %s\n", info.ServiceName)
	fmt.Printf("Timer:    %s\n", info.TimerName)
}

// PrintCollectorTimers formats multiple CollectorInfo structs into a table showing timing information.
// For machine-readable output, it returns JSON array.
// For human-readable output, it prints a table.
func printCollectorTimers(infos []*collectorapi.CollectorInfo) {
	if isOutputMachineReadable() {
		collectorEntries := make([]collectorTimersJSON, 0, len(infos))
		for _, info := range infos {
			collectorEntries = append(collectorEntries, collectorTimersJSON{
				Id:      info.Id,
				LastRun: info.LastRun,
				NextRun: info.NextRun,
			})
		}
		printMachineReadable(collectorEntries)
		return
	}

	if len(infos) == 0 {
		fmt.Println("No data collectors available.")
		return
	}
	headers := []string{"ID", "LAST", "NEXT"}
	var rows [][]string
	for _, info := range infos {
		lastRun := "-"
		if info.LastRun != nil {
			lastRunTime := time.Unix(int64(*info.LastRun), 0)
			lastRun = formatRelativeTime(time.Since(lastRunTime))
		}
		nextRun := "-"
		if info.NextRun != nil {
			nextRunTime := time.Unix(int64(*info.NextRun), 0)
			nextRun = formatRelativeTime(time.Until(nextRunTime))
		}
		rows = append(rows, []string{info.Id, lastRun, nextRun})
	}

	printTable(headers, rows)
	if len(infos) != 0 {
		fmt.Println("\nHint: Run 'rhc collector info COLLECTOR' to show more details.")
	}
}
