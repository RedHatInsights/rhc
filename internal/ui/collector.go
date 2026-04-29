package ui

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/redhatinsights/rhc/varlink/collectorapi"
)

const timeFormat = "Mon 2006-01-02 15:04 MST"

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

// PrintTable prints data in a table format using tabwriter.
// headers are the column headers, rows contain the data for each row.
func PrintTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		fmt.Println("No data collectors available.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func(w *tabwriter.Writer) {
		err := w.Flush()
		if err != nil {
			slog.Debug("Unable to flush tabwriter", "error", err)
			return
		}
	}(w)

	for i, header := range headers {
		if i == len(headers)-1 {
			_, _ = fmt.Fprint(w, header)
		} else {
			_, _ = fmt.Fprint(w, header+"\t")
		}
	}
	_, _ = fmt.Fprintln(w)

	for _, row := range rows {
		for i, cell := range row {
			if i == len(row)-1 {
				_, _ = fmt.Fprint(w, cell)
			} else {
				_, _ = fmt.Fprint(w, cell+"\t")
			}
		}
		_, _ = fmt.Fprintln(w)
	}
}

// printMachineReadable marshals data to JSON and prints it to stdout.
func printMachineReadable(data interface{}) {
	if slice, ok := data.([]*collectorapi.CollectorInfo); ok && len(slice) == 0 {
		fmt.Println("{}")
		return
	}
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
func PrintCollectorInfo(info *collectorapi.CollectorInfo) {
	if IsOutputMachineReadable() {
		printMachineReadable(info)
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
func PrintCollectorTimers(infos []*collectorapi.CollectorInfo) {
	if IsOutputMachineReadable() {
		printMachineReadable(infos)
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

	PrintTable(headers, rows)
	if len(infos) != 0 {
		fmt.Println("\nHint: Run 'rhc collector info COLLECTOR' to show more details.")
	}
}
