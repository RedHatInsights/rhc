package systemd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// TimerInfo represents the parsed output of a single systemd timer entry from systemctl list-timers.
// It contains scheduling information and the units involved.
type TimerInfo struct {
	// Next is the next scheduled activation time in microseconds since epoch (0 if n/a)
	Next uint64 `json:"next"`
	// Left is the time remaining until next activation in microseconds
	Left uint64 `json:"left"`
	// Last is the last activation time in microseconds since epoch (0 if n/a)
	Last uint64 `json:"last"`
	// Passed is the time since last activation in microseconds
	Passed uint64 `json:"passed"`
	// Unit is the timer unit name (e.g., "rhc-collector-insights.timer")
	Unit string `json:"unit"`
	// Activates contains the service unit(s) that this timer activates.
	// In JSON output, this is always an array (even for a single unit).
	Activates []string `json:"activates"`
}

// GetTimerInfo searches for a timer matching the given unit name and returns
// the complete timer information.
//
// The timer parameter specifies the timer unit name to query.
//
// Returns a TimerInfo struct and an error if the query or parsing fails.
func GetTimerInfo(timer string) (TimerInfo, error) {
	if !isValidTimerName(timer) {
		return TimerInfo{}, fmt.Errorf("invalid timer name %q: must match pattern name[@instance].timer", timer)
	}

	rawData, err := getSystemdTimerRaw(timer)
	if err != nil {
		slog.Debug("Failed to get systemd timer", "timer", timer, "error", err)
		return TimerInfo{}, fmt.Errorf("failed to get timer %q: %w", timer, err)
	}

	timerInfo, err := parseTimer(rawData)
	if err != nil {
		slog.Debug("Failed to parse systemd timer", "timer", timer, "error", err)
		return TimerInfo{}, fmt.Errorf("failed to parse timer %q: %w", timer, err)
	}

	return timerInfo, nil
}

// getSystemdTimerRaw executes systemctl list-timers and returns the raw JSON output.
// It uses --output=json for structured output and applies a 2-second timeout.
//
// Returns the command output as a string, or an error if systemctl execution fails or times out.
func getSystemdTimerRaw(timer string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", "list-timers", timer, "--output=json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Debug("systemctl list-timers command failed", "error", err, "stderr", stderr.String())
		return "", fmt.Errorf("systemctl list-timers failed: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

// parseTimer parses the JSON output from systemctl list-timers into a TimerInfo struct.
//
// The input should be JSON from systemctl list-timers --output=json.
// systemctl returns an array of timers, so this function extracts the first element.
//
// Returns a TimerInfo struct and an error if parsing fails or no timers are found.
func parseTimer(input string) (TimerInfo, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		err := fmt.Errorf("empty input")
		slog.Debug("Failed to parse timer: empty input", "error", err)
		return TimerInfo{}, err
	}

	// systemctl always returns a JSON array, even when querying a specific timer.
	// Parse into generic map structure first, then build TimerInfo manually.
	var rawTimers []map[string]interface{}
	if err := json.Unmarshal([]byte(input), &rawTimers); err != nil {
		slog.Debug("Failed to parse JSON from systemctl list-timers", "error", err)
		return TimerInfo{}, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if len(rawTimers) == 0 {
		err := fmt.Errorf("no timers found in output")
		slog.Debug("No timers in parsed JSON", "error", err)
		return TimerInfo{}, err
	}

	// Extract first (and only) timer and build TimerInfo
	rawTimer := rawTimers[0]
	timerInfo := TimerInfo{}

	if next, ok := rawTimer["next"].(float64); ok {
		timerInfo.Next = uint64(next)
	}
	if left, ok := rawTimer["left"].(float64); ok {
		timerInfo.Left = uint64(left)
	}
	if last, ok := rawTimer["last"].(float64); ok {
		timerInfo.Last = uint64(last)
	}
	if passed, ok := rawTimer["passed"].(float64); ok {
		timerInfo.Passed = uint64(passed)
	}
	if unit, ok := rawTimer["unit"].(string); ok {
		timerInfo.Unit = unit
	}

	// activates can be either a single string or an array of strings
	if activates, ok := rawTimer["activates"]; ok {
		switch v := activates.(type) {
		case string:
			timerInfo.Activates = []string{v}
		case []interface{}:
			timerInfo.Activates = make([]string, 0, len(v))
			for _, act := range v {
				if actStr, ok := act.(string); ok {
					timerInfo.Activates = append(timerInfo.Activates, actStr)
				}
			}
		}
	}

	return timerInfo, nil
}

// isValidTimerName validates whether a given string matches the systemd timer naming pattern.
// Returns true if the name is valid, false otherwise.
func isValidTimerName(name string) bool {
	timerNameRegex := regexp.MustCompile(`^[A-Za-z0-9:_.\\\-]+(@[A-Za-z0-9:_.\\\-]*)?\.timer$`)
	return timerNameRegex.MatchString(name)
}
