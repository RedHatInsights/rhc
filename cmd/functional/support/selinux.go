package support

import (
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// SELinuxAvailable reports whether the audit tools needed for AVC monitoring
// are present on this system.  When false, AVC monitoring is silently skipped.
func SELinuxAvailable() bool {
	for _, bin := range []string{"aureport", "ausearch"} {
		if _, err := exec.LookPath(bin); err != nil {
			slog.Debug("SELinux audit tool not found, AVC monitoring disabled", "binary", bin)
			return false
		}
	}
	return true
}

// AVCEntry holds a single AVC denial line as returned by `aureport --avc`.
type AVCEntry struct {
	Raw string
}

func (e AVCEntry) String() string { return e.Raw }

// AVCChecker records the time window of a test and retrieves any AVC denials
// that occurred within it.
type AVCChecker struct {
	startTime time.Time
	endTime   time.Time
	skiplist  []*regexp.Regexp
}

// NewAVCChecker creates an AVCChecker and starts the monitoring window.
func NewAVCChecker() *AVCChecker {
	return &AVCChecker{startTime: time.Now()}
}

// Stop closes the monitoring window.
func (c *AVCChecker) Stop() {
	c.endTime = time.Now()
}

// StartTime returns the time at which monitoring began.
func (c *AVCChecker) StartTime() time.Time {
	return c.startTime
}

// SkipPattern adds a regular expression; any AVC denial line matching it will
// be excluded from GetDenials results.
func (c *AVCChecker) SkipPattern(pattern string) {
	c.skiplist = append(c.skiplist, regexp.MustCompile(pattern))
}

// GetDenials returns AVC denial entries that occurred during the monitored
// window and do not match any skiplist pattern.
func (c *AVCChecker) GetDenials() ([]AVCEntry, error) {
	if !SELinuxAvailable() {
		return nil, nil
	}

	end := c.endTime
	if end.IsZero() {
		end = time.Now()
	}

	args := []string{
		"aureport", "--avc",
		"--start", c.startTime.Format("01/02/2006"), c.startTime.Format("15:04:05"),
		"--end", end.Format("01/02/2006"), end.Format("15:04:05"),
	}

	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		// aureport exits 1 when there are no events - that is not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("aureport failed: %w", err)
	}

	return c.parseAVCOutput(string(out)), nil
}

// HasUnexpectedDenials is a convenience wrapper that returns true if
// GetDenials finds any entries.
func (c *AVCChecker) HasUnexpectedDenials() (bool, error) {
	denials, err := c.GetDenials()
	return len(denials) > 0, err
}

func (c *AVCChecker) parseAVCOutput(output string) []AVCEntry {
	var entries []AVCEntry
	lines := strings.Split(output, "\n")

	// aureport --avc output has a header block before the actual entries.
	// Entries start after the second separator line ("===...===").
	separatorCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "===") {
			separatorCount++
			continue
		}
		if separatorCount < 2 || line == "" {
			continue
		}
		if line == "<no events of interest were found>" {
			break
		}

		entry := AVCEntry{Raw: line}
		if c.isSkipped(entry) {
			continue
		}
		entries = append(entries, entry)
	}

	return entries
}

func (c *AVCChecker) isSkipped(entry AVCEntry) bool {
	for _, re := range c.skiplist {
		if re.MatchString(entry.Raw) {
			return true
		}
	}
	return false
}
