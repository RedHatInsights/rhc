package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/coreos/go-systemd/v22/activation"
	govarlink "github.com/emersion/go-varlink"
	"github.com/redhatinsights/rhc/varlink/rhsmapi"

	"github.com/redhatinsights/rhc/internal/util"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/version"
	"github.com/redhatinsights/rhc/varlink/collectorapi"
)

const (
	socketPath     = "/run/rhc/com.redhat.rhc"
	pidFilePath    = "/run/rhc/rhc-server.pid"
	socketDirPerms = 0755
	socketPerms    = 0660
	pidFilePerms   = 0644

	// Channel buffer sizes for graceful shutdown
	signalChanBuffer = 1
	errorChanBuffer  = 1
)

func main() {
	// Acquire PID lock to ensure at most one instance runs at any given time
	cleanup, err := acquirePIDLock()
	if err != nil {
		slog.Error("Failed to acquire PID lock", "error", err)
		os.Exit(exitcode.Err)
	}
	defer cleanup()

	if err := run(); err != nil {
		slog.Error("rhc-server error", "error", err)
		os.Exit(exitcode.Err)
	}
}

func run() error {
	registry := govarlink.NewRegistry(&govarlink.RegistryOptions{
		Vendor:  "Red Hat",
		Product: "rhc",
		Version: version.Version,
		URL:     "https://github.com/redhatinsights/rhc",
	})

	collectorapi.Handler{Backend: NewCollectorBackend()}.Register(registry)
	rhsmapi.Handler{Backend: NewRHSMBackend()}.Register(registry)

	varlinkServer := &govarlink.Server{Handler: registry}

	// Try to get listener from systemd socket activation first
	listener, err := getListener()
	if err != nil {
		return fmt.Errorf("failed to get listener: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			slog.Error("Error closing listener", "error", err)
		}
	}()

	slog.Info("rhc-server starting", "version", version.Version)
	slog.Info("Listening on socket", "address", listener.Addr())

	// Set up a graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler for graceful shutdown on SIGINT/SIGTERM
	sigChan := make(chan os.Signal, signalChanBuffer)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the server in a goroutine so we can handle signals concurrently
	errChan := make(chan error, errorChanBuffer)
	go func() {
		errChan <- varlinkServer.Serve(listener)
	}()

	// Block until either:
	// - The server encounters an error (errChan)
	// - We receive a shutdown signal (sigChan)
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	case sig := <-sigChan:
		slog.Info("Received signal, shutting down gracefully", "signal", sig)
		cancel()
	}

	slog.Info("rhc-server stopped")
	return nil
}

// acquirePIDLock creates and locks a PID file to ensure only one instance runs.
// Returns a cleanup function that should be deferred to release the lock.
func acquirePIDLock() (func(), error) {
	// Ensure the PID directory exists
	dirPath := filepath.Dir(pidFilePath)
	if err := os.MkdirAll(dirPath, socketDirPerms); err != nil {
		return nil, fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Open or create the PID file
	pidFile, err := os.OpenFile(pidFilePath, os.O_CREATE|os.O_RDWR, pidFilePerms)
	if err != nil {
		return nil, fmt.Errorf("failed to open PID file: %w", err)
	}

	// Try to acquire an exclusive lock (LOCK_EX) and return immediately on error (LOCK_NB).
	if err = syscall.Flock(int(pidFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		defer func() { _ = pidFile.Close() }()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			errMsg := "another instance of rhc-server is already running"
			if pid := util.MustReadFile(pidFile); pid != "" {
				errMsg += fmt.Sprintf(" (pid=%s)", pid)
			}
			return nil, errors.New(errMsg)
		}
		return nil, fmt.Errorf("failed to lock PID file: %w", err)
	}

	// Create a closure to unlock and close the PID file on exit
	release := func() {
		_ = syscall.Flock(int(pidFile.Fd()), syscall.LOCK_UN)
		_ = pidFile.Close()
	}

	// Reset file content to ensure a clean state before writing the new PID
	if err := pidFile.Truncate(0); err != nil {
		release()
		return nil, fmt.Errorf("failed to truncate PID file: %w", err)
	}

	// Ensure the file cursor is at the beginning
	if _, err := pidFile.Seek(0, 0); err != nil {
		release()
		return nil, fmt.Errorf("failed to seek PID file: %w", err)
	}

	// Save the current process ID to the lock file
	pid := os.Getpid()
	if _, err := fmt.Fprintf(pidFile, "%d\n", pid); err != nil {
		release()
		return nil, fmt.Errorf("failed to write PID to file: %w", err)
	}

	// Commit pidFile content to stable storage
	if err := pidFile.Sync(); err != nil {
		release()
		return nil, fmt.Errorf("failed to sync PID file: %w", err)
	}

	slog.Info("PID lock acquired", "pid", pid, "pidFile", pidFilePath)

	// Return cleanup function
	cleanup := func() {
		release()
		if err := os.Remove(pidFilePath); err != nil {
			slog.Warn("Failed to remove PID file", "path", pidFilePath, "error", err)
		}
		slog.Info("PID lock released")
	}

	return cleanup, nil
}

// getListener attempts to get a listener via systemd socket activation and falls back
// to a unix socket if executed on its own.
func getListener() (net.Listener, error) {
	// Try systemd socket activation first
	listener, err := getSocketActivatedListener()
	if err != nil {
		return nil, err
	}
	if listener != nil {
		return listener, nil
	}

	// Fall back to creating our own unix socket
	return getUnixSocketListener(socketPath)
}

// getSocketActivatedListener attempts to get a listener via systemd socket activation.
func getSocketActivatedListener() (net.Listener, error) {
	listeners, err := activation.ListenersWithNames()
	if err != nil {
		return nil, fmt.Errorf("failed to get systemd listeners: %w", err)
	}

	if len(listeners) == 0 {
		slog.Debug("Unable to find systemd listeners")
		return nil, nil // No systemd socket available
	}

	// Look for the socket named "varlink" as per varlink spec
	if varlinkListeners, ok := listeners["varlink"]; ok && len(varlinkListeners) > 0 {
		for _, listener := range varlinkListeners {
			if listener.Addr().Network() == "unix" {
				slog.Info("Using systemd socket activation", "address", listener.Addr(), "name", "varlink")
				return listener, nil
			}
			slog.Warn("Skipping non-unix listener found in varlink group", "network", listener.Addr().Network())
			_ = listener.Close()
		}
		return nil, fmt.Errorf("no unix socket found within 'varlink' listeners")
	}

	return nil, fmt.Errorf("no socket named 'varlink' found in systemd listeners")
}

// ensureSocketDirectory creates the directory for the socket if it doesn't exist.
func ensureSocketDirectory(path string) error {
	dirPath := filepath.Dir(path)
	if err := os.MkdirAll(dirPath, socketDirPerms); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	return nil
}

// getUnixSocketListener creates a unix socket at the specified path.
func getUnixSocketListener(path string) (net.Listener, error) {
	slog.Info("Creating unix socket", "path", path)

	// Ensure the socket directory exists
	if err := ensureSocketDirectory(path); err != nil {
		return nil, err
	}

	// Remove the old socket file, if present. Since we already own the PID lock
	// through acquirePIDLock() called in main(), no other instance we would be
	// stealing a lock from is running.
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create the Varlink socket
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("failed to create unix socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(path, socketPerms); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, nil
}
