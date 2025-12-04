package main

import (
	"log/slog"
	"strings"

	"github.com/coreos/go-systemd/v22/journal"
	slogjournal "github.com/systemd/slog-journal"
)

// setupJournalLogging configures slog to write to the systemd journal
// instead of stdout/stderr. This allows detailed logging while keeping
// the CLI output clean and user-friendly.
func setupJournalLogging(level slog.Level) error {
	// Create a journal handler that writes to systemd journal
	// The syslog identifier "rhc" will be used, allowing filtering with:
	// journalctl -t rhc
	handler, err := slogjournal.NewHandler(&slogjournal.Options{
		Level: level,
		ReplaceGroup: func(k string) string {
			// Convert group names to uppercase, replace hyphens with underscores,
			// and prefix with RHC_
			return strings.ReplaceAll(strings.ToUpper(k), "-", "_")
		},
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Convert attribute keys to uppercase, replace hyphens with underscores,
			// and prefix with RHC_ for application-specific namespace isolation
			a.Key = strings.ReplaceAll(strings.ToUpper(a.Key), "-", "_")
			return a
		},
	})
	if err != nil {
		return err
	}

	// Set the default logger to use the journal handler
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return nil
}

// isJournalAvailable checks if systemd journal is available on the system
func isJournalAvailable() bool {
	return journal.Enabled()
}
