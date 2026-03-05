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

	"github.com/redhatinsights/rhc/varlink/internalapi"
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
	// Acquire PID lock to ensure only one instance runs
	cleanup, err := acquirePIDLock()
	if err != nil {
		slog.Error("Failed to acquire PID lock", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := run(); err != nil {
		slog.Error("rhc-server error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Create backend
	backend := NewBackend()

	// Create registry and register the internal API
	registry := govarlink.NewRegistry(&govarlink.RegistryOptions{
		Vendor:  "Red Hat",
		Product: "rhc",
		Version: Version,
		URL:     "https://github.com/redhatinsights/rhc",
	})

	// Register internal API
	handler := internalapi.Handler{Backend: backend}
	handler.Register(registry)

	// Create server
	varlinkServer := &govarlink.Server{
		Handler: registry,
	}

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

	slog.Info("rhc-server starting", "version", Version)
	slog.Info("Listening on socket", "address", listener.Addr())

	// Setup graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler for graceful shutdown on SIGINT/SIGTERM
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
			return fmt.Errorf("server error: %w", err)
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
	// Ensure the directory exists
	dirPath := filepath.Dir(pidFilePath)
	if err := os.MkdirAll(dirPath, socketDirPerms); err != nil {
		return nil, fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Open or create the PID file
	pidFile, err := os.OpenFile(pidFilePath, os.O_CREATE|os.O_RDWR, pidFilePerms)
	if err != nil {
		return nil, fmt.Errorf("failed to open PID file: %w", err)
	}

	// Try to acquire an exclusive lock (non-blocking)
	if err := syscall.Flock(int(pidFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = pidFile.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another instance of rhc-server is already running")
		}
		return nil, fmt.Errorf("failed to lock PID file: %w", err)
	}

	// release defines a local helper to unlock and close the PID file
	// during error handling or normal shutdown.
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

	// Save current process ID to the lock file
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

// trySystemdActivation attempts to get a listener from systemd socket activation.
func trySystemdActivation() (net.Listener, error) {
	listeners, err := activation.Listeners()
	if err != nil {
		return nil, fmt.Errorf("failed to get systemd listeners: %w", err)
	}

	if len(listeners) == 0 {
		slog.Debug("Unable to find systemd listeners")
		return nil, nil // No systemd socket available
	}

	// Find the first unix socket listener
	for _, listener := range listeners {
		if listener.Addr().Network() == "unix" {
			slog.Info("Using systemd socket activation", "address", listener.Addr())
			return listener, nil
		}
	}

	return nil, fmt.Errorf("no unix socket found in systemd listeners")
}

// ensureSocketDirectory creates the directory for the socket if it doesn't exist.
func ensureSocketDirectory(path string) error {
	dirPath := filepath.Dir(path)
	if err := os.MkdirAll(dirPath, socketDirPerms); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	return nil
}

// createUnixSocket creates a unix socket at the specified path.
func createUnixSocket(path string) (net.Listener, error) {
	slog.Info("Creating unix socket", "path", path)

	// Ensure the directory exists
	if err := ensureSocketDirectory(path); err != nil {
		return nil, err
	}

	// Remove existing socket file if it exists.
	// This is safe because we hold the PID lock, ensuring no other
	// rhc-server instance is running. The socket exists only because
	// a previous instance was terminated abnormally.
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create unix socket
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

// getListener tries to get a listener from systemd socket activation,
// falls back to creating a unix socket.
func getListener() (net.Listener, error) {
	// Try systemd socket activation first
	listener, err := trySystemdActivation()
	if err != nil {
		return nil, err
	}
	if listener != nil {
		return listener, nil
	}

	// Fall back to creating our own unix socket
	return createUnixSocket(socketPath)
}
