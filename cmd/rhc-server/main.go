package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jirihnidek/rhsm2"
	"github.com/redhatinsights/rhc/internal/util"
	"github.com/redhatinsights/rhc/pkg/exitcode"
	"github.com/redhatinsights/rhc/pkg/version"
	"github.com/varlink/go/varlink"

	collectorapi "github.com/redhatinsights/rhc/varlink/collector"
	rhsmapi "github.com/redhatinsights/rhc/varlink/rhsm/testing"
	contentapi "github.com/redhatinsights/rhc/varlink/rhsm/testing/content"
	overrideapi "github.com/redhatinsights/rhc/varlink/rhsm/testing/content/override"
	releaseapi "github.com/redhatinsights/rhc/varlink/rhsm/testing/content/release"
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
	// Set up a context for the varlink service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service, err := varlink.NewService(
		"Red Hat",
		"rhc",
		version.Version,
		"https://github.com/redhatinsights/rhc",
	)
	if err != nil {
		return fmt.Errorf("failed to create varlink service: %w", err)
	}

	// Try to initialize the rhsm client. It will create the singleton instance and further
	// calls of rhsm2.GetRHSMClient will return the same instance.
	appName := AppName
	_, err = rhsm2.GetRHSMClient(&appName, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize rhsm client: %w", err)
	}

	if err := service.RegisterInterface(emptyParametersCompat{collectorapi.VarlinkNew(NewCollectorBackend())}); err != nil {
		return fmt.Errorf("failed to register collector interface: %w", err)
	}
	if err := service.RegisterInterface(emptyParametersCompat{rhsmapi.VarlinkNew(NewRHSMBackend())}); err != nil {
		return fmt.Errorf("failed to register rhsm interface: %w", err)
	}
	if err := service.RegisterInterface(emptyParametersCompat{overrideapi.VarlinkNew(NewContentOverrideBackend())}); err != nil {
		return fmt.Errorf("failed to register content override interface: %w", err)
	}
	if err := service.RegisterInterface(emptyParametersCompat{releaseapi.VarlinkNew(NewComRedhatRhsmContentReleaseRHSMBackend())}); err != nil {
		return fmt.Errorf("failed to register release interface: %w", err)
	}
	if err := service.RegisterInterface(emptyParametersCompat{contentapi.VarlinkNew(NewComRedhatRhsmContentBackend())}); err != nil {
		return fmt.Errorf("failed to register content interface: %w", err)
	}

	// Bind the service to the socket. When the server is started by systemd socket
	// activation, varlink/go picks up the activated listener (the one named "varlink")
	// instead of creating a new socket.
	if err := ensureSocketDirectory(socketPath); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	if err := service.Bind(ctx, "unix:"+socketPath); err != nil {
		return fmt.Errorf("failed to bind varlink service: %w", err)
	}

	// When systemd socket activation is not used, then set the permissions of
	// the newly created socket file.
	listener, err := service.GetListener()
	if err != nil || listener == nil {
		return fmt.Errorf("failed to get listener: %w", err)
	}
	slog.Info("rhc-server starting", "version", version.Version)
	slog.Info("Listening on socket", "address", listener.Addr())
	if _, err := os.Stat(socketPath); err == nil {
		if err := os.Chmod(socketPath, socketPerms); err != nil {
			return fmt.Errorf("failed to set socket permissions: %w", err)
		}
	}

	// Set up a signal handler for graceful shutdown on SIGINT/SIGTERM
	sigChan := make(chan os.Signal, signalChanBuffer)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the server in a goroutine so we can handle signals concurrently
	errChan := make(chan error, errorChanBuffer)
	go func() {
		errChan <- service.DoListen(ctx, 0)
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
		if err := service.Shutdown(); err != nil {
			slog.Error("Error shutting down varlink service", "error", err)
		}
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

// ensureSocketDirectory creates the directory for the socket if it doesn't exist.
func ensureSocketDirectory(path string) error {
	dirPath := filepath.Dir(path)
	if err := os.MkdirAll(dirPath, socketDirPerms); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	return nil
}
