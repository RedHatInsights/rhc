package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/redhatinsights/rhc/internal/ui"
)

var (
	logFile *os.File = nil
)

// ensureLogDirectory ensures that the log directory exists and is writable by the current user.
// If the directory doesn't exist, it is created.
// If the program is running as root, the log directory is created under /var/log/rhc.
// Otherwise, the log directory is created under ~/.local/state/rhc.
func ensureLogDirectory() (string, error) {
	isRootUser := os.Getuid() == 0

	if isRootUser {
		// This path resolves to /var/log/rhc
		logDir := filepath.Join(LogDir, LongName)
		err := os.Mkdir(logDir, 0755)
		if err != nil && !os.IsExist(err) {
			return "", err
		}
		return logDir, nil
	}

	// Get $HOME and check if it exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	_, err = os.Stat(homeDir)
	if err != nil {
		return "", err
	}

	// This path resolves to ~/.local/state/rhc
	logDir := filepath.Join(homeDir, ".local", "state", LongName)

	// Unlike Mkdir, MkdirAll will not return an error if the path already exists
	if err = os.MkdirAll(logDir, 0700); err != nil {
		return "", err
	}

	return logDir, nil
}

// ensureLogFile ensures that the log file exists and is writable by the current user.
// It creates a log file named `rhc.log` within the directory returned by ensureLogDirectory.
func ensureLogFile() (*os.File, error) {
	logDir, err := ensureLogDirectory()
	if err != nil {
		return nil, err
	}

	// Attempt to open the log file
	logFilePath := filepath.Join(logDir, LongName+".log")
	return os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
}

// configureFileLogging sets up file-based logging to the configured log file path.
// If the log file can't be opened, it falls back to io.Discard, effectively ignoring all log messages.
func configureFileLogging(logLevel slog.Leveler) {
	file, err := ensureLogFile()

	var w io.Writer
	if err != nil {
		// Discard log messages if we can't open the log file
		w = io.Discard
		ui.Printf("Unable to open log file: %v. \n\nDetailed logs will not be available.\n\n", err)
	} else {
		logFile = file
		w = logFile
	}

	// Create and set the default logger
	h := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: logLevel,
	})

	logger := slog.New(h)
	slog.SetDefault(logger)
}

// closeLogFile syncs and then closes the log file.
func closeLogFile() error {
	if logFile == nil {
		return nil
	}

	// write empty line to separate log entries between runs of the program
	_, _ = fmt.Fprintln(logFile)

	syncErr := logFile.Sync()

	// Always attempt to close the file, even if sync failed
	closeErr := logFile.Close()

	logFile = nil

	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
